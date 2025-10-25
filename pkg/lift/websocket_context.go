package lift

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
)

const (
	connectRoute    = "$connect"
	disconnectRoute = "$disconnect"
	defaultRegion   = "us-east-1"
)

// WebSocketContext provides WebSocket-specific functionality backed by the AWS SDK v2.
type WebSocketContext struct {
	*Context
	managementAPI *apigatewaymanagementapi.Client
	region        string
	apiMutex      sync.Mutex
}

// AsWebSocket converts a regular context to a WebSocket context.
func (c *Context) AsWebSocket() (*WebSocketContext, error) {
	if c.Request.TriggerType != TriggerWebSocket {
		return nil, NewLiftError("NOT_WEBSOCKET", "Context is not from a WebSocket event", 400)
	}

	return &WebSocketContext{
		Context: c,
		region:  c.getRegionFromContext(),
	}, nil
}

// WithRegion sets a specific AWS region for the WebSocket context.
func (wc *WebSocketContext) WithRegion(region string) *WebSocketContext {
	wc.region = region
	// Reset cached client so it will be recreated with the new region.
	wc.apiMutex.Lock()
	wc.managementAPI = nil
	wc.apiMutex.Unlock()
	return wc
}

// GetRegion returns the configured AWS region, falling back to context/env defaults.
func (wc *WebSocketContext) GetRegion() string {
	if wc.region != "" {
		return wc.region
	}
	return wc.getRegionFromContext()
}

// ConnectionID returns the WebSocket connection ID.
func (wc *WebSocketContext) ConnectionID() string {
	if wc.Request.Metadata == nil {
		return ""
	}
	if connID, ok := wc.Request.Metadata["connectionId"].(string); ok {
		return connID
	}
	return ""
}

// RouteKey returns the WebSocket route key ($connect, $disconnect, or custom route).
func (wc *WebSocketContext) RouteKey() string {
	if wc.Request.Metadata == nil {
		return ""
	}
	if routeKey, ok := wc.Request.Metadata["routeKey"].(string); ok {
		return routeKey
	}
	return ""
}

// EventType returns the WebSocket event type (CONNECT, DISCONNECT, MESSAGE).
func (wc *WebSocketContext) EventType() string {
	if wc.Request.Metadata == nil {
		return ""
	}
	if eventType, ok := wc.Request.Metadata["eventType"].(string); ok {
		return eventType
	}
	return ""
}

// Stage returns the API Gateway stage.
func (wc *WebSocketContext) Stage() string {
	if wc.Request.Metadata == nil {
		return ""
	}
	if stage, ok := wc.Request.Metadata["stage"].(string); ok {
		return stage
	}
	return ""
}

// DomainName returns the API Gateway domain name.
func (wc *WebSocketContext) DomainName() string {
	if wc.Request.Metadata == nil {
		return ""
	}
	if domainName, ok := wc.Request.Metadata["domainName"].(string); ok {
		return domainName
	}
	return ""
}

// ManagementEndpoint returns the WebSocket management API endpoint.
func (wc *WebSocketContext) ManagementEndpoint() string {
	if wc.Request.Metadata == nil {
		return ""
	}
	if endpoint, ok := wc.Request.Metadata["managementEndpoint"].(string); ok {
		return endpoint
	}
	return ""
}

func (wc *WebSocketContext) baseContext() context.Context {
	switch {
	case wc == nil:
		return context.Background()
	case wc.Context != nil && wc.Context.Context != nil:
		return wc.Context.Context
	case wc.Context != nil:
		return wc.Context
	default:
		return context.Background()
	}
}

// GetManagementAPI returns an initialized API Gateway Management API client.
func (wc *WebSocketContext) GetManagementAPI() (*apigatewaymanagementapi.Client, error) {
	wc.apiMutex.Lock()
	defer wc.apiMutex.Unlock()

	if wc.managementAPI != nil {
		return wc.managementAPI, nil
	}

	endpoint := wc.ManagementEndpoint()
	if endpoint == "" {
		return nil, fmt.Errorf("management endpoint not found in WebSocket context")
	}

	ctx := wc.baseContext()
	region := wc.GetRegion()
	if region == "" {
		region = defaultRegion
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	wc.managementAPI = apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return wc.managementAPI, nil
}

// SendMessage sends a message to the current WebSocket connection.
func (wc *WebSocketContext) SendMessage(data []byte) error {
	connectionID := wc.ConnectionID()
	if connectionID == "" {
		return fmt.Errorf("connection ID not found")
	}

	mgmtAPI, err := wc.GetManagementAPI()
	if err != nil {
		return fmt.Errorf("failed to get management API: %w", err)
	}

	_, err = mgmtAPI.PostToConnection(wc.baseContext(), &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	})
	if err != nil {
		var goneErr *types.GoneException
		if errors.As(err, &goneErr) {
			return fmt.Errorf("connection %s is gone: %w", connectionID, err)
		}
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendJSONMessage sends a JSON message to the current WebSocket connection.
func (wc *WebSocketContext) SendJSONMessage(data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return wc.SendMessage(jsonData)
}

// BroadcastMessage sends a message to multiple WebSocket connections.
func (wc *WebSocketContext) BroadcastMessage(connectionIDs []string, data []byte) error {
	if len(connectionIDs) == 0 {
		return nil
	}

	mgmtAPI, err := wc.GetManagementAPI()
	if err != nil {
		return fmt.Errorf("failed to get management API: %w", err)
	}

	ctx := wc.baseContext()
	var broadcastErrors []error
	var goneConnections []string

	for _, connID := range connectionIDs {
		_, err := mgmtAPI.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(connID),
			Data:         data,
		})
		if err != nil {
			var goneErr *types.GoneException
			if errors.As(err, &goneErr) {
				goneConnections = append(goneConnections, connID)
			} else {
				broadcastErrors = append(broadcastErrors, fmt.Errorf("failed to send to %s: %w", connID, err))
			}
		}
	}

	if len(goneConnections) > 0 && wc.Logger != nil {
		wc.Logger.Info("Gone connections detected", map[string]any{
			"connections": goneConnections,
			"count":       len(goneConnections),
		})
	}

	if len(broadcastErrors) > 0 {
		return fmt.Errorf("broadcast errors: %v", broadcastErrors)
	}

	return nil
}

