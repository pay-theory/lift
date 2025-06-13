# WebSocket Adapter Analysis

**Date:** 2025-06-13-11_04_44  
**Author:** Pay Theory Streamer Team  
**Subject:** Current WebSocket Implementation Analysis

## Current State

After analyzing the Lift framework, I've discovered that WebSocket support already exists:

### Existing Components

1. **WebSocket Adapter** (`pkg/lift/adapters/websocket.go`)
   - Already handles API Gateway WebSocket events
   - Converts events to normalized Request structure
   - Maps route keys to HTTP methods

2. **WebSocket Context** (`pkg/lift/websocket_context.go`)
   - Provides WebSocket-specific functionality
   - Includes methods for sending messages, broadcasting
   - Uses AWS SDK v1 (older version)

3. **Working Example** (`examples/websocket-demo/main.go`)
   - Shows current usage patterns
   - Uses `AsWebSocket()` conversion
   - Implements JWT middleware for WebSocket

### Key Observations

1. **Current Pattern Requires Conversion**
   ```go
   wsCtx, err := ctx.AsWebSocket()
   if err != nil {
       return err
   }
   ```

2. **Routes Use HTTP Methods**
   - CONNECT → /connect
   - DISCONNECT → /disconnect  
   - MESSAGE → /message

3. **AWS SDK Version**
   - Currently using AWS SDK v1
   - Should migrate to v2 for consistency

## Improvement Opportunities

### 1. Simplify Handler Pattern
Instead of:
```go
func handleConnect(ctx *lift.Context) error {
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    // ... use wsCtx
}
```

We want:
```go
func handleConnect(ctx *lift.Context) error {
    // Direct access via ctx.WebSocket()
    return ctx.Store().Save(&Connection{
        ID: ctx.WebSocket().ConnectionID(),
    })
}
```

### 2. Native WebSocket Routing
Instead of:
```go
app.Handle("CONNECT", "/connect", handleConnect)
```

We want:
```go
app.WebSocket("$connect", handleConnect)
```

### 3. Improved Middleware
- WebSocket-specific middleware that doesn't require conversion
- Better JWT extraction from query params
- Automatic connection management

## Implementation Plan

1. **Enhance Existing Components** rather than replace
2. **Add WebSocket-specific routing** to App
3. **Create specialized middleware** for WebSocket
4. **Update to AWS SDK v2**
5. **Maintain backward compatibility**

## Next Steps

1. Create enhanced WebSocket routing in `app_websocket.go`
2. Update WebSocket context to use SDK v2
3. Build WebSocket-specific middleware
4. Create migration examples 