# Response to Streamer Team: WebSocket Support Now Available in Lift

Date: 2025-06-13
From: Lift Team
To: Streamer Team

## Great News - WebSocket Support is Now Live!

We've heard your integration challenges and have implemented comprehensive WebSocket support in Lift. This is now available in the latest version and ready for your use.

## Quick Start

Here's how to migrate your WebSocket handlers to Lift:

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // WebSocket routes use special HTTP methods
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect) 
    app.Handle("MESSAGE", "/message", handleMessage)
    
    app.Start() // Automatically creates the Lambda handler
}
```

## Answers to Your Specific Questions

### 1. WebSocket Lambda Integration ✅

No special wrapper needed! Lift now automatically detects and handles WebSocket events:

```go
// Your existing handler signature:
// func(context.Context, events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error)

// Becomes this simple Lift handler:
func handleConnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    
    // Access all WebSocket data
    connectionID := wsCtx.ConnectionID()
    routeKey := wsCtx.RouteKey()
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected!",
    })
}
```

### 2. JWT Validation Middleware ✅

We've designed middleware to work seamlessly with WebSocket's query parameter authentication:

```go
func WebSocketJWTMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            wsCtx, err := ctx.AsWebSocket()
            if err == nil && wsCtx.IsConnectEvent() {
                // Get JWT from query params (WebSocket standard)
                token := ctx.Query("Authorization")
                
                // Validate and store claims
                claims := validateJWT(token)
                ctx.Set("user_claims", claims)
                ctx.SetUserID(claims.UserID)
            }
            
            return next.Handle(ctx)
        })
    }
}

// Use it like any other middleware
app.Use(WebSocketJWTMiddleware())
```

### 3. Context and Logging ✅

The logger is directly available on the context:

```go
func handleMessage(ctx *lift.Context) error {
    // Direct logger access
    ctx.Logger.Info("Processing message", map[string]interface{}{
        "userId": ctx.UserID(),
        "action": "message",
    })
    
    // Store and retrieve values
    ctx.Set("messageCount", count)
    count := ctx.Get("messageCount").(int)
    
    return nil
}
```

### 4. Error Handling ✅

WebSocket error responses work naturally:

```go
func handleConnect(ctx *lift.Context) error {
    // For $connect/$disconnect - return HTTP-like responses
    if !authorized {
        return ctx.Status(401).JSON(map[string]string{
            "error": "Unauthorized",
        })
    }
    
    // For message events - send back through WebSocket
    wsCtx, _ := ctx.AsWebSocket()
    if err := processMessage(); err != nil {
        return wsCtx.SendJSONMessage(map[string]string{
            "error": "Processing failed",
            "details": err.Error(),
        })
    }
    
    return nil
}
```

### 5. Lambda Handler Signature ✅

**No conversion needed!** Lift handles everything automatically:

```go
// Just use standard Lift handlers
app.Handle("CONNECT", "/connect", func(ctx *lift.Context) error {
    // Your code here
    return ctx.OK(response)
})

// For legacy migration, we provide an adapter
legacyHandler := adaptLegacyHandler(yourExistingHandler)
app.Handle("CONNECT", "/connect", legacyHandler)
```

## New WebSocket-Specific Features

### WebSocket Context

Access WebSocket-specific functionality through the specialized context:

```go
wsCtx, err := ctx.AsWebSocket()

// Available methods:
wsCtx.ConnectionID()              // Get connection ID
wsCtx.RouteKey()                  // Get route key ($connect, etc.)
wsCtx.Stage()                     // API Gateway stage
wsCtx.ManagementEndpoint()        // Management API endpoint

// Messaging
wsCtx.SendMessage([]byte)         // Send raw message
wsCtx.SendJSONMessage(data)       // Send JSON message
wsCtx.BroadcastMessage(ids, data) // Broadcast to multiple connections

// Connection management  
wsCtx.Disconnect(connectionID)    // Force disconnect
wsCtx.GetConnectionInfo(id)       // Get connection details

// Helpers
wsCtx.IsConnectEvent()           // Check if $connect
wsCtx.IsDisconnectEvent()        // Check if $disconnect
wsCtx.IsMessageEvent()           // Check if message event
```

### Complete Working Example

We've created a full example at `examples/websocket-demo/` that shows:
- JWT authentication via query parameters
- Connection/disconnection handling
- Message routing and broadcasting
- Error handling patterns
- Legacy handler migration

## Migration Guide

1. **Update your imports** to use Lift
2. **Replace handler signatures** with `func(ctx *lift.Context) error`
3. **Use `ctx.AsWebSocket()`** to access WebSocket features
4. **Move JWT to query params** for $connect events
5. **Let Lift handle responses** - no need to manually format

## What's Next?

The WebSocket adapter is now part of Lift's core and will be maintained alongside other adapters. Future enhancements we're considering:

- Built-in DynamoDB connection tracking
- Automatic heartbeat handling
- WebSocket-specific metrics
- Rate limiting for messages

## Try It Now!

The WebSocket support is available in the latest version. Check out the example and let us know if you have any questions or need help migrating.

## Example Repository Structure

```
your-websocket-service/
├── main.go              # Your Lambda handler
├── handlers/
│   ├── connect.go       # $connect handler
│   ├── disconnect.go    # $disconnect handler
│   └── message.go       # Message handlers
├── middleware/
│   └── auth.go          # JWT validation
└── go.mod
```

## Need Help?

We're here to support your migration. Feel free to:
- Check the complete example at `examples/websocket-demo/`
- Review the implementation notes at `docs/development/notes/`
- Reach out with any questions or issues

Happy WebSocket handling with Lift! 🚀

---

**TL;DR**: WebSocket support is now native in Lift. No wrappers needed, full middleware support, clean API. Check `examples/websocket-demo/` for a complete working example. 