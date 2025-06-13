# Direct Answers to Middleware Configuration Questions

## 1. JWT Middleware Configuration

### ✅ How to properly configure it for RS256 with a public key

```go
jwtConfig := security.JWTConfig{
    SigningMethod:   "RS256",                    // Field name is SigningMethod
    PublicKeyPath:   "/path/to/public.key",      // Field name is PublicKeyPath
    Issuer:          "your-issuer",              // Field name is Issuer
    Audience:        []string{"your-audience"},  // Field name is Audience (slice)
    MaxAge:          time.Hour,
    RequireTenantID: true,
}

app.Use(middleware.JWT(jwtConfig))
```

### ✅ Specific field names for issuer validation

- **Issuer**: `Issuer` (string)
- **Audience**: `Audience` ([]string slice)
- **Signing Method**: `SigningMethod` (string: "RS256" or "HS256")
- **Public Key**: `PublicKeyPath` (string path to key file)
- **Secret Key**: `SecretKey` (string, for HS256 only)

### ✅ How JWT claims get stored in the context

Claims are automatically parsed and stored. Access them via:

```go
func handler(ctx *lift.Context) error {
    // Direct access methods
    userID := ctx.UserID()      // From "sub" claim
    tenantID := ctx.TenantID()  // From "tenant_id" claim
    
    // Security context access
    secCtx := lift.WithSecurity(ctx)
    principal := secCtx.GetPrincipal()
    
    roles := principal.Roles    // From "roles" claim
    scopes := principal.Scopes  // From "scopes" claim
    
    return ctx.JSON(map[string]interface{}{
        "user_id":   userID,
        "tenant_id": tenantID,
        "roles":     roles,
        "scopes":    scopes,
    })
}
```

## 2. Observability Middleware Setup

### ✅ How to create observability.StructuredLogger

**Option A: Zap Logger (Recommended for development)**
```go
import "github.com/pay-theory/lift/pkg/observability/zap"

factory := zap.NewZapLoggerFactory()
logger, err := factory.CreateConsoleLogger(observability.LoggerConfig{
    Level:  "info",
    Format: "json", // or "console"
})
```

**Option B: CloudWatch Logger (Production)**
```go
import "github.com/pay-theory/lift/pkg/observability/cloudwatch"

config := observability.LoggerConfig{
    LogGroup:      "/aws/lambda/my-function",
    LogStream:     "my-stream",
    BatchSize:     25,
    FlushInterval: 5 * time.Second,
}

client := cloudwatchlogs.NewFromConfig(awsConfig)
logger, err := cloudwatch.NewCloudWatchLogger(config, client)
```

**Option C: Test/NoOp Logger**
```go
factory := zap.NewZapLoggerFactory()
logger := factory.CreateTestLogger()
// or
logger := factory.CreateNoOpLogger()
```

### ✅ How to create observability.MetricsCollector

**Option A: CloudWatch Metrics**
```go
import "github.com/pay-theory/lift/pkg/observability/cloudwatch"

metricsConfig := cloudwatch.CloudWatchMetricsConfig{
    Namespace:     "MyApp/Production",
    BufferSize:    1000,
    FlushInterval: 60 * time.Second,
    Dimensions: map[string]string{
        "Environment": "production",
        "Service":     "my-service",
    },
}

client := cloudwatch.NewFromConfig(awsConfig)
metrics := cloudwatch.NewCloudWatchMetrics(client, metricsConfig)
```

**Option B: Test/NoOp Metrics**
```go
metrics := &lift.NoOpMetrics{}
```

### ✅ Factory functions exist

Yes! Factory functions are available:

- **Zap Logger Factory**: `zap.NewZapLoggerFactory()`
- **CloudWatch Factory**: Direct constructors like `cloudwatch.NewCloudWatchLogger()`
- **Test Factories**: `factory.CreateTestLogger()`, `factory.CreateNoOpLogger()`

## 3. WebSocket-Specific Middleware

### ✅ WebSocket Metrics Configuration

**Simple and straightforward - no complex config structure:**

```go
// Just pass your metrics collector
app.Use(middleware.WebSocketMetrics(metricsCollector))
```

