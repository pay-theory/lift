# Streamer Migration Guide - Lift 1.0.12

## Overview

Lift 1.0.12 introduces powerful WebSocket enhancements that will significantly simplify your Streamer implementation. This guide will help you migrate your existing code to take advantage of these new features.

## What's New in 1.0.12

### 1. Native WebSocket Routing
- Direct `app.WebSocket()` method for WebSocket-specific routes
- No more HTTP-style routing for WebSocket events
- Automatic route key handling

### 2. Automatic Connection Management
- Built-in connection tracking
- DynamoDB-backed connection store
- Automatic cleanup on disconnect

### 3. AWS SDK v2 Support
- Better performance and error handling
- Native context.Context support
- Typed exceptions for better error handling

### 4. WebSocket-Specific Middleware
- Middleware that understands WebSocket context
- Built-in auth middleware for query parameters
- Metrics collection for WebSocket events

## Quick Start

### Update Your Dependencies

```bash
go get github.com/pay-theory/lift@v1.0.12
```

### Before (Old Pattern)

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // Old: HTTP-style routing for WebSocket
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("MESSAGE", "/message", handleMessage)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
    
    app.Start()
}

func handleConnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return ctx.Status(500).JSON(map[string]string{
            "error": "Invalid WebSocket context",
        })
    }
    
    connectionID := wsCtx.ConnectionID()
    // Manual connection storage required
    
    return ctx.Status(200).JSON(map[string]string{
        "status": "connected",
    })
}
```

### After (New Pattern)

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New(lift.WithWebSocketSupport())
    
    // New: Direct WebSocket routing
    app.WebSocket("$connect", handleConnect)
    app.WebSocket("message", handleMessage)
    app.WebSocket("$disconnect", handleDisconnect)
    
    app.Start()
}

func handleConnect(ctx *lift.Context) error {
    // No conversion needed - already WebSocket context
    // Connection automatically stored if enabled
    
    return ctx.Status(200).JSON(map[string]string{
        "status": "connected",
    })
}
```

## Step-by-Step Migration

### Step 1: Enable WebSocket Support

```go
// Add WebSocket support when creating the app
app := lift.New(lift.WithWebSocketSupport())
```

### Step 2: Enable Automatic Connection Management (Optional but Recommended)

```go
import (
    "context"
    "github.com/pay-theory/lift/pkg/lift"
)

// Create DynamoDB connection store
store, err := lift.NewDynamoDBConnectionStore(context.Background(), lift.DynamoDBConnectionStoreConfig{
    TableName: "streamer-connections",
    Region:    "us-east-1",
    TTLHours:  24, // Connections expire after 24 hours
})
if err != nil {
    log.Fatal(err)
}

// Enable auto connection management
app := lift.New(lift.WithWebSocketSupport(lift.WebSocketOptions{
    EnableAutoConnectionManagement: true,
    ConnectionStore:                store,
}))
```

### Step 3: Migrate Route Handlers

Replace HTTP-style routes with WebSocket routes:

```go
// Old
app.Handle("CONNECT", "/connect", handler)
app.Handle("MESSAGE", "/message", handler)
app.Handle("DISCONNECT", "/disconnect", handler)

// New
app.WebSocket("$connect", handler)
app.WebSocket("message", handler)
app.WebSocket("$disconnect", handler)
```

### Step 4: Update Handler Functions

Remove WebSocket context conversion:

```go
// Old handler
func handleMessage(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Use wsCtx for WebSocket operations
    return wsCtx.SendMessage([]byte("Hello"))
}

// New handler
func handleMessage(ctx *lift.Context) error {
    // Direct WebSocket operations on context
    wsCtx, _ := ctx.AsWebSocketV2() // Use V2 for SDK v2 features
    return wsCtx.SendMessage(ctx.Context(), []byte("Hello"))
}
```

### Step 5: Add WebSocket Middleware

```go
import (
    "github.com/pay-theory/lift/pkg/middleware"
)

// Add authentication middleware
app.Use(middleware.WebSocketAuth(middleware.WebSocketAuthConfig{
    ValidateToken: func(token string) (string, error) {
        // Your token validation logic
        return userID, nil
    },
}))

// Add metrics collection
app.Use(middleware.WebSocketMetrics(metricsCollector))
```

## Streamer-Specific Features

### Broadcasting to Rooms

With automatic connection management, you can easily implement rooms:

```go
func handleJoinRoom(ctx *lift.Context) error {
    roomID := ctx.Body("roomId")
    
    // Store room membership in connection metadata
    ctx.Set("room_id", roomID)
    
    return ctx.Status(200).JSON(map[string]string{
        "joined": roomID,
    })
}

func broadcastToRoom(ctx *lift.Context, roomID string, message []byte) error {
    wsCtx, _ := ctx.AsWebSocketV2()
    
    // Get all connections in the room
    connections, err := store.ListByTenant(ctx.Context(), roomID)
    if err != nil {
        return err
    }
    
    // Extract connection IDs
    var connectionIDs []string
    for _, conn := range connections {
        connectionIDs = append(connectionIDs, conn.ID)
    }
    
    // Broadcast to all connections in the room
    return wsCtx.BroadcastMessage(ctx.Context(), connectionIDs, message)
}
```

### Handling Subscriptions

