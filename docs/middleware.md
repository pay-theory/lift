# Middleware

Middleware in Lift provides a powerful way to add cross-cutting functionality to your Lambda handlers. This guide covers built-in middleware, creating custom middleware, and best practices.

## Understanding Middleware

Middleware wraps your handlers, allowing you to:

- Process requests before they reach your handler
- Modify responses after your handler executes
- Short-circuit request processing
- Add common functionality across multiple handlers

### Middleware Flow

```
Request → Middleware 1 → Middleware 2 → Middleware 3 → Handler
            ↓               ↓               ↓            ↓
Response ← Middleware 1 ← Middleware 2 ← Middleware 3 ← Handler
```

## Using Middleware

### Global Middleware

Apply middleware to all routes:

```go
app := lift.New()

// Order matters! First added = first executed
app.Use(middleware.Logger())        // Logs all requests
app.Use(middleware.Recover())       // Catches panics
app.Use(middleware.RequestID())     // Adds request IDs
app.Use(middleware.CORS())          // Handles CORS

// All routes will use the middleware above
app.GET("/users", getUsers)
app.POST("/users", createUser)
```

### Route-Specific Middleware

Apply middleware to specific routes:

```go
// Method 1: Inline middleware
app.GET("/public", publicHandler)
app.GET("/private", middleware.Auth(), privateHandler)

// Method 2: Group routes
authGroup := app.Group("/api/v1", middleware.Auth())
authGroup.GET("/users", getUsers)
authGroup.POST("/users", createUser)

// Method 3: Compose handlers
protectedHandler := middleware.Chain(
    middleware.Auth(),
    middleware.RateLimit(),
    actualHandler,
)
app.GET("/protected", protectedHandler)
```

## Built-in Middleware

Lift comes with production-ready middleware out of the box.

### Logger Middleware

Structured logging for all requests:

```go
app.Use(middleware.Logger())

// Custom configuration
app.Use(middleware.Logger(middleware.LoggerConfig{
    Level:          "info",
    SkipPaths:      []string{"/health"},
    SensitiveHeaders: []string{"Authorization", "X-API-Key"},
    LogLatency:     true,
    LogRequestBody: false, // Don't log request bodies
}))
```

Log output includes:
- Request ID
- Method and path
- Status code
- Latency
- Error details (if any)

### Recovery Middleware

Catches panics and returns 500 errors:

```go
app.Use(middleware.Recover())

// Custom panic handler
app.Use(middleware.Recover(middleware.RecoverConfig{
    EnableStackTrace: true,
    LogPanics:       true,
    PanicHandler: func(ctx *lift.Context, err interface{}) error {
        // Custom panic handling
        ctx.Logger.Error("Panic recovered", map[string]interface{}{
            "error": err,
            "stack": debug.Stack(),
        })
        return lift.InternalError("Internal server error")
    },
}))
```

### Request ID Middleware

Generates unique request IDs for tracing:

```go
app.Use(middleware.RequestID())

// Custom configuration
app.Use(middleware.RequestID(middleware.RequestIDConfig{
    Generator: func() string {
        return "req_" + uuid.New().String()
    },
    HeaderName: "X-Request-ID",
}))

// Access in handlers
func handler(ctx *lift.Context) error {
    requestID := ctx.RequestID()
    ctx.Logger.Info("Processing request", map[string]interface{}{
        "request_id": requestID,
    })
    return ctx.JSON(response)
}
```

### CORS Middleware

Handle Cross-Origin Resource Sharing:

```go
// Default CORS (allows all origins)
app.Use(middleware.CORS())

// Custom CORS configuration
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://example.com", "https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    ExposeHeaders:    []string{"X-Total-Count"},
    AllowCredentials: true,
    MaxAge:          86400, // 24 hours
}))

// Dynamic CORS
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOriginFunc: func(origin string) bool {
        return strings.HasSuffix(origin, ".example.com")
    },
}))
```

### JWT Authentication

JWT token validation and parsing:

