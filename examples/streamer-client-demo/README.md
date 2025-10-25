# Streamer Client Demo

This example demonstrates how to use the `pkg/streamer` package for WebSocket connection management in Lift applications.

## Features

- **Direct Streamer Integration**: Uses the `streamer.Client` interface for fine-grained control over WebSocket connections
- **Connection Management**: Handle connect, disconnect, and message events
- **Error Handling**: Proper handling of streamer-specific errors (GoneError, ForbiddenError, etc.)
- **Connection Info**: Retrieve detailed information about WebSocket connections
- **Broadcasting**: Send messages to multiple connections with error handling

## What This Example Shows

1. **Creating a Streamer Client**: 
   - Initialize the streamer client from WebSocket context
   - Configure endpoint and region

2. **Sending Messages**:
   ```go
   client, err := streamer.NewClient(ctx, streamer.ClientConfig{
       Endpoint: endpoint,
       Region:   region,
   })
   err = client.PostToConnection(ctx, connectionID, data)
   ```

3. **Getting Connection Info**:
   ```go
   connInfo, err := client.GetConnection(ctx, connectionID)
   fmt.Printf("Connected at: %v\n", connInfo.ConnectedAt)
   fmt.Printf("Is Active: %v\n", connInfo.IsActive())
   ```

4. **Error Handling**:
   ```go
   if errors.Is(err, streamer.ErrConnectionGone) {
       // Handle disconnected connection
   }
   ```

## Supported Actions

### Echo
```json
{
  "action": "echo",
  "data": "Hello, World!"
}
```
Echoes back the received data to the sender.

### Broadcast
```json
{
  "action": "broadcast",
  "data": {
    "text": "Hello everyone!",
    "connectionIds": ["conn-1", "conn-2"]
  }
}
```
Broadcasts a message to specified connections (or sender if not specified).

### Get Info
```json
{
  "action": "getInfo",
  "data": {}
}
```
Retrieves and returns connection information including:
- Connection ID
- Connected timestamp
- Last active timestamp
- Source IP
- User agent
- Active status
- Connection age
- Idle time

## Deployment

1. Build the Lambda function:
```bash
GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
zip function.zip bootstrap
```

2. Deploy using AWS CDK (recommended) or SAM/CloudFormation

3. Configure API Gateway WebSocket API with routes:
   - `$connect`
   - `$disconnect`
   - `message` (with route selection expression: `$request.body.action`)
   - `$default`

## Testing Locally

You can test the handlers using the Lift testing utilities:

```go
package main

import (
    "testing"
    "github.com/pay-theory/lift/pkg/testing"
)

func TestEchoHandler(t *testing.T) {
    // Create a test WebSocket event
    event := testing.NewWebSocketEvent("message", map[string]any{
        "action": "echo",
        "data":   "test message",
    })

    // Test the handler
    // ... your test code
}
```

## Error Handling

The example demonstrates proper error handling for all streamer error types:

- **GoneError (410)**: Connection no longer exists
- **ForbiddenError (403)**: Operation not permitted
- **PayloadTooLargeError (413)**: Message exceeds size limit (128KB)
- **ThrottlingError (429)**: Rate limit exceeded
- **InternalServerError (500)**: AWS service error

## Comparison with WebSocketContext

This example uses the lower-level `streamer.Client` directly, which provides:
- More control over connection management
- Better error typing and handling
- Access to connection metadata
- Ability to test with mocks (`testing.NewStreamerClientMock()`)

For simpler use cases, you can use `lift.WebSocketContext` which wraps streamer internally:

```go
wsCtx, _ := ctx.AsWebSocket()
wsCtx.SendMessage(data)  // Uses streamer internally
```

## Security Considerations

1. **Authentication**: Always authenticate connections using query parameters or custom authorizers
2. **Authorization**: Verify permissions before allowing broadcast to other connections
3. **Rate Limiting**: Implement rate limiting to prevent abuse
4. **Input Validation**: Validate all incoming messages
5. **Connection Tracking**: Store connection metadata in DynamoDB for authorization checks

## Related Examples

- [websocket-demo](../websocket-demo/) - Basic WebSocket example using WebSocketContext
- [websocket-enhanced](../websocket-enhanced/) - Advanced WebSocket patterns
- [multi-service-demo](../multi-service-demo/) - Testing WebSocket handlers

## Streamer Package Features

The `pkg/streamer` package provides:

- **Unified Interface**: Clean abstraction over AWS API Gateway Management API
- **Structured Errors**: Type-safe error handling with sentinel errors
- **Connection Metadata**: Rich information about connections (age, idle time, etc.)
- **Testing Support**: Full mock implementation for unit tests
- **AWS SDK v2**: Built on modern AWS SDK with proper context support

