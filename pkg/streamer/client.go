package streamer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
)

// Client defines the interface for WebSocket connection management.
type Client interface {
	// PostToConnection sends data to a specific WebSocket connection.
	PostToConnection(ctx context.Context, connectionID string, data []byte) error

	// DeleteConnection forcefully disconnects a WebSocket connection.
	DeleteConnection(ctx context.Context, connectionID string) error

	// GetConnection retrieves information about a WebSocket connection.
	GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error)
}

// AWSClient implements the Client interface using AWS SDK v2.
type AWSClient struct {
	api      *apigatewaymanagementapi.Client
	endpoint string
	region   string
}

// ClientConfig holds configuration for creating a new client.
type ClientConfig struct {
	AWSConfig *aws.Config
	Endpoint  string
	Region    string
}

// NewClient creates a new WebSocket client with the given configuration.
func NewClient(ctx context.Context, cfg ClientConfig) (*AWSClient, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	// Resolve region from multiple sources
	region := resolveRegion(cfg)

	var awsConfig aws.Config
	var err error

	if cfg.AWSConfig != nil {
		awsConfig = *cfg.AWSConfig
		// If AWS config has a region, use it
		if awsConfig.Region != "" {
			region = awsConfig.Region
		}
	} else {
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
	}

	api := apigatewaymanagementapi.NewFromConfig(awsConfig, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
	})

	return &AWSClient{
		api:      api,
		endpoint: cfg.Endpoint,
		region:   region,
	}, nil
}

// PostToConnection sends data to a specific WebSocket connection.
func (c *AWSClient) PostToConnection(ctx context.Context, connectionID string, data []byte) error {
	if connectionID == "" {
		return fmt.Errorf("%w: connection ID cannot be empty", ErrInvalidConnection)
	}

	_, err := c.api.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	})

	if err != nil {
		return c.wrapError(err, connectionID)
	}

	return nil
}

// DeleteConnection forcefully disconnects a WebSocket connection.
func (c *AWSClient) DeleteConnection(ctx context.Context, connectionID string) error {
	if connectionID == "" {
		return fmt.Errorf("%w: connection ID cannot be empty", ErrInvalidConnection)
	}

	_, err := c.api.DeleteConnection(ctx, &apigatewaymanagementapi.DeleteConnectionInput{
		ConnectionId: aws.String(connectionID),
	})

	if err != nil {
		// GoneException is acceptable for delete - connection is already gone
		var goneErr *types.GoneException
		if errors.As(err, &goneErr) {
			return nil
		}
		return c.wrapError(err, connectionID)
	}

	return nil
}

// GetConnection retrieves information about a WebSocket connection.
func (c *AWSClient) GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error) {
	if connectionID == "" {
		return nil, fmt.Errorf("%w: connection ID cannot be empty", ErrInvalidConnection)
	}

	output, err := c.api.GetConnection(ctx, &apigatewaymanagementapi.GetConnectionInput{
		ConnectionId: aws.String(connectionID),
	})

	if err != nil {
		return nil, c.wrapError(err, connectionID)
	}

	info := &ConnectionInfo{
		ConnectionID: connectionID,
	}

	if output.ConnectedAt != nil {
		info.ConnectedAt = *output.ConnectedAt
	}

	if output.LastActiveAt != nil {
		info.LastActiveAt = *output.LastActiveAt
	}

	if output.Identity != nil {
		info.Identity = make(map[string]any)
		if output.Identity.SourceIp != nil {
			info.SourceIP = *output.Identity.SourceIp
			info.Identity["sourceIp"] = *output.Identity.SourceIp
		}
		if output.Identity.UserAgent != nil {
			info.UserAgent = *output.Identity.UserAgent
			info.Identity["userAgent"] = *output.Identity.UserAgent
		}
	}

	return info, nil
}

// wrapError converts AWS SDK errors to streamer error types.
func (c *AWSClient) wrapError(err error, connectionID string) error {
	var goneErr *types.GoneException
	if errors.As(err, &goneErr) {
		return GoneError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("connection %s no longer exists", connectionID),
		}
	}

	var forbiddenErr *types.ForbiddenException
	if errors.As(err, &forbiddenErr) {
		return ForbiddenError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("operation forbidden for connection %s", connectionID),
		}
	}

	var payloadErr *types.PayloadTooLargeException
	if errors.As(err, &payloadErr) {
		return PayloadTooLargeError{
			ConnectionID: connectionID,
			Message:      "payload exceeds maximum size",
		}
	}

	var limitErr *types.LimitExceededException
	if errors.As(err, &limitErr) {
		return ThrottlingError{
			ConnectionID: connectionID,
			Message:      "rate limit exceeded",
		}
	}

	// Default to internal server error for unknown errors
	return InternalServerError{
		Message: fmt.Sprintf("API error: %v", err),
	}
}

// Endpoint returns the API Gateway endpoint.
func (c *AWSClient) Endpoint() string {
	return c.endpoint
}

// Region returns the AWS region.
func (c *AWSClient) Region() string {
	return c.region
}

// resolveRegion determines the AWS region from multiple sources in priority order:
// 1. Explicit cfg.Region
// 2. Region extracted from endpoint URL
// 3. AWS_REGION environment variable
// 4. Default to us-east-1
func resolveRegion(cfg ClientConfig) string {
	// 1. Explicit region in config
	if cfg.Region != "" {
		return cfg.Region
	}

	// 2. Try to extract from endpoint URL
	if region := extractRegionFromEndpoint(cfg.Endpoint); region != "" {
		return region
	}

	// 3. Check environment variables
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}

	// 4. Default fallback
	return "us-east-1"
}

// extractRegionFromEndpoint attempts to extract the AWS region from an API Gateway endpoint URL.
// Example: https://abc123.execute-api.us-west-2.amazonaws.com/production -> us-west-2
func extractRegionFromEndpoint(endpoint string) string {
	// Look for pattern: .execute-api.<region>.amazonaws.com
	// This regex matches: execute-api.{region}.amazonaws.com
	start := strings.Index(endpoint, ".execute-api.")
	if start == -1 {
		return ""
	}

	start += len(".execute-api.")
	end := strings.Index(endpoint[start:], ".")
	if end == -1 {
		return ""
	}

	region := endpoint[start : start+end]

	// Validate it looks like a region (e.g., us-east-1, eu-west-1)
	if len(region) > 3 && (strings.Contains(region, "-") || strings.Contains(region, "gov")) {
		return region
	}

	return ""
}

// Ensure AWSClient implements Client interface
var _ Client = (*AWSClient)(nil)
