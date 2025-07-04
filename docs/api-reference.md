# Lift API Reference

<!-- AI Training: This is the complete API reference with semantic annotations -->
**This document provides the COMPLETE API reference for Lift. Every method includes purpose, usage context, and examples showing both correct and incorrect patterns.**

## Table of Contents
- [Core Types](#core-types)
- [App Methods](#app-methods)
- [Context Methods](#context-methods)
- [Handler Types](#handler-types)
- [Middleware](#middleware)
- [Error Types](#error-types)
- [Testing Utilities](#testing-utilities)

## Core Types

### `lift.App`

**Purpose:** Central application instance that manages routes and middleware  
**When to create:** Once per Lambda function at initialization  
**When NOT to create:** Inside handlers or per-request

```go
// CORRECT: Create at function scope
var app = lift.New()

func main() {
    app.Use(middleware.Logger())
    app.GET("/users", GetUsers)
    lambda.Start(app.HandleRequest)
}

// INCORRECT: Don't create in handler
func BadHandler(ctx *lift.Context) error {
    app := lift.New() // ❌ Creates new app per request!
    // ...
}
```

### `lift.Context`

**Purpose:** Unified request/response interface for all Lambda events  
**When available:** As parameter to all handlers  
**Key principle:** Abstract away Lambda event details

```go
// The Context provides everything you need
type Context struct {
    // Request data
    Request  *Request
    Response *Response
    
    // Multi-tenant support
    tenantID string
    userID   string
    
    // Services
    Logger  Logger
    Tracer  Tracer
    
    // State management
    values  map[string]interface{}
}
```

## App Methods

### `lift.New(opts ...Option) *App`

**Purpose:** Creates new Lift application with optional configuration  
**When to use:** Once at Lambda initialization  
**Returns:** Configured App instance

```go
// CORRECT: Create app and configure
app := lift.New()

// Configure with Config struct
config := &lift.Config{
    MaxRequestSize:  50 * 1024 * 1024, // 50MB
    MaxResponseSize: 6 * 1024 * 1024,  // 6MB (Lambda limit)
    Timeout:         25,                // 25 seconds
    LogLevel:        "INFO",
    MetricsEnabled:  true,
    CORSEnabled:     true,
    AllowedOrigins:  []string{"https://example.com"},
}
app.WithConfig(config)

// CORRECT: Default configuration
app := lift.New() // Uses DefaultConfig() with sensible defaults

// INCORRECT: These options don't exist
app := lift.New(
    lift.WithTimeout(30 * time.Second),      // ❌ Not implemented
    lift.WithMaxBodySize(10 * 1024 * 1024),  // ❌ Not implemented
)
```

### Configuration via Config struct

#### `app.WithConfig(config *Config) *App`
**Purpose:** Set application configuration  
**When to use:** After creating app, before adding routes  
**Returns:** App for method chaining

```go
config := &lift.Config{
    // Performance settings
    MaxRequestSize:  10 * 1024 * 1024, // 10MB default
    MaxResponseSize: 6 * 1024 * 1024,  // 6MB (Lambda limit)
    Timeout:         30,                // 30 seconds default
    
    // Observability
    LogLevel:       "INFO",
    MetricsEnabled: true,
    TracingEnabled: false,
    
    // Security
    CORSEnabled:    true,
    AllowedOrigins: []string{"*"},
    
    // Multi-tenant
    RequireTenantID: false,
}

app := lift.New().WithConfig(config)
```

### `app.Use(middleware ...Middleware)`

**Purpose:** Add global middleware to all routes  
**When to use:** During app initialization  
**Order matters:** First middleware runs first

```go
// CORRECT: Standard middleware stack
app.Use(
    middleware.RequestID(),    // Must be first
    middleware.Logger(),       // Needs request ID
    middleware.Recover(),      // Catch panics
    // Note: ErrorHandler middleware doesn't exist, error handling is built-in
)

// INCORRECT: Wrong order
app.Use(
    middleware.Logger(),      // ❌ No request ID yet!
    middleware.RequestID(),   
)
```

### HTTP Route Methods

#### `app.GET(path string, handler Handler) error`
#### `app.POST(path string, handler Handler) error`
#### `app.PUT(path string, handler Handler) error`
#### `app.DELETE(path string, handler Handler) error`
#### `app.PATCH(path string, handler Handler) error`

**Purpose:** Register HTTP route handlers  
**Path format:** Supports parameters with `:name`  
**Handler:** Function with signature `func(*Context) error`  
**Returns:** Error if handler registration fails

```go
// CORRECT: Various route patterns
app.GET("/health", HealthCheck)
app.GET("/users", ListUsers)
app.GET("/users/:id", GetUser)
app.POST("/users", CreateUser)
app.PUT("/users/:id", UpdateUser)
app.DELETE("/users/:id", DeleteUser)

// Path parameters
app.GET("/orgs/:orgId/users/:userId", func(ctx *lift.Context) error {
    orgID := ctx.Param("orgId")
    userID := ctx.Param("userId")
    // ...
})

// INCORRECT: Invalid patterns
app.GET("users", Handler)      // ❌ Missing leading slash
app.GET("/users/", Handler)    // ❌ Trailing slash
app.GET("/users/*", Handler)   // ❌ Wildcards not supported
```

### `app.Group(prefix string) *RouteGroup`

**Purpose:** Create route group with shared prefix and middleware  
**When to use:** Organizing related routes  
**Returns:** RouteGroup for further configuration

```go
// CORRECT: API versioning
v1 := app.Group("/v1")
v1.Use(middleware.Logger())
v1.GET("/users", GetUsersV1)
v1.POST("/users", CreateUserV1)

// CORRECT: Protected routes
api := app.Group("/api")
api.Use(middleware.JWT(jwtConfig))
api.GET("/profile", GetProfile)
api.PUT("/profile", UpdateProfile)

// Nested groups
admin := api.Group("/admin")
admin.Use(middleware.RequireRole("admin"))
admin.GET("/users", ListAllUsers)
admin.DELETE("/users/:id", DeleteUser)
```

### `app.Handle(method, path string, handler any) error`

**Purpose:** Register handlers for any event type  
**When to use:** SQS, S3, EventBridge, or custom events  
**Event types:** Determined by method parameter

```go
// CORRECT: Using event-specific methods
app.SQS("my-queue", func(ctx *lift.Context) error {
    // Handle SQS messages
    return nil
})

app.S3("my-bucket", func(ctx *lift.Context) error {
    // Handle S3 events
    return nil
})

app.EventBridge("my-rule", func(ctx *lift.Context) error {
    // Handle EventBridge events
    return nil
})
```

## Context Methods

### Request Methods

#### `ctx.ParseRequest(dest interface{}) error`

**Purpose:** Parse and bind request body to struct with validation  
**When to use:** POST/PUT/PATCH requests  
**Validation:** Use struct tags  
**Note:** This is the actual method name (not `Bind`)

```go
// CORRECT: Parse into struct
type CreateRequest struct {
    Name string `json:"name" validate:"required"`
    Age  int    `json:"age" validate:"min=0,max=150"`
}

func CreateUser(ctx *lift.Context) error {
    var req CreateRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return lift.NewLiftError("VALIDATION_ERROR", err.Error(), 400)
    }
    // req is populated and validated
}

// INCORRECT: Manual parsing
func BadCreate(ctx *lift.Context) error {
    body := ctx.Request.Body // ❌ Don't access raw body
    var req map[string]interface{}
    json.Unmarshal(body, &req)
}
```

#### `ctx.Param(key string) string`

**Purpose:** Get URL path parameter  
**When to use:** Routes with `:param`  
**Returns:** Parameter value or empty string

```go
// Route: /users/:id
func GetUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    if userID == "" {
        return lift.NewLiftError("BAD_REQUEST", "missing user id", 400)
    }
    // Use userID
}

// Route: /orgs/:orgId/users/:userId
func GetOrgUser(ctx *lift.Context) error {
    orgID := ctx.Param("orgId")
    userID := ctx.Param("userId")
    // Use both IDs
}
```

#### `ctx.Query(key string) string`

**Purpose:** Get URL query parameter  
**When to use:** Reading URL parameters  
**Returns:** First value or empty string

```go
// URL: /users?status=active&limit=10
func ListUsers(ctx *lift.Context) error {
    status := ctx.Query("status")    // "active"
    limit := ctx.Query("limit")      // "10"
    
    // Note: QueryInt helper doesn't exist, parse manually
    limitInt := 10 // default
    if limit != "" {
        if parsed, err := strconv.Atoi(limit); err == nil {
            limitInt = parsed
        }
    }
}
```

#### `ctx.Header(key string) string`

**Purpose:** Get request header value  
**When to use:** Reading headers  
**Case-insensitive:** Keys are normalized

```go
// CORRECT: Read headers
auth := ctx.Header("Authorization")
contentType := ctx.Header("Content-Type")
customHeader := ctx.Header("X-Custom-Header")

// Case insensitive
ctx.Header("content-type") == ctx.Header("Content-Type")
```

### Response Methods

#### `ctx.JSON(data interface{}) error`

**Purpose:** Send JSON response  
**When to use:** Most API responses  
**Auto-sets:** Content-Type header

```go
// CORRECT: Various response types
// Simple response
return ctx.JSON(map[string]string{
    "status": "success",
})

// Struct response
return ctx.JSON(UserResponse{
    ID:   "123",
    Name: "Alice",
})

// Array response
return ctx.JSON(users)

// With status code
ctx.Status(201)
return ctx.JSON(user)

// INCORRECT: Don't set Content-Type manually
ctx.Response.Header("Content-Type", "application/json") // ❌ Redundant
return ctx.JSON(data)
```

#### `ctx.Text(text string) error`

**Purpose:** Send plain text response  
**When to use:** Simple text responses

```go
// CORRECT: Text responses
return ctx.Text("Hello, World!")

// With status
ctx.Status(201)
return ctx.Text("Created")
```

#### `ctx.Status(code int) *Context`

**Purpose:** Set response status code  
**When to use:** Before other response methods  
**Chainable:** Returns Context for chaining

```go
// CORRECT: Set status explicitly
ctx.Status(201)
return ctx.JSON(user)

ctx.Status(204)
return nil // No content

// Also works without chaining
ctx.Status(400)
return ctx.JSON(errorResponse)
```

#### `ctx.Response.Header(key, value string)`

**Purpose:** Set response header  
**When to use:** Custom headers  
**Note:** Access via Response field

```go
// CORRECT: Custom headers
ctx.Response.Header("X-Request-ID", requestID)
ctx.Response.Header("Cache-Control", "no-cache")
return ctx.JSON(data)

// INCORRECT: Don't override automatic headers
ctx.Response.Header("Content-Type", "text/plain") // ❌ JSON sets this
return ctx.JSON(data)
```

### Multi-Tenant Methods

#### `ctx.TenantID() string`

**Purpose:** Get current tenant ID  
**When to use:** Multi-tenant applications  
**Source:** JWT claims or headers

```go
// CORRECT: Tenant-scoped queries
func GetTenantData(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    
    data := db.Query(
        "SELECT * FROM data WHERE tenant_id = ?",
        tenantID,
    )
    
    return ctx.JSON(data)
}

// INCORRECT: Don't trust client headers directly
tenantID := ctx.Header("X-Tenant-ID") // ❌ Security risk!
```

#### `ctx.UserID() string`

**Purpose:** Get authenticated user ID  
**When to use:** After authentication  
**Source:** JWT claims

```go
// CORRECT: User-specific operations
func GetMyProfile(ctx *lift.Context) error {
    userID := ctx.UserID()
    if userID == "" {
        return lift.Unauthorized("Authentication required")
    }
    
    profile := getUserProfile(userID)
    return ctx.JSON(profile)
}
```

### State Management

#### `ctx.Set(key string, value interface{})`

**Purpose:** Store value in context  
**When to use:** Passing data between middleware  
**Scope:** Current request only

```go
// CORRECT: Middleware setting value
func AuthMiddleware(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        user := authenticateUser(ctx)
        ctx.Set("user", user)
        return next.Handle(ctx)
    })
}

// Handler reading value
func Handler(ctx *lift.Context) error {
    user := ctx.Get("user").(*User)
    // Use user
}
```

#### `ctx.Get(key string) interface{}`

**Purpose:** Retrieve value from context  
**When to use:** Reading middleware data  
**Returns:** Value or nil

```go
// CORRECT: Safe type assertion
if val := ctx.Get("user"); val != nil {
    user := val.(*User)
    // Use user
}

// With type check
user, ok := ctx.Get("user").(*User)
if !ok {
    return lift.Unauthorized("Authentication required")
}
```

### Logging

#### `ctx.Logger`

**Purpose:** Structured logger with request context  
**When to use:** All logging in handlers  
**Includes:** Request ID, tenant ID, user ID

```go
// CORRECT: Use context logger
ctx.Logger.Info("Processing request",
    "user_id", userID,
    "action", "create_order",
)

ctx.Logger.Error("Database error",
    "error", err,
    "query", query,
)

// INCORRECT: Don't use package logger
log.Println("Processing") // ❌ No context
```

## Handler Types

### Basic Handler

**Signature:** `func(ctx *lift.Context) error`  
**When to use:** Most handlers  
**Features:** Full control over request/response

```go
// CORRECT: Basic handler pattern
func GetUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    user, err := db.GetUser(userID)
    if err != nil {
        return lift.NotFound("user not found")
    }
    
    return ctx.JSON(user)
}
```

### SimpleHandler

**Signature:** `func(ctx *lift.Context, req T) (R, error)`  
**When to use:** Structured request/response  
**Features:** Automatic parsing and validation

```go
// CORRECT: Type-safe handler
type CreateRequest struct {
    Name string `json:"name" validate:"required"`
}

type CreateResponse struct {
    ID string `json:"id"`
}

app.POST("/users", lift.SimpleHandler(createUser))

func createUser(ctx *lift.Context, req CreateRequest) (CreateResponse, error) {
    // req is already parsed and validated
    user := CreateResponse{
        ID: generateID(),
    }
    return user, nil
}

// INCORRECT: Don't parse manually in SimpleHandler
func badCreate(ctx *lift.Context, req CreateRequest) (CreateResponse, error) {
    var realReq CreateRequest
    ctx.ParseRequest(&realReq) // ❌ Already parsed!
}
```

## Middleware

### Standard Middleware

#### `middleware.RequestID()`

**Purpose:** Generate unique request ID for tracing  
**When to use:** ALWAYS - should be first middleware  
**Adds to context:** Request ID for correlation

```go
// CORRECT: First in chain
app.Use(
    middleware.RequestID(), // First!
    middleware.Logger(),    // Uses request ID
)
```

#### `middleware.Logger()`

**Purpose:** Structured request/response logging  
**When to use:** All production applications  
**Logs:** Method, path, status, duration, request ID

```go
// CORRECT: After RequestID
app.Use(
    middleware.RequestID(),
    middleware.Logger(),
)

// Logs like:
// {"level":"info","method":"GET","path":"/users","status":200,"duration":"25ms","request_id":"abc-123"}
```

#### `middleware.Recover()`

**Purpose:** Catch panics and return 500 error  
**When to use:** All production applications  
**Prevents:** Lambda crash from panic

```go
// CORRECT: Protect against panics
app.Use(middleware.Recover())

// Catches:
func BadHandler(ctx *lift.Context) error {
    panic("something went wrong") // Returns 500 instead of crashing
}
```

### Security Middleware

#### `middleware.CORS(allowedOrigins []string)`

**Purpose:** Handle CORS headers  
**When to use:** Browser-facing APIs  
**Parameter:** Array of allowed origins

```go
// CORRECT: Configure CORS
app.Use(middleware.CORS([]string{"https://app.example.com", "https://www.example.com"}))

// Development - allow all
app.Use(middleware.CORS([]string{"*"}))
```

#### `middleware.JWT(config JWTConfig)`

**Purpose:** Validate JWT tokens  
**When to use:** Protected routes  
**Sets in context:** User claims

```go
// CORRECT: JWT protection
import "github.com/pay-theory/lift/pkg/middleware"

api := app.Group("/api")
api.Use(middleware.JWT(middleware.JWTConfig{
    Secret: os.Getenv("JWT_SECRET"),
}))

// Or use JWTAuth function
api.Use(middleware.JWTAuth(middleware.JWTConfig{
    Secret:    os.Getenv("JWT_SECRET"),
    Algorithm: "HS256",
    TokenLookup: "header:Authorization",
}))

// Access claims in handler
func Protected(ctx *lift.Context) error {
    claims := ctx.Get("claims").(jwt.MapClaims)
    userID := claims["user_id"].(string)
}
```

### Rate Limiting

#### `middleware.IPRateLimitWithLimited(limit int, window time.Duration) (lift.Middleware, error)`

**Purpose:** Rate limit by IP address  
**When to use:** Public endpoints  
**Storage:** DynamoDB via Limited library  
**Returns:** Middleware and error

```go
// CORRECT: Simple IP rate limiting
limiter, err := middleware.IPRateLimitWithLimited(
    1000,           // 1000 requests
    time.Hour,      // per hour
)
if err != nil {
    panic(err)
}

app.Use(limiter)
```

#### `middleware.UserRateLimitWithLimited(limit int, window time.Duration) (lift.Middleware, error)`

**Purpose:** Rate limit by authenticated user  
**When to use:** After authentication  
**Storage:** DynamoDB via Limited library  
**Returns:** Middleware and error

```go
// CORRECT: User-based limiting
api := app.Group("/api")
api.Use(middleware.JWT(jwtConfig))

userLimiter, err := middleware.UserRateLimitWithLimited(100, 15*time.Minute)
if err != nil {
    panic(err)
}
api.Use(userLimiter)
```

## Error Types

### Creating Errors

#### `lift.NewLiftError(code, message string, statusCode int) *LiftError`

**Purpose:** Create structured error with details  
**When to use:** All error responses  
**Returns:** Error with HTTP status code

```go
// CORRECT: Detailed error
return lift.NewLiftError("VALIDATION_ERROR", "Invalid input", 400).
    WithDetail("field", "email").
    WithDetail("reason", "invalid format")

// Response:
// {
//   "code": "VALIDATION_ERROR",
//   "message": "Invalid input",
//   "details": {
//     "field": "email",
//     "reason": "invalid format"
//   }
// }
```

### Convenience Error Functions

These functions create common HTTP errors:

#### `lift.Unauthorized(message string) *LiftError`
**Status:** 401  
**Use case:** Missing/invalid authentication

```go
if token == "" {
    return lift.Unauthorized("missing auth token")
}
```

#### `lift.NotFound(message string) *LiftError`
**Status:** 404  
**Use case:** Resource not found

```go
user, err := db.GetUser(id)
if err == sql.ErrNoRows {
    return lift.NotFound("user not found")
}
```

#### `lift.AuthorizationError(message string) *LiftError`
**Status:** 403  
**Use case:** Insufficient permissions

```go
if !user.IsAdmin {
    return lift.AuthorizationError("admin access required")
}
```

#### `lift.ValidationError(message string) *LiftError`
**Status:** 422  
**Use case:** Validation failures

```go
if !isValidEmail(email) {
    return lift.ValidationError("invalid email format")
}
```

#### Other Error Constructors

For other status codes, use `NewLiftError`:

```go
// 400 Bad Request
return lift.NewLiftError("BAD_REQUEST", "Invalid input", 400)

// 409 Conflict
return lift.NewLiftError("CONFLICT", "Resource already exists", 409)

// 500 Internal Server Error
return lift.NewLiftError("INTERNAL_ERROR", "Server error", 500)
```

## Testing Utilities

### Test App

#### `testing.NewTestApp() *TestApp`

**Purpose:** Create app for testing  
**When to use:** Unit tests  
**Features:** Same API as regular app

```go
import lifttesting "github.com/pay-theory/lift/pkg/testing"

func TestHandler(t *testing.T) {
    app := lifttesting.NewTestApp()
    app.POST("/users", CreateUser)
    
    // Test the handler
}
```

### Test Context

#### `testing.NewTestContext(opts ...TestOption) *TestContext`

**Purpose:** Create context for testing  
**When to use:** Testing individual handlers  
**Configurable:** Request, headers, body

```go
// CORRECT: Complete test
func TestCreateUser(t *testing.T) {
    ctx := lifttesting.NewTestContext(
        lifttesting.WithMethod("POST"),
        lifttesting.WithPath("/users"),
        lifttesting.WithBody(`{"name": "Alice"}`),
        lifttesting.WithHeaders(map[string]string{
            "Authorization": "Bearer token",
        }),
    )
    
    err := CreateUser(ctx)
    assert.NoError(t, err)
    assert.Equal(t, 201, ctx.Response.StatusCode)
}
```

## Best Practices Summary

<!-- AI Training: Key patterns to reinforce -->

### ✅ DO:
- Create app once at function scope
- Use middleware for cross-cutting concerns
- Use SimpleHandler for type safety
- Handle errors with lift error types
- Use context logger for all logging
- Test with lift testing utilities

### ❌ DON'T:
- Create app inside handlers
- Access raw Lambda events
- Parse JSON manually with SimpleHandler
- Expose internal errors to clients
- Use package-level logging
- Test by invoking real Lambda functions

## Complete Example

<!-- AI Training: Putting it all together -->
```go
package main

import (
    "os"
    "time"
    
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

// Request/Response types
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}

type UserResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    TenantID  string    `json:"tenant_id"`
    CreatedAt time.Time `json:"created_at"`
}

func main() {
    // Create app with production config
    app := lift.New()
    
    config := &lift.Config{
        MaxRequestSize:  10 * 1024 * 1024, // 10MB
        Timeout:         29,                // 29 seconds
        LogLevel:        "INFO",
        MetricsEnabled:  true,
        CORSEnabled:     true,
        AllowedOrigins:  []string{os.Getenv("ALLOWED_ORIGIN")},
    }
    app.WithConfig(config)
    
    // Global middleware
    app.Use(
        middleware.RequestID(),
        middleware.Logger(),
        middleware.Recover(),
    )
    
    // Public routes
    app.GET("/health", HealthCheck)
    
    // Protected API routes
    api := app.Group("/api")
    
    // JWT middleware
    jwtMiddleware, _ := middleware.JWTAuth(middleware.JWTConfig{
        Secret: os.Getenv("JWT_SECRET"),
    })
    api.Use(jwtMiddleware)
    
    // Rate limiting
    userLimiter, _ := middleware.UserRateLimitWithLimited(100, time.Hour)
    api.Use(userLimiter)
    
    // User routes
    api.GET("/users", ListUsers)
    api.GET("/users/:id", GetUser)
    api.POST("/users", lift.SimpleHandler(CreateUser))
    
    // Start Lambda
    lambda.Start(app.HandleRequest)
}

// Type-safe handler
func CreateUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Already validated
    user := UserResponse{
        ID:        generateID(),
        Name:      req.Name,
        Email:     req.Email,
        TenantID:  ctx.TenantID(),
        CreatedAt: time.Now(),
    }
    
    // Save to database
    if err := db.SaveUser(user); err != nil {
        ctx.Logger.Error("Failed to save user", "error", err)
        return UserResponse{}, lift.NewLiftError("DATABASE_ERROR", "Failed to create user", 500)
    }
    
    ctx.Logger.Info("User created", "user_id", user.ID)
    return user, nil
}
```

This API reference serves as comprehensive training data for AI assistants to generate correct Lift code patterns.