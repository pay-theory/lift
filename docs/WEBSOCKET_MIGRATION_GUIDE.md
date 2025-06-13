# WebSocket Migration Guide

This guide helps you migrate from the current WebSocket implementation pattern to the enhanced WebSocket support in Lift.

## Overview

The enhanced WebSocket support provides:
- Native WebSocket routing with `app.WebSocket()`
- WebSocket-specific middleware
- Automatic connection management
- Cleaner, more maintainable code

## Migration Steps

### 1. Update App Initialization

**Before:**
```go
app := lift.New()
```

**After:**
```go
app := lift.New(lift.WithWebSocketSupport())

// Or with automatic connection management:
app := lift.New(lift.WithWebSocketSupport(lift.WebSocketOptions{
    EnableAutoConnectionManagement: true,
    ConnectionStore:                myConnectionStore,
}))
```

### 2. Update Route Registration

**Before:**
```go
app.Handle("CONNECT", "/connect", handleConnect)
app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
app.Handle("MESSAGE", "/message", handleMessage)
app.Handle("MESSAGE", "/ping", handlePing)
```

**After:**
```go
app.WebSocket("$connect", handleConnect)
app.WebSocket("$disconnect", handleDisconnect)
app.WebSocket("$default", handleDefault)
app.WebSocket("ping", handlePing)
```

### 3. Update Lambda Handler

**Before:**
```go
lambda.Start(app.HandleRequest)
```

**After:**
```go
lambda.Start(app.WebSocketHandler())
```

### 4. Update Middleware

#### Authentication Middleware

**Before:**
```go
func WebSocketJWTMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            wsCtx, err := ctx.AsWebSocket()
            if err == nil && wsCtx.IsConnectEvent() {
                token := ctx.Query("Authorization")
                // Manual JWT validation...
            }
            return next.Handle(ctx)
        })
    }
}
```

**After:**
```go
app.Use(middleware.WebSocketAuth(middleware.WebSocketAuthConfig{
    JWTConfig: security.JWTConfig{
        SigningMethod: "RS256",
        PublicKeyPath: os.Getenv("JWT_PUBLIC_KEY_PATH"),
        Issuer:        os.Getenv("JWT_ISSUER"),
    },
}))
```

#### Metrics Middleware

**Before:**
```go
// Manual metrics collection in each handler
```

**After:**
```go
app.Use(middleware.WebSocketMetrics(metricsCollector))
```

### 5. Handler Pattern Updates

The handler pattern remains mostly the same, but with cleaner context access:

**Before & After (Compatible):**
```go
func handleConnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    connectionID := wsCtx.ConnectionID()
    // ... rest of handler logic
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
    })
}
```

### 6. Connection Management

With automatic connection management enabled:

**Before:**
```go
func handleConnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Manual connection storage
    connection := &Connection{
        ID:       wsCtx.ConnectionID(),
        UserID:   ctx.UserID(),
        TenantID: ctx.Get("tenant_id").(string),
    }
    
    err = storeConnection(connection)
    if err != nil {
        return ctx.Status(500).JSON(map[string]string{
            "error": "Failed to store connection",
        })
    }
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
    })
}

func handleDisconnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Manual connection removal
    err = removeConnection(wsCtx.ConnectionID())
    if err != nil {
        // Log error but don't fail
        ctx.Logger.Error("Failed to remove connection", map[string]interface{}{
            "error": err.Error(),
        })
    }
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Disconnected",
    })
}
```

**After:**
```go
func handleConnect(ctx *lift.Context) error {
    // Connection automatically saved by framework
    // Just set user context
    ctx.SetUserID(getUserID(ctx))
    ctx.Set("tenant_id", getTenantID(ctx))
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
    })
}

func handleDisconnect(ctx *lift.Context) error {
    // Connection automatically removed by framework
    // Just handle any cleanup logic
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Disconnected",
    })
}
```

## Complete Example

### Before (Current Pattern)

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // Manual middleware
    app.Use(WebSocketJWTMiddleware())
    app.Use(LoggingMiddleware())
    
    // HTTP-style routing
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
    app.Handle("MESSAGE", "/message", handleMessage)
    
    lambda.Start(app.HandleRequest)
}

func handleConnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return ctx.Status(500).JSON(map[string]string{
            "error": "Invalid WebSocket context",
        })
    }
    
    // Manual connection storage
    // Manual JWT validation
    // Manual metrics
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
    })
}
```

### After (Enhanced Pattern)

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/lift/pkg/security"
)

func main() {
    // Create app with WebSocket support
    app := lift.New(lift.WithWebSocketSupport(lift.WebSocketOptions{
        EnableAutoConnectionManagement: true,
        ConnectionStore:                NewDynamoDBStore(),
    }))
    
    // Configure middleware
    app.Use(middleware.WebSocketAuth(middleware.WebSocketAuthConfig{
        JWTConfig: security.JWTConfig{
            SigningMethod: "RS256",
            PublicKeyPath: os.Getenv("JWT_PUBLIC_KEY_PATH"),
            Issuer:        os.Getenv("JWT_ISSUER"),
        },
    }))
    app.Use(middleware.WebSocketMetrics(metrics))
    
    // Native WebSocket routing
    app.WebSocket("$connect", handleConnect)
    app.WebSocket("$disconnect", handleDisconnect)
    app.WebSocket("$default", handleDefault)
    
    // Use WebSocket handler
    lambda.Start(app.WebSocketHandler())
}

func handleConnect(ctx *lift.Context) error {
    // Much simpler - framework handles the heavy lifting
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
        "user_id": ctx.UserID(), // Set by auth middleware
    })
}
```

## Benefits of Migration

1. **Code Reduction**: ~30% less boilerplate code
2. **Better Separation of Concerns**: Middleware handles cross-cutting concerns
3. **Automatic Features**: Connection management, metrics, auth
4. **Type Safety**: Stronger typing throughout
5. **Testability**: Easier to unit test
6. **Performance**: Optimized routing and middleware execution

## Gradual Migration

The enhanced WebSocket support is backward compatible. You can:

1. Start by updating just the app initialization
2. Gradually move routes to `app.WebSocket()`
3. Replace custom middleware with built-in ones
4. Enable automatic features when ready

## Common Issues

### Issue: Middleware Type Mismatch
If you see type errors with middleware, ensure you're using `lift.Middleware` not `middleware.Middleware`.

### Issue: SendMessage in Response
Remember that WebSocket messages can't be sent as part of the Lambda response except for `$connect` events. Use the API Gateway Management API for sending messages after the response.

### Issue: Missing Context Methods
Some context methods like `BindJSON` are not available. Use `json.Unmarshal(ctx.Request.Body, &data)` instead.

## Need Help?

- Check the [examples/websocket-enhanced](../examples/websocket-enhanced) directory
- Review the [technical specification](../docs/development/notes/2025-06-13-11_04_44-websocket-adapter-analysis.md)
- Open an issue on GitHub

## Next Steps

After migration, consider:
- Implementing custom connection stores
- Adding room/channel abstractions
- Building real-time features
- Contributing improvements back to Lift 