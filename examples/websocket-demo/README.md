# WebSocket Lambda Demo with Lift

This example demonstrates how to use Lift with AWS API Gateway WebSocket events.

## Features

- WebSocket connection handling ($connect, $disconnect, message routes)
- JWT authentication via query parameters
- WebSocket-specific context helpers
- Message broadcasting capabilities
- Legacy handler adaptation

## Usage

### Handler Setup

```go
app := lift.New()

// Handle WebSocket routes
app.Handle("CONNECT", "/connect", handleConnect)
app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
app.Handle("MESSAGE", "/message", handleMessage)
```

### WebSocket Context

Lift provides a specialized WebSocket context with helper methods:

```go
func handleConnect(ctx *lift.Context) error {
    // Convert to WebSocket context
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Access WebSocket-specific data
    connectionID := wsCtx.ConnectionID()
    routeKey := wsCtx.RouteKey()
    
    // Send messages back to the client
    return wsCtx.SendJSONMessage(map[string]string{
        "message": "Welcome!",
    })
}
```

### JWT Authentication

Since WebSocket connections don't always support headers, JWT tokens are commonly passed via query parameters:

```go
func WebSocketJWTMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            wsCtx, err := ctx.AsWebSocket()
            if err == nil && wsCtx.IsConnectEvent() {
                // Extract JWT from query parameters
                token := ctx.Query("Authorization")
                // Validate and store claims...
            }
            return next.Handle(ctx)
        })
    }
}
```

### Available WebSocket Context Methods

- `ConnectionID()` - Get the WebSocket connection ID
- `RouteKey()` - Get the route key ($connect, $disconnect, or custom)
- `EventType()` - Get the event type
- `Stage()` - Get the API Gateway stage
- `DomainName()` - Get the API Gateway domain
- `ManagementEndpoint()` - Get the management API endpoint
- `SendMessage([]byte)` - Send raw message to connection
- `SendJSONMessage(interface{})` - Send JSON message
- `BroadcastMessage([]string, []byte)` - Send to multiple connections
- `Disconnect(connectionID)` - Force disconnect a connection
- `GetConnectionInfo(connectionID)` - Get connection details
- `IsConnectEvent()` - Check if this is a $connect event
- `IsDisconnectEvent()` - Check if this is a $disconnect event
- `IsMessageEvent()` - Check if this is a message event

### Adapting Legacy Handlers

If you have existing WebSocket handlers, you can adapt them:

```go
func adaptLegacyHandler(handler func(context.Context, map[string]interface{}) (map[string]interface{}, error)) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        response, err := handler(ctx.Context, ctx.Request.RawEvent.(map[string]interface{}))
        if err != nil {
            return err
        }
        // Convert response...
        return ctx.JSON(response)
    })
}
```

## Deployment

Deploy this as a Lambda function behind an API Gateway WebSocket API. Make sure to:

1. Configure routes for $connect, $disconnect, and custom routes
2. Set up proper IAM permissions for the Lambda to use the management API
3. Configure authorization (if using JWT or other auth methods)

## Testing

You can test WebSocket connections using tools like `wscat`:

```bash
# Connect with JWT token
wscat -c "wss://your-api-id.execute-api.region.amazonaws.com/stage?Authorization=your-jwt-token"

# Send a message
> {"action": "ping"}

# Receive response
< {"type": "pong", "message": "pong"}
``` 