# Lift Error Handling Guide

*Date: 2025-06-16-06_56_39*
*Author: Pay Theory Lift Team*

## Overview

Lift provides a comprehensive error handling system designed for serverless applications. This guide covers structured errors, recovery strategies, circuit breakers, and production-ready error handling patterns.

## 1. LiftError - Structured Error Handling

### Basic LiftError Usage

Lift uses `LiftError` for structured, consistent error responses:

```go
import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/errors"
)

// Create structured errors
func validateUser(user *User) error {
    if user.Email == "" {
        return lift.BadRequest("Email is required")
    }
    
    if !isValidEmail(user.Email) {
        return lift.BadRequest("Invalid email format").
            WithDetails(map[string]interface{}{
                "field": "email",
                "value": user.Email,
            })
    }
    
    return nil
}
```

### Built-in Error Constructors

```go
// 4xx Client Errors
func handleClientErrors(ctx *lift.Context) error {
    // 400 Bad Request
    if invalidInput {
        return lift.BadRequest("Invalid input provided")
    }
    
    // 401 Unauthorized
    if !authenticated {
        return lift.Unauthorized("Authentication required")
    }
    
    // 403 Forbidden
    if !authorized {
        return lift.Forbidden("Access denied")
    }
    
    // 404 Not Found
    if !exists {
        return lift.NotFound("Resource not found")
    }
    
    // 409 Conflict
    if duplicate {
        return lift.Conflict("Resource already exists")
    }
    
    // 422 Unprocessable Entity
    if validationFailed {
        return lift.ValidationError("name", "Name must be at least 2 characters")
    }
    
    return nil
}

// 5xx Server Errors
func handleServerErrors(ctx *lift.Context) error {
    // 500 Internal Server Error
    if internalError {
        return lift.InternalError("An unexpected error occurred")
    }
    
    // 502 Bad Gateway
    if upstreamError {
        return lift.NewLiftError("BAD_GATEWAY", "Upstream service error", 502)
    }
    
    // 503 Service Unavailable
    if serviceDown {
        return lift.NewLiftError("SERVICE_UNAVAILABLE", "Service temporarily unavailable", 503)
    }
    
    // 504 Gateway Timeout
    if timeout {
        return lift.NewLiftError("GATEWAY_TIMEOUT", "Request timeout", 504)
    }
    
    return nil
}
```

### Enhanced Error Details

```go
func createUserWithDetails(ctx *lift.Context) error {
    var user CreateUserRequest
    if err := ctx.ParseRequest(&user); err != nil {
        return lift.BadRequest("Invalid request body").
            WithCause(err).
            WithDetails(map[string]interface{}{
                "request_id": ctx.RequestID,
                "timestamp":  time.Now().Unix(),
            })
    }
    
    // Check if user exists
    existing, err := userService.GetByEmail(user.Email)
    if err != nil {
        return lift.InternalError("Failed to check existing user").
            WithCause(err).
            WithDetails(map[string]interface{}{
                "operation": "user_lookup",
                "email":     user.Email,
            })
    }
    
    if existing != nil {
        return lift.Conflict("User already exists").
            WithDetails(map[string]interface{}{
                "existing_user_id": existing.ID,
                "email":           user.Email,
                "created_at":      existing.CreatedAt,
            })
    }
    
    return ctx.Created(user)
}
```

## 2. Context Error Methods

### Direct Context Error Methods

The Lift context provides convenient error methods:

```go
func userHandler(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    // Validate input
    if userID == "" {
        return ctx.BadRequest("User ID is required", nil)
    }
    
    // Check authentication
    if !ctx.IsAuthenticated() {
        return ctx.Unauthorized("Authentication required", nil)
    }
    
    // Check permissions
    if !hasPermission(ctx.UserID(), "read:users") {
        return ctx.Forbidden("Insufficient permissions", nil)
    }
    
    // Get user
    user, err := userService.Get(userID)
    if err != nil {
        if err == ErrUserNotFound {
            return ctx.NotFound("User not found", err)
        }
        return ctx.InternalError("Failed to retrieve user", err)
    }
    
    return ctx.OK(user)
}
```

