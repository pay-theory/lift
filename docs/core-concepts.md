# Core Concepts

Understanding Lift's core concepts will help you build better serverless applications. This guide covers the fundamental building blocks and architectural decisions behind Lift.

## Architecture Overview

Lift follows a middleware-based architecture similar to popular web frameworks, adapted for the serverless environment:

```
Lambda Event → Event Adapter → Router → Middleware Chain → Handler → Response
```

### Event Flow

1. **Lambda Event**: Raw event from AWS Lambda (API Gateway, SQS, S3, etc.)
2. **Event Adapter**: Converts various event formats into unified Request
3. **Router**: Matches request to appropriate handler
4. **Middleware Chain**: Processes request through middleware stack
5. **Handler**: Your business logic
6. **Response**: Formatted response appropriate for the event source

## The App Container

The App is the central container that orchestrates your Lambda function:

```go
app := lift.New()

// Configure the app
app.Use(middleware.Logger())
app.GET("/users", getUsers)
app.POST("/users", createUser)

// Start handling Lambda events
lambda.Start(app.HandleRequest)
```

### Key Responsibilities

- Route registration and matching
- Middleware management
- Event detection and adaptation
- Error handling coordination
- Configuration management

## Context

The Context is the heart of every request in Lift. It provides a rich set of utilities for handling requests:

```go
type Context struct {
    // Core fields
    Request  *Request
    Response *Response
    Logger   Logger
    Metrics  MetricsCollector
    
    // Private fields for state management
    // ...
}
```

### Context Methods

```go
// Request data access
userID := ctx.Param("id")        // Path parameters
search := ctx.Query("q")          // Query parameters
auth := ctx.Header("Authorization") // Headers
ctx.ParseJSON(&data)              // Parse body

// Response helpers
ctx.JSON(data)                    // JSON response
ctx.Status(201).JSON(data)        // With status
ctx.Text("Hello")                 // Text response
ctx.NoContent()                   // 204 No Content

// Multi-tenant support
tenantID := ctx.TenantID()
userID := ctx.UserID()

// State management
ctx.Set("key", value)
value := ctx.Get("key")

// Error handling
ctx.Error("Something went wrong")
```

### Context Lifecycle

1. Created when request arrives
2. Populated with request data
3. Enhanced by middleware
4. Passed to handler
5. Cleaned up after response

## Handlers

Handlers are the core of your business logic. Lift supports multiple handler patterns:

### Basic Handler

```go
func handleRequest(ctx *lift.Context) error {
    // Process request
    return ctx.JSON(response)
}
```

### Typed Handler

```go
func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Automatic parsing and validation
    user := processUser(req)
    return UserResponse{ID: user.ID}, nil
}

// Register with TypedHandler wrapper
app.POST("/users", lift.TypedHandler(createUser))
```

### Handler Interface

```go
type Handler interface {
    Handle(ctx *Context) error
}

// Custom handler implementation
type CustomHandler struct {
    service UserService
}

func (h *CustomHandler) Handle(ctx *lift.Context) error {
    // Custom logic
    return ctx.JSON(result)
}
```

## Request and Response

### Request Structure

```go
type Request struct {
    // Common fields
    TriggerType TriggerType
    Method      string
    Path        string
    Headers     map[string]string
    Query       map[string]string
    Body        []byte
    
    // Multi-tenant fields
    TenantID    string
    UserID      string
    
    // Event-specific data
    Records     []interface{}         // For batch events
    Metadata    map[string]interface{} // Event metadata
}
```

### Response Structure

```go
type Response struct {
    StatusCode int
    Headers    map[string]string
    Body       interface{}
}
```

### Response Builder Pattern

```go
// Fluent API for building responses
ctx.Status(201).
    Header("X-Request-ID", requestID).
    JSON(data)
```

## Event Adapters

Event adapters normalize different Lambda event sources into a unified format:

### Adapter Interface

```go
type EventAdapter interface {
    CanHandle(event interface{}) bool
    Adapt(event interface{}) (*Request, error)
}
```

### Built-in Adapters

- **API Gateway V1/V2**: HTTP requests
- **SQS**: Queue messages with batch support
- **S3**: Object events
- **EventBridge**: Custom events
- **WebSocket**: Connection and message events
- **Scheduled**: CloudWatch scheduled events

### Adapter Registry

```go
// Adapters are automatically registered
registry := adapters.NewAdapterRegistry()
registry.Register(NewAPIGatewayV2Adapter())
registry.Register(NewSQSAdapter())
// ... more adapters

// Automatic detection
request, err := registry.DetectAndAdapt(lambdaEvent)
```

## Routing

Lift's router provides efficient request matching with support for path parameters:

### Route Registration

```go
// HTTP-style routes
app.GET("/users", getUsers)
app.POST("/users", createUser)
app.PUT("/users/:id", updateUser)
app.DELETE("/users/:id", deleteUser)

// Generic handler for any method
app.Handle("ANY", "/webhook", handleWebhook)

// Non-HTTP events
app.Handle("SQS", "/process-queue", processSQS)
app.Handle("S3", "/process-upload", processS3)
```

### Path Parameters

