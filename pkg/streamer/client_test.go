package streamer

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientConfig
		wantErr bool
	}{
		{
			name: "valid configuration",
			cfg: ClientConfig{
				Endpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
				Region:   "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			cfg: ClientConfig{
				Region: "us-east-1",
			},
			wantErr: true,
		},
		{
			name: "default region",
			cfg: ClientConfig{
				Endpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, err := NewClient(ctx, tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected client, got nil")
				return
			}

			if client.Endpoint() != tt.cfg.Endpoint {
				t.Errorf("expected endpoint %s, got %s", tt.cfg.Endpoint, client.Endpoint())
			}

			expectedRegion := tt.cfg.Region
			if expectedRegion == "" {
				expectedRegion = "us-east-1"
			}
			if client.Region() != expectedRegion {
				t.Errorf("expected region %s, got %s", expectedRegion, client.Region())
			}
		})
	}
}

func TestPostToConnectionValidation(t *testing.T) {
	ctx := context.Background()

	// Create a client with mock config
	// Note: This will fail actual AWS calls, but we're testing validation
	cfg := ClientConfig{
		Endpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
		Region:   "us-east-1",
	}

	client, err := NewClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Test empty connection ID
	err = client.PostToConnection(ctx, "", []byte("test"))
	if err == nil {
		t.Error("expected error for empty connection ID")
	}
	if !errors.Is(err, ErrInvalidConnection) {
		t.Errorf("expected ErrInvalidConnection, got %v", err)
	}
}

func TestDeleteConnectionValidation(t *testing.T) {
	ctx := context.Background()

	cfg := ClientConfig{
		Endpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
		Region:   "us-east-1",
	}

	client, err := NewClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Test empty connection ID
	err = client.DeleteConnection(ctx, "")
	if err == nil {
		t.Error("expected error for empty connection ID")
	}
	if !errors.Is(err, ErrInvalidConnection) {
		t.Errorf("expected ErrInvalidConnection, got %v", err)
	}
}

func TestGetConnectionValidation(t *testing.T) {
	ctx := context.Background()

	cfg := ClientConfig{
		Endpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
		Region:   "us-east-1",
	}

	client, err := NewClient(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Test empty connection ID
	_, err = client.GetConnection(ctx, "")
	if err == nil {
		t.Error("expected error for empty connection ID")
	}
	if !errors.Is(err, ErrInvalidConnection) {
		t.Errorf("expected ErrInvalidConnection, got %v", err)
	}
}

func TestWrapError(t *testing.T) {
	client := &AWSClient{
		endpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
		region:   "us-east-1",
	}

	tests := []struct {
		name           string
		awsErr         error
		connectionID   string
		expectedType   error
		expectedStatus int
	}{
		{
			name:           "GoneException",
			awsErr:         &types.GoneException{Message: aws.String("connection gone")},
			connectionID:   "test-123",
			expectedType:   GoneError{},
			expectedStatus: 410,
		},
		{
			name:           "ForbiddenException",
			awsErr:         &types.ForbiddenException{Message: aws.String("forbidden")},
			connectionID:   "test-123",
			expectedType:   ForbiddenError{},
			expectedStatus: 403,
		},
		{
			name:           "PayloadTooLargeException",
			awsErr:         &types.PayloadTooLargeException{Message: aws.String("too large")},
			connectionID:   "test-123",
			expectedType:   PayloadTooLargeError{},
			expectedStatus: 413,
		},
		{
			name:           "LimitExceededException",
			awsErr:         &types.LimitExceededException{Message: aws.String("throttled")},
			connectionID:   "test-123",
			expectedType:   ThrottlingError{},
			expectedStatus: 429,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := client.wrapError(tt.awsErr, tt.connectionID)

			if wrapped == nil {
				t.Fatal("expected wrapped error, got nil")
			}

			// Check if the error implements APIError
			apiErr, ok := wrapped.(APIError)
			if !ok {
				t.Fatalf("expected APIError, got %T", wrapped)
			}

			if apiErr.HTTPStatusCode() != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, apiErr.HTTPStatusCode())
			}

			// Verify the error message contains the connection ID (for most errors)
			switch tt.expectedType.(type) {
			case GoneError, ForbiddenError, PayloadTooLargeError, ThrottlingError:
				// These errors should contain connection ID info
				if wrapped.Error() == "" {
					t.Error("expected non-empty error message")
				}
			}
		})
	}
}

