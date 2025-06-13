# Enhanced WebSocket Example

This example demonstrates the enhanced WebSocket support in the Lift framework, showcasing the new simplified patterns and features.

## Features Demonstrated

1. **Native WebSocket Routing** - Using `app.WebSocket()` for clean route registration
2. **WebSocket Authentication** - JWT validation from query parameters
3. **WebSocket Metrics** - Automatic metrics collection
4. **Automatic Connection Management** - Framework handles connection lifecycle
5. **Clean Handler Pattern** - Simplified handler functions

## Key Improvements

### Before (Old Pattern)
```go
app.Handle("CONNECT", "/connect", handleConnect)
app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
app.Handle("MESSAGE", "/message", handleMessage)
```

### After (Enhanced Pattern)
```go
app.WebSocket("$connect", handleConnect)
app.WebSocket("$disconnect", handleDisconnect)
app.WebSocket("$default", handleDefault)
app.WebSocket("ping", handlePing)
```

## Running the Example

1. Set environment variables:
```bash
export JWT_PUBLIC_KEY_PATH=/path/to/public.key
export JWT_ISSUER=your-issuer
```

2. Build and deploy:
```bash
go build -o bootstrap
zip function.zip bootstrap
# Deploy to AWS Lambda
```

3. Configure API Gateway WebSocket API to use this Lambda

## Handler Patterns

### Connection Handler
```go
func handleConnect(ctx *lift.Context) error {
    ws, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Connection is automatically saved by framework
    // Just handle business logic
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
    })
}
```

### Message Handler
```go
func handlePing(ctx *lift.Context) error {
    ws, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    return ws.SendMessage([]byte(`{"type":"pong"}`))
}
```

## Middleware Stack

1. **WebSocket Authentication** - Validates JWT tokens from query parameters
2. **WebSocket Metrics** - Records connection and message metrics
3. **Error Handling** - Automatic error responses

## Connection Management

With `EnableAutoConnectionManagement: true`, the framework automatically:
- Saves connections on `$connect`
- Removes connections on `$disconnect`
- Provides connection store access in handlers

## Testing

Use wscat or similar WebSocket client:

```bash
# Connect with JWT token
wscat -c "wss://your-api.execute-api.region.amazonaws.com/stage?Authorization=your-jwt-token"

# Send ping message
> {"action": "ping"}
< {"type": "pong"}

# Send to default route
> {"action": "unknown", "data": "test"}
< {"type": "echo", "message": {...}}
```

## Benefits

1. **Less Code** - Reduced boilerplate compared to standard pattern
2. **Better Middleware** - WebSocket-aware middleware for cross-cutting concerns
3. **Automatic Features** - Connection management handled by framework
4. **Type Safety** - Strong typing throughout
5. **Testability** - Easy to unit test handlers

## Next Steps

- Implement real connection store (DynamoDB)
- Add more sophisticated message routing
- Implement room/channel concepts
- Add binary message support 