```go
// Basic JWT middleware
app.Use(middleware.JWT(middleware.JWTConfig{
    SecretKey: []byte(os.Getenv("JWT_SECRET")),
}))

// Advanced configuration
app.Use(middleware.JWT(middleware.JWTConfig{
    // Use public key for RS256
    PublicKey: publicKey,
    
    // Custom token extraction
    TokenExtractor: func(ctx *lift.Context) (string, error) {
        // Try Authorization header first
        if auth := ctx.Header("Authorization"); auth != "" {
            return strings.TrimPrefix(auth, "Bearer "), nil
        }
        // Fall back to cookie
        return ctx.Cookie("token")
    },
    
    // Skip certain paths
    SkipPaths: []string{"/login", "/register", "/health"},
    
    // Custom claims validation
    ValidateFunc: func(claims jwt.MapClaims) error {
        if exp, ok := claims["exp"].(float64); ok {
            if time.Now().Unix() > int64(exp) {
                return errors.New("token expired")
            }
        }
        return nil
    },
    
    // Error handler
    ErrorHandler: func(ctx *lift.Context, err error) error {
        return lift.Unauthorized("Invalid token: " + err.Error())
    },
}))

// Access claims in handlers
func handler(ctx *lift.Context) error {
    claims := ctx.Get("claims").(jwt.MapClaims)
    userID := claims["sub"].(string)
    
    return ctx.JSON(map[string]interface{}{
        "user_id": userID,
        "claims":  claims,
    })
}
```

### Rate Limiting

Protect against abuse with rate limiting:

```go
// Basic rate limiting
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    WindowSize:  time.Minute,
    MaxRequests: 100,
}))

// Advanced configuration
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    WindowSize:  time.Minute,
    MaxRequests: 100,
    
    // Key function for rate limiting
    KeyFunc: func(ctx *lift.Context) string {
        // Rate limit by tenant
        if tenantID := ctx.TenantID(); tenantID != "" {
            return "tenant:" + tenantID
        }
        // Fall back to IP
        return "ip:" + ctx.ClientIP()
    },
    
    // Store implementation (default: in-memory)
    Store: dynamoRateLimitStore,
    
    // Custom exceeded handler
    ExceededHandler: func(ctx *lift.Context) error {
        return ctx.Status(429).JSON(map[string]interface{}{
            "error": "Rate limit exceeded",
            "retry_after": 60,
        })
    },
    
    // Skip certain paths
    SkipPaths: []string{"/health"},
}))
```

### Compression

Compress responses to save bandwidth:

```go
app.Use(middleware.Compress())

// Custom configuration
app.Use(middleware.Compress(middleware.CompressConfig{
    Level:            gzip.BestSpeed,
    MinContentLength: 1024, // Only compress if > 1KB
    SkipPaths:        []string{"/metrics"},
    ContentTypes: []string{
        "application/json",
        "text/html",
        "application/javascript",
    },
}))
```

### Security Headers

Add security headers to responses:

```go
app.Use(middleware.Security())

// Custom security headers
app.Use(middleware.Security(middleware.SecurityConfig{
    XSSProtection:         "1; mode=block",
    ContentTypeNosniff:    "nosniff",
    XFrameOptions:         "DENY",
    HSTSMaxAge:           31536000,
    HSTSIncludeSubdomains: true,
    CSPPolicy:            "default-src 'self'",
    ReferrerPolicy:       "same-origin",
}))
```

### Timeout Middleware

Prevent long-running requests:

```go
app.Use(middleware.Timeout(25 * time.Second))

// Custom timeout handling
app.Use(middleware.Timeout(middleware.TimeoutConfig{
    Timeout: 25 * time.Second,
    ErrorHandler: func(ctx *lift.Context) error {
        return lift.ServiceUnavailable("Request timeout")
    },
}))
```

## Creating Custom Middleware

### Basic Middleware Pattern

```go
func MyMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Before handler
            ctx.Logger.Info("Before handler")
            
            // Call next handler
            err := next.Handle(ctx)
            
            // After handler
            ctx.Logger.Info("After handler")
            
            return err
        })
    }
}

// Use it
app.Use(MyMiddleware())
```

### Middleware with Configuration

```go
type MetricsConfig struct {
    Namespace  string
    Enabled    bool
    SampleRate float64
}

func Metrics(config MetricsConfig) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            if !config.Enabled {
                return next.Handle(ctx)
            }
            
            start := time.Now()
            
            // Execute handler
            err := next.Handle(ctx)
            
            // Record metrics
            duration := time.Since(start)
            status := ctx.Response.StatusCode
            
            if rand.Float64() < config.SampleRate {
                ctx.Metrics.Record(config.Namespace+".request", map[string]interface{}{
                    "duration": duration.Milliseconds(),
                    "status":   status,
                    "method":   ctx.Request.Method,
                    "path":     ctx.Request.Path,
                })
            }
            
            return err
        })
    }
}

// Use with configuration
app.Use(Metrics(MetricsConfig{
    Namespace:  "api",
    Enabled:    true,
    SampleRate: 0.1, // Sample 10% of requests
}))
```