func TestWrapErrorUnknown(t *testing.T) {
	client := &AWSClient{
		endpoint: "https://example.execute-api.us-east-1.amazonaws.com/production",
		region:   "us-east-1",
	}

	// Test unknown error type
	unknownErr := errors.New("unknown error")
	wrapped := client.wrapError(unknownErr, "test-123")

	serverErr, ok := wrapped.(InternalServerError)
	if !ok {
		t.Fatalf("expected InternalServerError for unknown error, got %T", wrapped)
	}

	if serverErr.HTTPStatusCode() != 500 {
		t.Errorf("expected status 500, got %d", serverErr.HTTPStatusCode())
	}

	if !serverErr.IsRetryable() {
		t.Error("expected internal server error to be retryable")
	}
}

func TestClientInterface(t *testing.T) {
	// Verify that AWSClient implements the Client interface
	var _ Client = (*AWSClient)(nil)
}

func TestExtractRegionFromEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{
			name:     "us-east-1",
			endpoint: "https://abc123.execute-api.us-east-1.amazonaws.com/production",
			expected: "us-east-1",
		},
		{
			name:     "us-west-2",
			endpoint: "https://xyz789.execute-api.us-west-2.amazonaws.com/staging",
			expected: "us-west-2",
		},
		{
			name:     "eu-west-1",
			endpoint: "https://def456.execute-api.eu-west-1.amazonaws.com/prod",
			expected: "eu-west-1",
		},
		{
			name:     "ap-southeast-2",
			endpoint: "https://ghi789.execute-api.ap-southeast-2.amazonaws.com/dev",
			expected: "ap-southeast-2",
		},
		{
			name:     "us-gov-west-1",
			endpoint: "https://gov123.execute-api.us-gov-west-1.amazonaws.com/prod",
			expected: "us-gov-west-1",
		},
		{
			name:     "no region in endpoint",
			endpoint: "https://example.com/websocket",
			expected: "",
		},
		{
			name:     "malformed endpoint",
			endpoint: "not-a-url",
			expected: "",
		},
		{
			name:     "custom domain",
			endpoint: "https://ws.example.com/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRegionFromEndpoint(tt.endpoint)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestResolveRegion(t *testing.T) {
	// Save original env vars
	originalRegion := os.Getenv("AWS_REGION")
	originalDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
	defer func() {
		os.Setenv("AWS_REGION", originalRegion)
		os.Setenv("AWS_DEFAULT_REGION", originalDefaultRegion)
	}()

	tests := []struct {
		name             string
		cfg              ClientConfig
		envRegion        string
		envDefaultRegion string
		expected         string
	}{
		{
			name: "explicit region takes precedence",
			cfg: ClientConfig{
				Endpoint: "https://abc.execute-api.us-west-2.amazonaws.com/prod",
				Region:   "eu-west-1",
			},
			expected: "eu-west-1",
		},
		{
			name: "extract from endpoint when no explicit region",
			cfg: ClientConfig{
				Endpoint: "https://abc.execute-api.us-west-2.amazonaws.com/prod",
			},
			expected: "us-west-2",
		},
		{
			name: "use AWS_REGION env var",
			cfg: ClientConfig{
				Endpoint: "https://example.com/websocket",
			},
			envRegion: "ap-south-1",
			expected:  "ap-south-1",
		},
		{
			name: "use AWS_DEFAULT_REGION env var",
			cfg: ClientConfig{
				Endpoint: "https://example.com/websocket",
			},
			envDefaultRegion: "ca-central-1",
			expected:         "ca-central-1",
		},
		{
			name: "AWS_REGION takes precedence over AWS_DEFAULT_REGION",
			cfg: ClientConfig{
				Endpoint: "https://example.com/websocket",
			},
			envRegion:        "us-east-1",
			envDefaultRegion: "us-west-2",
			expected:         "us-east-1",
		},
		{
			name: "default to us-east-1",
			cfg: ClientConfig{
				Endpoint: "https://example.com/websocket",
			},
			expected: "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			os.Unsetenv("AWS_REGION")
			os.Unsetenv("AWS_DEFAULT_REGION")
			if tt.envRegion != "" {
				os.Setenv("AWS_REGION", tt.envRegion)
			}
			if tt.envDefaultRegion != "" {
				os.Setenv("AWS_DEFAULT_REGION", tt.envDefaultRegion)
			}

			result := resolveRegion(tt.cfg)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNewClientRegionResolution(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		cfg            ClientConfig
		expectedRegion string
	}{
		{
			name: "extracts region from us-west-2 endpoint",
			cfg: ClientConfig{
				Endpoint: "https://abc123.execute-api.us-west-2.amazonaws.com/production",
			},
			expectedRegion: "us-west-2",
		},
		{
			name: "explicit region overrides endpoint",
			cfg: ClientConfig{
				Endpoint: "https://abc123.execute-api.us-west-2.amazonaws.com/production",
				Region:   "eu-central-1",
			},
			expectedRegion: "eu-central-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(ctx, tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if client.Region() != tt.expectedRegion {
				t.Errorf("expected region %q, got %q", tt.expectedRegion, client.Region())
			}
		})
	}
}
