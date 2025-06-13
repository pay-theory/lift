# CloudWatch and API Gateway Testing with Lift

This guide covers testing patterns for Lift applications, with a focus on API Gateway WebSocket connections, AWS SDK v2 error mocking, and CloudWatch integration.

## Table of Contents
- [Testing Overview](#testing-overview)
- [WebSocket Testing](#websocket-testing)
- [AWS SDK v2 Error Mocking](#aws-sdk-v2-error-mocking)
- [Error Conversion Testing](#error-conversion-testing)
- [CloudWatch Integration Testing](#cloudwatch-integration-testing)
- [API Gateway Testing](#api-gateway-testing)
- [Best Practices](#best-practices)

## Testing Overview

Lift provides several testing utilities and patterns to help you write comprehensive tests for your Lambda handlers, especially when dealing with AWS services.

```go
import (
    "testing"
    "github.com/pay-theory/lift"
    "github.com/pay-theory/lift/testing"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)
```

## WebSocket Testing

### Mocking API Gateway Management API Client

Lift provides testing utilities for mocking the API Gateway Management API client used in WebSocket connections.

```go
// mock_websocket_client.go
package mocks

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
    "github.com/stretchr/testify/mock"
)

// MockAPIGatewayManagementClient mocks the API Gateway Management API client
type MockAPIGatewayManagementClient struct {
    mock.Mock
}

func (m *MockAPIGatewayManagementClient) PostToConnection(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
    args := m.Called(ctx, params)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*apigatewaymanagementapi.PostToConnectionOutput), args.Error(1)
}

func (m *MockAPIGatewayManagementClient) DeleteConnection(ctx context.Context, params *apigatewaymanagementapi.DeleteConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.DeleteConnectionOutput, error) {
    args := m.Called(ctx, params)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*apigatewaymanagementapi.DeleteConnectionOutput), args.Error(1)
}

func (m *MockAPIGatewayManagementClient) GetConnection(ctx context.Context, params *apigatewaymanagementapi.GetConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.GetConnectionOutput, error) {
    args := m.Called(ctx, params)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*apigatewaymanagementapi.GetConnectionOutput), args.Error(1)
}
```

### Testing WebSocket Handlers

```go
// websocket_handler_test.go
package handlers_test

import (
    "context"
    "testing"
    "github.com/pay-theory/lift"
    "github.com/pay-theory/lift/websocket"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestWebSocketMessageHandler(t *testing.T) {
    // Create mock client
    mockClient := new(MockAPIGatewayManagementClient)
    
    // Create test context with WebSocket support
    ctx := websocket.NewTestContext(context.Background(), websocket.TestConfig{
        ConnectionID: "test-connection-123",
        RouteKey:     "message",
        APIClient:    mockClient,
    })
    
    // Set up expectations
    mockClient.On("PostToConnection", mock.Anything, &apigatewaymanagementapi.PostToConnectionInput{
        ConnectionId: aws.String("test-connection-123"),
        Data:        []byte(`{"type":"echo","message":"Hello, WebSocket!"}`),
    }).Return(&apigatewaymanagementapi.PostToConnectionOutput{}, nil)
    
    // Create handler
    handler := &MessageHandler{}
    
    // Test the handler
    response, err := handler.Handle(ctx, websocket.Message{
        Action: "echo",
        Data:   map[string]interface{}{"message": "Hello, WebSocket!"},
    })
    
    assert.NoError(t, err)
    assert.Equal(t, 200, response.StatusCode)
    mockClient.AssertExpectations(t)
}
```

## AWS SDK v2 Error Mocking

### Mocking Common API Gateway Errors

```go
// error_mocks.go
package testing

import (
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
    "github.com/aws/smithy-go"
)

// CreateGoneException creates a mock GoneException
func CreateGoneException(message string) error {
    return &types.GoneException{
        Message: &message,
    }
}

// CreateForbiddenException creates a mock ForbiddenException
func CreateForbiddenException(message string) error {
    return &types.ForbiddenException{
        Message: &message,
    }
}

// CreatePayloadTooLargeException creates a mock PayloadTooLargeException
func CreatePayloadTooLargeException(message string) error {
    return &types.PayloadTooLargeException{
        Message: &message,
    }
}

// CreateAPIError creates a generic AWS API error
func CreateAPIError(code, message string) error {
    return &smithy.GenericAPIError{
        Code:    code,
        Message: message,
    }
}
```

### Testing Error Scenarios

```go
func TestWebSocketErrorHandling(t *testing.T) {
    tests := []struct {
        name          string
        setupMock     func(*MockAPIGatewayManagementClient)
        expectedCode  int
        expectedError string
    }{
        {
            name: "Gone Exception - Connection No Longer Exists",
            setupMock: func(m *MockAPIGatewayManagementClient) {
                m.On("PostToConnection", mock.Anything, mock.Anything).
                    Return(nil, CreateGoneException("Connection no longer exists"))
            },
            expectedCode:  410,
            expectedError: "Connection no longer exists",
        },
        {
            name: "Forbidden Exception - Not Authorized",
            setupMock: func(m *MockAPIGatewayManagementClient) {
                m.On("PostToConnection", mock.Anything, mock.Anything).
                    Return(nil, CreateForbiddenException("Not authorized to send to connection"))
            },
            expectedCode:  403,
            expectedError: "Not authorized to send to connection",
        },
        {
            name: "Payload Too Large",
            setupMock: func(m *MockAPIGatewayManagementClient) {
                m.On("PostToConnection", mock.Anything, mock.Anything).
                    Return(nil, CreatePayloadTooLargeException("Message exceeds maximum size"))
            },
            expectedCode:  413,
            expectedError: "Message exceeds maximum size",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient := new(MockAPIGatewayManagementClient)
            tt.setupMock(mockClient)
            
            ctx := websocket.NewTestContext(context.Background(), websocket.TestConfig{
                ConnectionID: "test-connection",
                APIClient:    mockClient,
            })
            
            handler := &MessageHandler{}
            response, err := handler.Handle(ctx, websocket.Message{
                Action: "send",
                Data:   map[string]interface{}{"message": "test"},
            })
            
            assert.Error(t, err)
            assert.Equal(t, tt.expectedCode, response.StatusCode)
            assert.Contains(t, response.Body, tt.expectedError)
            mockClient.AssertExpectations(t)
        })
    }
}
```

## Error Conversion Testing

### Testing Error Type Conversion

```go
// error_converter_test.go
package handlers_test

import (
    "testing"
    "github.com/pay-theory/lift"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
)

func TestErrorConverter(t *testing.T) {
    tests := []struct {
        name         string
        inputError   error
        expectedCode int
        expectedMsg  string
    }{
        {
            name:         "GoneException to 410",
            inputError:   &types.GoneException{Message: aws.String("Connection gone")},
            expectedCode: 410,
            expectedMsg:  "Connection gone",
        },
        {
            name:         "ForbiddenException to 403",
            inputError:   &types.ForbiddenException{Message: aws.String("Access denied")},
            expectedCode: 403,
            expectedMsg:  "Access denied",
        },
        {
            name:         "Generic error to 500",
            inputError:   errors.New("Internal error"),
            expectedCode: 500,
            expectedMsg:  "Internal server error",
        },
    }
    
    converter := NewErrorConverter()
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            httpError := converter.Convert(tt.inputError)
            assert.Equal(t, tt.expectedCode, httpError.StatusCode)
            assert.Equal(t, tt.expectedMsg, httpError.Message)
        })
    }
}
```

### Testing Error Response Formatting

```go
func TestErrorResponseFormat(t *testing.T) {
    // Create a test app with error handling middleware
    app := lift.New()
    app.Use(lift.ErrorHandler())
    
    // Add a handler that returns various errors
    app.POST("/test", func(ctx *lift.Context) error {
        errorType := ctx.Query("error_type")
        
        switch errorType {
        case "validation":
            return lift.NewValidationError("Invalid input", map[string]string{
                "field1": "required",
                "field2": "must be positive",
            })
        case "not_found":
            return lift.NewNotFoundError("Resource not found")
        case "unauthorized":
            return lift.NewUnauthorizedError("Invalid credentials")
        default:
            return errors.New("Unknown error")
        }
    })
    
    // Test validation error response
    t.Run("Validation Error Response", func(t *testing.T) {
        event := createTestAPIGatewayEvent("POST", "/test?error_type=validation")
        response, err := app.Handler(context.Background(), event)
        
        assert.NoError(t, err)
        assert.Equal(t, 400, response.StatusCode)
        
        var errorResponse map[string]interface{}
        json.Unmarshal([]byte(response.Body), &errorResponse)
        
        assert.Equal(t, "validation_error", errorResponse["error"])
        assert.Equal(t, "Invalid input", errorResponse["message"])
        assert.NotNil(t, errorResponse["details"])
    })
}
```

## CloudWatch Integration Testing

### Testing Structured Logging

```go
// cloudwatch_logging_test.go
package handlers_test

import (
    "bytes"
    "encoding/json"
    "testing"
    "github.com/pay-theory/lift"
    "github.com/pay-theory/lift/logger"
)

func TestStructuredLogging(t *testing.T) {
    // Capture log output
    var buf bytes.Buffer
    logger := logger.New(logger.Config{
        Output: &buf,
        Level:  logger.DebugLevel,
    })
    
    app := lift.New()
    app.Use(lift.Logger(logger))
    
    app.GET("/test", func(ctx *lift.Context) error {
        ctx.Logger().Info("Processing request",
            "user_id", "123",
            "action", "test",
            "tenant_id", ctx.TenantID(),
        )
        return ctx.JSON(200, map[string]string{"status": "ok"})
    })
    
    // Execute request
    event := createTestAPIGatewayEvent("GET", "/test")
    _, err := app.Handler(context.Background(), event)
    assert.NoError(t, err)
    
    // Verify log output
    var logEntry map[string]interface{}
    json.Unmarshal(buf.Bytes(), &logEntry)
    
    assert.Equal(t, "Processing request", logEntry["message"])
    assert.Equal(t, "123", logEntry["user_id"])
    assert.Equal(t, "test", logEntry["action"])
    assert.NotNil(t, logEntry["request_id"])
    assert.NotNil(t, logEntry["timestamp"])
}
```

### Testing Metrics Emission

```go
// metrics_test.go
package handlers_test

import (
    "testing"
    "github.com/pay-theory/lift"
    "github.com/pay-theory/lift/metrics"
)

type MockMetricsCollector struct {
    metrics []metrics.Metric
}

func (m *MockMetricsCollector) Record(metric metrics.Metric) {
    m.metrics = append(m.metrics, metric)
}

func TestMetricsCollection(t *testing.T) {
    collector := &MockMetricsCollector{}
    
    app := lift.New()
    app.Use(lift.Metrics(collector))
    
    app.POST("/api/process", func(ctx *lift.Context) error {
        // Record custom metric
        ctx.Metric("items_processed", 10, metrics.Tags{
            "tenant_id": ctx.TenantID(),
            "status":    "success",
        })
        
        return ctx.JSON(200, map[string]string{"status": "processed"})
    })
    
    // Execute request
    event := createTestAPIGatewayEvent("POST", "/api/process")
    _, err := app.Handler(context.Background(), event)
    assert.NoError(t, err)
    
    // Verify metrics
    assert.Len(t, collector.metrics, 2) // Request metric + custom metric
    
    // Check request metric
    requestMetric := collector.metrics[0]
    assert.Equal(t, "http_request_duration", requestMetric.Name)
    assert.Equal(t, "POST", requestMetric.Tags["method"])
    assert.Equal(t, "/api/process", requestMetric.Tags["path"])
    assert.Equal(t, "200", requestMetric.Tags["status"])
    
    // Check custom metric
    customMetric := collector.metrics[1]
    assert.Equal(t, "items_processed", customMetric.Name)
    assert.Equal(t, float64(10), customMetric.Value)
    assert.Equal(t, "success", customMetric.Tags["status"])
}
```

## API Gateway Testing

### Testing API Gateway Event Sources

```go
// apigateway_event_test.go
package handlers_test

import (
    "testing"
    "github.com/pay-theory/lift"
    "github.com/pay-theory/lift/testing/events"
    "github.com/aws/aws-lambda-go/events"
)

func TestAPIGatewayV2HTTPEvent(t *testing.T) {
    app := lift.New()
    
    app.GET("/users/:id", func(ctx *lift.Context) error {
        userID := ctx.Param("id")
        auth := ctx.Header("Authorization")
        
        return ctx.JSON(200, map[string]interface{}{
            "user_id": userID,
            "auth":    auth,
            "query":   ctx.QueryParams(),
        })
    })
    
    // Create test event with helper
    event := events.CreateAPIGatewayV2HTTPEvent(events.V2EventConfig{
        Method: "GET",
        Path:   "/users/123",
        Headers: map[string]string{
            "Authorization": "Bearer test-token",
        },
        QueryParams: map[string]string{
            "include": "profile",
            "expand":  "teams",
        },
        PathParameters: map[string]string{
            "id": "123",
        },
    })
    
    response, err := app.Handler(context.Background(), event)
    assert.NoError(t, err)
    assert.Equal(t, 200, response.StatusCode)
    
    var body map[string]interface{}
    json.Unmarshal([]byte(response.Body), &body)
    
    assert.Equal(t, "123", body["user_id"])
    assert.Equal(t, "Bearer test-token", body["auth"])
    assert.Equal(t, "profile", body["query"].(map[string]interface{})["include"])
}
```

### Testing WebSocket Events

```go
func TestWebSocketEvents(t *testing.T) {
    app := websocket.NewApp()
    
    // Mock connection store
    store := websocket.NewMockConnectionStore()
    app.SetConnectionStore(store)
    
    t.Run("Connect Event", func(t *testing.T) {
        event := events.CreateWebSocketConnectEvent(events.WebSocketConfig{
            ConnectionID: "conn-123",
            Headers: map[string]string{
                "Authorization": "Bearer token",
            },
        })
        
        response, err := app.Handler(context.Background(), event)
        assert.NoError(t, err)
        assert.Equal(t, 200, response.StatusCode)
        
        // Verify connection was stored
        conn, exists := store.Get("conn-123")
        assert.True(t, exists)
        assert.NotNil(t, conn)
    })
    
    t.Run("Message Event", func(t *testing.T) {
        event := events.CreateWebSocketMessageEvent(events.WebSocketMessageConfig{
            ConnectionID: "conn-123",
            RouteKey:    "message",
            Body:        `{"action":"echo","data":"Hello"}`,
        })
        
        response, err := app.Handler(context.Background(), event)
        assert.NoError(t, err)
        assert.Equal(t, 200, response.StatusCode)
    })
    
    t.Run("Disconnect Event", func(t *testing.T) {
        event := events.CreateWebSocketDisconnectEvent(events.WebSocketConfig{
            ConnectionID: "conn-123",
        })
        
        response, err := app.Handler(context.Background(), event)
        assert.NoError(t, err)
        assert.Equal(t, 200, response.StatusCode)
        
        // Verify connection was removed
        _, exists := store.Get("conn-123")
        assert.False(t, exists)
    })
}
```

## Best Practices

### 1. Use Table-Driven Tests

```go
func TestWebSocketMessageHandling(t *testing.T) {
    tests := []struct {
        name           string
        message        websocket.Message
        mockSetup      func(*MockAPIGatewayManagementClient)
        expectedStatus int
        expectedError  bool
    }{
        {
            name: "Successful echo",
            message: websocket.Message{
                Action: "echo",
                Data:   map[string]interface{}{"text": "Hello"},
            },
            mockSetup: func(m *MockAPIGatewayManagementClient) {
                m.On("PostToConnection", mock.Anything, mock.Anything).
                    Return(&apigatewaymanagementapi.PostToConnectionOutput{}, nil)
            },
            expectedStatus: 200,
            expectedError:  false,
        },
        {
            name: "Connection gone",
            message: websocket.Message{
                Action: "echo",
                Data:   map[string]interface{}{"text": "Hello"},
            },
            mockSetup: func(m *MockAPIGatewayManagementClient) {
                m.On("PostToConnection", mock.Anything, mock.Anything).
                    Return(nil, &types.GoneException{Message: aws.String("Gone")})
            },
            expectedStatus: 410,
            expectedError:  true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 2. Create Test Helpers

```go
// test_helpers.go
package testing

// WebSocketTestSuite provides common setup for WebSocket tests
type WebSocketTestSuite struct {
    App        *websocket.App
    MockClient *MockAPIGatewayManagementClient
    Store      *websocket.MockConnectionStore
}

func NewWebSocketTestSuite() *WebSocketTestSuite {
    mockClient := new(MockAPIGatewayManagementClient)
    store := websocket.NewMockConnectionStore()
    
    app := websocket.NewApp()
    app.SetConnectionStore(store)
    app.SetAPIClient(mockClient)
    
    return &WebSocketTestSuite{
        App:        app,
        MockClient: mockClient,
        Store:      store,
    }
}

// Helper method to create and store a test connection
func (s *WebSocketTestSuite) CreateConnection(id string, metadata map[string]interface{}) {
    s.Store.Set(id, &websocket.Connection{
        ID:       id,
        Metadata: metadata,
    })
}
```

### 3. Test Error Boundaries

```go
func TestErrorBoundaries(t *testing.T) {
    // Test maximum payload size
    t.Run("Payload Too Large", func(t *testing.T) {
        largeData := make([]byte, 32*1024) // 32KB
        
        mockClient := new(MockAPIGatewayManagementClient)
        mockClient.On("PostToConnection", mock.Anything, mock.MatchedBy(func(input *apigatewaymanagementapi.PostToConnectionInput) bool {
            return len(input.Data) > 32*1024
        })).Return(nil, &types.PayloadTooLargeException{
            Message: aws.String("Payload exceeds maximum size"),
        })
        
        // Test handling
    })
    
    // Test connection limits
    t.Run("Connection Limit Exceeded", func(t *testing.T) {
        store := websocket.NewMockConnectionStore()
        store.SetMaxConnections(100)
        
        // Add 100 connections
        for i := 0; i < 100; i++ {
            store.Set(fmt.Sprintf("conn-%d", i), &websocket.Connection{})
        }
        
        // Try to add one more
        err := store.Set("conn-101", &websocket.Connection{})
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "connection limit exceeded")
    })
}
```

### 4. Integration Test Pattern

```go
// integration_test.go
//go:build integration

package handlers_test

import (
    "testing"
    "os"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

func TestRealAPIGatewayIntegration(t *testing.T) {
    if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
        t.Skip("Skipping integration test")
    }
    
    // Load real AWS config
    cfg, err := config.LoadDefaultConfig(context.Background())
    assert.NoError(t, err)
    
    // Create real client
    client := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
        o.EndpointResolver = apigatewaymanagementapi.EndpointResolverFunc(
            func(region string, options apigatewaymanagementapi.EndpointResolverOptions) (aws.Endpoint, error) {
                return aws.Endpoint{
                    URL: os.Getenv("WEBSOCKET_API_ENDPOINT"),
                }, nil
            },
        )
    })
    
    // Run integration tests
}
```

### 5. Performance Testing

```go
func BenchmarkWebSocketMessageHandling(b *testing.B) {
    suite := NewWebSocketTestSuite()
    
    // Setup mock to always succeed
    suite.MockClient.On("PostToConnection", mock.Anything, mock.Anything).
        Return(&apigatewaymanagementapi.PostToConnectionOutput{}, nil)
    
    // Create test context
    ctx := websocket.NewTestContext(context.Background(), websocket.TestConfig{
        ConnectionID: "bench-conn",
        APIClient:    suite.MockClient,
    })
    
    handler := &MessageHandler{}
    message := websocket.Message{
        Action: "echo",
        Data:   map[string]interface{}{"text": "benchmark"},
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        _, _ = handler.Handle(ctx, message)
    }
}
```

## Summary

This guide provides comprehensive patterns for testing Lift applications with CloudWatch and API Gateway, including:

- **WebSocket Testing**: Mock clients, event simulation, and connection management
- **Error Mocking**: AWS SDK v2 error types and conversion testing
- **CloudWatch Integration**: Structured logging and metrics collection
- **API Gateway Events**: HTTP and WebSocket event testing
- **Best Practices**: Table-driven tests, test helpers, error boundaries, and performance testing

These patterns help ensure your Lift applications are thoroughly tested and production-ready. 