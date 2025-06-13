# Answers to WebSocket Lambda Integration Questions - v2

Date: 2025-06-13
From: Lift Team

## 1. Lambda Handler Creation ✅

The correct way to create a Lambda handler from a Lift app is:

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // Configure your routes
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
    
    // Start the Lambda handler
    lambda.Start(app.HandleRequest)  // Pass the HandleRequest method directly
}
```

**Note:** There's no `app.Handler()` method. Use `app.HandleRequest` which has the correct signature for Lambda.

## 2. Multiple Middleware Registration ✅

You need to call `Use()` multiple times for each middleware:

```go
// Correct way to register multiple middleware
app.Use(handler.WebSocketJWTMiddleware())
app.Use(handler.MetricsMiddleware())
app.Use(handler.TracingMiddleware())

// Or chain them for clarity
app.
    Use(handler.WebSocketJWTMiddleware()).
    Use(handler.MetricsMiddleware()).
    Use(handler.TracingMiddleware())
```

The `Use()` method returns the app instance, allowing for method chaining.

## 3. WebSocket Context Mock/Test Support ✅

For testing WebSocket handlers, here's the recommended approach:

```go
package handler_test

import (
    "testing"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/lift/adapters"
)

func TestWebSocketHandler(t *testing.T) {
    // Create a WebSocket event
    wsEvent := map[string]interface{}{
        "requestContext": map[string]interface{}{
            "connectionId": "test-connection",
            "routeKey":     "$connect",
            "stage":        "test",
            "requestId":    "test-request",
            "domainName":   "test.execute-api.us-east-1.amazonaws.com",
            "apiId":        "test-api",
        },
        "queryStringParameters": map[string]interface{}{
            "Authorization": "Bearer test-token",
        },
    }

    // Create adapter and process event
    adapter := adapters.NewWebSocketAdapter()
    request, err := adapter.Adapt(wsEvent)
    if err != nil {
        t.Fatal(err)
    }

    // Create test context
    ctx := lift.NewContext(context.Background(), &lift.Request{Request: request})
    
    // Add test logger if needed
    ctx.Logger = &testLogger{}
    
    // Execute handler
    err = handleConnect(ctx)
    if err != nil {
        t.Errorf("Handler failed: %v", err)
    }

    // Check response
    if ctx.Response.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", ctx.Response.StatusCode)
    }
    
    // For WebSocket context specific tests
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        t.Fatal("Failed to get WebSocket context")
    }
    
    if wsCtx.ConnectionID() != "test-connection" {
        t.Errorf("Wrong connection ID")
    }
}

// Helper to check response body
func getResponseBody(ctx *lift.Context) (map[string]interface{}, error) {
    var body map[string]interface{}
    if err := json.Unmarshal([]byte(ctx.Response.Body), &body); err != nil {
        return nil, err
    }
    return body, nil
}
```

## 4. Context Creation for Tests ✅

The correct way to create a test context:

```go
import (
    "context"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/lift/adapters"
)

// Create a test context with custom request data
func createTestContext(method, path string, body []byte) *lift.Context {
    // Create an adapter request
    adapterReq := &adapters.Request{
        TriggerType: adapters.TriggerAPIGateway,
        Method:      method,
        Path:        path,
        Headers:     map[string]string{"Content-Type": "application/json"},
        QueryParams: map[string]string{},
        Body:        body,
    }
    
    // Wrap in lift.Request
    request := &lift.Request{Request: adapterReq}
    
    // Create context
    ctx := lift.NewContext(context.Background(), request)
    
    return ctx
}

// For WebSocket tests specifically
func createWebSocketTestContext(routeKey, connectionID string) *lift.Context {
    adapterReq := &adapters.Request{
        TriggerType: adapters.TriggerWebSocket,
        Method:      "MESSAGE",
        Path:        "/" + routeKey,
        Metadata: map[string]interface{}{
            "connectionId": connectionID,
            "routeKey":     routeKey,
        },
    }
    
    request := &lift.Request{Request: adapterReq}
    return lift.NewContext(context.Background(), request)
}
```

## 5. Shared Types ✅

For metric names and other shared types, define your own:

```go
// Define your own metric types
type MetricName string

const (
    MetricWebSocketConnect    MetricName = "websocket.connect"
    MetricWebSocketDisconnect MetricName = "websocket.disconnect"
    MetricWebSocketMessage    MetricName = "websocket.message"
    MetricWebSocketError      MetricName = "websocket.error"
)

// Use with your metrics collector
func recordMetric(ctx *lift.Context, metric MetricName, value float64) {
    if ctx.Metrics != nil {
        ctx.Metrics.Counter(string(metric)).Add(value)
    }
}
```

Don't use internal Lift types - define your own domain-specific types.

## 6. Request/Response Access in Tests ✅

Access response data after handler execution:

```go
func TestHandlerResponse(t *testing.T) {
    // Create test context
    ctx := createTestContext("POST", "/api/test", []byte(`{"test": true}`))
    
    // Execute handler
    err := myHandler(ctx)
    if err != nil {
        t.Fatalf("Handler failed: %v", err)
    }
    
    // Access response data
    
    // 1. Status Code
    if ctx.Response.StatusCode != 200 {
        t.Errorf("Expected status 200, got %d", ctx.Response.StatusCode)
    }
    
    // 2. Response Body (as string)
    body := ctx.Response.Body
    t.Logf("Response body: %s", body)
    
    // 3. Response Body (as JSON)
    var responseData map[string]interface{}
    if err := json.Unmarshal([]byte(body), &responseData); err != nil {
        t.Errorf("Failed to parse response JSON: %v", err)
    }
    
    // 4. Response Headers
    contentType := ctx.Response.Headers["Content-Type"]
    if contentType != "application/json" {
        t.Errorf("Expected JSON content type, got %s", contentType)
    }
}