The middleware automatically tracks:
- Connection counts (`websocket.connections.new`, `websocket.connections.active`)
- Message counts (`websocket.messages`)
- Latency metrics (`websocket.latency`)
- Error rates (`websocket.errors`)

### ✅ WebSocket Auth Configuration Structure

```go
wsAuthConfig := middleware.WebSocketAuthConfig{
    JWTConfig: security.JWTConfig{
        SigningMethod: "RS256",
        PublicKeyPath: "/path/to/public.key",
        Issuer:        "your-issuer",
        Audience:      []string{"your-audience"},
    },
    TokenExtractor: func(ctx *lift.Context) string {
        // Default extracts from query params: Authorization, authorization, token
        token := ctx.Query("Authorization")
        if token == "" {
            token = ctx.Query("token")
        }
        return strings.TrimPrefix(token, "Bearer ")
    },
    OnError: func(ctx *lift.Context, err error) error {
        return ctx.Status(401).JSON(map[string]string{
            "error": "Authentication failed: " + err.Error(),
        })
    },
    SkipRoutes: []string{"$connect"}, // Routes to skip auth
}

app.Use(middleware.WebSocketAuth(wsAuthConfig))
```

## 📚 Working Examples in the Codebase

### 1. JWT Examples
- `examples/jwt-auth/main.go` - Complete JWT setup
- `examples/jwt-auth-demo/main.go` - JWT with middleware
- `pkg/middleware/auth_test.go` - Test examples

### 2. Observability Examples  
- `examples/observability-demo/main.go` - **Complete observability setup**
- Shows Zap logger, CloudWatch logger, multi-tenant logging
- Performance testing and stats collection

### 3. WebSocket Examples
- `examples/websocket-enhanced/main.go` - Enhanced WebSocket with middleware
- `docs/development/notes/2025-06-13-07_48_28-websocket-integration-answers-v2.md` - WebSocket patterns

## 🎯 Quick Start Template

```go
package main

import (
    "os"
    "time"
    
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/lift/pkg/security"
    "github.com/pay-theory/lift/pkg/observability"
    "github.com/pay-theory/lift/pkg/observability/zap"
)

func main() {
    // 1. Create logger
    factory := zap.NewZapLoggerFactory()
    logger, _ := factory.CreateConsoleLogger(observability.LoggerConfig{
        Level:  "info",
        Format: "json",
    })

    // 2. Create metrics
    metrics := &lift.NoOpMetrics{} // Use CloudWatch in production

    // 3. Create app
    app := lift.New()

    // 4. JWT Authentication
    app.Use(middleware.JWT(security.JWTConfig{
        SigningMethod:   "RS256",
        PublicKeyPath:   os.Getenv("JWT_PUBLIC_KEY_PATH"),
        Issuer:          os.Getenv("JWT_ISSUER"),
        Audience:        []string{"lift-api"},
        MaxAge:          time.Hour,
        RequireTenantID: true,
    }))

    // 5. Observability
    app.Use(middleware.ObservabilityMiddleware(middleware.ObservabilityConfig{
        Logger:  logger,
        Metrics: metrics,
    }))

    // 6. WebSocket middleware (if needed)
    app.Use(middleware.WebSocketMetrics(metrics))

    // 7. Routes
    app.GET("/protected", func(ctx *lift.Context) error {
        return ctx.JSON(map[string]interface{}{
            "user_id":   ctx.UserID(),
            "tenant_id": ctx.TenantID(),
            "message":   "Success!",
        })
    })

    lambda.Start(app.HandleRequest)
}
```

## ✅ Summary

1. **JWT Config**: Use `security.JWTConfig` with `SigningMethod`, `PublicKeyPath`, `Issuer`, `Audience`
2. **Logger**: Use `zap.NewZapLoggerFactory()` or `cloudwatch.NewCloudWatchLogger()`
3. **Metrics**: Use `cloudwatch.NewCloudWatchMetrics()` or `&lift.NoOpMetrics{}`
4. **WebSocket Metrics**: Simple `middleware.WebSocketMetrics(collector)` - no complex config
5. **Examples**: Check `examples/observability-demo/` for complete working code

All the middleware configurations are well-documented and have working examples in the codebase! 