```go
// Route: /users/:id/posts/:postId
func getPost(ctx *lift.Context) error {
    userID := ctx.Param("id")
    postID := ctx.Param("postId")
    
    // Fetch and return post
    return ctx.JSON(post)
}
```

### Route Matching

1. Exact matches have priority
2. Pattern matches are checked next
3. First matching route wins
4. 404 if no route matches

## Middleware

Middleware provides a powerful way to add cross-cutting concerns:

### Middleware Interface

```go
type Middleware func(Handler) Handler
```

### Creating Middleware

```go
func TimingMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            start := time.Now()
            
            // Call next handler
            err := next.Handle(ctx)
            
            // Log timing
            duration := time.Since(start)
            ctx.Logger.Info("Request completed", map[string]interface{}{
                "duration": duration,
                "path":     ctx.Request.Path,
            })
            
            return err
        })
    }
}
```

### Middleware Chain

Middleware executes in the order it's added:

```go
app.Use(middleware.Logger())      // 1st
app.Use(middleware.Auth())        // 2nd
app.Use(middleware.RateLimit())   // 3rd

// Request flow: Logger → Auth → RateLimit → Handler
// Response flow: Handler → RateLimit → Auth → Logger
```

## Error Handling

Lift provides structured error handling with automatic HTTP status codes:

### Error Types

```go
// Built-in error constructors
lift.BadRequest("Invalid input")           // 400
lift.Unauthorized("Invalid credentials")    // 401
lift.Forbidden("Access denied")            // 403
lift.NotFound("Resource not found")        // 404
lift.InternalError("Something went wrong")  // 500

// Custom errors
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
```

### Error Response Format

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Validation failed",
        "details": {
            "field": "email",
            "reason": "invalid format"
        },
        "request_id": "req_123",
        "timestamp": 1234567890
    }
}
```

## Type Safety

Lift leverages Go's type system for compile-time safety:

### Generic Handlers

```go
// Type-safe handler with automatic parsing
func TypedHandler[Req any, Resp any](
    handler func(*Context, Req) (Resp, error),
) Handler
```

### Validation

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}

// Automatic validation with struct tags
func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // req is already validated
    return UserResponse{ID: "123"}, nil
}
```

## Multi-Tenancy

Lift has first-class support for multi-tenant applications:

### Tenant Isolation

```go
// Middleware sets tenant context
func TenantMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Extract tenant from JWT/header/path
            tenantID := extractTenant(ctx)
            ctx.SetTenantID(tenantID)
            
            return next.Handle(ctx)
        })
    }
}

// Handlers access tenant-scoped data
func getUsers(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    users := getUsersForTenant(tenantID)
    return ctx.JSON(users)
}
```

### Database Integration

```go
// DynamORM automatically scopes queries
db := dynamorm.FromContext(ctx)
// All queries automatically filtered by tenant
```

## Performance Considerations

### Zero Allocation Goals

Lift is designed to minimize allocations:

- Route matching uses efficient algorithms
- Middleware chains are composed at startup
- Context pooling reduces GC pressure
- Response building minimizes copies

### Benchmarks

```
BenchmarkRouting-8          5000000   250 ns/op    0 B/op   0 allocs/op
BenchmarkMiddleware-8       3000000   450 ns/op   48 B/op   1 allocs/op
BenchmarkFullRequest-8      1000000  1050 ns/op  320 B/op   8 allocs/op
```

### Best Practices

1. **Minimize middleware**: Only use what you need
2. **Pool resources**: Use connection pooling
3. **Cache when possible**: Cache expensive operations
4. **Profile regularly**: Use benchmarks and pprof

## Configuration

### App Configuration

```go
type Config struct {
    AppName        string
    Environment    string
    LogLevel       string
    EnableMetrics  bool
    EnableTracing  bool
    DefaultTimeout time.Duration
    
    // Custom settings
    Custom map[string]interface{}
}
```

### Environment-based Config

```go
config := lift.Config{
    Environment: os.Getenv("LIFT_ENV"),
    LogLevel:    os.Getenv("LIFT_LOG_LEVEL"),
}

if config.Environment == "production" {
    config.EnableMetrics = true
    config.EnableTracing = true
}
```

## Dependency Injection

While Lift doesn't enforce a DI pattern, it supports clean dependency management:

```go
// Service initialization
userService := services.NewUserService(db)
authService := services.NewAuthService(jwt)

// Option 1: Closure-based handlers
app.POST("/users", func(ctx *lift.Context) error {
    return userService.CreateUser(ctx)
})

// Option 2: Struct-based handlers
handler := &UserHandler{
    userService: userService,
    authService: authService,
}
app.POST("/users", handler.CreateUser)

// Option 3: Context-based services
app.Use(func(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        ctx.Set("userService", userService)
        return next.Handle(ctx)
    })
})
```

## Summary

Lift's core concepts work together to provide a powerful, type-safe, and performant framework for Lambda development:

- **App Container**: Orchestrates your application
- **Context**: Rich request/response handling
- **Handlers**: Your business logic with type safety
- **Event Adapters**: Unified interface for all Lambda triggers
- **Middleware**: Composable, reusable functionality
- **Error Handling**: Structured, consistent error responses
- **Multi-tenancy**: Built-in tenant isolation

Understanding these concepts will help you build robust serverless applications with Lift. 