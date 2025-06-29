package lift

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/lift/pkg/lift/adapters"
)

// WebSocketHandler is a function that handles WebSocket events
type WebSocketHandler func(ctx *Context) error

// WebSocketRoute stores WebSocket route information
type webSocketRoute struct {
	routeKey string
	handler  WebSocketHandler
}

// WebSocketOptions configures WebSocket support
type WebSocketOptions struct {
	// EnableAutoConnectionManagement automatically handles connection tracking
	EnableAutoConnectionManagement bool

	// ConnectionStore is used for automatic connection management
	ConnectionStore ConnectionStore

	// DefaultHandler is called when no specific route matches
	DefaultHandler WebSocketHandler
}

// WebSocket registers a WebSocket route handler
func (a *App) WebSocket(routeKey string, handler WebSocketHandler) *App {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.wsRoutes == nil {
		a.wsRoutes = make(map[string]WebSocketHandler)
	}

	a.wsRoutes[routeKey] = handler
	return a
}

// RouteWebSocket finds a handler for the given route key
func (a *App) RouteWebSocket(routeKey string) Handler {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if handler, ok := a.wsRoutes[routeKey]; ok {
		return HandlerFunc(func(ctx *Context) error {
			return handler(ctx)
		})
	}

	// Check for default handler
	if a.wsOptions != nil && a.wsOptions.DefaultHandler != nil {
		return HandlerFunc(func(ctx *Context) error {
			return a.wsOptions.DefaultHandler(ctx)
		})
	}

	// Fall back to $default route if exists
	if handler, ok := a.wsRoutes["$default"]; ok {
		return HandlerFunc(func(ctx *Context) error {
			return handler(ctx)
		})
	}

	return nil
}

// WithWebSocketSupport enables WebSocket support in the app
func WithWebSocketSupport(options ...WebSocketOptions) AppOption {
	return func(a *App) {
		a.wsRoutes = make(map[string]WebSocketHandler)
		a.features["websocket"] = true

		if len(options) > 0 {
			a.wsOptions = &options[0]
		}
	}
}

// WebSocketHandler returns a Lambda handler for WebSocket events
func (a *App) WebSocketHandler() any {
	return func(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
		// Convert to generic event format for adapter
		genericEvent := convertWebSocketEventToGeneric(event)

		// Use existing adapter infrastructure
		req, err := a.parseEvent(genericEvent)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       fmt.Sprintf(`{"error": "Failed to parse event: %v"}`, err),
			}, nil
		}

		// Create Lift context
		liftCtx := NewContext(ctx, req)

		// Set dependencies
		if a.logger != nil {
			liftCtx.Logger = a.logger
		}
		if a.metrics != nil {
			liftCtx.Metrics = a.metrics
		}
		if a.db != nil {
			liftCtx.DB = a.db
		}

		// For WebSocket events, route based on route key instead of HTTP method/path
		if req.TriggerType == adapters.TriggerWebSocket {
			routeKey := ""
			if metadata, ok := req.Metadata["routeKey"].(string); ok {
				routeKey = metadata
			}

			handler := a.RouteWebSocket(routeKey)
			if handler == nil {
				return events.APIGatewayProxyResponse{
					StatusCode: 404,
					Body:       fmt.Sprintf(`{"error": "No handler for route: %s"}`, routeKey),
				}, nil
			}

			// Apply middleware and execute handler
			finalHandler := handler
			for i := len(a.middleware) - 1; i >= 0; i-- {
				finalHandler = a.middleware[i](finalHandler)
			}

			// Execute with automatic connection management if enabled
			if a.wsOptions != nil && a.wsOptions.EnableAutoConnectionManagement {
				finalHandler = wrapWithConnectionManagement(finalHandler, a.wsOptions.ConnectionStore)
			}

			if err := finalHandler.Handle(liftCtx); err != nil {
				// Check if it's a LiftError with status code
				if liftErr, ok := err.(*LiftError); ok {
					return events.APIGatewayProxyResponse{
						StatusCode: liftErr.StatusCode,
						Body:       fmt.Sprintf(`{"error": "%s"}`, liftErr.Message),
					}, nil
				}

				return events.APIGatewayProxyResponse{
					StatusCode: 500,
					Body:       fmt.Sprintf(`{"error": "%v"}`, err),
				}, nil
			}

			// Return success response
			return events.APIGatewayProxyResponse{
				StatusCode: 200,
				Body:       "OK",
			}, nil
		}

		// Non-WebSocket event, use regular routing
		if err := a.router.Handle(liftCtx); err != nil {
			resp, _ := a.handleError(liftCtx, err)
			if apiResp, ok := resp.(events.APIGatewayProxyResponse); ok {
				return apiResp, nil
			}
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       `{"error": "Internal server error"}`,
			}, nil
		}

		// Convert response to API Gateway format
		if liftCtx.Response != nil && liftCtx.Response.StatusCode > 0 {
			body := ""
			switch v := liftCtx.Response.Body.(type) {
			case string:
				body = v
			case []byte:
				body = string(v)
			default:
				// Try to marshal as JSON
				if data, err := json.Marshal(v); err == nil {
					body = string(data)
				}
			}

			return events.APIGatewayProxyResponse{
				StatusCode: liftCtx.Response.StatusCode,
				Body:       body,
				Headers:    liftCtx.Response.Headers,
			}, nil
		}

		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       "OK",
		}, nil
	}
}

