# Lift Middleware Testing Guide for Mockery

## Table of Contents
1. [Understanding Lift Middleware](#understanding-lift-middleware)
2. [Creating Custom Middleware](#creating-custom-middleware)
3. [Middleware Execution Order](#middleware-execution-order)
4. [Testing Middleware](#testing-middleware)
5. [Common Middleware Patterns](#common-middleware-patterns)
6. [Troubleshooting](#troubleshooting)
7. [Best Practices](#best-practices)

## Understanding Lift Middleware

### What is Middleware?

Middleware in Lift is a function that wraps handler execution, allowing you to:
- Execute code before and/or after handlers
- Modify requests and responses
- Short-circuit request processing
- Share data between middleware and handlers via context

### Middleware Signature

```go
type Middleware func(Handler) Handler
type Handler interface {
    Handle(ctx *Context) error
}
```

## Creating Custom Middleware

### Basic Middleware Structure

```go
func MyMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Pre-processing
            log.Println("Before handler")
            
            // Call next handler
            err := next.Handle(ctx)
            
            // Post-processing
            log.Println("After handler")
            
            return err
        })
    }
}
```

### Middleware with Configuration

```go
func RateLimitMiddleware(limit int, window time.Duration) lift.Middleware {
    limiter := NewRateLimiter(limit, window)
    
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            key := ctx.Header("X-API-Key")
            if !limiter.Allow(key) {
                return lift.NewLiftError("RATE_LIMIT_EXCEEDED", "Too many requests", 429)
            }
            return next.Handle(ctx)
        })
    }
}
```

### Middleware that Sets Context Values

```go
func DatabaseMiddleware(db *sql.DB) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Set database in context
            ctx.Set("db", db)
            
            // Set up transaction if needed
            tx, err := db.BeginTx(ctx, nil)
            if err != nil {
                return lift.NewLiftError("DB_ERROR", "Failed to start transaction", 500).WithCause(err)
            }
            
            ctx.Set("tx", tx)
            
            // Execute handler
            err = next.Handle(ctx)
            
            // Commit or rollback based on error
            if err != nil {
                tx.Rollback()
                return err
            }
            
            if err := tx.Commit(); err != nil {
                return lift.NewLiftError("DB_ERROR", "Failed to commit transaction", 500).WithCause(err)
            }
            
            return nil
        })
    }
}
```

## Middleware Execution Order

### Registration Order Matters

```go
app := lift.New()

// Middleware executes in the order registered
app.Use(RecoveryMiddleware())    // 1st - Catches panics
app.Use(LoggingMiddleware())      // 2nd - Logs all requests
app.Use(AuthMiddleware())         // 3rd - Authenticates
app.Use(DatabaseMiddleware())     // 4th - Sets up DB

// Handler executes last
app.GET("/users", GetUsers)
```

### Execution Flow

```
Request → Recovery → Logging → Auth → Database → Handler
                                                      ↓
Response ← Recovery ← Logging ← Auth ← Database ←────┘
```

## Testing Middleware

### Unit Testing Individual Middleware

```go
func TestAuthMiddleware(t *testing.T) {
    // Create middleware
    authMiddleware := AuthMiddleware("secret-key")
    
    // Create a mock handler to verify it's called
    handlerCalled := false
    mockHandler := lift.HandlerFunc(func(ctx *lift.Context) error {
        handlerCalled = true
        // Verify context values were set
        assert.Equal(t, "user123", ctx.UserID())
        assert.Equal(t, "tenant456", ctx.TenantID())
        return nil
    })
    
    // Wrap handler with middleware
    wrappedHandler := authMiddleware(mockHandler)
    
    // Create test context with valid token
    ctx := createTestContext("GET", "/test", nil)
    ctx.Request.Headers["Authorization"] = "Bearer valid-token"
    
    // Execute
    err := wrappedHandler.Handle(ctx)
    
    // Assertions
    assert.NoError(t, err)
    assert.True(t, handlerCalled)
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
    authMiddleware := AuthMiddleware("secret-key")
    
    handlerCalled := false
    mockHandler := lift.HandlerFunc(func(ctx *lift.Context) error {
        handlerCalled = true
        return nil
    })
    
    wrappedHandler := authMiddleware(mockHandler)
    
    // Test with invalid token
    ctx := createTestContext("GET", "/test", nil)
    ctx.Request.Headers["Authorization"] = "Bearer invalid-token"
    
    err := wrappedHandler.Handle(ctx)
    
    // Should return error and not call handler
    assert.Error(t, err)
    assert.False(t, handlerCalled)
    
    // Verify error type
    liftErr, ok := err.(*lift.LiftError)
    assert.True(t, ok)
    assert.Equal(t, 401, liftErr.StatusCode)
}
```

### Integration Testing with Multiple Middleware

```go
func TestMiddlewareChain(t *testing.T) {
    app := lift.New()
    
    // Track execution order
    var executionOrder []string
    
    // Add middleware that tracks execution
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            executionOrder = append(executionOrder, "middleware1-before")
            err := next.Handle(ctx)
            executionOrder = append(executionOrder, "middleware1-after")
            return err
        })
    })
    
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            executionOrder = append(executionOrder, "middleware2-before")
            err := next.Handle(ctx)
            executionOrder = append(executionOrder, "middleware2-after")
            return err
        })
    })
    
    // Add handler
    app.GET("/test", func(ctx *lift.Context) error {
        executionOrder = append(executionOrder, "handler")
        return ctx.JSON(map[string]string{"status": "ok"})
    })
    
    // Create test context
    ctx := createTestContext("GET", "/test", nil)
    
    // Execute through app
    err := app.HandleTestRequest(ctx)
    
    // Verify
    assert.NoError(t, err)
    assert.Equal(t, []string{
        "middleware1-before",
        "middleware2-before",
        "handler",
        "middleware2-after",
        "middleware1-after",
    }, executionOrder)
}
```

### Testing Middleware Error Handling

```go
func TestMiddlewareErrorPropagation(t *testing.T) {
    app := lift.New()
    
    // Middleware that logs errors
    errorLogged := false
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err != nil {
                errorLogged = true
            }
            return err
        })
    })
    
    // Handler that returns error
    app.GET("/error", func(ctx *lift.Context) error {
        return lift.NewLiftError("TEST_ERROR", "Something went wrong", 500)
    })
    
    ctx := createTestContext("GET", "/error", nil)
    err := app.HandleTestRequest(ctx)
    
    // Error should be handled by framework, not returned
    assert.NoError(t, err)
    assert.True(t, errorLogged)
    assert.Equal(t, 500, ctx.Response.StatusCode)
}
```

### Testing Context Value Propagation

```go
func TestMiddlewareContextValues(t *testing.T) {
    app := lift.New()
    
    // Middleware sets values
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            ctx.Set("service", &MyService{Name: "test-service"})
            ctx.Set("request-id", "req-123")
            return next.Handle(ctx)
        })
    })
    
    // Handler uses values
    app.GET("/test", func(ctx *lift.Context) error {
        service := ctx.Get("service").(*MyService)
        requestID := ctx.Get("request-id").(string)
        
        return ctx.JSON(map[string]string{
            "service": service.Name,
            "request_id": requestID,
        })
    })
    
    ctx := createTestContext("GET", "/test", nil)
    err := app.HandleTestRequest(ctx)
    
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    // Verify response
    var resp map[string]string
    json.Unmarshal(ctx.Response.Body.([]byte), &resp)
    assert.Equal(t, "test-service", resp["service"])
    assert.Equal(t, "req-123", resp["request_id"])
}
```

## Common Middleware Patterns

### Authentication Middleware

```go
func JWTAuthMiddleware(secretKey string) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Skip auth for public endpoints
            if isPublicEndpoint(ctx.Request.Path) {
                return next.Handle(ctx)
            }
            
            // Extract token
            authHeader := ctx.Header("Authorization")
            if authHeader == "" {
                return lift.NewLiftError("UNAUTHORIZED", "Missing authorization header", 401)
            }
            
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            
            // Validate token
            token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
                return []byte(secretKey), nil
            })
            
            if err != nil || !token.Valid {
                return lift.NewLiftError("UNAUTHORIZED", "Invalid token", 401)
            }
            
            // Extract claims and set in context
            if claims, ok := token.Claims.(jwt.MapClaims); ok {
                ctx.SetClaims(claims)
            }
            
            return next.Handle(ctx)
        })
    }
}
```

### Request ID Middleware

```go
func RequestIDMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            requestID := ctx.Header("X-Request-ID")
            if requestID == "" {
                requestID = generateRequestID()
            }
            
            ctx.SetRequestID(requestID)
            ctx.Response.Header("X-Request-ID", requestID)
            
            return next.Handle(ctx)
        })
    }
}
```

### Metrics Middleware

```go
func MetricsMiddleware(collector MetricsCollector) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            start := time.Now()
            
            // Execute handler
            err := next.Handle(ctx)
            
            // Record metrics
            duration := time.Since(start)
            labels := map[string]string{
                "method": ctx.Request.Method,
                "path":   ctx.Request.Path,
                "status": fmt.Sprintf("%d", ctx.Response.StatusCode),
            }
            
            collector.RecordDuration("http_request_duration", duration, labels)
            collector.IncrementCounter("http_requests_total", labels)
            
            if err != nil {
                collector.IncrementCounter("http_errors_total", labels)
            }
            
            return err
        })
    }
}
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Middleware Not Executing

**Problem**: Middleware doesn't seem to run in Lambda
```go
// WRONG - Middleware won't be transferred
lambda.Start(app.HandleRequest)
```

**Solution**: Ensure app.Start() is called (fixed in latest version)
```go
// The fix ensures Start() is called internally
lambda.Start(app.HandleRequest) // Now works correctly
```

#### 2. Context Values Not Available

**Problem**: Handler can't access values set by middleware
```go
// Middleware sets value
ctx.Set("user", user)

// Handler gets nil
user := ctx.Get("user") // nil
```

**Solution**: Ensure middleware is registered before routes
```go
app := lift.New()
app.Use(AuthMiddleware()) // Register middleware first
app.GET("/users", handler) // Then register routes
```

#### 3. Middleware Order Issues

**Problem**: Dependencies between middleware not respected
```go
// WRONG - DB middleware needs auth context
app.Use(DatabaseMiddleware()) // Needs tenant ID
app.Use(AuthMiddleware())     // Sets tenant ID
```

**Solution**: Register in dependency order
```go
// CORRECT
app.Use(AuthMiddleware())     // Sets tenant ID first
app.Use(DatabaseMiddleware()) // Can now use tenant ID
```

## Best Practices

### 1. Keep Middleware Focused

Each middleware should have a single responsibility:
```go
// GOOD - Single responsibility
app.Use(AuthenticationMiddleware())
app.Use(AuthorizationMiddleware())
app.Use(LoggingMiddleware())

// BAD - Doing too much
app.Use(AuthAndLoggingAndMetricsMiddleware())
```

### 2. Handle Errors Gracefully

```go
func SafeMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            defer func() {
                if r := recover(); r != nil {
                    ctx.Logger.Error("Middleware panic", map[string]any{
                        "panic": r,
                        "stack": string(debug.Stack()),
                    })
                    ctx.Status(500).JSON(map[string]string{
                        "error": "Internal server error",
                    })
                }
            }()
            
            return next.Handle(ctx)
        })
    }
}
```

### 3. Use Type-Safe Context Access

```go
// Define typed getters
func GetDatabase(ctx *lift.Context) (*sql.DB, error) {
    db, ok := ctx.Get("db").(*sql.DB)
    if !ok {
        return nil, errors.New("database not initialized")
    }
    return db, nil
}

// Use in handlers
func GetUsers(ctx *lift.Context) error {
    db, err := GetDatabase(ctx)
    if err != nil {
        return lift.NewLiftError("INTERNAL_ERROR", "Database not available", 500)
    }
    
    // Use db safely
    users, err := db.Query("SELECT * FROM users")
    // ...
}
```

### 4. Test Middleware in Isolation

```go
func TestMiddleware(t *testing.T) {
    tests := []struct {
        name          string
        setupContext  func(*lift.Context)
        middleware    lift.Middleware
        expectError   bool
        expectContext func(*testing.T, *lift.Context)
    }{
        {
            name: "successful auth",
            setupContext: func(ctx *lift.Context) {
                ctx.Request.Headers["Authorization"] = "Bearer valid-token"
            },
            middleware:  AuthMiddleware(),
            expectError: false,
            expectContext: func(t *testing.T, ctx *lift.Context) {
                assert.NotEmpty(t, ctx.UserID())
            },
        },
        {
            name: "missing auth",
            setupContext: func(ctx *lift.Context) {
                // No auth header
            },
            middleware:  AuthMiddleware(),
            expectError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup
            ctx := createTestContext("GET", "/test", nil)
            tt.setupContext(ctx)
            
            // Create mock handler
            handler := lift.HandlerFunc(func(ctx *lift.Context) error {
                return nil
            })
            
            // Apply middleware
            wrapped := tt.middleware(handler)
            
            // Execute
            err := wrapped.Handle(ctx)
            
            // Assert
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                if tt.expectContext != nil {
                    tt.expectContext(t, ctx)
                }
            }
        })
    }
}
```

### 5. Document Middleware Dependencies

```go
// AuthMiddleware sets the following context values:
// - "user_id": string
// - "tenant_id": string
// - "roles": []string
//
// Required headers:
// - Authorization: Bearer <token>
func AuthMiddleware() lift.Middleware {
    // Implementation
}

// DatabaseMiddleware requires:
// - "tenant_id" to be set in context (by AuthMiddleware)
//
// Sets:
// - "db": *sql.DB (tenant-scoped database connection)
func DatabaseMiddleware() lift.Middleware {
    // Implementation
}
```

## Helper Functions for Testing

```go
// createTestContext creates a test context for middleware testing
func createTestContext(method, path string, body []byte) *lift.Context {
    req := &lift.Request{
        Method:      method,
        Path:        path,
        Headers:     make(map[string]string),
        QueryParams: make(map[string]string),
        Body:        body,
    }
    return lift.NewContext(context.Background(), req)
}

// assertMiddlewareOrder verifies middleware execution order
func assertMiddlewareOrder(t *testing.T, app *lift.App, expectedOrder []string) {
    var actualOrder []string
    
    // Add tracking middleware
    for i, name := range expectedOrder {
        middlewareName := name // Capture in closure
        app.Use(func(next lift.Handler) lift.Handler {
            return lift.HandlerFunc(func(ctx *lift.Context) error {
                actualOrder = append(actualOrder, middlewareName+"-before")
                err := next.Handle(ctx)
                actualOrder = append(actualOrder, middlewareName+"-after")
                return err
            })
        })
    }
    
    // Add handler
    app.GET("/test", func(ctx *lift.Context) error {
        actualOrder = append(actualOrder, "handler")
        return nil
    })
    
    // Execute
    ctx := createTestContext("GET", "/test", nil)
    app.HandleTestRequest(ctx)
    
    // Build expected full order
    var expected []string
    for _, name := range expectedOrder {
        expected = append(expected, name+"-before")
    }
    expected = append(expected, "handler")
    for i := len(expectedOrder) - 1; i >= 0; i-- {
        expected = append(expected, expectedOrder[i]+"-after")
    }
    
    assert.Equal(t, expected, actualOrder)
}
```

## Summary

Testing Lift middleware effectively requires:

1. **Understanding the execution model** - Middleware wraps handlers in layers
2. **Testing in isolation** - Unit test individual middleware
3. **Testing integration** - Verify middleware chains work together
4. **Testing error cases** - Ensure errors are handled properly
5. **Testing context propagation** - Verify values pass through correctly

Remember that middleware is just a function that returns a function. This makes it highly testable - you can create mock handlers, control the context, and verify behavior at each step.

The key to successful middleware testing is to:
- Test each middleware in isolation first
- Test common combinations
- Test error propagation
- Test context value sharing
- Document dependencies clearly

With these patterns, you can build and test robust middleware for your Lift applications.