// BroadcastJSONMessage sends a JSON message to multiple WebSocket connections.
func (wc *WebSocketContext) BroadcastJSONMessage(connectionIDs []string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return wc.BroadcastMessage(connectionIDs, jsonData)
}

// Disconnect forcefully disconnects a WebSocket connection.
func (wc *WebSocketContext) Disconnect(connectionID string) error {
	mgmtAPI, err := wc.GetManagementAPI()
	if err != nil {
		return fmt.Errorf("failed to get management API: %w", err)
	}

	_, err = mgmtAPI.DeleteConnection(wc.baseContext(), &apigatewaymanagementapi.DeleteConnectionInput{
		ConnectionId: aws.String(connectionID),
	})
	if err != nil {
		var goneErr *types.GoneException
		if errors.As(err, &goneErr) {
			return nil // Already disconnected
		}
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	return nil
}

// GetConnectionInfo retrieves information about a WebSocket connection.
func (wc *WebSocketContext) GetConnectionInfo(connectionID string) (*apigatewaymanagementapi.GetConnectionOutput, error) {
	mgmtAPI, err := wc.GetManagementAPI()
	if err != nil {
		return nil, fmt.Errorf("failed to get management API: %w", err)
	}

	output, err := mgmtAPI.GetConnection(wc.baseContext(), &apigatewaymanagementapi.GetConnectionInput{
		ConnectionId: aws.String(connectionID),
	})
	if err != nil {
		var goneErr *types.GoneException
		if errors.As(err, &goneErr) {
			return nil, fmt.Errorf("connection %s not found", connectionID)
		}
		return nil, fmt.Errorf("failed to get connection info: %w", err)
	}

	return output, nil
}

// ConnectionMetadata represents metadata about a WebSocket connection.
type ConnectionMetadata struct {
	Identity     map[string]string
	ConnectedAt  *time.Time
	LastActiveAt *time.Time
	ConnectionID string
}

// GetConnectionMetadata retrieves metadata about a connection.
func (wc *WebSocketContext) GetConnectionMetadata(connectionID string) (*ConnectionMetadata, error) {
	info, err := wc.GetConnectionInfo(connectionID)
	if err != nil {
		return nil, err
	}

	metadata := &ConnectionMetadata{
		ConnectionID: connectionID,
		ConnectedAt:  info.ConnectedAt,
		LastActiveAt: info.LastActiveAt,
	}

	if info.Identity != nil && info.Identity.SourceIp != nil {
		metadata.Identity = map[string]string{
			"sourceIp": *info.Identity.SourceIp,
		}
	}

	return metadata, nil
}

// IsConnectEvent returns true if this is a $connect event.
func (wc *WebSocketContext) IsConnectEvent() bool {
	return wc.RouteKey() == connectRoute
}

// IsDisconnectEvent returns true if this is a $disconnect event.
func (wc *WebSocketContext) IsDisconnectEvent() bool {
	return wc.RouteKey() == disconnectRoute
}

// IsMessageEvent returns true if this is a message event (not connect/disconnect).
func (wc *WebSocketContext) IsMessageEvent() bool {
	routeKey := wc.RouteKey()
	return routeKey != connectRoute && routeKey != disconnectRoute
}

// GetAuthorizationFromQuery extracts authorization token from query parameters.
func (wc *WebSocketContext) GetAuthorizationFromQuery() string {
	return wc.Query("Authorization")
}

// getRegionFromContext extracts AWS region from context, environment, or defaults.
func (c *Context) getRegionFromContext() string {
	if region := c.Get("aws_region"); region != nil {
		if regionStr, ok := region.(string); ok && regionStr != "" {
			return regionStr
		}
	}

	if c.Request != nil && c.Request.Metadata != nil {
		if region, ok := c.Request.Metadata["region"].(string); ok && region != "" {
			return region
		}

		if requestContext := c.Request.RequestContext(); len(requestContext) > 0 {
			if region, ok := requestContext["region"].(string); ok && region != "" {
				return region
			}
		}
	}

	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}

	return defaultRegion
}

// getRegionFromContext for WebSocketContext (delegate to embedded Context).
func (wc *WebSocketContext) getRegionFromContext() string {
	if wc.Context == nil {
		return defaultRegion
	}
	return wc.Context.getRegionFromContext()
}