// convertWebSocketEventToGeneric converts the strongly-typed WebSocket event to generic map format
func convertWebSocketEventToGeneric(event events.APIGatewayWebsocketProxyRequest) map[string]any {
	return map[string]any{
		"requestContext": map[string]any{
			"routeKey":          event.RequestContext.RouteKey,
			"messageId":         event.RequestContext.MessageID,
			"eventType":         event.RequestContext.EventType,
			"extendedRequestId": event.RequestContext.ExtendedRequestID,
			"requestTime":       event.RequestContext.RequestTime,
			"messageDirection":  event.RequestContext.MessageDirection,
			"stage":             event.RequestContext.Stage,
			"connectedAt":       event.RequestContext.ConnectedAt,
			"requestTimeEpoch":  event.RequestContext.RequestTimeEpoch,
			"requestId":         event.RequestContext.RequestID,
			"domainName":        event.RequestContext.DomainName,
			"connectionId":      event.RequestContext.ConnectionID,
			"apiId":             event.RequestContext.APIID,
		},
		"body":                            event.Body,
		"isBase64Encoded":                 event.IsBase64Encoded,
		"stageVariables":                  event.StageVariables,
		"headers":                         event.Headers,
		"multiValueHeaders":               event.MultiValueHeaders,
		"queryStringParameters":           event.QueryStringParameters,
		"multiValueQueryStringParameters": event.MultiValueQueryStringParameters,
	}
}

// wrapWithConnectionManagement adds automatic connection tracking
func wrapWithConnectionManagement(handler Handler, store ConnectionStore) Handler {
	return HandlerFunc(func(ctx *Context) error {
		wsCtx, err := ctx.AsWebSocket()
		if err != nil {
			// Not a WebSocket context, pass through
			return handler.Handle(ctx)
		}

		// Handle connection lifecycle
		switch wsCtx.RouteKey() {
		case "$connect":
			// Let handler process first
			if err := handler.Handle(ctx); err != nil {
				return err
			}

			// Auto-save connection if handler succeeded
			if store != nil {
				conn := &Connection{
					ID:        wsCtx.ConnectionID(),
					UserID:    ctx.UserID(),
					CreatedAt: ctx.Request.Timestamp,
				}

				// Extract additional metadata
				if tenantID, ok := ctx.Get("tenant_id").(string); ok {
					conn.TenantID = tenantID
				}

				return store.Save(ctx.Context, conn)
			}

		case "$disconnect":
			// Auto-remove connection before handler
			if store != nil {
				_ = store.Delete(ctx.Context, wsCtx.ConnectionID())
			}

			// Then let handler process
			return handler.Handle(ctx)

		default:
			// Regular message, just pass through
			return handler.Handle(ctx)
		}

		return nil
	})
}

// ConnectionStore interface for automatic connection management
type ConnectionStore interface {
	Save(ctx context.Context, conn *Connection) error
	Get(ctx context.Context, connectionID string) (*Connection, error)
	Delete(ctx context.Context, connectionID string) error
	ListByUser(ctx context.Context, userID string) ([]*Connection, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*Connection, error)
	CountActive(ctx context.Context) (int64, error) // Count total active connections
}

// Connection represents a WebSocket connection
type Connection struct {
	ID        string
	UserID    string
	TenantID  string
	CreatedAt string
	Metadata  map[string]any
}