### Conditional Middleware

```go
func ConditionalMiddleware(condition func(*lift.Context) bool, mw lift.Middleware) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            if condition(ctx) {
                // Apply middleware
                return mw(next).Handle(ctx)
            }
            // Skip middleware
            return next.Handle(ctx)
        })
    }
}

// Example: Only log non-health check requests
app.Use(ConditionalMiddleware(
    func(ctx *lift.Context) bool {
        return ctx.Request.Path != "/health"
    },
    middleware.Logger(),
))
```

### State Sharing Middleware

```go
func DatabaseMiddleware(db *sql.DB) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Add database to context
            ctx.Set("db", db)
            
            // Optional: Add helper methods
            ctx.Set("getDB", func() *sql.DB {
                return db
            })
            
            return next.Handle(ctx)
        })
    }
}

// Use in handlers
func handler(ctx *lift.Context) error {
    db := ctx.Get("db").(*sql.DB)
    // Use database...
    return ctx.JSON(result)
}
```

### Error Handling Middleware

```go
func ErrorHandler() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Log error
            ctx.Logger.Error("Handler error", map[string]interface{}{
                "error": err.Error(),
                "path":  ctx.Request.Path,
            })
            
            // Format error response
            switch e := err.(type) {
            case lift.HTTPError:
                return ctx.Status(e.Status()).JSON(map[string]interface{}{
                    "error": map[string]interface{}{
                        "message": e.Error(),
                        "code":    e.Code(),
                    },
                })
            default:
                // Hide internal errors in production
                if ctx.Environment() == "production" {
                    return ctx.Status(500).JSON(map[string]interface{}{
                        "error": map[string]interface{}{
                            "message": "Internal server error",
                            "code":    "INTERNAL_ERROR",
                        },
                    })
                }
                return ctx.Status(500).JSON(map[string]interface{}{
                    "error": map[string]interface{}{
                        "message": err.Error(),
                        "code":    "INTERNAL_ERROR",
                    },
                })
            }
        })
    }
}
```

## Advanced Middleware Patterns

### Middleware Chains

```go
func Chain(middlewares ...lift.Middleware) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            next = middlewares[i](next)
        }
        return next
    }
}

// Use chains for grouped functionality
authChain := Chain(
    middleware.RateLimit(),
    middleware.JWT(),
    middleware.RequireRole("admin"),
)

app.GET("/admin/users", authChain(adminHandler))
```

### Async Middleware

```go
func AsyncLogger() lift.Middleware {
    logChan := make(chan LogEntry, 1000)
    
    // Start async worker
    go func() {
        for entry := range logChan {
            // Process logs asynchronously
            sendToLogService(entry)
        }
    }()
    
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            start := time.Now()
            
            err := next.Handle(ctx)
            
            // Log asynchronously
            select {
            case logChan <- LogEntry{
                Time:     start,
                Duration: time.Since(start),
                Path:     ctx.Request.Path,
                Status:   ctx.Response.StatusCode,
            }:
            default:
                // Channel full, skip logging
            }
            
            return err
        })
    }
}
```

### Circuit Breaker Middleware

```go
func CircuitBreaker(threshold int, timeout time.Duration) lift.Middleware {
    var (
        failures int
        lastFail time.Time
        mu       sync.Mutex
    )
    
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            mu.Lock()
            if failures >= threshold && time.Since(lastFail) < timeout {
                mu.Unlock()
                return lift.ServiceUnavailable("Circuit breaker open")
            }
            mu.Unlock()
            
            err := next.Handle(ctx)
            
            mu.Lock()
            if err != nil {
                failures++
                lastFail = time.Now()
            } else {
                failures = 0
            }
            mu.Unlock()
            
            return err
        })
    }
}
```

### Retry Middleware

```go
func Retry(maxAttempts int, backoff time.Duration) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            var err error
            
            for attempt := 1; attempt <= maxAttempts; attempt++ {
                err = next.Handle(ctx)
                
                // Success or non-retryable error
                if err == nil || !isRetryable(err) {
                    return err
                }
                
                // Last attempt
                if attempt == maxAttempts {
                    break
                }
                
                // Exponential backoff
                delay := backoff * time.Duration(math.Pow(2, float64(attempt-1)))
                ctx.Logger.Warn("Retrying request", map[string]interface{}{
                    "attempt": attempt,
                    "delay":   delay,
                    "error":   err.Error(),
                })
                
                time.Sleep(delay)
            }
            
            return err
        })
    }
}

func isRetryable(err error) bool {
    // Define which errors are retryable
    var httpErr lift.HTTPError
    if errors.As(err, &httpErr) {
        return httpErr.Status() >= 500
    }
    return false
}
```