### Context Error Methods vs LiftError

```go
// Method 1: Using context error methods (sets status automatically)
func methodOne(ctx *lift.Context) error {
    return ctx.BadRequest("Invalid input", nil) // Sets 400 status
}

// Method 2: Using LiftError constructors (more flexible)
func methodTwo(ctx *lift.Context) error {
    return lift.BadRequest("Invalid input") // Returns error, status set by middleware
}

// Method 3: Custom LiftError with details
func methodThree(ctx *lift.Context) error {
    return lift.NewLiftError("CUSTOM_ERROR", "Custom error message", 422).
        WithDetails(map[string]interface{}{
            "field": "custom_field",
            "code":  "CUSTOM_CODE",
        })
}
```

## 3. Error Recovery Strategies

### Retry Recovery Strategy

```go
import "github.com/pay-theory/lift/pkg/errors"

// Configure retry recovery
func setupRetryRecovery() *errors.DefaultErrorHandler {
    handler := errors.NewDefaultErrorHandler()
    
    // Add retry strategy for network errors
    retryStrategy := &errors.RetryRecoveryStrategy{
        MaxRetries: 3,
        RetryDelay: time.Second,
        RetryableFunc: func(ctx context.Context) error {
            // Retry the operation
            return performNetworkOperation()
        },
    }
    
    handler.RecoveryStrategies = append(handler.RecoveryStrategies, retryStrategy)
    return handler
}

// Use in middleware
func RetryMiddleware() lift.Middleware {
    errorHandler := setupRetryRecovery()
    
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err != nil {
                // Attempt recovery
                recoveredErr := errorHandler.HandleError(ctx.Context, err)
                return recoveredErr
            }
            return nil
        })
    }
}
```

### Circuit Breaker Recovery

```go
import "github.com/pay-theory/lift/pkg/middleware"

// Configure circuit breaker
func setupCircuitBreaker() lift.Middleware {
    config := middleware.CircuitBreakerConfig{
        Name:             "external-service",
        FailureThreshold: 5,                // Open after 5 failures
        SuccessThreshold: 3,                // Close after 3 successes
        Timeout:          60 * time.Second, // Stay open for 60 seconds
        
        // Custom failure detection
        ShouldTrip: func(err error) bool {
            // Only trip on 5xx errors, not 4xx
            if liftErr, ok := err.(*lift.LiftError); ok {
                return liftErr.StatusCode >= 500
            }
            return true
        },
        
        // Custom fallback
        FallbackHandler: func(ctx *lift.Context) error {
            return ctx.Status(503).JSON(map[string]interface{}{
                "error":   "Service temporarily unavailable",
                "message": "External service is down, please try again later",
                "code":    "CIRCUIT_BREAKER_OPEN",
                "retry_after": 60,
            })
        },
    }
    
    return middleware.CircuitBreakerMiddleware(config)
}

// Apply to specific routes
func setupRoutes(app *lift.App) {
    // Apply circuit breaker to external service calls
    externalGroup := app.Group("/api/external")
    externalGroup.Use(setupCircuitBreaker())
    externalGroup.GET("/data", fetchExternalData)
}
```

### Fallback Recovery Strategy

```go
// Fallback strategy for degraded functionality
func setupFallbackStrategy() *errors.FallbackRecoveryStrategy {
    return &errors.FallbackRecoveryStrategy{
        FallbackFunc: func(ctx context.Context, err error) error {
            // Provide cached or default response
            return provideFallbackResponse(ctx, err)
        },
    }
}

func provideFallbackResponse(ctx context.Context, err error) error {
    // Check if we have cached data
    if cachedData := getCachedData(ctx); cachedData != nil {
        return nil // Success with cached data
    }
    
    // Provide default response
    return provideDefaultResponse(ctx)
}
```

## 4. Error Middleware Patterns

