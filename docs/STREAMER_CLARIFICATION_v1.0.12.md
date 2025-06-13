# Clarification: Lift v1.0.12 WebSocket API

## For the Streamer Team

I understand there's confusion about the v1.0.12 API. Let me clarify what's actually available in the release.

## ✅ What IS in v1.0.12

The WebSocket functionality described in the migration guide IS included in v1.0.12:

1. **`app.WebSocket()` method** - Available ✅
2. **`WithWebSocketSupport()` option** - Available ✅
3. **Automatic connection management** - Available ✅
4. **WebSocket-specific routing** - Available ✅

## How to Verify

You can verify the WebSocket methods are available:

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    // This WILL compile with v1.0.12
    app := lift.New(lift.WithWebSocketSupport())
    
    // This WILL work
    app.WebSocket("$connect", handleConnect)
    app.WebSocket("message", handleMessage)
    app.WebSocket("$disconnect", handleDisconnect)
}
```

## Common Issues and Solutions

### Issue 1: "undefined: lift.WithWebSocketSupport"

**Cause**: Not using v1.0.12
**Solution**: 
```bash
go get github.com/pay-theory/lift@v1.0.12
go mod tidy
```

### Issue 2: "app.WebSocket undefined"

**Cause**: The WebSocket methods are in a separate file that might not be imported
**Solution**: Make sure you have the complete v1.0.12 package:
```bash
go clean -modcache
go get github.com/pay-theory/lift@v1.0.12
```

### Issue 3: Import errors

**Cause**: Missing dependencies
**Solution**: The WebSocket features require these dependencies:
```bash
go get github.com/aws/aws-lambda-go/events
go get github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi
go get github.com/aws/aws-sdk-go-v2/service/dynamodb
```

## Complete Working Example

Here's a minimal working example with v1.0.12:

```go
package main

import (
    "context"
    "log"
    
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    // Create app with WebSocket support
    app := lift.New(lift.WithWebSocketSupport())
    
    // Register WebSocket routes
    app.WebSocket("$connect", func(ctx *lift.Context) error {
        log.Printf("Connection established: %s", ctx.Request.Metadata["connectionId"])
        return ctx.Status(200).JSON(map[string]string{
            "message": "Connected successfully",
        })
    })
    
    app.WebSocket("$disconnect", func(ctx *lift.Context) error {
        log.Printf("Connection closed: %s", ctx.Request.Metadata["connectionId"])
        return nil
    })
    
    app.WebSocket("message", func(ctx *lift.Context) error {
        var msg map[string]interface{}
        if err := ctx.Bind(&msg); err != nil {
            return ctx.Status(400).JSON(map[string]string{
                "error": "Invalid message format",
            })
        }
        
        return ctx.Status(200).JSON(map[string]string{
            "echo": msg["text"].(string),
        })
    })
    
    // Start the Lambda handler
    lambda.Start(app.WebSocketHandler())
}
```

## Deployment Configuration

Make sure your `serverless.yml` or SAM template includes:

```yaml
functions:
  websocket:
    handler: bin/websocket
    events:
      - websocket:
          route: $connect
      - websocket:
          route: $disconnect
      - websocket:
          route: $default
```

## Quick Test

To quickly test if v1.0.12 is working:

```go
package main

import (
    "fmt"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New(lift.WithWebSocketSupport())
    fmt.Println("WebSocket support enabled!")
    
    // This should print the function
    app.WebSocket("test", func(ctx *lift.Context) error {
        return nil
    })
    fmt.Println("WebSocket route registered!")
}
```

## If You're Still Having Issues

1. **Check your go.mod**:
   ```
   require github.com/pay-theory/lift v1.0.12
   ```

2. **Clear your module cache**:
   ```bash
   go clean -modcache
   go mod download
   ```

3. **Verify the package contents**:
   ```bash
   go list -m -versions github.com/pay-theory/lift
   ```

4. **Check for conflicting versions**:
   ```bash
   go mod graph | grep lift
   ```

## Contact for Help

If you're still experiencing issues:
1. Share your `go.mod` file
2. Share the exact error messages
3. Share a minimal code example that's failing

The WebSocket functionality IS in v1.0.12 and should work as described in the migration guide. 