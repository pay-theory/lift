# Lift Error Handling Guide

## Table of Contents
1. [Overview](#overview)
2. [The LiftError Type](#the-lifterror-type)
3. [Creating Errors](#creating-errors)
4. [Common Error Patterns](#common-error-patterns)
5. [Error Handling in Middleware](#error-handling-in-middleware)
6. [Error Handling in Handlers](#error-handling-in-handlers)
7. [Testing Error Scenarios](#testing-error-scenarios)
8. [Migration from Old API](#migration-from-old-api)
9. [Best Practices](#best-practices)

## Overview

Lift provides a structured error handling system that ensures consistent error responses across your application. The system is designed to:
- Provide consistent error formats
- Support proper HTTP status codes
- Enable detailed error information for debugging
- Maintain security by not exposing sensitive information
- Work seamlessly with middleware chains

## The LiftError Type

### Structure

```go
type LiftError struct {
    Code       string                 `json:"code"`
    Message    string                 `json:"message"`
    StatusCode int                    `json:"-"`
    Details    map[string]interface{} `json:"details,omitempty"`
    Cause      error                  `json:"-"`
}
```

### Key Features
- **Code**: A machine-readable error code (e.g., "VALIDATION_ERROR", "NOT_FOUND")
- **Message**: A human-readable error message
- **StatusCode**: HTTP status code for the response
- **Details**: Optional additional information about the error
- **Cause**: The underlying error (not exposed in JSON responses)

## Creating Errors

### Basic Error Creation

```go
// Create a simple error
err := lift.NewLiftError("NOT_FOUND", "Resource not found", 404)

// Create error with details
err := lift.NewLiftError("VALIDATION_ERROR", "Invalid request data", 400).
    WithDetail("field", "email").
    WithDetail("reason", "invalid format")

// Create error with underlying cause
err := lift.NewLiftError("DB_ERROR", "Database operation failed", 500).
    WithCause(dbErr)
```

### Using Context Error Methods

```go
func handler(ctx *lift.Context) error {
    // Client errors (4xx)
    ctx.BadRequest("Invalid input")
    ctx.Unauthorized("Authentication required")
    ctx.Forbidden("Access denied")
    ctx.NotFound("Resource not found")
    
    // Server errors (5xx)
    ctx.SystemError("Internal error", err)
    
    // Custom status codes
    ctx.Error(409, "CONFLICT", "Resource already exists")
}
```

## Common Error Patterns

### Validation Errors

```go
func CreateUser(ctx *lift.Context) error {
    var req CreateUserRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return lift.NewLiftError("VALIDATION_ERROR", "Invalid request body", 400).
            WithCause(err)
    }
    
    // Manual validation
    if req.Age < 0 || req.Age > 120 {
        return lift.NewLiftError("VALIDATION_ERROR", "Invalid age", 400).
            WithDetail("field", "age").
            WithDetail("min", 0).
            WithDetail("max", 120)
    }
    
    // Or use ValidationError helper
    if req.Email == "" {
        return lift.ValidationError("Email is required").
            WithDetail("field", "email")
    }
    
    // Success case
    return ctx.JSON(map[string]string{"id": "user_123"})
}
```

### Database Errors

```go
func GetUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    user, err := db.GetUser(userID)
    if err != nil {
        if err == sql.ErrNoRows {
            return lift.NewLiftError("NOT_FOUND", "User not found", 404).
                WithDetail("user_id", userID)
        }
        
        // Don't expose database errors to clients
        return lift.NewLiftError("DB_ERROR", "Failed to retrieve user", 500).
            WithCause(err) // Logged but not exposed
    }
    
    return ctx.JSON(user)
}
```

### External Service Errors

```go
func CallExternalAPI(ctx *lift.Context) error {
    resp, err := http.Get("https://api.example.com/data")
    if err != nil {
        return lift.NewLiftError("EXTERNAL_SERVICE_ERROR", 
            "Failed to connect to external service", 503).
            WithCause(err).
            WithDetail("service", "example-api")
    }
    
    if resp.StatusCode >= 400 {
        return lift.NewLiftError("EXTERNAL_SERVICE_ERROR",
            "External service returned error", 502).
            WithDetail("service", "example-api").
            WithDetail("status", resp.StatusCode)
    }
    
    // Process response...
    return nil
}
```

### Business Logic Errors

```go
func TransferFunds(ctx *lift.Context) error {
    var req TransferRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return err
    }
    
    // Check business rules
    balance, err := getAccountBalance(req.FromAccount)
    if err != nil {
        return lift.NewLiftError("ACCOUNT_ERROR", 
            "Failed to retrieve account balance", 500).
            WithCause(err)
    }
    
    if balance < req.Amount {
        return lift.NewLiftError("INSUFFICIENT_FUNDS",
            "Insufficient funds for transfer", 400).
            WithDetail("available", balance).
            WithDetail("requested", req.Amount)
    }
    
    // Process transfer...
    return ctx.JSON(map[string]string{"status": "completed"})
}
```

## Error Handling in Middleware

### Error Recovery Middleware

```go
func ErrorRecoveryMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            defer func() {
                if r := recover(); r != nil {
                    // Log the panic
                    if ctx.Logger != nil {
                        ctx.Logger.Error("Handler panicked", map[string]any{
                            "panic": r,
                            "stack": string(debug.Stack()),
                        })
                    }
                    
                    // Return a safe error response
                    err := lift.NewLiftError("INTERNAL_ERROR", 
                        "An unexpected error occurred", 500)
                    ctx.Status(err.StatusCode).JSON(err)
                }
            }()
            
            return next.Handle(ctx)
        })
    }
}
```

### Error Logging Middleware

```go
func ErrorLoggingMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err != nil {
                // Log error details
                if ctx.Logger != nil {
                    fields := map[string]any{
                        "error": err.Error(),
                        "path": ctx.Request.Path,
                        "method": ctx.Request.Method,
                    }
                    
                    // Add LiftError details if available
                    if liftErr, ok := err.(*lift.LiftError); ok {
                        fields["error_code"] = liftErr.Code
                        fields["status_code"] = liftErr.StatusCode
                        if liftErr.Cause != nil {
                            fields["cause"] = liftErr.Cause.Error()
                        }
                    }
                    
                    ctx.Logger.Error("Request failed", fields)
                }
            }
            return err
        })
    }
}
```

### Error Transformation Middleware

```go
func ErrorTransformationMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Transform specific errors
            switch {
            case errors.Is(err, sql.ErrNoRows):
                return lift.NewLiftError("NOT_FOUND", "Resource not found", 404)
                
            case errors.Is(err, context.DeadlineExceeded):
                return lift.NewLiftError("TIMEOUT", "Request timeout", 504)
                
            case errors.Is(err, ErrUnauthorized):
                return lift.NewLiftError("UNAUTHORIZED", "Authentication required", 401)
                
            default:
                // Return as-is if already a LiftError
                if _, ok := err.(*lift.LiftError); ok {
                    return err
                }
                
                // Wrap unknown errors
                return lift.NewLiftError("INTERNAL_ERROR", 
                    "An error occurred", 500).WithCause(err)
            }
        })
    }
}
```

## Error Handling in Handlers

### Automatic Error Handling

```go
// Lift automatically handles errors returned from handlers
func GetUserHandler(ctx *lift.Context) error {
    user, err := getUserFromDB(ctx.Param("id"))
    if err != nil {
        // This error will be caught by the framework
        return lift.NewLiftError("NOT_FOUND", "User not found", 404)
    }
    
    return ctx.JSON(user)
}
```

### Manual Error Handling

```go
// Sometimes you want to handle errors within the handler
func ComplexHandler(ctx *lift.Context) error {
    // Try primary data source
    data, err := getPrimaryData()
    if err != nil {
        // Log error but try fallback
        if ctx.Logger != nil {
            ctx.Logger.Warn("Primary data source failed", map[string]any{
                "error": err.Error(),
            })
        }
        
        // Try fallback
        data, err = getFallbackData()
        if err != nil {
            // Both failed, return error
            return lift.NewLiftError("DATA_ERROR", 
                "Failed to retrieve data", 503).WithCause(err)
        }
    }
    
    return ctx.JSON(data)
}
```

### Type-Safe Handler Errors

```go
// Using SimpleHandler with automatic error handling
app.POST("/users", lift.SimpleHandler(func(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Validation is automatic via struct tags
    
    // Business logic error
    if exists := checkUserExists(req.Email); exists {
        return UserResponse{}, lift.NewLiftError("USER_EXISTS", 
            "User with this email already exists", 409).
            WithDetail("email", req.Email)
    }
    
    user, err := createUser(req)
    if err != nil {
        return UserResponse{}, lift.NewLiftError("CREATE_ERROR",
            "Failed to create user", 500).WithCause(err)
    }
    
    return UserResponse{
        ID:    user.ID,
        Email: user.Email,
    }, nil
}))
```

## Testing Error Scenarios

### Unit Testing Errors

```go
func TestErrorHandling(t *testing.T) {
    app := lift.New()
    
    // Handler that returns various errors
    app.GET("/error/:type", func(ctx *lift.Context) error {
        errorType := ctx.Param("type")
        
        switch errorType {
        case "validation":
            return lift.ValidationError("Invalid input").
                WithDetail("field", "email")
        case "notfound":
            return lift.NewLiftError("NOT_FOUND", "Resource not found", 404)
        case "panic":
            panic("test panic")
        default:
            return nil
        }
    })
    
    // Add error recovery middleware
    app.Use(ErrorRecoveryMiddleware())
    
    tests := []struct {
        name           string
        errorType      string
        expectedStatus int
        expectedCode   string
    }{
        {
            name:           "validation error",
            errorType:      "validation",
            expectedStatus: 400,
            expectedCode:   "VALIDATION_ERROR",
        },
        {
            name:           "not found error",
            errorType:      "notfound",
            expectedStatus: 404,
            expectedCode:   "NOT_FOUND",
        },
        {
            name:           "panic recovery",
            errorType:      "panic",
            expectedStatus: 500,
            expectedCode:   "INTERNAL_ERROR",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := createTestContext("GET", "/error/"+tt.errorType, nil)
            
            err := app.HandleTestRequest(ctx)
            
            assert.NoError(t, err) // Framework handles errors
            assert.Equal(t, tt.expectedStatus, ctx.Response.StatusCode)
            
            var errResp map[string]any
            json.Unmarshal(ctx.Response.Body.([]byte), &errResp)
            assert.Equal(t, tt.expectedCode, errResp["code"])
        })
    }
}
```

### Integration Testing with Error Scenarios

```go
func TestAPIErrorScenarios(t *testing.T) {
    app := testing.NewTestApp()
    
    // Setup routes
    app.POST("/api/users", CreateUserHandler)
    app.GET("/api/users/:id", GetUserHandler)
    
    scenarios := []testing.TestScenario{
        {
            Name: "missing required fields",
            Request: func(app *testing.TestApp) *testing.TestResponse {
                return app.POST("/api/users", map[string]any{
                    // Missing required email field
                    "name": "Test User",
                })
            },
            Assertions: func(t *testing.T, resp *testing.TestResponse) {
                resp.AssertStatus(400)
                resp.AssertJSONPath("$.code", "VALIDATION_ERROR")
                resp.AssertJSONPathExists("$.details.field")
            },
        },
        {
            Name: "duplicate user",
            Setup: func(app *testing.TestApp) error {
                // Create initial user
                resp := app.POST("/api/users", map[string]any{
                    "email": "test@example.com",
                    "name": "Test User",
                })
                return resp.Error()
            },
            Request: func(app *testing.TestApp) *testing.TestResponse {
                // Try to create duplicate
                return app.POST("/api/users", map[string]any{
                    "email": "test@example.com",
                    "name": "Another User",
                })
            },
            Assertions: func(t *testing.T, resp *testing.TestResponse) {
                resp.AssertStatus(409)
                resp.AssertJSONPath("$.code", "USER_EXISTS")
            },
        },
        {
            Name: "user not found",
            Request: func(app *testing.TestApp) *testing.TestResponse {
                return app.GET("/api/users/nonexistent-id")
            },
            Assertions: func(t *testing.T, resp *testing.TestResponse) {
                resp.AssertStatus(404)
                resp.AssertJSONPath("$.code", "NOT_FOUND")
            },
        },
    }
    
    testing.RunScenarios(t, app, scenarios)
}
```

## Migration from Old API

### Before (Old API)
```go
// Old error functions that no longer exist
return lift.BadRequest("Invalid input")
return lift.InternalError("Server error")
return ctx.InternalError("Error occurred", err)
return lift.ValidationError("field", "message")
```

### After (New API)
```go
// New error creation
return lift.NewLiftError("BAD_REQUEST", "Invalid input", 400)
return lift.NewLiftError("INTERNAL_ERROR", "Server error", 500)
return ctx.SystemError("Error occurred", err)
return lift.ValidationError("message").WithDetail("field", "field")
```

### Migration Table

| Old API | New API |
|---------|---------|
| `lift.BadRequest(msg)` | `lift.NewLiftError("BAD_REQUEST", msg, 400)` |
| `lift.InternalError(msg)` | `lift.NewLiftError("INTERNAL_ERROR", msg, 500)` |
| `ctx.InternalError(msg, err)` | `ctx.SystemError(msg, err)` |
| `lift.ValidationError(field, msg)` | `lift.ValidationError(msg).WithDetail("field", field)` |
| `lift.NotFound(msg)` | `lift.NewLiftError("NOT_FOUND", msg, 404)` |
| `lift.Unauthorized(msg)` | `lift.NewLiftError("UNAUTHORIZED", msg, 401)` |

## Best Practices

### 1. Use Consistent Error Codes

```go
// Define error codes as constants
const (
    ErrCodeValidation     = "VALIDATION_ERROR"
    ErrCodeNotFound       = "NOT_FOUND"
    ErrCodeUnauthorized   = "UNAUTHORIZED"
    ErrCodeForbidden      = "FORBIDDEN"
    ErrCodeConflict       = "CONFLICT"
    ErrCodeInternal       = "INTERNAL_ERROR"
    ErrCodeExternalAPI    = "EXTERNAL_API_ERROR"
    ErrCodeTimeout        = "TIMEOUT"
    ErrCodeRateLimit      = "RATE_LIMIT_EXCEEDED"
)

// Use consistently
return lift.NewLiftError(ErrCodeNotFound, "User not found", 404)
```

### 2. Don't Expose Sensitive Information

```go
// BAD - Exposes internal details
return lift.NewLiftError("DB_ERROR", 
    fmt.Sprintf("Query failed: %v", dbErr), 500)

// GOOD - Generic message with cause for logging
return lift.NewLiftError("DB_ERROR", 
    "Failed to retrieve data", 500).
    WithCause(dbErr) // Logged but not exposed
```

### 3. Provide Useful Details

```go
// Provide actionable information
return lift.NewLiftError("VALIDATION_ERROR", "Invalid date format", 400).
    WithDetail("field", "start_date").
    WithDetail("expected_format", "YYYY-MM-DD").
    WithDetail("received", userInput)
```

### 4. Handle Errors at the Right Level

```go
// Repository layer - return raw errors
func (r *UserRepo) GetUser(id string) (*User, error) {
    var user User
    err := r.db.Get(&user, "SELECT * FROM users WHERE id = ?", id)
    return &user, err // Return raw error
}

// Service layer - wrap with context
func (s *UserService) GetUser(id string) (*User, error) {
    user, err := s.repo.GetUser(id)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, lift.NewLiftError("NOT_FOUND", 
                "User not found", 404).
                WithDetail("user_id", id)
        }
        return nil, lift.NewLiftError("DB_ERROR",
            "Failed to retrieve user", 500).
            WithCause(err)
    }
    return user, nil
}

// Handler layer - return service errors
func GetUserHandler(ctx *lift.Context) error {
    user, err := userService.GetUser(ctx.Param("id"))
    if err != nil {
        return err // Service already wrapped the error
    }
    return ctx.JSON(user)
}
```

### 5. Test Error Paths

```go
func TestErrorPaths(t *testing.T) {
    // Test each error condition
    testCases := []struct {
        name          string
        setup         func()
        expectedError *lift.LiftError
    }{
        {
            name: "database error",
            setup: func() {
                mockDB.ExpectError(errors.New("connection lost"))
            },
            expectedError: &lift.LiftError{
                Code:       "DB_ERROR",
                StatusCode: 500,
            },
        },
        // More test cases...
    }
}
```

### 6. Use Error Middleware

```go
// Apply error handling middleware globally
app := lift.New()
app.Use(middleware.Recover())        // Panic recovery
app.Use(middleware.ErrorHandler())   // Error transformation
app.Use(middleware.Logger())         // Error logging
```

### 7. Document Error Responses

```go
// Document your API errors
// @Summary Create user
// @Description Create a new user account
// @Success 201 {object} UserResponse
// @Failure 400 {object} lift.LiftError "Validation error"
// @Failure 409 {object} lift.LiftError "User already exists"
// @Failure 500 {object} lift.LiftError "Internal server error"
func CreateUserHandler(ctx *lift.Context) error {
    // Implementation...
}
```

## Summary

The Lift error handling system provides:

1. **Structured Errors**: Consistent error format with codes, messages, and details
2. **Type Safety**: Compile-time checks prevent errors
3. **Security**: Sensitive information is logged but not exposed
4. **Flexibility**: Errors can be created and transformed at any layer
5. **Testing**: Easy to test error scenarios
6. **Middleware Integration**: Errors flow naturally through middleware chains

By following these patterns and best practices, you can build robust error handling that provides great developer experience and clear feedback to API consumers.