### Comprehensive Error Handling Middleware

```go
func ErrorHandlingMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Execute handler
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Log error with context
            logError(ctx, err)
            
            // Handle different error types
            switch e := err.(type) {
            case *lift.LiftError:
                return handleLiftError(ctx, e)
            case *ValidationError:
                return handleValidationError(ctx, e)
            case *DatabaseError:
                return handleDatabaseError(ctx, e)
            default:
                return handleGenericError(ctx, e)
            }
        })
    }
}

func logError(ctx *lift.Context, err error) {
    if ctx.Logger != nil {
        ctx.Logger.Error("Request failed", map[string]interface{}{
            "error":      sanitizeError(err),
            "path":       ctx.Request.Path,
            "method":     ctx.Request.Method,
            "request_id": ctx.RequestID,
            "user_id":    ctx.UserID(),
            "tenant_id":  ctx.TenantID(),
        })
    }
}

func handleLiftError(ctx *lift.Context, err *lift.LiftError) error {
    // Add request context to error
    err.RequestID = ctx.RequestID
    
    // Set status and return JSON
    return ctx.Status(err.StatusCode).JSON(map[string]interface{}{
        "error": map[string]interface{}{
            "code":       err.Code,
            "message":    err.Message,
            "details":    err.Details,
            "request_id": err.RequestID,
            "timestamp":  err.Timestamp,
        },
    })
}
```

### Recovery Middleware

```go
func RecoveryMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            defer func() {
                if r := recover(); r != nil {
                    // Log panic with stack trace
                    if ctx.Logger != nil {
                        ctx.Logger.Error("Handler panicked", map[string]interface{}{
                            "panic":      fmt.Sprintf("%v", r),
                            "stack":      string(debug.Stack()),
                            "request_id": ctx.RequestID,
                            "path":       ctx.Request.Path,
                            "method":     ctx.Request.Method,
                        })
                    }
                    
                    // Return structured error response
                    ctx.Status(500).JSON(map[string]interface{}{
                        "error": map[string]interface{}{
                            "code":       "PANIC_RECOVERED",
                            "message":    "Internal server error",
                            "request_id": ctx.RequestID,
                            "timestamp":  time.Now().Unix(),
                        },
                    })
                }
            }()
            
            return next.Handle(ctx)
        })
    }
}
```

## 5. Custom Error Types

### Application-Specific Errors

```go
// Define custom error types
type ValidationError struct {
    Field   string                 `json:"field"`
    Message string                 `json:"message"`
    Code    string                 `json:"code"`
    Details map[string]interface{} `json:"details,omitempty"`
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

type BusinessLogicError struct {
    Code       string                 `json:"code"`
    Message    string                 `json:"message"`
    StatusCode int                    `json:"-"`
    Context    map[string]interface{} `json:"context,omitempty"`
}

func (e *BusinessLogicError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *BusinessLogicError) Status() int {
    return e.StatusCode
}

// Usage in handlers
func createOrder(ctx *lift.Context) error {
    var order CreateOrderRequest
    if err := ctx.ParseRequest(&order); err != nil {
        return &ValidationError{
            Field:   "request_body",
            Message: "Invalid JSON format",
            Code:    "INVALID_JSON",
        }
    }
    
    // Business logic validation
    if order.Amount <= 0 {
        return &BusinessLogicError{
            Code:       "INVALID_AMOUNT",
            Message:    "Order amount must be positive",
            StatusCode: 422,
            Context: map[string]interface{}{
                "provided_amount": order.Amount,
                "minimum_amount":  0.01,
            },
        }
    }
    
    return ctx.Created(order)
}
```

### Error Aggregation

