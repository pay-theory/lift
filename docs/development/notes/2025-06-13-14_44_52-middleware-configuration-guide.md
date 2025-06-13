# Middleware Configuration Guide

## JWT Middleware Configuration

### 1. JWT Configuration Structure

The JWT middleware uses `security.JWTConfig` structure with the following key fields:

```go
type JWTConfig struct {
    // Signing configuration
    SigningMethod  string `json:"signing_method"` // "RS256" or "HS256"
    PublicKeyPath  string `json:"public_key_path"`
    PrivateKeyPath string `json:"private_key_path"`
    SecretKey      string `json:"secret_key,omitempty"` // For HS256

    // Validation settings
    Issuer   string        `json:"issuer"`
    Audience []string      `json:"audience"`
    MaxAge   time.Duration `json:"max_age"`

    // Multi-tenant settings
    RequireTenantID bool                        `json:"require_tenant_id"`
    ValidateTenant  func(tenantID string) error `json:"-"` // Custom validation function

    // Key rotation
    KeyRotation    bool          `json:"key_rotation"`
    RotationPeriod time.Duration `json:"rotation_period"`
}
```

### 2. RS256 Configuration with Public Key

For RS256 with a public key, configure as follows:

```go
jwtConfig := security.JWTConfig{
    SigningMethod:   "RS256",
    PublicKeyPath:   "/path/to/public.key", // Path to RSA public key file
    Issuer:          "your-issuer",
    Audience:        []string{"your-audience"},
    MaxAge:          time.Hour,
    RequireTenantID: true,
    ValidateTenant: func(tenantID string) error {
        // Custom tenant validation logic
        validTenants := []string{"tenant1", "tenant2", "premium-tenant"}
        for _, valid := range validTenants {
            if tenantID == valid {
                return nil
            }
        }
        return security.NewSecurityError("INVALID_TENANT", "Tenant not found")
    },
}

// Apply JWT middleware
app.Use(middleware.JWT(jwtConfig))
```

### 3. JWT Claims in Context

JWT claims are automatically stored in the context and can be accessed via:

```go
func handler(ctx *lift.Context) error {
    // Get security context
    secCtx := lift.WithSecurity(ctx)
    principal := secCtx.GetPrincipal()
    
    // Access claims
    userID := principal.UserID      // From "sub" claim
    tenantID := principal.TenantID  // From "tenant_id" claim
    roles := principal.Roles        // From "roles" claim
    scopes := principal.Scopes      // From "scopes" claim
    
    // Alternative: Direct access from context
    userID := ctx.UserID()
    tenantID := ctx.TenantID()
    
    return ctx.JSON(map[string]interface{}{
        "user_id":   userID,
        "tenant_id": tenantID,
        "roles":     roles,
        "scopes":    scopes,
    })
}
```

### 4. Expected JWT Token Format

```json
{
  "sub": "user123",                    // User ID (required)
  "iss": "your-issuer",                // Issuer (must match config)
  "aud": ["your-audience"],            // Audience (must match config)
  "exp": 1640995200,                   // Expiration timestamp
  "iat": 1640991600,                   // Issued at timestamp
  "tenant_id": "tenant1",              // Tenant ID (required if RequireTenantID is true)
  "account_id": "account123",          // Account ID (optional)
  "roles": ["user", "manager"],        // User roles for RBAC
  "scopes": ["payments:read", "users:read"] // User scopes for permissions
}
```

## Observability Middleware Configuration

### 1. Creating StructuredLogger

You have several options to create a `StructuredLogger`:

#### Option A: Using Zap Logger Factory

```go
import (
    "github.com/pay-theory/lift/pkg/observability"
    "github.com/pay-theory/lift/pkg/observability/zap"
)

// Create logger factory
factory := zap.NewZapLoggerFactory()

// Create console logger for development
loggerConfig := observability.LoggerConfig{
    Level:        "debug",
    Format:       "console", // or "json"
    EnableCaller: true,
    EnableStack:  true,
}

logger, err := factory.CreateConsoleLogger(loggerConfig)
if err != nil {
    panic(err)
}
```

#### Option B: Using CloudWatch Logger

```go
import (
    "github.com/pay-theory/lift/pkg/observability/cloudwatch"
)

// Create CloudWatch logger
loggerConfig := observability.LoggerConfig{
    Level:         "info",
    Format:        "json",
    LogGroup:      "/aws/lambda/my-function",
    LogStream:     "my-stream",
    BatchSize:     25,
    FlushInterval: 5 * time.Second,
    BufferSize:    100,
}

// You need to provide a CloudWatch client
client := cloudwatchlogs.NewFromConfig(awsConfig)
logger, err := cloudwatch.NewCloudWatchLogger(loggerConfig, client)
if err != nil {
    panic(err)
}
```

#### Option C: Test/NoOp Logger

```go
// For testing
factory := zap.NewZapLoggerFactory()
logger := factory.CreateTestLogger()

// Or no-op logger
logger := factory.CreateNoOpLogger()
```

### 2. Creating MetricsCollector

#### Option A: Using CloudWatch Metrics

```go
import (
    "github.com/pay-theory/lift/pkg/observability/cloudwatch"
)

// Create CloudWatch metrics collector
metricsConfig := cloudwatch.CloudWatchMetricsConfig{
    Namespace:     "MyApp/Production",
    BufferSize:    1000,
    FlushSize:     20,
    FlushInterval: 60 * time.Second,
    Dimensions: map[string]string{
        "Environment": "production",
        "Service":     "my-service",
    },
}

client := cloudwatch.NewFromConfig(awsConfig)
metrics := cloudwatch.NewCloudWatchMetrics(client, metricsConfig)
```

