# Error Handling

Lift provides a comprehensive error handling system that ensures consistent, informative error responses across your serverless applications. This guide covers error types, custom errors, error middleware, and best practices.

## Overview

Lift's error handling system provides:

- **Structured Error Responses**: Consistent JSON error format
- **HTTP Status Codes**: Automatic status code mapping
- **Error Types**: Built-in error constructors for common scenarios
- **Custom Errors**: Define application-specific errors
- **Error Context**: Include request IDs and metadata
- **Production Safety**: Hide sensitive information in production

## Built-in Error Types

### Client Errors (4xx)

```go
// 400 Bad Request
func validateInput(ctx *lift.Context) error {
    if input == "" {
        return lift.BadRequest("Input cannot be empty")
    }
    return nil
}

// 401 Unauthorized
func requireAuth(ctx *lift.Context) error {
    token := ctx.Header("Authorization")
    if token == "" {
        return lift.Unauthorized("Authentication required")
    }
    return nil
}

// 403 Forbidden
func checkPermission(ctx *lift.Context) error {
    if !hasPermission(ctx.UserID(), resource) {
        return lift.Forbidden("You don't have permission to access this resource")
    }
    return nil
}

// 404 Not Found
func getUser(ctx *lift.Context) error {
    user, err := userService.Get(id)
    if err == ErrNotFound {
        return lift.NotFound("User not found")
    }
    return ctx.JSON(user)
}

// 409 Conflict
func createUser(ctx *lift.Context) error {
    if userExists(email) {
        return lift.Conflict("User with this email already exists")
    }
    return nil
}

// 422 Unprocessable Entity
func processRequest(ctx *lift.Context) error {
    if !canProcess(data) {
        return lift.UnprocessableEntity("Cannot process request with current data")
    }
    return nil
}

// 429 Too Many Requests
func rateLimited(ctx *lift.Context) error {
    if isRateLimited(ctx.UserID()) {
        return lift.TooManyRequests("Rate limit exceeded. Try again later.")
    }
    return nil
}
```

### Server Errors (5xx)

```go
// 500 Internal Server Error
func handleRequest(ctx *lift.Context) error {
    result, err := riskyOperation()
    if err != nil {
        ctx.Logger.Error("Operation failed", map[string]interface{}{
            "error": err.Error(),
        })
        return lift.InternalError("An unexpected error occurred")
    }
    return ctx.JSON(result)
}

// 502 Bad Gateway
func callUpstream(ctx *lift.Context) error {
    resp, err := upstreamService.Call()
    if err != nil {
        return lift.BadGateway("Upstream service error")
    }
    return ctx.JSON(resp)
}

// 503 Service Unavailable
func healthCheck(ctx *lift.Context) error {
    if !isHealthy() {
        return lift.ServiceUnavailable("Service temporarily unavailable")
    }
    return ctx.JSON(map[string]string{"status": "healthy"})
}

// 504 Gateway Timeout
func longOperation(ctx *lift.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
    defer cancel()
    
    result, err := performOperation(ctx)
    if err == context.DeadlineExceeded {
        return lift.GatewayTimeout("Operation timed out")
    }
    return ctx.JSON(result)
}
```

## Error Response Format

Lift automatically formats errors as JSON:

```json
{
    "error": {
        "code": "NOT_FOUND",
        "message": "User not found",
        "details": {
            "user_id": "123",
            "searched_in": "primary_db"
        },
        "request_id": "req_abc123",
        "timestamp": 1234567890
    }
}
```

### Customizing Error Responses

```go
// Create detailed error
type DetailedError struct {
    StatusCode int
    Code       string
    Message    string
    Details    map[string]interface{}
}

func (e DetailedError) Error() string {
    return e.Message
}

func (e DetailedError) Status() int {
    return e.StatusCode
}

// Use in handler
func handler(ctx *lift.Context) error {
    return DetailedError{
        StatusCode: 400,
        Code:       "VALIDATION_ERROR",
        Message:    "Validation failed",
        Details: map[string]interface{}{
            "fields": []string{"email", "password"},
            "reasons": map[string]string{
                "email":    "Invalid format",
                "password": "Too short",
            },
        },
    }
}
```

## Custom Error Types

### Application Errors