```go
// Collect multiple errors
type ErrorCollection struct {
    Errors []error `json:"errors"`
}

func (ec *ErrorCollection) Error() string {
    messages := make([]string, len(ec.Errors))
    for i, err := range ec.Errors {
        messages[i] = err.Error()
    }
    return strings.Join(messages, "; ")
}

func (ec *ErrorCollection) Add(err error) {
    ec.Errors = append(ec.Errors, err)
}

func (ec *ErrorCollection) HasErrors() bool {
    return len(ec.Errors) > 0
}

// Usage for batch operations
func processBatch(ctx *lift.Context) error {
    var batch BatchRequest
    if err := ctx.ParseRequest(&batch); err != nil {
        return lift.BadRequest("Invalid batch request")
    }
    
    var errors ErrorCollection
    var results []Result
    
    for i, item := range batch.Items {
        result, err := processItem(item)
        if err != nil {
            errors.Add(fmt.Errorf("item %d: %w", i, err))
            continue
        }
        results = append(results, result)
    }
    
    if errors.HasErrors() {
        return ctx.Status(207).JSON(map[string]interface{}{
            "processed": results,
            "errors":    errors.Errors,
            "status":    "partial_success",
        })
    }
    
    return ctx.OK(map[string]interface{}{
        "processed": results,
        "status":    "success",
    })
}
```

## 6. Production Error Handling

### Environment-Aware Error Responses

```go
func ProductionErrorMiddleware(env string) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Sanitize errors for production
            sanitizedErr := sanitizeErrorForEnvironment(err, env)
            
            // Log full error details
            logFullError(ctx, err)
            
            // Return sanitized error to client
            return sanitizedErr
        })
    }
}

func sanitizeErrorForEnvironment(err error, env string) error {
    if env == "production" {
        // Hide internal details in production
        if liftErr, ok := err.(*lift.LiftError); ok {
            if liftErr.StatusCode >= 500 {
                return lift.InternalError("An internal error occurred")
            }
            // Client errors are safe to expose
            return err
        }
        // Unknown errors become generic internal errors
        return lift.InternalError("An internal error occurred")
    }
    
    // Development/staging - show full errors
    return err
}
```

### Error Monitoring Integration

```go
func ErrorMonitoringMiddleware(monitor ErrorMonitor) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err != nil {
                // Send to monitoring service
                go func() {
                    monitor.ReportError(ErrorReport{
                        Error:       err,
                        RequestID:   ctx.RequestID,
                        UserID:      ctx.UserID(),
                        TenantID:    ctx.TenantID(),
                        Path:        ctx.Request.Path,
                        Method:      ctx.Request.Method,
                        Timestamp:   time.Now(),
                        Environment: os.Getenv("ENVIRONMENT"),
                        Version:     os.Getenv("APP_VERSION"),
                    })
                }()
            }
            return err
        })
    }
}

type ErrorMonitor interface {
    ReportError(report ErrorReport) error
}

type ErrorReport struct {
    Error       error                  `json:"error"`
    RequestID   string                 `json:"request_id"`
    UserID      string                 `json:"user_id"`
    TenantID    string                 `json:"tenant_id"`
    Path        string                 `json:"path"`
    Method      string                 `json:"method"`
    Timestamp   time.Time              `json:"timestamp"`
    Environment string                 `json:"environment"`
    Version     string                 `json:"version"`
    Context     map[string]interface{} `json:"context,omitempty"`
}
```

## 7. Testing Error Handling

### Unit Testing Errors

```go
func TestErrorHandling(t *testing.T) {
    tests := []struct {
        name           string
        handler        lift.Handler
        expectedStatus int
        expectedCode   string
    }{
        {
            name: "validation error",
            handler: lift.HandlerFunc(func(ctx *lift.Context) error {
                return lift.BadRequest("Invalid input")
            }),
            expectedStatus: 400,
            expectedCode:   "BAD_REQUEST",
        },
        {
            name: "not found error",
            handler: lift.HandlerFunc(func(ctx *lift.Context) error {
                return lift.NotFound("Resource not found")
            }),
            expectedStatus: 404,
            expectedCode:   "NOT_FOUND",
        },
        {
            name: "internal error",
            handler: lift.HandlerFunc(func(ctx *lift.Context) error {
                return lift.InternalError("Something went wrong")
            }),
            expectedStatus: 500,
            expectedCode:   "INTERNAL_ERROR",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := createTestContext("GET", "/test", nil)
            
            err := tt.handler.Handle(ctx)
            
            // Check that error was returned
            assert.Error(t, err)
            
            // Check error type and properties
            if liftErr, ok := err.(*lift.LiftError); ok {
                assert.Equal(t, tt.expectedStatus, liftErr.StatusCode)
                assert.Equal(t, tt.expectedCode, liftErr.Code)
            } else {
                t.Errorf("Expected LiftError, got %T", err)
            }
        })
    }
}
```