#### Option B: Test/NoOp Metrics

```go
// For testing
metrics := &lift.NoOpMetrics{}
```

### 3. Observability Middleware Configuration

```go
import (
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/lift/pkg/observability"
)

// Basic observability middleware
observabilityConfig := middleware.ObservabilityConfig{
    Logger:  logger,  // Created above
    Metrics: metrics, // Created above
    OperationNameFunc: func(ctx *lift.Context) string {
        return fmt.Sprintf("%s_%s", ctx.Request.Method, ctx.Request.Path)
    },
}

app.Use(middleware.ObservabilityMiddleware(observabilityConfig))
```

### 4. Enhanced Observability Configuration

For more advanced features:

```go
enhancedConfig := middleware.EnhancedObservabilityConfig{
    // Core components
    Logger:  logger,
    Metrics: metrics,
    Tracer:  xrayTracer, // Optional X-Ray tracer

    // Feature flags
    EnableLogging: true,
    EnableMetrics: true,
    EnableTracing: true,

    // Custom extractors
    OperationNameFunc: func(ctx *lift.Context) string {
        return fmt.Sprintf("%s_%s", ctx.Request.Method, ctx.Request.Path)
    },
    TenantIDFunc: func(ctx *lift.Context) string {
        return ctx.TenantID()
    },
    UserIDFunc: func(ctx *lift.Context) string {
        return ctx.UserID()
    },

    // Performance settings
    LogRequestBody:  false, // Set to true for debugging
    LogResponseBody: false,
    MaxBodyLogSize:  1024,
    SampleRate:      1.0, // Sample all requests

    // Default tags for all metrics
    DefaultTags: map[string]string{
        "service":     "my-service",
        "environment": "production",
    },
}

app.Use(middleware.EnhancedObservabilityMiddleware(enhancedConfig))
```

## WebSocket-Specific Middleware

### 1. WebSocket Metrics Configuration

The WebSocket metrics middleware is straightforward:

```go
// Simple WebSocket metrics
app.Use(middleware.WebSocketMetrics(metricsCollector))
```

There's no complex configuration structure - it uses the provided `MetricsCollector` and automatically tracks:
- Connection counts
- Message counts
- Latency metrics
- Error rates
- Connection lifecycle events

### 2. WebSocket Authentication Configuration

```go
wsAuthConfig := middleware.WebSocketAuthConfig{
    JWTConfig: security.JWTConfig{
        SigningMethod: "RS256",
        PublicKeyPath: "/path/to/public.key",
        Issuer:        "your-issuer",
        Audience:      []string{"your-audience"},
    },
    TokenExtractor: func(ctx *lift.Context) string {
        // Custom token extraction logic
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
    SkipRoutes: []string{"$connect"}, // Skip auth for certain routes
}

app.Use(middleware.WebSocketAuth(wsAuthConfig))
```

## Complete Working Example

Here's a complete example showing all middleware configurations:

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
    // Create logger
    factory := zap.NewZapLoggerFactory()
    logger, err := factory.CreateConsoleLogger(observability.LoggerConfig{
        Level:  "info",
        Format: "json",
    })
    if err != nil {
        panic(err)
    }

    // Create metrics (using NoOp for demo)
    metrics := &lift.NoOpMetrics{}

    // Create app
    app := lift.New()

    // JWT Authentication
    jwtConfig := security.JWTConfig{
        SigningMethod:   "RS256",
        PublicKeyPath:   os.Getenv("JWT_PUBLIC_KEY_PATH"),
        Issuer:          os.Getenv("JWT_ISSUER"),
        Audience:        []string{"lift-api"},
        MaxAge:          time.Hour,
        RequireTenantID: true,
    }
    app.Use(middleware.JWT(jwtConfig))

    // Observability
    observabilityConfig := middleware.ObservabilityConfig{
        Logger:  logger,
        Metrics: metrics,
    }
    app.Use(middleware.ObservabilityMiddleware(observabilityConfig))

    // WebSocket-specific middleware (if using WebSocket)
    if os.Getenv("ENABLE_WEBSOCKET") == "true" {
        wsAuthConfig := middleware.WebSocketAuthConfig{
            JWTConfig: jwtConfig,
        }
        app.Use(middleware.WebSocketAuth(wsAuthConfig))
        app.Use(middleware.WebSocketMetrics(metrics))
    }

    // Register routes
    app.GET("/health", func(ctx *lift.Context) error {
        return ctx.OK(map[string]string{"status": "healthy"})
    })

    app.GET("/protected", func(ctx *lift.Context) error {
        return ctx.JSON(map[string]interface{}{
            "user_id":   ctx.UserID(),
            "tenant_id": ctx.TenantID(),
            "message":   "This is a protected endpoint",
        })
    })

    // Start Lambda
    lambda.Start(app.HandleRequest)
}
```

## Key Points to Remember

1. **JWT Configuration**: Use `security.JWTConfig` with proper field names (`SigningMethod`, `PublicKeyPath`, `Issuer`, `Audience`)

2. **Claims Access**: JWT claims are automatically parsed and available via `ctx.UserID()`, `ctx.TenantID()`, or through the security context

3. **Logger Creation**: Use factory functions from `observability/zap` or `observability/cloudwatch` packages

4. **Metrics Creation**: Use CloudWatch metrics or NoOp metrics for testing

5. **WebSocket Metrics**: Simple configuration - just pass the metrics collector

6. **Middleware Order**: Apply middleware in the correct order (auth first, then observability, then WebSocket-specific)

7. **Environment Variables**: Use environment variables for sensitive configuration like key paths and issuers

This guide should provide you with all the information needed to properly configure the middleware stack in your Lift application. 