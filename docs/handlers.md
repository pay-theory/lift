# Handlers

Handlers are the core of your application logic in Lift. This guide covers all handler patterns, type safety features, and best practices.

## Handler Types

Lift supports multiple handler patterns to match your coding style and requirements.

### Basic Handler

The fundamental handler pattern that all other patterns build upon:

```go
func handleRequest(ctx *lift.Context) error {
    // Your business logic
    return ctx.JSON(response)
}

// Register the handler
app.GET("/users", handleRequest)
```

### Handler Interface

For more complex scenarios, implement the Handler interface:

```go
type Handler interface {
    Handle(ctx *Context) error
}

// Example implementation
type UserHandler struct {
    userService UserService
    logger      Logger
}

func (h *UserHandler) Handle(ctx *lift.Context) error {
    users, err := h.userService.GetUsers(ctx.TenantID())
    if err != nil {
        h.logger.Error("Failed to get users", err)
        return lift.InternalError("Failed to retrieve users")
    }
    
    return ctx.JSON(users)
}

// Register
handler := &UserHandler{userService, logger}
app.GET("/users", handler)
```

### HandlerFunc Type

Convert any function to a Handler:

```go
// HandlerFunc implements Handler interface
type HandlerFunc func(*Context) error

func (f HandlerFunc) Handle(ctx *Context) error {
    return f(ctx)
}

// Use inline
app.GET("/health", lift.HandlerFunc(func(ctx *lift.Context) error {
    return ctx.JSON(map[string]string{"status": "healthy"})
}))
```

## Type-Safe Handlers

Lift's most powerful feature is type-safe handlers with automatic parsing and validation.

### TypedHandler Pattern

```go
// Define your request and response types
type CreateUserRequest struct {
    Name     string `json:"name" validate:"required,min=3,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"min=18,max=120"`
    Role     string `json:"role" validate:"required,oneof=admin user guest"`
}

type UserResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

// Write your handler with typed parameters
func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // req is already parsed and validated!
    
    // Business logic
    user, err := userService.Create(req)
    if err != nil {
        return UserResponse{}, err
    }
    
    // Return typed response
    return UserResponse{
        ID:        user.ID,
        Name:      user.Name,
        Email:     user.Email,
        CreatedAt: user.CreatedAt,
    }, nil
}