## Middleware Best Practices

### 1. Order Matters

```go
// GOOD: Correct order
app.Use(middleware.Logger())        // 1. Log everything
app.Use(middleware.Recover())       // 2. Catch panics
app.Use(middleware.RequestID())     // 3. Add request IDs
app.Use(middleware.Auth())          // 4. Authenticate
app.Use(middleware.RateLimit())     // 5. Rate limit authenticated requests

// BAD: Wrong order
app.Use(middleware.RateLimit())     // Rate limits before auth
app.Use(middleware.Auth())          // Can't rate limit by user
```

### 2. Keep Middleware Focused

```go
// GOOD: Single responsibility
func RequestID() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            ctx.SetRequestID(generateID())
            return next.Handle(ctx)
        })
    }
}

// BAD: Doing too much
func EverythingMiddleware() Middleware {
    // Logging, auth, rate limiting, etc. all in one
}
```

### 3. Make Middleware Configurable

```go
// GOOD: Configurable middleware
type LoggerConfig struct {
    Level       string
    SkipPaths   []string
    LogHeaders  bool
}

func Logger(config LoggerConfig) Middleware {
    // Use configuration
}

// BAD: Hard-coded middleware
func Logger() Middleware {
    // No way to customize behavior
}
```

### 4. Handle Errors Gracefully

```go
// GOOD: Graceful error handling
func Auth() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            token := ctx.Header("Authorization")
            if token == "" {
                return lift.Unauthorized("Missing token")
            }
            
            user, err := validateToken(token)
            if err != nil {
                return lift.Unauthorized("Invalid token")
            }
            
            ctx.Set("user", user)
            return next.Handle(ctx)
        })
    }
}
```

### 5. Avoid Blocking Operations

```go
// GOOD: Non-blocking logging
func AsyncLogger() Middleware {
    logChan := make(chan LogEntry, 1000)
    go processLogs(logChan)
    
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            // Non-blocking send
            select {
            case logChan <- createLogEntry(ctx):
            default:
                // Channel full, skip
            }
            return next.Handle(ctx)
        })
    }
}

// BAD: Blocking operation
func BlockingLogger() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            // This could block the request
            sendToSlowLogService(ctx)
            return next.Handle(ctx)
        })
    }
}
```

### 6. Test Middleware Independently

```go
func TestAuthMiddleware(t *testing.T) {
    // Create test handler
    testHandler := lift.HandlerFunc(func(ctx *lift.Context) error {
        user := ctx.Get("user").(User)
        return ctx.JSON(user)
    })
    
    // Wrap with middleware
    handler := Auth()(testHandler)
    
    // Test with valid token
    ctx := createTestContext()
    ctx.Request.Headers["Authorization"] = "valid-token"
    
    err := handler.Handle(ctx)
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    // Test with invalid token
    ctx = createTestContext()
    ctx.Request.Headers["Authorization"] = "invalid-token"
    
    err = handler.Handle(ctx)
    assert.Error(t, err)
    assert.Equal(t, 401, err.(lift.HTTPError).Status())
}
```

## Performance Considerations

### Minimize Allocations

```go
// GOOD: Reuse objects
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func CompressionMiddleware() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            buf := bufferPool.Get().(*bytes.Buffer)
            defer bufferPool.Put(buf)
            buf.Reset()
            
            // Use buffer...
            return next.Handle(ctx)
        })
    }
}
```

### Skip When Possible

```go
func ExpensiveMiddleware(config Config) Middleware {
    skipPaths := make(map[string]bool)
    for _, path := range config.SkipPaths {
        skipPaths[path] = true
    }
    
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            // Fast path for skipped routes
            if skipPaths[ctx.Request.Path] {
                return next.Handle(ctx)
            }
            
            // Expensive operation
            return doExpensiveOperation(ctx, next)
        })
    }
}
```

## Summary

Middleware in Lift provides:

- **Reusability**: Write once, use everywhere
- **Composability**: Combine middleware for complex behaviors
- **Separation of Concerns**: Keep handlers focused on business logic
- **Flexibility**: Configure behavior without changing code
- **Testing**: Test middleware independently

Use middleware to build maintainable, scalable serverless applications. 