### Integration Testing with Error Middleware

```go
func TestErrorMiddleware(t *testing.T) {
    app := lift.New()
    app.Use(ErrorHandlingMiddleware())
    app.Use(RecoveryMiddleware())
    
    // Test panic recovery
    app.GET("/panic", func(ctx *lift.Context) error {
        panic("test panic")
    })
    
    // Test error handling
    app.GET("/error", func(ctx *lift.Context) error {
        return lift.InternalError("test error")
    })
    
    t.Run("panic recovery", func(t *testing.T) {
        ctx := createTestContext("GET", "/panic", nil)
        
        err := app.HandleTestRequest(ctx)
        assert.NoError(t, err) // Middleware should handle the panic
        assert.Equal(t, 500, ctx.Response.StatusCode)
    })
    
    t.Run("error handling", func(t *testing.T) {
        ctx := createTestContext("GET", "/error", nil)
        
        err := app.HandleTestRequest(ctx)
        assert.NoError(t, err) // Middleware should handle the error
        assert.Equal(t, 500, ctx.Response.StatusCode)
    })
}
```

## 8. Best Practices

### 1. Use Structured Errors

```go
// Good: Structured error with context
return lift.BadRequest("Invalid email format").
    WithDetails(map[string]interface{}{
        "field":         "email",
        "provided_value": email,
        "expected_format": "user@domain.com",
    })

// Avoid: Generic error messages
return errors.New("bad email")
```

### 2. Provide Actionable Error Messages

```go
// Good: Actionable error message
return lift.Unauthorized("Authentication token expired. Please log in again.")

// Avoid: Vague error message
return lift.Unauthorized("Auth failed")
```

### 3. Use Appropriate HTTP Status Codes

```go
// Good: Correct status codes
if user == nil {
    return lift.NotFound("User not found") // 404
}
if !hasPermission {
    return lift.Forbidden("Insufficient permissions") // 403
}
if validationFailed {
    return lift.BadRequest("Invalid input") // 400
}

// Avoid: Wrong status codes
if user == nil {
    return lift.InternalError("User not found") // Wrong: should be 404
}
```

### 4. Log Errors Appropriately

```go
// Good: Log with context
func handleRequest(ctx *lift.Context) error {
    user, err := userService.Get(userID)
    if err != nil {
        ctx.Logger.Error("Failed to get user", map[string]interface{}{
            "user_id":    userID,
            "error":      err.Error(),
            "request_id": ctx.RequestID,
            "operation":  "user_lookup",
        })
        return lift.InternalError("Failed to retrieve user")
    }
    return ctx.OK(user)
}
```

### 5. Handle Errors at the Right Level

```go
// Good: Handle errors where you have context
func createUser(ctx *lift.Context) error {
    user, err := userService.Create(userData)
    if err != nil {
        if err == ErrUserExists {
            return lift.Conflict("User already exists")
        }
        if err == ErrInvalidData {
            return lift.BadRequest("Invalid user data")
        }
        // Log unexpected errors
        ctx.Logger.Error("Unexpected error creating user", map[string]interface{}{
            "error": err.Error(),
        })
        return lift.InternalError("Failed to create user")
    }
    return ctx.Created(user)
}
```

This comprehensive guide covers all aspects of error handling in Lift, from basic structured errors to advanced recovery strategies and production best practices. 
## 3. Error Recovery Strategies