```go
type Subscription struct {
    UserID    string
    Channel   string
    ConnID    string
}

func handleSubscribe(ctx *lift.Context) error {
    channel := ctx.Body("channel")
    userID := ctx.GetUserID()
    
    // Store subscription
    ctx.Set("subscriptions", append(
        ctx.Get("subscriptions").([]string), 
        channel,
    ))
    
    return ctx.Status(200).JSON(map[string]string{
        "subscribed": channel,
    })
}
```

### Real-time Event Streaming

```go
func handleStreamStart(ctx *lift.Context) error {
    streamID := ctx.Body("streamId")
    
    // Set up stream metadata
    ctx.Set("stream_id", streamID)
    ctx.Set("stream_start", time.Now())
    
    // Notify other viewers
    return notifyViewers(ctx, streamID, "stream_started")
}
```

## Performance Improvements

The new implementation provides significant performance benefits:

- **70% less code** for typical WebSocket handlers
- **Near-zero overhead** for context conversion (1.117 ns/op)
- **4% overhead** for automatic connection management
- **Better memory efficiency** with SDK v2

## Testing Your Migration

### Unit Tests

```go
func TestWebSocketHandler(t *testing.T) {
    app := lift.New(lift.WithWebSocketSupport())
    
    app.WebSocket("message", func(ctx *lift.Context) error {
        return ctx.Status(200).JSON(map[string]string{
            "received": ctx.Body("data"),
        })
    })
    
    // Test the handler
    event := createTestWebSocketEvent("message", "conn123", `{"data":"test"}`)
    response, err := app.WebSocketHandler()(context.Background(), event)
    
    assert.NoError(t, err)
    assert.Equal(t, 200, response.StatusCode)
}
```

### Integration Tests

```go
func TestWebSocketIntegration(t *testing.T) {
    // Set up test DynamoDB table
    store := setupTestStore(t)
    
    app := lift.New(lift.WithWebSocketSupport(lift.WebSocketOptions{
        EnableAutoConnectionManagement: true,
        ConnectionStore:                store,
    }))
    
    // Test connection lifecycle
    // ... test implementation
}
```

## Common Patterns

### Pattern 1: Authenticated Connections

```go
app.WebSocket("$connect", func(ctx *lift.Context) error {
    // Get auth token from query parameters
    token := ctx.Query("token")
    
    userID, err := validateToken(token)
    if err != nil {
        return ctx.Status(401).JSON(map[string]string{
            "error": "Unauthorized",
        })
    }
    
    // Set user ID for connection tracking
    ctx.SetUserID(userID)
    
    return ctx.Status(200).JSON(map[string]string{
        "status": "connected",
        "userId": userID,
    })
})
```

### Pattern 2: Message Routing

```go
app.WebSocket("message", func(ctx *lift.Context) error {
    var msg struct {
        Type string          `json:"type"`
        Data json.RawMessage `json:"data"`
    }
    
    if err := ctx.Bind(&msg); err != nil {
        return ctx.Status(400).JSON(map[string]string{
            "error": "Invalid message format",
        })
    }
    
    switch msg.Type {
    case "chat":
        return handleChatMessage(ctx, msg.Data)
    case "presence":
        return handlePresenceUpdate(ctx, msg.Data)
    case "stream":
        return handleStreamEvent(ctx, msg.Data)
    default:
        return ctx.Status(400).JSON(map[string]string{
            "error": "Unknown message type",
        })
    }
})
```

### Pattern 3: Graceful Disconnection

```go
app.WebSocket("$disconnect", func(ctx *lift.Context) error {
    // Connection automatically removed from store
    
    // Clean up any stream-specific resources
    if streamID := ctx.Get("stream_id"); streamID != nil {
        endStream(ctx, streamID.(string))
    }
    
    // Notify other users
    if roomID := ctx.Get("room_id"); roomID != nil {
        notifyRoom(ctx, roomID.(string), "user_left")
    }
    
    return nil // No response needed for disconnect
})
```

## Troubleshooting

### Issue: "Context is not from a WebSocket event"
**Solution**: Ensure you're using `app.WebSocket()` instead of `app.Handle()`

### Issue: Connections not being stored
**Solution**: Enable automatic connection management in WebSocketOptions

### Issue: Cannot send messages after Lambda returns
**Solution**: Use the WebSocket Management API for async messaging

### Issue: Authentication failing
**Solution**: Pass auth tokens as query parameters for $connect events

## Best Practices

1. **Always use SDK v2 context** for better performance
2. **Enable connection management** to simplify your code
3. **Use middleware** for cross-cutting concerns
4. **Handle errors gracefully** - disconnected clients are common
5. **Set appropriate TTLs** for connection records
6. **Monitor CloudWatch metrics** for performance insights

## Need Help?

- Check the [WebSocket examples](../examples/websocket-enhanced)
- Review the [performance analysis](development/notes/2025-06-13-14_15_00-websocket-performance-analysis.md)
- Contact the Lift team for migration support

## Summary

Migrating to Lift 1.0.12 will:
- Reduce your WebSocket code by ~70%
- Improve performance and reliability
- Simplify connection management
- Enable better monitoring and debugging

The migration is straightforward and can be done incrementally. Start with one handler at a time and test thoroughly before moving to the next. 