// Register with TypedHandler wrapper
app.POST("/users", lift.TypedHandler(createUser))
```

### TypedHandler Benefits

1. **Automatic Parsing**: Request body is parsed automatically
2. **Validation**: Struct tags are validated before handler is called
3. **Type Safety**: Compile-time type checking
4. **Clean Code**: No manual parsing or validation code
5. **Error Handling**: Validation errors return 400 automatically

### Advanced TypedHandler

```go
// Handler with custom error types
func updateUser(ctx *lift.Context, req UpdateUserRequest) (UserResponse, error) {
    // Check permissions
    if !ctx.HasPermission("user:update") {
        return UserResponse{}, lift.Forbidden("Insufficient permissions")
    }
    
    // Check if user exists
    user, err := userService.Get(req.ID)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return UserResponse{}, lift.NotFound("User not found")
        }
        return UserResponse{}, err
    }
    
    // Update user
    updated, err := userService.Update(user, req)
    if err != nil {
        return UserResponse{}, lift.InternalError("Failed to update user")
    }
    
    return mapToResponse(updated), nil
}
```

## Request Parsing

Multiple ways to parse request data based on your needs.

### JSON Parsing

```go
func handleJSON(ctx *lift.Context) error {
    var req CreateOrderRequest
    
    // Basic parsing
    if err := ctx.ParseJSON(&req); err != nil {
        return lift.BadRequest("Invalid JSON: " + err.Error())
    }
    
    // Parse and validate
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err // Automatic 400 with validation details
    }
    
    // Process request...
    return ctx.JSON(response)
}
```

### Form Data Parsing

```go
func handleForm(ctx *lift.Context) error {
    // Parse form values
    name := ctx.FormValue("name")
    email := ctx.FormValue("email")
    
    // Parse into struct
    var form ContactForm
    if err := ctx.ParseForm(&form); err != nil {
        return lift.BadRequest("Invalid form data")
    }
    
    return ctx.JSON(response)
}
```

### Query Parameters

```go
func handleQuery(ctx *lift.Context) error {
    // Single values
    page := ctx.Query("page")
    limit := ctx.Query("limit")
    
    // With defaults
    pageNum := ctx.QueryInt("page", 1)
    limitNum := ctx.QueryInt("limit", 20)
    
    // Multiple values
    tags := ctx.QueryArray("tag") // ?tag=go&tag=lambda
    
    // Parse into struct
    var filters FilterParams
    if err := ctx.ParseQuery(&filters); err != nil {
        return lift.BadRequest("Invalid query parameters")
    }
    
    return ctx.JSON(results)
}
```

### Path Parameters

```go
// Route: /users/:id/posts/:postId
func getPost(ctx *lift.Context) error {
    userID := ctx.Param("id")
    postID := ctx.Param("postId")
    
    // Type conversion with validation
    userIDInt, err := ctx.ParamInt("id")
    if err != nil {
        return lift.BadRequest("Invalid user ID")
    }
    
    post, err := postService.Get(userIDInt, postID)
    if err != nil {
        return lift.NotFound("Post not found")
    }
    
    return ctx.JSON(post)
}
```

### Headers

```go
func handleHeaders(ctx *lift.Context) error {
    // Get header
    auth := ctx.Header("Authorization")
    contentType := ctx.Header("Content-Type")
    
    // Check header existence
    if auth == "" {
        return lift.Unauthorized("Authorization required")
    }
    
    // Parse custom headers
    apiVersion := ctx.Header("X-API-Version")
    if apiVersion != "v2" {
        return lift.BadRequest("API version not supported")
    }
    
    return ctx.JSON(response)
}
```

## Response Building

Lift provides a fluent API for building responses.

### JSON Responses

```go
func handleJSON(ctx *lift.Context) error {
    user := getUserData()
    
    // Simple JSON
    return ctx.JSON(user)
    
    // With status code
    return ctx.Status(201).JSON(user)
    
    // With headers
    return ctx.
        Status(201).
        Header("X-Total-Count", "100").
        Header("Cache-Control", "no-cache").
        JSON(user)
}
```

### Text Responses

```go
func handleText(ctx *lift.Context) error {
    // Plain text
    return ctx.Text("Hello, World!")
    
    // HTML
    return ctx.HTML("<h1>Welcome</h1>")
    
    // XML
    return ctx.XML(xmlData)
}
```

### Binary Responses

```go
func handleBinary(ctx *lift.Context) error {
    imageData, err := loadImage()
    if err != nil {
        return lift.NotFound("Image not found")
    }
    
    // Set appropriate content type
    return ctx.
        Header("Content-Type", "image/png").
        Header("Cache-Control", "max-age=3600").
        Binary(imageData)
}
```

### No Content

```go
func handleDelete(ctx *lift.Context) error {
    id := ctx.Param("id")
    
    if err := service.Delete(id); err != nil {
        return lift.NotFound("Resource not found")
    }
    
    // Returns 204 No Content
    return ctx.NoContent()
}
```

### Redirects

```go
func handleRedirect(ctx *lift.Context) error {
    // Temporary redirect (302)
    return ctx.Redirect("/new-location")
    
    // Permanent redirect (301)
    return ctx.RedirectPermanent("/new-location")
    
    // Custom status redirect
    return ctx.Status(307).Header("Location", "/new-location").Text("")
}
```

## Error Handling

Proper error handling is crucial for good API design.

### Built-in Error Types

```go
func handleErrors(ctx *lift.Context) error {
    // 400 Bad Request
    if !isValid {
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
    if isDuplicate {
        return lift.Conflict("Resource already exists")
    }
    
    // 429 Too Many Requests
    if rateLimited {
        return lift.TooManyRequests("Rate limit exceeded")
    }
    
    // 500 Internal Server Error
    if serverError {
        return lift.InternalError("Something went wrong")
    }
    
    // 503 Service Unavailable
    if !serviceAvailable {
        return lift.ServiceUnavailable("Service temporarily unavailable")
    }
    
    return ctx.JSON(response)
}
```

### Custom Error Types

```go
// Define custom error
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Code    string `json:"code"`
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

// Custom error with status code
type APIError struct {
    StatusCode int
    Code       string
    Message    string
    Details    interface{}
}

func (e APIError) Error() string {
    return e.Message
}

func (e APIError) Status() int {
    return e.StatusCode
}

// Use in handler
func handleRequest(ctx *lift.Context) error {
    if err := validate(input); err != nil {
        return APIError{
            StatusCode: 400,
            Code:       "VALIDATION_ERROR",
            Message:    "Validation failed",
            Details:    err,
        }
    }
    
    return ctx.JSON(response)
}
```

### Error Response Format

Lift automatically formats errors as JSON:

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Validation failed",
        "details": {
            "field": "email",
            "reason": "invalid format"
        },
        "request_id": "req_123abc",
        "timestamp": 1234567890
    }
}
```

## Handler Composition

Compose handlers for reusable functionality.

### Middleware as Handlers

```go
// Authentication handler
func requireAuth(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        token := ctx.Header("Authorization")
        if token == "" {
            return lift.Unauthorized("Authentication required")
        }
        
        user, err := validateToken(token)
        if err != nil {
            return lift.Unauthorized("Invalid token")
        }
        
        ctx.Set("user", user)
        return next.Handle(ctx)
    })
}

// Use with routes
app.GET("/protected", requireAuth(protectedHandler))
```

### Handler Chains

```go
// Chain multiple handlers
func chainHandlers(handlers ...lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        for _, handler := range handlers {
            if err := handler.Handle(ctx); err != nil {
                return err
            }
        }
        return nil
    })
}

// Use case: validation -> authorization -> business logic
app.POST("/admin/users", chainHandlers(
    validateAdminRequest,
    requireAdminRole,
    createUser,
))
```

### Conditional Handlers

```go
func conditionalHandler(condition func(*lift.Context) bool, ifTrue, ifFalse lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        if condition(ctx) {
            return ifTrue.Handle(ctx)
        }
        return ifFalse.Handle(ctx)
    })
}

// Example: Different handlers for different API versions
app.GET("/users", conditionalHandler(
    func(ctx *lift.Context) bool {
        return ctx.Header("API-Version") == "v2"
    },
    getUsersV2,
    getUsersV1,
))
```

## Context Usage

The Context object provides rich functionality for handlers.

### State Management

```go
func handler(ctx *lift.Context) error {
    // Set values
    ctx.Set("request_time", time.Now())
    ctx.Set("processed", true)
    
    // Get values
    if processed, ok := ctx.Get("processed").(bool); ok && processed {
        // Already processed
    }
    
    // Type-safe getters
    user := ctx.MustGet("user").(*User)
    
    return ctx.JSON(response)
}
```

### Multi-Tenant Context

```go
func tenantHandler(ctx *lift.Context) error {
    // Access tenant information
    tenantID := ctx.TenantID()
    userID := ctx.UserID()
    
    // Tenant-scoped operations
    data := getTenantData(tenantID)
    
    // Audit logging
    ctx.Logger.Info("Data accessed", map[string]interface{}{
        "tenant_id": tenantID,
        "user_id":   userID,
        "action":    "read",
    })
    
    return ctx.JSON(data)
}
```

### Request Metadata

```go
func metadataHandler(ctx *lift.Context) error {
    // Request ID for tracing
    requestID := ctx.RequestID()
    
    // Event source information
    trigger := ctx.Request.TriggerType
    
    // Lambda context
    if lambdaCtx, ok := ctx.Get("lambda_context").(context.Context); ok {
        deadline, _ := lambdaCtx.Deadline()
        ctx.Logger.Info("Request deadline", map[string]interface{}{
            "deadline": deadline,
        })
    }
    
    return ctx.JSON(map[string]interface{}{
        "request_id": requestID,
        "trigger":    trigger,
    })
}
```

## Async Handlers

Handle asynchronous operations properly.

### Goroutine Management

```go
func asyncHandler(ctx *lift.Context) error {
    // Create a copy of context for goroutines
    asyncCtx := ctx.Copy()
    
    // Start async operation
    go func() {
        // Use the copied context
        asyncCtx.Logger.Info("Processing async task")
        
        // Perform async work
        processInBackground(asyncCtx)
    }()
    
    // Return immediately
    return ctx.JSON(map[string]string{
        "status": "processing",
        "message": "Task queued for processing",
    })
}
```

### Concurrent Operations

```go
func concurrentHandler(ctx *lift.Context) error {
    var wg sync.WaitGroup
    results := make(chan Result, 3)
    errors := make(chan error, 3)
    
    // Launch concurrent operations
    operations := []func() (Result, error){
        fetchUserData,
        fetchOrderData,
        fetchAnalytics,
    }
    
    for _, op := range operations {
        wg.Add(1)
        go func(operation func() (Result, error)) {
            defer wg.Done()
            result, err := operation()
            if err != nil {
                errors <- err
                return
            }
            results <- result
        }(op)
    }
    
    // Wait for completion
    wg.Wait()
    close(results)
    close(errors)
    
    // Check for errors
    if len(errors) > 0 {
        return lift.InternalError("Failed to fetch data")
    }
    
    // Collect results
    var allResults []Result
    for result := range results {
        allResults = append(allResults, result)
    }
    
    return ctx.JSON(allResults)
}
```

## Best Practices

### 1. Use TypedHandler When Possible

```go
// GOOD: Type-safe, automatic validation
func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Business logic only
    return userService.Create(req)
}

// AVOID: Manual parsing and validation
func createUser(ctx *lift.Context) error {
    var req CreateUserRequest
    if err := ctx.ParseJSON(&req); err != nil {
        return lift.BadRequest("Invalid JSON")
    }
    if err := validate(req); err != nil {
        return lift.BadRequest(err.Error())
    }
    // Business logic
}
```

### 2. Return Appropriate Error Types

```go
func handler(ctx *lift.Context) error {
    resource, err := service.Get(id)
    if err != nil {
        // Use specific error types
        if errors.Is(err, ErrNotFound) {
            return lift.NotFound("Resource not found")
        }
        if errors.Is(err, ErrUnauthorized) {
            return lift.Unauthorized("Access denied")
        }
        // Generic error for unexpected issues
        return lift.InternalError("Failed to retrieve resource")
    }
    
    return ctx.JSON(resource)
}
```

### 3. Keep Handlers Focused

```go
// GOOD: Single responsibility
func getUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    user, err := userService.Get(userID)
    if err != nil {
        return lift.NotFound("User not found")
    }
    return ctx.JSON(user)
}

// AVOID: Doing too much in one handler
func handler(ctx *lift.Context) error {
    // Authentication
    // Validation
    // Business logic
    // Logging
    // Response formatting
    // etc...
}
```

### 4. Use Middleware for Cross-Cutting Concerns

```go
// GOOD: Reusable middleware
app.Use(middleware.Logger())
app.Use(middleware.Auth())
app.Use(middleware.RateLimit())

app.GET("/users", getUsers) // Clean handler

// AVOID: Repeating logic in handlers
func getUsers(ctx *lift.Context) error {
    // Log request
    // Check auth
    // Check rate limit
    // Actual logic
}
```

### 5. Handle Partial Failures

```go
func batchHandler(ctx *lift.Context) error {
    var successCount int
    var errors []error
    
    for _, item := range items {
        if err := processItem(item); err != nil {
            errors = append(errors, err)
            ctx.Logger.Error("Failed to process item", map[string]interface{}{
                "item_id": item.ID,
                "error":   err.Error(),
            })
        } else {
            successCount++
        }
    }
    
    // Return partial success information
    return ctx.JSON(map[string]interface{}{
        "processed": successCount,
        "failed":    len(errors),
        "errors":    errors,
    })
}
```

## Summary

Lift handlers provide:

- **Multiple Patterns**: Choose the style that fits your needs
- **Type Safety**: Compile-time type checking with TypedHandler
- **Automatic Validation**: Struct tag validation
- **Rich Context**: Access to request data, logging, metrics
- **Error Handling**: Structured error responses
- **Composition**: Build complex handlers from simple ones

Master these patterns to build robust, maintainable serverless applications. 