```go
// Define application-specific errors
type AppError struct {
    Type    string
    Message string
    Details interface{}
}

const (
    ErrTypeValidation   = "VALIDATION_ERROR"
    ErrTypeNotFound     = "NOT_FOUND"
    ErrTypeUnauthorized = "UNAUTHORIZED"
    ErrTypeInternal     = "INTERNAL_ERROR"
)

func NewAppError(errType, message string, details interface{}) *AppError {
    return &AppError{
        Type:    errType,
        Message: message,
        Details: details,
    }
}

func (e *AppError) Error() string {
    return e.Message
}

func (e *AppError) Status() int {
    switch e.Type {
    case ErrTypeValidation:
        return 400
    case ErrTypeNotFound:
        return 404
    case ErrTypeUnauthorized:
        return 401
    default:
        return 500
    }
}

// Use in handlers
func getResource(ctx *lift.Context) error {
    resource, err := service.Get(id)
    if err != nil {
        return NewAppError(
            ErrTypeNotFound,
            "Resource not found",
            map[string]string{"id": id},
        )
    }
    return ctx.JSON(resource)
}
```

### Validation Errors

```go
// Validation error with field details
type ValidationError struct {
    Fields []FieldError `json:"fields"`
}

type FieldError struct {
    Field   string `json:"field"`
    Code    string `json:"code"`
    Message string `json:"message"`
}

func (e ValidationError) Error() string {
    return "Validation failed"
}

func (e ValidationError) Status() int {
    return 400
}

// Validation helper
func validateUser(user *User) error {
    var errors []FieldError
    
    if user.Email == "" {
        errors = append(errors, FieldError{
            Field:   "email",
            Code:    "required",
            Message: "Email is required",
        })
    } else if !isValidEmail(user.Email) {
        errors = append(errors, FieldError{
            Field:   "email",
            Code:    "invalid_format",
            Message: "Email format is invalid",
        })
    }
    
    if len(user.Password) < 8 {
        errors = append(errors, FieldError{
            Field:   "password",
            Code:    "too_short",
            Message: "Password must be at least 8 characters",
        })
    }
    
    if len(errors) > 0 {
        return ValidationError{Fields: errors}
    }
    
    return nil
}
```

### Business Logic Errors

```go
// Domain-specific errors
type InsufficientFundsError struct {
    AccountID string
    Available float64
    Required  float64
}

func (e InsufficientFundsError) Error() string {
    return fmt.Sprintf("Insufficient funds: available %.2f, required %.2f", 
        e.Available, e.Required)
}

func (e InsufficientFundsError) Status() int {
    return 400
}

// Use in business logic
func transferFunds(ctx *lift.Context, transfer TransferRequest) error {
    account, err := getAccount(transfer.FromAccountID)
    if err != nil {
        return lift.NotFound("Account not found")
    }
    
    if account.Balance < transfer.Amount {
        return InsufficientFundsError{
            AccountID: account.ID,
            Available: account.Balance,
            Required:  transfer.Amount,
        }
    }
    
    // Process transfer...
    return ctx.JSON(transferResult)
}
```

## Error Middleware

### Global Error Handler

```go
func ErrorHandlerMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Execute handler
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Log error with context
            ctx.Logger.Error("Request failed", map[string]interface{}{
                "error":      err.Error(),
                "path":       ctx.Request.Path,
                "method":     ctx.Request.Method,
                "request_id": ctx.RequestID(),
            })
            
            // Check error type
            switch e := err.(type) {
            case lift.HTTPError:
                // Built-in HTTP errors
                return formatHTTPError(ctx, e)
                
            case ValidationError:
                // Custom validation errors
                return ctx.Status(400).JSON(map[string]interface{}{
                    "error": map[string]interface{}{
                        "code":    "VALIDATION_ERROR",
                        "message": "Validation failed",
                        "fields":  e.Fields,
                    },
                })
                
            case *AppError:
                // Application errors
                return ctx.Status(e.Status()).JSON(map[string]interface{}{
                    "error": map[string]interface{}{
                        "code":    e.Type,
                        "message": e.Message,
                        "details": e.Details,
                    },
                })
                
            default:
                // Unknown errors - hide details in production
                if ctx.Environment() == "production" {
                    return ctx.Status(500).JSON(map[string]interface{}{
                        "error": map[string]interface{}{
                            "code":    "INTERNAL_ERROR",
                            "message": "An unexpected error occurred",
                        },
                    })
                }
                
                // Show details in development
                return ctx.Status(500).JSON(map[string]interface{}{
                    "error": map[string]interface{}{
                        "code":    "INTERNAL_ERROR",
                        "message": err.Error(),
                        "type":    fmt.Sprintf("%T", err),
                    },
                })
            }
        })
    }
}

// Use the middleware
app.Use(ErrorHandlerMiddleware())
```

