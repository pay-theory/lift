# Custom Errors with Lift - Complete Guide

## Table of Contents
1. [Overview](#overview)
2. [Creating Custom Errors](#creating-custom-errors)
3. [Custom Error Types](#custom-error-types)
4. [Error Wrapping and Context](#error-wrapping-and-context)
5. [Domain-Specific Errors](#domain-specific-errors)
6. [Error Hierarchies](#error-hierarchies)
7. [Best Practices](#best-practices)
8. [Testing Custom Errors](#testing-custom-errors)

## Overview

Lift provides a flexible error system that allows you to create custom errors tailored to your application's needs while maintaining consistency with HTTP standards and API best practices.

## Creating Custom Errors

### Basic Custom Error
```go
// Create a custom error with specific code and status
err := lift.NewLiftError("PAYMENT_DECLINED", "Payment was declined by bank", 402).
    WithDetail("reason", "insufficient_funds").
    WithDetail("last_four", "1234")
```

### Custom Error Constants
```go
// Define your application's error codes
const (
    // Authentication & Authorization
    ErrCodeInvalidToken      = "INVALID_TOKEN"
    ErrCodeExpiredToken      = "EXPIRED_TOKEN"
    ErrCodeInsufficientPerms = "INSUFFICIENT_PERMISSIONS"
    
    // Business Logic
    ErrCodePaymentFailed     = "PAYMENT_FAILED"
    ErrCodeInsufficientFunds = "INSUFFICIENT_FUNDS"
    ErrCodeDuplicateOrder    = "DUPLICATE_ORDER"
    ErrCodeInventoryLow      = "INVENTORY_LOW"
    
    // External Services
    ErrCodeStripeError       = "STRIPE_ERROR"
    ErrCodeEmailServiceDown  = "EMAIL_SERVICE_DOWN"
    ErrCodeSMSDeliveryFailed = "SMS_DELIVERY_FAILED"
)
```

## Custom Error Types

### Creating Domain-Specific Error Types

```go
// PaymentError represents payment-specific errors
type PaymentError struct {
    *lift.LiftError
    TransactionID string
    PaymentMethod string
    Amount        float64
}

func NewPaymentError(code, message string, statusCode int, txID string) *PaymentError {
    return &PaymentError{
        LiftError:     lift.NewLiftError(code, message, statusCode),
        TransactionID: txID,
    }
}

// Add payment-specific details
func (e *PaymentError) WithPaymentDetails(method string, amount float64) *PaymentError {
    e.PaymentMethod = method
    e.Amount = amount
    e.WithDetail("transaction_id", e.TransactionID).
        WithDetail("payment_method", method).
        WithDetail("amount", amount)
    return e
}
```

### Using Custom Error Types

```go
func ProcessPayment(ctx *lift.Context) error {
    var req PaymentRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return err
    }
    
    // Process payment
    txID := generateTransactionID()
    result, err := paymentGateway.Charge(req.Amount, req.Card)
    
    if err != nil {
        // Create custom payment error
        paymentErr := NewPaymentError(
            ErrCodePaymentFailed,
            "Payment processing failed",
            402,
            txID,
        ).WithPaymentDetails(req.Card.Type, req.Amount)
        
        // Add gateway-specific error details
        if stripeErr, ok := err.(*stripe.Error); ok {
            paymentErr.WithDetail("stripe_code", stripeErr.Code).
                      WithDetail("decline_code", stripeErr.DeclineCode)
        }
        
        return paymentErr
    }
    
    return ctx.JSON(PaymentResponse{TransactionID: txID})
}
```

## Error Wrapping and Context

### Creating Contextual Error Chains

```go
// ValidationError with field-specific details
type ValidationError struct {
    *lift.LiftError
    Fields []FieldError
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Code    string `json:"code"`
}

func NewValidationError(fields ...FieldError) *ValidationError {
    err := &ValidationError{
        LiftError: lift.NewLiftError("VALIDATION_ERROR", "Validation failed", 400),
        Fields:    fields,
    }
    
    // Add fields to details
    fieldDetails := make([]map[string]string, len(fields))
    for i, f := range fields {
        fieldDetails[i] = map[string]string{
            "field":   f.Field,
            "message": f.Message,
            "code":    f.Code,
        }
    }
    err.WithDetail("fields", fieldDetails)
    
    return err
}

// Usage
func CreateUser(ctx *lift.Context) error {
    var req CreateUserRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return err
    }
    
    // Validate multiple fields
    var fieldErrors []FieldError
    
    if !isValidEmail(req.Email) {
        fieldErrors = append(fieldErrors, FieldError{
            Field:   "email",
            Message: "Invalid email format",
            Code:    "INVALID_FORMAT",
        })
    }
    
    if len(req.Password) < 8 {
        fieldErrors = append(fieldErrors, FieldError{
            Field:   "password",
            Message: "Password must be at least 8 characters",
            Code:    "TOO_SHORT",
        })
    }
    
    if len(fieldErrors) > 0 {
        return NewValidationError(fieldErrors...)
    }
    
    // Continue with user creation...
}
```

## Domain-Specific Errors

### Business Logic Errors

```go
// InventoryError for inventory management
type InventoryError struct {
    *lift.LiftError
    ProductID      string
    RequestedQty   int
    AvailableQty   int
}

func NewInventoryError(productID string, requested, available int) *InventoryError {
    msg := fmt.Sprintf("Insufficient inventory for product %s", productID)
    return &InventoryError{
        LiftError:    lift.NewLiftError("INSUFFICIENT_INVENTORY", msg, 409),
        ProductID:    productID,
        RequestedQty: requested,
        AvailableQty: available,
    }
}

func (e *InventoryError) Error() string {
    return fmt.Sprintf("%s: requested %d, available %d", 
        e.LiftError.Error(), e.RequestedQty, e.AvailableQty)
}

// RateLimitError for API rate limiting
type RateLimitError struct {
    *lift.LiftError
    Limit       int
    Window      string
    ResetTime   time.Time
}

func NewRateLimitError(limit int, window string, reset time.Time) *RateLimitError {
    msg := fmt.Sprintf("Rate limit exceeded: %d requests per %s", limit, window)
    err := &RateLimitError{
        LiftError: lift.NewLiftError("RATE_LIMIT_EXCEEDED", msg, 429),
        Limit:     limit,
        Window:    window,
        ResetTime: reset,
    }
    
    err.WithDetail("limit", limit).
        WithDetail("window", window).
        WithDetail("reset_at", reset.Unix())
    
    return err
}
```

### External Service Errors

```go
// ExternalServiceError for third-party integrations
type ExternalServiceError struct {
    *lift.LiftError
    Service       string
    OriginalError error
    Retryable     bool
}

func NewExternalServiceError(service string, err error) *ExternalServiceError {
    return &ExternalServiceError{
        LiftError: lift.NewLiftError(
            "EXTERNAL_SERVICE_ERROR",
            fmt.Sprintf("%s service error", service),
            503,
        ),
        Service:       service,
        OriginalError: err,
        Retryable:     isRetryableError(err),
    }
}

func (e *ExternalServiceError) WithRetryInfo(after time.Duration) *ExternalServiceError {
    e.WithDetail("retry_after", after.Seconds()).
        WithDetail("retryable", e.Retryable)
    return e
}
```

## Error Hierarchies

### Creating Error Hierarchies

```go
// BaseApplicationError for all app-specific errors
type BaseApplicationError interface {
    error
    GetCode() string
    GetStatusCode() int
    GetDetails() map[string]interface{}
    IsRetryable() bool
}

// AuthError hierarchy
type AuthError struct {
    *lift.LiftError
    UserID string
    Reason string
}

type TokenExpiredError struct {
    *AuthError
    ExpiredAt time.Time
}

type InsufficientPermissionsError struct {
    *AuthError
    RequiredPermission string
    UserPermissions    []string
}

// Factory functions
func NewTokenExpiredError(userID string, expiredAt time.Time) *TokenExpiredError {
    base := &AuthError{
        LiftError: lift.NewLiftError("TOKEN_EXPIRED", "Authentication token has expired", 401),
        UserID:    userID,
        Reason:    "token_expired",
    }
    
    return &TokenExpiredError{
        AuthError: base,
        ExpiredAt: expiredAt,
    }
}

func NewInsufficientPermissionsError(userID, required string, perms []string) *InsufficientPermissionsError {
    base := &AuthError{
        LiftError: lift.NewLiftError("INSUFFICIENT_PERMISSIONS", 
            fmt.Sprintf("Missing required permission: %s", required), 403),
        UserID: userID,
        Reason: "insufficient_permissions",
    }
    
    base.WithDetail("required_permission", required).
         WithDetail("user_permissions", perms)
    
    return &InsufficientPermissionsError{
        AuthError:          base,
        RequiredPermission: required,
        UserPermissions:    perms,
    }
}
```

### Using Error Type Assertions

```go
func HandleAuthError(err error) (*lift.Response, error) {
    switch e := err.(type) {
    case *TokenExpiredError:
        // Handle expired token specifically
        return &lift.Response{
            StatusCode: 401,
            Headers: map[string]string{
                "X-Token-Expired": "true",
                "X-Expired-At":    e.ExpiredAt.Format(time.RFC3339),
            },
            Body: map[string]interface{}{
                "code":    e.GetCode(),
                "message": "Please refresh your token",
                "expired_at": e.ExpiredAt,
            },
        }, nil
        
    case *InsufficientPermissionsError:
        // Handle permissions error
        return &lift.Response{
            StatusCode: 403,
            Body: map[string]interface{}{
                "code":     e.GetCode(),
                "message":  e.Message,
                "required": e.RequiredPermission,
                "current":  e.UserPermissions,
            },
        }, nil
        
    case *AuthError:
        // Handle generic auth error
        return &lift.Response{
            StatusCode: e.StatusCode,
            Body:       e,
        }, nil
        
    default:
        // Not an auth error
        return nil, err
    }
}
```

## Best Practices

### 1. Define Error Codes as Constants

```go
package errors

// Define all error codes in one place
const (
    // Client errors (4xx)
    ErrCodeValidation        = "VALIDATION_ERROR"
    ErrCodeNotFound          = "NOT_FOUND"
    ErrCodeUnauthorized      = "UNAUTHORIZED"
    ErrCodeForbidden         = "FORBIDDEN"
    ErrCodeConflict          = "CONFLICT"
    ErrCodePreconditionFailed = "PRECONDITION_FAILED"
    
    // Server errors (5xx)
    ErrCodeInternal          = "INTERNAL_ERROR"
    ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
    ErrCodeTimeout           = "TIMEOUT"
)
```

### 2. Create Error Factories

```go
package errors

// Factory functions for common errors
func NotFound(resource string, id string) *lift.LiftError {
    return lift.NewLiftError(
        ErrCodeNotFound,
        fmt.Sprintf("%s not found", resource),
        404,
    ).WithDetail("resource", resource).
      WithDetail("id", id)
}

func Conflict(resource string, field string, value interface{}) *lift.LiftError {
    return lift.NewLiftError(
        ErrCodeConflict,
        fmt.Sprintf("%s with %s '%v' already exists", resource, field, value),
        409,
    ).WithDetail("resource", resource).
      WithDetail("field", field).
      WithDetail("value", value)
}

func ServiceUnavailable(service string, retryAfter time.Duration) *lift.LiftError {
    return lift.NewLiftError(
        ErrCodeServiceUnavailable,
        fmt.Sprintf("%s service is temporarily unavailable", service),
        503,
    ).WithDetail("service", service).
      WithDetail("retry_after", retryAfter.Seconds())
}
```

### 3. Use Error Middleware

```go
// CustomErrorMiddleware adds application-specific error handling
func CustomErrorMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Transform known errors
            switch {
            case errors.Is(err, sql.ErrNoRows):
                return NotFound("resource", ctx.Param("id"))
                
            case errors.Is(err, context.DeadlineExceeded):
                return lift.NewLiftError("TIMEOUT", "Request timeout", 504)
                
            case isNetworkError(err):
                return ServiceUnavailable("external", 30*time.Second)
            }
            
            // Check for custom error types
            if customErr, ok := err.(BaseApplicationError); ok {
                return lift.NewLiftError(
                    customErr.GetCode(),
                    customErr.Error(),
                    customErr.GetStatusCode(),
                )
            }
            
            return err
        })
    }
}
```

### 4. Include Correlation IDs

```go
func EnrichErrorsMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Generate request ID
            requestID := uuid.New().String()
            ctx.Set("request_id", requestID)
            
            err := next.Handle(ctx)
            if err != nil {
                // Add request ID to all errors
                if liftErr, ok := err.(*lift.LiftError); ok {
                    liftErr.WithDetail("request_id", requestID)
                }
            }
            
            return err
        })
    }
}
```

## Testing Custom Errors

### Testing Error Responses

```go
func TestCustomErrorResponses(t *testing.T) {
    app := lift.New()
    
    app.POST("/payment", func(ctx *lift.Context) error {
        return NewPaymentError(
            "PAYMENT_DECLINED",
            "Card was declined",
            402,
            "tx_123",
        ).WithPaymentDetails("visa", 99.99)
    })
    
    // Test the response
    ctx := createTestContext("POST", "/payment", nil)
    err := app.HandleTestRequest(ctx)
    
    assert.NoError(t, err)
    assert.Equal(t, 402, ctx.Response.StatusCode)
    
    // Check response body
    body := ctx.Response.Body.(map[string]any)
    assert.Equal(t, "PAYMENT_DECLINED", body["code"])
    assert.Equal(t, "Card was declined", body["message"])
    
    details := body["details"].(map[string]any)
    assert.Equal(t, "tx_123", details["transaction_id"])
    assert.Equal(t, "visa", details["payment_method"])
    assert.Equal(t, 99.99, details["amount"])
}
```

### Testing Error Type Assertions

```go
func TestErrorTypeAssertions(t *testing.T) {
    tests := []struct {
        name          string
        error         error
        expectedType  string
        expectedCode  string
    }{
        {
            name:         "token expired error",
            error:        NewTokenExpiredError("user_123", time.Now()),
            expectedType: "*TokenExpiredError",
            expectedCode: "TOKEN_EXPIRED",
        },
        {
            name:         "permissions error",
            error:        NewInsufficientPermissionsError("user_123", "admin", []string{"user"}),
            expectedType: "*InsufficientPermissionsError",
            expectedCode: "INSUFFICIENT_PERMISSIONS",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test type assertion
            switch e := tt.error.(type) {
            case *TokenExpiredError:
                assert.Equal(t, "*TokenExpiredError", tt.expectedType)
                assert.NotNil(t, e.ExpiredAt)
            case *InsufficientPermissionsError:
                assert.Equal(t, "*InsufficientPermissionsError", tt.expectedType)
                assert.NotEmpty(t, e.RequiredPermission)
            }
            
            // Test base error interface
            if baseErr, ok := tt.error.(BaseApplicationError); ok {
                assert.Equal(t, tt.expectedCode, baseErr.GetCode())
            }
        })
    }
}
```

## Summary

Custom errors in Lift allow you to:
1. Create domain-specific error types with rich context
2. Maintain consistent error responses across your API
3. Provide detailed debugging information securely
4. Build error hierarchies for complex applications
5. Transform errors at different layers of your application

By following these patterns, you can build a robust error handling system that provides excellent developer experience and clear communication to API consumers.