### Circuit Breaker Recovery

```go
import "github.com/pay-theory/lift/pkg/middleware"

// Configure circuit breaker
func setupCircuitBreaker() lift.Middleware {
    config := middleware.CircuitBreakerConfig{
        Name:             "external-service",
        FailureThreshold: 5,                // Open after 5 failures
        SuccessThreshold: 3,                // Close after 3 successes
        Timeout:          60 * time.Second, // Stay open for 60 seconds
        
        // Custom failure detection
        ShouldTrip: func(err error) bool {
            // Only trip on 5xx errors, not 4xx
            if liftErr, ok := err.(*lift.LiftError); ok {
                return liftErr.StatusCode >= 500
            }
            return true
        },
        
        // Custom fallback
        FallbackHandler: func(ctx *lift.Context) error {
            return ctx.Status(503).JSON(map[string]interface{}{
                "error":   "Service temporarily unavailable",
                "message": "External service is down, please try again later",
                "code":    "CIRCUIT_BREAKER_OPEN",
                "retry_after": 60,
            })
        },
    }
    
    return middleware.CircuitBreakerMiddleware(config)
}
```

## 4. Error Middleware Patterns

### Comprehensive Error Handling Middleware

```go
func ErrorHandlingMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Execute handler
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Log error with context
            if ctx.Logger != nil {
                ctx.Logger.Error("Request failed", map[string]interface{}{
                    "error":      err.Error(),
                    "path":       ctx.Request.Path,
                    "method":     ctx.Request.Method,
                    "request_id": ctx.RequestID,
                    "user_id":    ctx.UserID(),
                    "tenant_id":  ctx.TenantID(),
                })
            }
            
            // Handle LiftError
            if liftErr, ok := err.(*lift.LiftError); ok {
                liftErr.RequestID = ctx.RequestID
                return ctx.Status(liftErr.StatusCode).JSON(map[string]interface{}{
                    "error": map[string]interface{}{
                        "code":       liftErr.Code,
                        "message":    liftErr.Message,
                        "details":    liftErr.Details,
                        "request_id": liftErr.RequestID,
                        "timestamp":  liftErr.Timestamp,
                    },
                })
            }
            
            // Handle generic errors
            return ctx.Status(500).JSON(map[string]interface{}{
                "error": map[string]interface{}{
                    "code":    "INTERNAL_ERROR",
                    "message": "An internal error occurred",
                },
            })
        })
    }
}
```

## Summary

This guide covers the essential patterns for error handling in Lift:

1. **LiftError**: Use structured errors with proper HTTP status codes
2. **Context Methods**: Leverage context error methods for convenience  
3. **Recovery Strategies**: Implement circuit breakers and retry logic
4. **Middleware**: Use error handling and recovery middleware
5. **Best Practices**: Follow structured error patterns with proper logging

### Key Error Handling APIs

- `lift.BadRequest(message)` - 400 errors
- `lift.Unauthorized(message)` - 401 errors
- `lift.Forbidden(message)` - 403 errors
- `lift.NotFound(message)` - 404 errors
- `lift.Conflict(message)` - 409 errors
- `lift.InternalError(message)` - 500 errors
- `lift.NewLiftError(code, message, statusCode)` - Custom errors

### Context Error Methods

- `ctx.BadRequest(message, err)` - Sets status and returns JSON
- `ctx.Unauthorized(message, err)` - Sets status and returns JSON
- `ctx.Forbidden(message, err)` - Sets status and returns JSON
- `ctx.NotFound(message, err)` - Sets status and returns JSON
- `ctx.InternalError(message, err)` - Sets status and returns JSON

### Error Enhancement

- `err.WithDetails(details)` - Add structured details
- `err.WithCause(cause)` - Chain underlying errors
- `err.WithRequestID(requestID)` - Add request tracing

For more advanced patterns like custom error types, error aggregation, monitoring integration, and testing strategies, refer to the existing error handling documentation in the codebase.