### Recovery Middleware

```go
func RecoveryMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            defer func() {
                if r := recover(); r != nil {
                    // Log panic with stack trace
                    ctx.Logger.Error("Panic recovered", map[string]interface{}{
                        "panic": r,
                        "stack": string(debug.Stack()),
                    })
                    
                    // Return error response
                    ctx.Status(500).JSON(map[string]interface{}{
                        "error": map[string]interface{}{
                            "code":    "PANIC",
                            "message": "Internal server error",
                        },
                    })
                }
            }()
            
            return next.Handle(ctx)
        })
    }
}
```

## Error Handling Patterns

### Wrapping Errors

```go
// Wrap errors with context
func getUserData(userID string) (*User, error) {
    user, err := db.GetUser(userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get user %s: %w", userID, err)
    }
    
    profile, err := db.GetProfile(userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get profile for user %s: %w", userID, err)
    }
    
    user.Profile = profile
    return user, nil
}

// Handle wrapped errors
func handler(ctx *lift.Context) error {
    user, err := getUserData(userID)
    if err != nil {
        // Check wrapped error
        if errors.Is(err, sql.ErrNoRows) {
            return lift.NotFound("User not found")
        }
        
        // Log full error chain
        ctx.Logger.Error("Failed to get user data", map[string]interface{}{
            "error": err.Error(),
            "user_id": userID,
        })
        
        return lift.InternalError("Failed to retrieve user data")
    }
    
    return ctx.JSON(user)
}
```

### Error Aggregation

```go
// Collect multiple errors
type ErrorList struct {
    Errors []error
}

func (e ErrorList) Error() string {
    messages := make([]string, len(e.Errors))
    for i, err := range e.Errors {
        messages[i] = err.Error()
    }
    return strings.Join(messages, "; ")
}

func (e ErrorList) Status() int {
    return 400
}

// Use for batch operations
func processBatch(ctx *lift.Context, items []Item) error {
    var errors ErrorList
    var processed []Result
    
    for _, item := range items {
        result, err := processItem(item)
        if err != nil {
            errors.Errors = append(errors.Errors, 
                fmt.Errorf("item %s: %w", item.ID, err))
            continue
        }
        processed = append(processed, result)
    }
    
    if len(errors.Errors) > 0 {
        return ctx.Status(207).JSON(map[string]interface{}{
            "processed": processed,
            "errors":    errors.Errors,
        })
    }
    
    return ctx.JSON(processed)
}
```

### Retry Logic

```go
// Retryable error interface
type RetryableError interface {
    error
    Retryable() bool
}

// Implementation
type ServiceError struct {
    Message   string
    Retryable bool
}

func (e ServiceError) Error() string {
    return e.Message
}

func (e ServiceError) Retryable() bool {
    return e.Retryable
}

// Retry middleware
func RetryMiddleware(maxRetries int) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            var err error
            
            for attempt := 0; attempt <= maxRetries; attempt++ {
                err = next.Handle(ctx)
                
                // Success
                if err == nil {
                    return nil
                }
                
                // Check if retryable
                var retryable RetryableError
                if !errors.As(err, &retryable) || !retryable.Retryable() {
                    return err
                }
                
                // Log retry
                ctx.Logger.Warn("Retrying request", map[string]interface{}{
                    "attempt": attempt + 1,
                    "error":   err.Error(),
                })
                
                // Exponential backoff
                if attempt < maxRetries {
                    time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
                }
            }
            
            return fmt.Errorf("max retries exceeded: %w", err)
        })
    }
}
```

## Production Considerations

### Error Sanitization

```go
func SanitizeError(err error, env string) error {
    // In production, hide internal details
    if env == "production" {
        switch err.(type) {
        case *AppError, ValidationError, lift.HTTPError:
            // Known errors are safe to expose
            return err
        default:
            // Hide unknown errors
            return lift.InternalError("An error occurred")
        }
    }
    
    // In development, show full errors
    return err
}

// Use in middleware
func ProductionErrorMiddleware(env string) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err != nil {
                return SanitizeError(err, env)
            }
            return nil
        })
    }
}
```

### Error Monitoring

