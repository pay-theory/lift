# WebSocket Support Implementation for Lift

Date: 2025-06-13
Author: Lift Team

## Overview

We've implemented comprehensive WebSocket support in Lift to address the critical need for WebSocket Lambda handlers. This implementation provides a clean, idiomatic way to handle API Gateway WebSocket events while maintaining consistency with Lift's design patterns.

## Implementation Details

### 1. WebSocket Adapter (`pkg/lift/adapters/websocket.go`)

Created a new adapter that:
- Detects WebSocket events by checking for `connectionId` and `routeKey` in the request context
- Maps WebSocket route keys (`$connect`, `$disconnect`, custom routes) to HTTP-like methods
- Extracts all WebSocket-specific metadata (connection ID, stage, domain, etc.)
- Stores metadata in the Request's new `Metadata` field for easy access

### 2. Enhanced Request Structure

Added a `Metadata` field to the `adapters.Request` struct:
```go
// Additional metadata for specific event types (e.g., WebSocket)
Metadata map[string]interface{} `json:"metadata,omitempty"`
```

This allows event-specific data to be stored without polluting the core request structure.

### 3. WebSocket Context (`pkg/lift/websocket_context.go`)

Created a specialized WebSocket context that provides:
- Easy access to WebSocket-specific data (connection ID, route key, etc.)
- Built-in API Gateway Management API client
- Helper methods for sending messages, broadcasting, and managing connections
- Type-safe conversion from regular Lift context

### 4. Integration Points

- Added `TriggerWebSocket` constant to trigger types
- Registered WebSocket adapter in the default adapter registry
- Updated request.go to re-export the new WebSocket trigger type

## Answers to Developer Questions

### Q1: WebSocket Lambda Integration Pattern

**Recommended Pattern:**
```go
app := lift.New()

// Use special HTTP methods for WebSocket routes
app.Handle("CONNECT", "/connect", handleConnect)
app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
app.Handle("MESSAGE", "/message", handleMessage)
app.Handle("MESSAGE", "/custom-route", handleCustomRoute)
```

The adapter automatically detects WebSocket events and maps route keys to appropriate paths.

### Q2: Custom Middleware for WebSocket Context

**JWT Validation Example:**
```go
func WebSocketJWTMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            wsCtx, err := ctx.AsWebSocket()
            if err == nil && wsCtx.IsConnectEvent() {
                // Extract JWT from query parameters
                token := ctx.Query("Authorization")
                
                // Validate token and store claims
                claims := validateJWT(token)
                ctx.Set("user_claims", claims)
                ctx.SetUserID(claims.UserID)
            }
            
            return next.Handle(ctx)
        })
    }
}
```

### Q3: Context and Logging

**Using Lift's Logger:**
```go
func handleConnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Use the logger from context
    ctx.Logger.Info("WebSocket connection", map[string]interface{}{
        "connectionId": wsCtx.ConnectionID(),
        "userId": ctx.UserID(),
    })
    
    return nil
}
```

**Storing and Retrieving Values:**
```go
// Store values
ctx.Set("custom_data", myData)
ctx.SetUserID(userID)

// Retrieve values
data := ctx.Get("custom_data")
userID := ctx.UserID()
```

### Q4: Error Handling

**WebSocket-Compatible Error Responses:**
```go
func handleMessage(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Validate message
    if err := validateMessage(ctx.Request.Body); err != nil {
        // Send error back to WebSocket client
        return wsCtx.SendJSONMessage(map[string]string{
            "error": "Invalid message format",
            "details": err.Error(),
        })
    }
    
    // For connection events, return HTTP-like responses
    if wsCtx.IsConnectEvent() {
        return ctx.Status(401).JSON(map[string]string{
            "error": "Unauthorized",
        })
    }
    
    return nil
}
```

### Q5: Lambda Handler Signature

**No Wrapper Needed!** Lift automatically handles the conversion. Simply use:
```go
func main() {
    app := lift.New()
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Start() // This creates the proper Lambda handler
}
```

For legacy handler migration:
```go
// Adapt existing handlers
legacyHandler := func(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Legacy code
}

// Convert to Lift handler
liftHandler := adaptLegacyHandler(func(ctx context.Context, event map[string]interface{}) (map[string]interface{}, error) {
    // Transform as needed
    return legacyHandler(ctx, transformEvent(event))
})

app.Handle("CONNECT", "/connect", liftHandler)
```

## Key Features

1. **Automatic Event Detection**: The WebSocket adapter automatically detects and handles WebSocket events
2. **Unified Interface**: WebSocket handlers use the same `lift.Handler` interface as HTTP handlers
3. **Rich Context**: WebSocketContext provides all necessary methods for WebSocket operations
4. **Middleware Support**: Standard Lift middleware works with WebSocket handlers
5. **Type Safety**: Maintains Go's type safety while handling dynamic WebSocket events

## Migration Guide

To migrate existing WebSocket handlers to Lift:

1. Replace handler signatures with Lift handlers
2. Use `ctx.AsWebSocket()` to access WebSocket-specific functionality
3. Extract JWT/auth from query parameters instead of headers for $connect
4. Use WebSocketContext methods for sending messages instead of manual API calls
5. Let Lift handle the response formatting

## Example Usage

See `examples/websocket-demo/` for a complete working example including:
- Connection management
- JWT authentication
- Message handling
- Broadcasting
- Error handling

## Future Enhancements

1. Built-in connection tracking with DynamoDB
2. Automatic heartbeat/ping-pong handling
3. Connection pooling for management API
4. WebSocket-specific metrics and monitoring
5. Rate limiting for WebSocket messages 