// Helper function for common assertions
func assertJSONResponse(t *testing.T, ctx *lift.Context, expectedStatus int) map[string]interface{} {
    t.Helper()
    
    if ctx.Response.StatusCode != expectedStatus {
        t.Errorf("Expected status %d, got %d", expectedStatus, ctx.Response.StatusCode)
    }
    
    var body map[string]interface{}
    if err := json.Unmarshal([]byte(ctx.Response.Body), &body); err != nil {
        t.Fatalf("Failed to parse JSON response: %v", err)
    }
    
    return body
}
```

## Complete Testing Example

Here's a complete example showing all the patterns together:

```go
package websocket_test

import (
    "context"
    "encoding/json"
    "testing"
    
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/lift/adapters"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWebSocketConnect(t *testing.T) {
    // Create WebSocket connect event
    event := createWebSocketEvent("$connect", "conn123", map[string]string{
        "Authorization": "Bearer valid-token",
    })
    
    // Create app with middleware
    app := lift.New()
    app.Use(WebSocketJWTMiddleware())
    app.Use(MetricsMiddleware())
    app.Handle("CONNECT", "/connect", handleConnect)
    
    // Process event through the app
    response, err := app.HandleRequest(context.Background(), event)
    require.NoError(t, err)
    
    // Assert response
    resp, ok := response.(*lift.Response)
    require.True(t, ok)
    assert.Equal(t, 200, resp.StatusCode)
    
    // Check response body
    var body map[string]interface{}
    err = json.Unmarshal([]byte(resp.Body), &body)
    require.NoError(t, err)
    assert.Equal(t, "Connected successfully", body["message"])
}

func TestWebSocketMessage(t *testing.T) {
    // Create test context directly
    ctx := createWebSocketContext(t, "sendMessage", "conn123", map[string]interface{}{
        "action": "ping",
    })
    
    // Add mock metrics
    ctx.Metrics = &mockMetrics{}
    
    // Execute handler
    err := handleMessage(ctx)
    require.NoError(t, err)
    
    // Verify WebSocket context
    wsCtx, err := ctx.AsWebSocket()
    require.NoError(t, err)
    assert.Equal(t, "conn123", wsCtx.ConnectionID())
    assert.True(t, wsCtx.IsMessageEvent())
}

// Helper functions

func createWebSocketEvent(routeKey, connectionID string, queryParams map[string]string) map[string]interface{} {
    event := map[string]interface{}{
        "requestContext": map[string]interface{}{
            "connectionId": connectionID,
            "routeKey":     routeKey,
            "stage":        "test",
            "requestId":    "test-req-123",
            "domainName":   "test.execute-api.us-east-1.amazonaws.com",
            "apiId":        "test-api",
        },
    }
    
    if queryParams != nil {
        params := make(map[string]interface{})
        for k, v := range queryParams {
            params[k] = v
        }
        event["queryStringParameters"] = params
    }
    
    return event
}

func createWebSocketContext(t *testing.T, routeKey, connectionID string, body interface{}) *lift.Context {
    bodyBytes, err := json.Marshal(body)
    require.NoError(t, err)
    
    event := createWebSocketEvent(routeKey, connectionID, nil)
    event["body"] = string(bodyBytes)
    
    adapter := adapters.NewWebSocketAdapter()
    request, err := adapter.Adapt(event)
    require.NoError(t, err)
    
    return lift.NewContext(context.Background(), &lift.Request{Request: request})
}

// Mock implementations

type mockMetrics struct{}

func (m *mockMetrics) Counter(name string, tags ...map[string]string) lift.Counter {
    return &mockCounter{}
}

func (m *mockMetrics) Histogram(name string, tags ...map[string]string) lift.Histogram {
    return &mockHistogram{}
}

func (m *mockMetrics) Gauge(name string, tags ...map[string]string) lift.Gauge {
    return &mockGauge{}
}

func (m *mockMetrics) Flush() error {
    return nil
}

type mockCounter struct{ value float64 }
func (c *mockCounter) Inc()              { c.value++ }
func (c *mockCounter) Add(value float64) { c.value += value }

type mockHistogram struct{}
func (h *mockHistogram) Observe(value float64) {}

type mockGauge struct{ value float64 }
func (g *mockGauge) Set(value float64) { g.value = value }
func (g *mockGauge) Inc()              { g.value++ }
func (g *mockGauge) Dec()              { g.value-- }
func (g *mockGauge) Add(value float64) { g.value += value }
```

## Key Points

1. **Lambda Handler**: Use `lambda.Start(app.HandleRequest)` not `app.Handler()`
2. **Multiple Middleware**: Call `Use()` multiple times, not all at once
3. **Testing**: Create contexts using `lift.NewContext()` with proper adapter requests
4. **Response Access**: Use `ctx.Response.StatusCode`, `ctx.Response.Body`, and `ctx.Response.Headers`
5. **Type Definitions**: Define your own types rather than using internal Lift types
6. **WebSocket Testing**: Use the WebSocket adapter to create proper test events

## Migration Tips

- Replace direct Lambda handler functions with Lift handlers
- Use the adapter pattern for creating test events
- Access response data through the context's Response field
- Define your own metric and constant types
- Chain middleware calls for clarity 