```go
// Send errors to monitoring service
func ErrorMonitoringMiddleware(monitor ErrorMonitor) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err != nil {
                // Don't monitor client errors
                if httpErr, ok := err.(lift.HTTPError); ok {
                    if httpErr.Status() < 500 {
                        return err
                    }
                }
                
                // Send to monitoring
                monitor.Report(ErrorReport{
                    Error:       err,
                    RequestID:   ctx.RequestID(),
                    UserID:      ctx.UserID(),
                    TenantID:    ctx.TenantID(),
                    Path:        ctx.Request.Path,
                    Method:      ctx.Request.Method,
                    Environment: ctx.Environment(),
                    Timestamp:   time.Now(),
                })
            }
            
            return err
        })
    }
}
```

## Testing Error Handling

### Unit Tests

```go
func TestErrorHandler(t *testing.T) {
    tests := []struct {
        name           string
        handler        lift.HandlerFunc
        expectedStatus int
        expectedCode   string
    }{
        {
            name: "not found error",
            handler: func(ctx *lift.Context) error {
                return lift.NotFound("User not found")
            },
            expectedStatus: 404,
            expectedCode:   "NOT_FOUND",
        },
        {
            name: "validation error",
            handler: func(ctx *lift.Context) error {
                return ValidationError{
                    Fields: []FieldError{
                        {Field: "email", Code: "required"},
                    },
                }
            },
            expectedStatus: 400,
            expectedCode:   "VALIDATION_ERROR",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create test context
            ctx := createTestContext()
            
            // Execute handler with error middleware
            handler := ErrorHandlerMiddleware()(tt.handler)
            err := handler.Handle(ctx)
            
            // Check response
            assert.Equal(t, tt.expectedStatus, ctx.Response.StatusCode)
            
            // Check error format
            var response map[string]interface{}
            json.Unmarshal(ctx.Response.Body.([]byte), &response)
            
            errorData := response["error"].(map[string]interface{})
            assert.Equal(t, tt.expectedCode, errorData["code"])
        })
    }
}
```

### Integration Tests

```go
func TestErrorScenarios(t *testing.T) {
    app := createTestApp()
    
    // Test 404
    resp := app.TestRequest("GET", "/nonexistent", nil)
    assert.Equal(t, 404, resp.StatusCode)
    
    // Test validation error
    resp = app.TestRequest("POST", "/users", map[string]interface{}{
        "email": "invalid",
    })
    assert.Equal(t, 400, resp.StatusCode)
    
    var errorResp ErrorResponse
    json.Unmarshal(resp.Body, &errorResp)
    assert.Contains(t, errorResp.Error.Message, "validation")
}
```

## Best Practices

### 1. Use Appropriate Error Types

```go
// GOOD: Specific error types
if !exists {
    return lift.NotFound("Resource not found")
}
if !authorized {
    return lift.Forbidden("Access denied")
}

// AVOID: Generic errors
if !exists {
    return errors.New("error")
}
```

### 2. Include Context

```go
// GOOD: Contextual error messages
return lift.NotFound(fmt.Sprintf("User with ID %s not found", userID))

// AVOID: Generic messages
return lift.NotFound("Not found")
```

### 3. Log Appropriately

```go
// GOOD: Log server errors with details
if err != nil {
    ctx.Logger.Error("Database query failed", map[string]interface{}{
        "error": err.Error(),
        "query": query,
        "duration": duration,
    })
    return lift.InternalError("Failed to retrieve data")
}

// AVOID: Logging client errors as errors
if input == "" {
    ctx.Logger.Error("Bad request") // Don't log expected errors
    return lift.BadRequest("Input required")
}
```

### 4. Hide Sensitive Information

```go
// GOOD: Safe error messages
if err != nil {
    ctx.Logger.Error("Auth failed", map[string]interface{}{
        "error": err.Error(),
        "user": username,
    })
    return lift.Unauthorized("Invalid credentials")
}

// AVOID: Exposing internals
return lift.Unauthorized(fmt.Sprintf("LDAP bind failed: %v", err))
```

### 5. Consistent Error Responses

```go
// GOOD: Consistent structure
type APIError struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}

// Use everywhere
return ctx.Status(400).JSON(map[string]interface{}{
    "error": APIError{
        Code:    "VALIDATION_ERROR",
        Message: "Invalid input",
        Details: validationDetails,
    },
})
```

## Summary

Lift's error handling provides:

- **Type Safety**: Strongly typed errors with automatic status codes
- **Consistency**: Uniform error response format
- **Flexibility**: Custom error types for domain logic
- **Production Ready**: Error sanitization and monitoring
- **Developer Friendly**: Clear error messages and logging

Proper error handling makes your APIs more reliable and easier to debug. 