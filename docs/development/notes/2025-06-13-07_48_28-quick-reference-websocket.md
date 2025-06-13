# Quick Reference: WebSocket Lambda Integration with Lift

## Starting Lambda Handler

```go
import "github.com/aws/aws-lambda-go/lambda"

func main() {
    app := lift.New()
    
    // Configure routes
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
    app.Handle("MESSAGE", "/message", handleMessage)
    
    // Start Lambda - use HandleRequest, NOT Handler()
    lambda.Start(app.HandleRequest)
}
```

## Registering Multiple Middleware

```go
// ❌ WRONG - Can't pass multiple arguments
app.Use(middleware1, middleware2, middleware3)

// ✅ CORRECT - Chain calls
app.Use(middleware1)
app.Use(middleware2)
app.Use(middleware3)

// ✅ ALSO CORRECT - Method chaining
app.
    Use(middleware1).
    Use(middleware2).
    Use(middleware3)
```

## Testing WebSocket Handlers

```go
// Create WebSocket test event
event := map[string]interface{}{
    "requestContext": map[string]interface{}{
        "connectionId": "test-conn-123",
        "routeKey":     "$connect",
        "stage":        "test",
        "domainName":   "test.execute-api.us-east-1.amazonaws.com",
    },
    "queryStringParameters": map[string]interface{}{
        "Authorization": "Bearer test-token",
    },
}

// Option 1: Test through app
response, err := app.HandleRequest(context.Background(), event)
resp := response.(*lift.Response)
assert.Equal(t, 200, resp.StatusCode)

// Option 2: Test handler directly
adapter := adapters.NewWebSocketAdapter()
request, _ := adapter.Adapt(event)
ctx := lift.NewContext(context.Background(), &lift.Request{Request: request})
err := handleConnect(ctx)

// Access response data
statusCode := ctx.Response.StatusCode
body := ctx.Response.Body // This is an interface{}, not string
headers := ctx.Response.Headers
```

## Creating Test Contexts

```go
// For HTTP-like tests
func createTestContext(method, path string, body []byte) *lift.Context {
    adapterReq := &adapters.Request{
        TriggerType: adapters.TriggerAPIGateway,
        Method:      method,
        Path:        path,
        Headers:     map[string]string{"Content-Type": "application/json"},
        Body:        body,
    }
    
    request := &lift.Request{Request: adapterReq}
    return lift.NewContext(context.Background(), request)
}

// For WebSocket tests
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

## Define Your Own Types

```go
// ❌ DON'T use internal Lift types
import "github.com/pay-theory/lift/internal/shared"
metric := shared.MetricName("websocket.connect")

// ✅ DO define your own
type MetricName string

const (
    MetricWebSocketConnect    MetricName = "websocket.connect"
    MetricWebSocketDisconnect MetricName = "websocket.disconnect"
    MetricWebSocketMessage    MetricName = "websocket.message"
)
```

## Accessing Response in Tests

```go
// After handler execution
err := myHandler(ctx)

// Get response data
statusCode := ctx.Response.StatusCode
headers := ctx.Response.Headers["Content-Type"]

// Response body is interface{}, convert as needed
switch body := ctx.Response.Body.(type) {
case string:
    // Plain text response
    assert.Equal(t, "expected text", body)
case map[string]interface{}:
    // JSON response (already unmarshaled)
    assert.Equal(t, "success", body["status"])
default:
    // Convert to JSON
    jsonBytes, _ := json.Marshal(body)
    var result map[string]interface{}
    json.Unmarshal(jsonBytes, &result)
}
```

## Common Gotchas

1. **No `app.Handler()` method** - Use `app.HandleRequest`
2. **Multiple middleware** - Call `Use()` multiple times
3. **Response.Body is `interface{}`** - Not always a string
4. **Define your own types** - Don't use internal packages
5. **WebSocket auth** - JWT comes from query params, not headers

## Complete Working Example

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // Add middleware (one at a time)
    app.Use(WebSocketJWTMiddleware())
    app.Use(MetricsMiddleware())
    app.Use(TracingMiddleware())
    
    // Register routes
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
    app.Handle("MESSAGE", "/message", handleMessage)
    
    // Start Lambda
    lambda.Start(app.HandleRequest)
}

func WebSocketJWTMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            wsCtx, err := ctx.AsWebSocket()
            if err == nil && wsCtx.IsConnectEvent() {
                token := ctx.Query("Authorization")
                // Validate token...
            }
            return next.Handle(ctx)
        })
    }
}

func handleConnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    
    // Log with metrics
    ctx.Logger.Info("New connection", map[string]interface{}{
        "connectionId": wsCtx.ConnectionID(),
    })
    
    if ctx.Metrics != nil {
        ctx.Metrics.Counter("websocket.connect").Inc()
    }
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
    })
}
``` 