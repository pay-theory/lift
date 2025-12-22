# Streamer Package Guide

The `pkg/streamer` package provides WebSocket connection management for AWS API Gateway. It offers a clean, unified interface for managing WebSocket connections with robust error handling and connection lifecycle management.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Error Handling](#error-handling)
- [Connection Management](#connection-management)
- [Testing](#testing)
- [Best Practices](#best-practices)
- [Examples](#examples)

## Overview

The streamer package is a first-class component of Lift that provides:

- **Clean Interface**: Type-safe abstraction over AWS API Gateway Management API
- **Structured Errors**: Comprehensive error types with HTTP status codes
- **Connection Metadata**: Rich information about WebSocket connections
- **Testing Support**: Full mock implementation for unit tests
- **AWS SDK v2**: Built on modern AWS SDK with proper context support

### When to Use Streamer

Use the streamer package when you need:

- Fine-grained control over WebSocket connections
- Detailed connection metadata (age, idle time, source IP, etc.)
- Structured error handling with type assertions
- Testable WebSocket code with mocks
- Direct access to API Gateway Management API

For simpler use cases, consider using `lift.WebSocketContext` which wraps streamer internally.

**Not SSE:** The streamer package is for **WebSockets**, not Server-Sent Events (SSE). For one-way HTTP streaming, see [Response Streaming (SSE)](./response-streaming.md).

## Installation

The streamer package is included with Lift. No additional installation needed:

```go
import "github.com/pay-theory/lift/pkg/streamer"
```

## Quick Start

### Creating a Client

```go
import (
    "context"
    "github.com/pay-theory/lift/pkg/streamer"
)

// Create a client with configuration
client, err := streamer.NewClient(context.Background(), streamer.ClientConfig{
    Endpoint: "https://abc123.execute-api.us-east-1.amazonaws.com/production",
    Region:   "us-east-1",
})
if err != nil {
    log.Fatal(err)
}
```

### Sending Messages

```go
data := []byte(`{"message": "Hello, WebSocket!"}`)
err := client.PostToConnection(ctx, connectionID, data)
if err != nil {
    if errors.Is(err, streamer.ErrConnectionGone) {
        // Handle disconnected connection
    }
}
```

### Getting Connection Info

```go
info, err := client.GetConnection(ctx, connectionID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Connected at: %v\n", info.ConnectedAt)
fmt.Printf("Is Active: %v\n", info.IsActive())
fmt.Printf("Age: %v\n", info.Age())
```

### Disconnecting a Connection

```go
err := client.DeleteConnection(ctx, connectionID)
if err != nil {
    log.Printf("Failed to disconnect: %v", err)
}
```

### EventBus Fanout

Streamer commonly pairs with Lift’s EventBus for “publish → push to sockets” flows:

```go
result, err := services.FanoutEventBusEvent(ctx, client, event, services.EventBusFanoutOptions{
    ResolveConnectionIDs: services.TenantConnectionsResolver(connectionStore),
})
_ = result // result.Gone contains stale/gone connections
_ = err
```

### Subscription-Aware Fanout (Topics)

For “topic/stream scoped” fanout (home/public/list/hashtag/etc.) without scans, pair `services.TopicConnectionsResolver` with a `lift.SubscriptionStore`.

Lift ships a DynamORM-backed subscription store that co-locates subscription rows in the same `websocket-connections` table:

```go
subscriptionStore, _ := lift.NewDynamoDBSubscriptionStoreWithDB(db, lift.DynamoDBSubscriptionStoreConfig{})

resolver := services.TopicConnectionsResolver(subscriptionStore, func(e *services.Event) []string {
    // Derive topics from the event (metadata/tags are common)
    return []string{"home", "public"}
})

result, err := services.FanoutEventBusEvent(ctx, client, event, services.EventBusFanoutOptions{
    ResolveConnectionIDs: resolver,
})
_ = result
_ = err
```

On disconnect, you can clean up subscriptions efficiently with `subscriptionStore.DeleteByConnection(ctx, connectionID)`.

## API Reference

### Client Interface

```go
type Client interface {
    // PostToConnection sends data to a specific WebSocket connection
    PostToConnection(ctx context.Context, connectionID string, data []byte) error
    
    // DeleteConnection forcefully disconnects a WebSocket connection
    DeleteConnection(ctx context.Context, connectionID string) error
    
    // GetConnection retrieves information about a WebSocket connection
    GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error)
}
```

### ClientConfig

```go
type ClientConfig struct {
    Endpoint  string      // Required: API Gateway endpoint
    Region    string      // Optional: AWS region (defaults to "us-east-1")
    AWSConfig *aws.Config // Optional: Custom AWS configuration
}
```

### ConnectionInfo

```go
type ConnectionInfo struct {
    ConnectionID string
    ConnectedAt  time.Time
    LastActiveAt time.Time
    SourceIP     string
    UserAgent    string
    Identity     map[string]any
}

// Helper methods
func (c ConnectionInfo) IsActive() bool           // Returns true if connection is active
func (c ConnectionInfo) Age() time.Duration       // Returns connection age
func (c ConnectionInfo) IdleDuration() time.Duration  // Returns idle time
```

### Connection States

```go
const (
    ConnectionStateActive       ConnectionState = "ACTIVE"
    ConnectionStateDisconnected ConnectionState = "DISCONNECTED"
    ConnectionStateStale        ConnectionState = "STALE"
)
```

## Error Handling

The streamer package provides structured error types that implement the `APIError` interface.

### APIError Interface

```go
type APIError interface {
    error
    HTTPStatusCode() int
    ErrorCode() string
    IsRetryable() bool
}
```

### Error Types

#### GoneError (410)

Connection no longer exists. This is the most common error and indicates the WebSocket has been disconnected.

```go
err := client.PostToConnection(ctx, connectionID, data)
if errors.Is(err, streamer.ErrConnectionGone) {
    // Connection is gone, remove from your database
    log.Printf("Connection %s no longer exists", connectionID)
}

// Or type assert for more details
if goneErr, ok := err.(streamer.GoneError); ok {
    fmt.Printf("Connection ID: %s\n", goneErr.ConnectionID)
    fmt.Printf("Message: %s\n", goneErr.Message)
}
```

#### ForbiddenError (403)

Operation not permitted. Usually due to missing IAM permissions.

```go
if errors.Is(err, streamer.ErrForbidden) {
    log.Printf("Operation forbidden - check IAM permissions")
}
```

#### PayloadTooLargeError (413)

Message exceeds maximum size (128KB for API Gateway WebSockets).

```go
if errors.Is(err, streamer.ErrPayloadTooLarge) {
    log.Printf("Message too large - split into smaller chunks")
}

// Get size details
if payloadErr, ok := err.(streamer.PayloadTooLargeError); ok {
    fmt.Printf("Size: %d, Max: %d\n", payloadErr.PayloadSize, payloadErr.MaxSize)
}
```

#### ThrottlingError (429)

Rate limit exceeded. This error is retryable.

```go
if errors.Is(err, streamer.ErrThrottled) {
    // Implement exponential backoff
    time.Sleep(time.Second)
    // Retry
}

if throttleErr, ok := err.(streamer.ThrottlingError); ok {
    fmt.Printf("Retry after %d seconds\n", throttleErr.RetryAfter)
}
```

#### InternalServerError (500)

AWS service error. This error is retryable.

```go
if errors.Is(err, streamer.ErrInternalServer) {
    // Retry with backoff
}
```

### Sentinel Errors

Use sentinel errors with `errors.Is()` for clean error handling:

```go
var (
    ErrConnectionGone   = errors.New("connection no longer exists")
    ErrForbidden        = errors.New("operation forbidden")
    ErrPayloadTooLarge  = errors.New("payload exceeds maximum size")
    ErrThrottled        = errors.New("request throttled")
    ErrInternalServer   = errors.New("internal server error")
    ErrInvalidConnection = errors.New("invalid connection ID")
)
```

## Connection Management

### Tracking Connection Lifecycle

```go
// On connect
func handleConnect(ctx *lift.Context) error {
    connectionID := ctx.Request.Metadata["connectionId"].(string)
    
    // Store in DynamoDB
    connection := &Connection{
        ID:          connectionID,
        ConnectedAt: time.Now(),
        UserID:      getUserID(ctx),
    }
    db.SaveConnection(connection)
    
    return ctx.Status(200).JSON(map[string]string{
        "message": "Connected",
    })
}

// On disconnect
func handleDisconnect(ctx *lift.Context) error {
    connectionID := ctx.Request.Metadata["connectionId"].(string)
    
    // Remove from DynamoDB
    db.DeleteConnection(connectionID)
    
    return nil
}
```

### Broadcasting to Multiple Connections

```go
func broadcast(client streamer.Client, connectionIDs []string, message []byte) error {
    var errs []error
    var staleConnections []string
    
    for _, connID := range connectionIDs {
        err := client.PostToConnection(context.Background(), connID, message)
        if err != nil {
            if errors.Is(err, streamer.ErrConnectionGone) {
                // Track stale connections for cleanup
                staleConnections = append(staleConnections, connID)
            } else {
                errs = append(errs, err)
            }
        }
    }
    
    // Clean up stale connections
    for _, connID := range staleConnections {
        db.DeleteConnection(connID)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("broadcast errors: %v", errs)
    }
    
    return nil
}
```

### Connection Health Checks

```go
func checkConnectionHealth(client streamer.Client, connectionID string) (bool, error) {
    info, err := client.GetConnection(context.Background(), connectionID)
    if err != nil {
        if errors.Is(err, streamer.ErrConnectionGone) {
            return false, nil // Connection is gone
        }
        return false, err // Other error
    }
    
    // Check if connection is active (not idle for too long)
    if info.IdleDuration() > 2*time.Hour {
        return false, nil
    }
    
    return true, nil
}
```

## Testing

The streamer package includes a comprehensive mock for testing.

### Using the Mock

```go
import (
    "testing"
    "github.com/pay-theory/lift/pkg/testing"
    "github.com/pay-theory/lift/pkg/streamer"
)

func TestSendMessage(t *testing.T) {
    // Create a mock client
    mock := testing.NewStreamerClientMock()
    
    // Add a connection
    mock.WithConnection("conn-123", nil)
    
    // Test sending a message
    err := mock.PostToConnection(context.Background(), "conn-123", []byte("test"))
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
    
    // Verify message was sent
    messages := mock.GetMessages("conn-123")
    if len(messages) != 1 {
        t.Errorf("Expected 1 message, got %d", len(messages))
    }
}
```

### Simulating Errors

```go
func TestErrorHandling(t *testing.T) {
    mock := testing.NewStreamerClientMock()
    
    // Simulate a gone error
    mock.WithGoneError("conn-gone", "Connection has been closed")
    
    err := mock.PostToConnection(context.Background(), "conn-gone", []byte("test"))
    
    // Verify error type
    if !errors.Is(err, streamer.ErrConnectionGone) {
        t.Error("Expected ErrConnectionGone")
    }
}
```

### Testing Connection Lifecycle

```go
func TestConnectionLifecycle(t *testing.T) {
    mock := testing.NewStreamerClientMock()
    
    // Add connection
    mock.WithConnection("conn-123", nil)
    
    // Get connection info
    info, err := mock.GetConnection(context.Background(), "conn-123")
    if err != nil {
        t.Fatalf("Failed to get connection: %v", err)
    }
    
    if info.ConnectionID != "conn-123" {
        t.Errorf("Expected conn-123, got %s", info.ConnectionID)
    }
    
    // Delete connection
    err = mock.DeleteConnection(context.Background(), "conn-123")
    if err != nil {
        t.Errorf("Failed to delete: %v", err)
    }
    
    // Verify connection state
    conn := mock.GetConnectionState("conn-123")
    if conn.State != streamer.ConnectionStateDisconnected {
        t.Errorf("Expected disconnected state")
    }
}
```

## Best Practices

### 1. Always Handle Gone Errors

```go
err := client.PostToConnection(ctx, connectionID, data)
if err != nil {
    if errors.Is(err, streamer.ErrConnectionGone) {
        // Clean up the connection
        db.DeleteConnection(connectionID)
        return nil // Don't treat as error
    }
    return err // Other errors should be handled
}
```

### 2. Implement Retry Logic for Retryable Errors

```go
func sendWithRetry(client streamer.Client, connID string, data []byte) error {
    maxRetries := 3
    backoff := time.Second
    
    for i := 0; i < maxRetries; i++ {
        err := client.PostToConnection(context.Background(), connID, data)
        if err == nil {
            return nil
        }
        
        // Check if retryable
        if apiErr, ok := err.(streamer.APIError); ok && apiErr.IsRetryable() {
            time.Sleep(backoff)
            backoff *= 2
            continue
        }
        
        return err // Non-retryable error
    }
    
    return fmt.Errorf("max retries exceeded")
}
```

### 3. Validate Message Size

```go
const maxMessageSize = 128 * 1024 // 128KB

func sendMessage(client streamer.Client, connID string, data []byte) error {
    if len(data) > maxMessageSize {
        return fmt.Errorf("message too large: %d bytes", len(data))
    }
    
    return client.PostToConnection(context.Background(), connID, data)
}
```

### 4. Use Context for Timeouts

```go
func sendWithTimeout(client streamer.Client, connID string, data []byte) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    return client.PostToConnection(ctx, connID, data)
}
```

### 5. Track Connection Metadata

```go
type ConnectionMetadata struct {
    ID          string
    UserID      string
    ConnectedAt time.Time
    LastPing    time.Time
    Metadata    map[string]any
}

// Store in DynamoDB
func trackConnection(db *dynamodb.Client, info *streamer.ConnectionInfo, userID string) error {
    metadata := &ConnectionMetadata{
        ID:          info.ConnectionID,
        UserID:      userID,
        ConnectedAt: info.ConnectedAt,
        LastPing:    time.Now(),
        Metadata: map[string]any{
            "sourceIP":  info.SourceIP,
            "userAgent": info.UserAgent,
        },
    }
    
    return db.SaveConnection(metadata)
}
```

## Examples

### Complete Lambda Handler

```go
func handleMessage(ctx *lift.Context, client streamer.Client) error {
    connectionID := ctx.Request.Metadata["connectionId"].(string)
    
    // Parse message
    var msg Message
    if err := ctx.ParseRequest(&msg); err != nil {
        return ctx.Status(400).JSON(map[string]string{
            "error": "Invalid message",
        })
    }
    
    // Process message
    response, err := processMessage(msg)
    if err != nil {
        return ctx.Status(500).JSON(map[string]string{
            "error": err.Error(),
        })
    }
    
    // Send response
    responseData, _ := json.Marshal(response)
    err = client.PostToConnection(ctx.Context, connectionID, responseData)
    if err != nil {
        if errors.Is(err, streamer.ErrConnectionGone) {
            log.Printf("Connection %s gone, cleaning up", connectionID)
            db.DeleteConnection(connectionID)
            return nil
        }
        return err
    }
    
    return ctx.Status(200).JSON(map[string]string{
        "status": "sent",
    })
}
```

### Broadcast System

```go
type BroadcastService struct {
    client streamer.Client
    db     *dynamodb.Client
}

func (s *BroadcastService) BroadcastToRoom(roomID string, message []byte) error {
    // Get all connections in room
    connections, err := s.db.GetConnectionsByRoom(roomID)
    if err != nil {
        return err
    }
    
    // Send to all connections
    var wg sync.WaitGroup
    errors := make(chan error, len(connections))
    
    for _, conn := range connections {
        wg.Add(1)
        go func(connID string) {
            defer wg.Done()
            
            err := s.client.PostToConnection(context.Background(), connID, message)
            if err != nil {
                if errors.Is(err, streamer.ErrConnectionGone) {
                    // Clean up stale connection
                    s.db.DeleteConnection(connID)
                } else {
                    errors <- err
                }
            }
        }(conn.ID)
    }
    
    wg.Wait()
    close(errors)
    
    // Check for non-gone errors
    var errs []error
    for err := range errors {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("broadcast errors: %v", errs)
    }
    
    return nil
}
```

## Related Documentation

- [WebSocket Guide](./websocket-modular-pattern.md) - General WebSocket patterns in Lift
- [Testing Guide](./testing-guide.md) - Testing WebSocket handlers
- [API Reference](./api-reference.md) - Complete Lift API documentation
- [Examples](../examples/streamer-client-demo/) - Working code examples

## Migration from WebSocketContext

If you're currently using `lift.WebSocketContext`, you can migrate to streamer for more control:

### Before (WebSocketContext)

```go
wsCtx, _ := ctx.AsWebSocket()
err := wsCtx.SendMessage(data)
```

### After (Streamer)

```go
client, _ := streamer.NewClient(ctx, streamer.ClientConfig{
    Endpoint: wsCtx.ManagementEndpoint(),
    Region:   wsCtx.GetRegion(),
})
err := client.PostToConnection(ctx.Context, connectionID, data)
if errors.Is(err, streamer.ErrConnectionGone) {
    // Handle gracefully
}
```

The streamer approach provides:
- Better error handling
- Access to connection metadata
- Easier testing with mocks
- More control over connection management

## Troubleshooting

### "Connection not found" errors

```go
// Always check if connection exists before sending
info, err := client.GetConnection(ctx, connectionID)
if errors.Is(err, streamer.ErrConnectionGone) {
    // Connection doesn't exist, remove from database
    return nil
}
```

### "Payload too large" errors

```go
// Split large messages into chunks
const chunkSize = 100 * 1024 // 100KB chunks

func sendLargeMessage(client streamer.Client, connID string, data []byte) error {
    for i := 0; i < len(data); i += chunkSize {
        end := i + chunkSize
        if end > len(data) {
            end = len(data)
        }
        
        chunk := data[i:end]
        if err := client.PostToConnection(ctx, connID, chunk); err != nil {
            return err
        }
    }
    return nil
}
```

### Rate limiting

```go
// Implement token bucket or leaky bucket algorithm
type RateLimiter struct {
    tokens    int
    maxTokens int
    refillRate time.Duration
    mu        sync.Mutex
}

func (r *RateLimiter) Allow() bool {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if r.tokens > 0 {
        r.tokens--
        return true
    }
    return false
}
```

## Support

For questions or issues:
- Review the [examples](../examples/)
- Check the [troubleshooting guide](./troubleshooting.md)
- Open an issue on GitHub
