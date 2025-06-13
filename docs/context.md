# Context

The Context object is the heart of every request in Lift. It provides a unified interface for accessing request data, building responses, logging, metrics, and managing request-scoped state.

## Overview

Every handler receives a Context that encapsulates:

- Request data (headers, body, parameters)
- Response builder
- Logging with request correlation
- Metrics collection
- Multi-tenant information
- Request-scoped state storage
- Event source metadata

## Basic Usage

```go
func handler(ctx *lift.Context) error {
    // Access request data
    userID := ctx.Param("id")
    
    // Log with context
    ctx.Logger.Info("Fetching user", map[string]interface{}{
        "user_id": userID,
    })
    
    // Get data
    user, err := userService.Get(userID)
    if err != nil {
        return lift.NotFound("User not found")
    }
    
    // Return response
    return ctx.JSON(user)
}
```

## Request Data Access

### Path Parameters

```go
// Route: /users/:id/posts/:postId
func getPost(ctx *lift.Context) error {
    // String parameters
    userID := ctx.Param("id")
    postID := ctx.Param("postId")
    
    // Typed parameters with error handling
    userIDInt, err := ctx.ParamInt("id")
    if err != nil {
        return lift.BadRequest("Invalid user ID")
    }
    
    // With validation
    if userID == "" {
        return lift.BadRequest("User ID required")
    }
    
    return ctx.JSON(response)
}
```

### Query Parameters

```go
func listUsers(ctx *lift.Context) error {
    // Single value
    search := ctx.Query("q")
    sort := ctx.Query("sort")
    
    // With defaults
    page := ctx.QueryInt("page", 1)
    limit := ctx.QueryInt("limit", 20)
    
    // Multiple values
    tags := ctx.QueryArray("tag") // ?tag=go&tag=serverless
    
    // Boolean values
    includeDeleted := ctx.QueryBool("include_deleted", false)
    
    // Parse into struct
    var filters FilterParams
    if err := ctx.ParseQuery(&filters); err != nil {
        return lift.BadRequest("Invalid query parameters")
    }
    
    return ctx.JSON(users)
}
```

### Headers

```go
func handleRequest(ctx *lift.Context) error {
    // Get header value
    auth := ctx.Header("Authorization")
    contentType := ctx.Header("Content-Type")
    
    // Case-insensitive
    userAgent := ctx.Header("User-Agent")
    
    // Check existence
    if auth == "" {
        return lift.Unauthorized("Authorization header required")
    }
    
    // Custom headers
    apiVersion := ctx.Header("X-API-Version")
    requestID := ctx.Header("X-Request-ID")
    
    return ctx.JSON(response)
}
```

### Request Body

```go
// JSON body
func createUser(ctx *lift.Context) error {
    var user CreateUserRequest
    
    // Parse JSON
    if err := ctx.ParseJSON(&user); err != nil {
        return lift.BadRequest("Invalid JSON: " + err.Error())
    }
    
    // Parse and validate
    if err := ctx.ParseAndValidate(&user); err != nil {
        return err // Automatic 400 with validation errors
    }
    
    return ctx.JSON(createdUser)
}

// Form data
func handleForm(ctx *lift.Context) error {
    // Individual form values
    username := ctx.FormValue("username")
    password := ctx.FormValue("password")
    
    // Parse into struct
    var form LoginForm
    if err := ctx.ParseForm(&form); err != nil {
        return lift.BadRequest("Invalid form data")
    }
    
    return ctx.JSON(response)
}

// Raw body
func handleRaw(ctx *lift.Context) error {
    body := ctx.Body() // []byte
    
    // Process raw data
    return ctx.JSON(response)
}
```

### Cookies

```go
func handleCookies(ctx *lift.Context) error {
    // Read cookie
    sessionID, err := ctx.Cookie("session_id")
    if err != nil {
        // Cookie not found
        return lift.Unauthorized("Session required")
    }
    
    // Set cookie (via headers)
    ctx.Header("Set-Cookie", "session_id=abc123; HttpOnly; Secure; SameSite=Strict")
    
    return ctx.JSON(response)
}
```

## Response Building

### JSON Responses

```go
func jsonExamples(ctx *lift.Context) error {
    // Simple JSON
    return ctx.JSON(map[string]interface{}{
        "message": "Success",
        "data":    userData,
    })
    
    // With status code
    return ctx.Status(201).JSON(createdResource)
    
    // Structured response
    type Response struct {
        Success bool        `json:"success"`
        Data    interface{} `json:"data,omitempty"`
        Error   string      `json:"error,omitempty"`
    }
    
    return ctx.JSON(Response{
        Success: true,
        Data:    results,
    })
}
```

### Status Codes

```go
func statusExamples(ctx *lift.Context) error {
    // Success responses
    ctx.Status(200) // OK (default)
    ctx.Status(201) // Created
    ctx.Status(204) // No Content
    
    // Client errors
    ctx.Status(400) // Bad Request
    ctx.Status(401) // Unauthorized
    ctx.Status(403) // Forbidden
    ctx.Status(404) // Not Found
    
    // Server errors
    ctx.Status(500) // Internal Server Error
    ctx.Status(503) // Service Unavailable
    
    // Chaining
    return ctx.Status(201).
        Header("Location", "/users/123").
        JSON(created)
}
```

### Headers

```go
func headerExamples(ctx *lift.Context) error {
    // Set single header
    ctx.Header("X-Request-ID", requestID)
    
    // Multiple headers
    ctx.Header("Cache-Control", "max-age=3600")
    ctx.Header("X-Total-Count", "1000")
    ctx.Header("X-Page", "1")
    
    // Content type
    ctx.Header("Content-Type", "application/json; charset=utf-8")
    
    // CORS headers
    ctx.Header("Access-Control-Allow-Origin", "*")
    ctx.Header("Access-Control-Allow-Methods", "GET, POST")
    
    return ctx.JSON(data)
}
```

### Different Response Types

```go
// Text response
func textResponse(ctx *lift.Context) error {
    return ctx.Text("Hello, World!")
}

// HTML response
func htmlResponse(ctx *lift.Context) error {
    return ctx.HTML("<h1>Welcome</h1><p>Hello, World!</p>")
}

// XML response
func xmlResponse(ctx *lift.Context) error {
    return ctx.XML(xmlData)
}

// Binary response
func binaryResponse(ctx *lift.Context) error {
    imageData := loadImage()
    return ctx.Header("Content-Type", "image/png").Binary(imageData)
}

// No content
func deleteResource(ctx *lift.Context) error {
    // Delete operation
    return ctx.NoContent() // 204
}

// Redirect
func redirectResponse(ctx *lift.Context) error {
    return ctx.Redirect("/new-location") // 302
    // or
    return ctx.RedirectPermanent("/new-location") // 301
}
```

### Response Building Pattern

```go
func fluentResponse(ctx *lift.Context) error {
    // Build response fluently
    return ctx.
        Status(201).
        Header("X-Request-ID", ctx.RequestID()).
        Header("Cache-Control", "no-cache").
        Header("X-RateLimit-Remaining", "99").
        JSON(map[string]interface{}{
            "id":        "123",
            "created":   time.Now(),
            "message":   "Resource created successfully",
        })
}
```

## State Management

### Setting and Getting Values

```go
func stateManagement(ctx *lift.Context) error {
    // Set values
    ctx.Set("user_id", "123")
    ctx.Set("start_time", time.Now())
    ctx.Set("processed", true)
    
    // Get values with type assertion
    userID, ok := ctx.Get("user_id").(string)
    if !ok {
        return lift.InternalError("User ID not found")
    }
    
    // Get with default
    processed := ctx.GetBool("processed", false)
    count := ctx.GetInt("count", 0)
    name := ctx.GetString("name", "anonymous")
    
    // Must get (panics if not found)
    user := ctx.MustGet("user").(*User)
    
    return ctx.JSON(response)
}
```

### Request-Scoped Data

```go
// Middleware sets data
func authMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            user, err := authenticate(ctx)
            if err != nil {
                return lift.Unauthorized("Invalid credentials")
            }
            
            // Set user for handlers
            ctx.Set("user", user)
            ctx.SetUserID(user.ID)
            ctx.SetTenantID(user.TenantID)
            
            return next.Handle(ctx)
        })
    }
}

// Handler uses data
func protectedHandler(ctx *lift.Context) error {
    user := ctx.Get("user").(*User)
    userID := ctx.UserID()
    tenantID := ctx.TenantID()
    
    // Use authenticated user data
    return ctx.JSON(getUserData(user))
}
```

## Multi-Tenant Support

### Tenant Context

```go
func tenantHandler(ctx *lift.Context) error {
    // Access tenant information
    tenantID := ctx.TenantID()
    if tenantID == "" {
        return lift.BadRequest("Tenant ID required")
    }
    
    // User within tenant
    userID := ctx.UserID()
    
    // Tenant-scoped operations
    db := getDatabase(tenantID)
    users := db.GetUsers()
    
    // Audit log with tenant context
    ctx.Logger.Info("Users accessed", map[string]interface{}{
        "tenant_id": tenantID,
        "user_id":   userID,
        "count":     len(users),
    })
    
    return ctx.JSON(users)
}
```

### Setting Tenant Context

```go
func tenantMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Extract from subdomain
            host := ctx.Header("Host")
            tenantID := extractTenantFromHost(host)
            
            // Or from header
            if tenantID == "" {
                tenantID = ctx.Header("X-Tenant-ID")
            }
            
            // Or from JWT claims
            if tenantID == "" {
                if claims, ok := ctx.Get("claims").(jwt.MapClaims); ok {
                    tenantID = claims["tenant_id"].(string)
                }
            }
            
            if tenantID == "" {
                return lift.BadRequest("Tenant identification required")
            }
            
            ctx.SetTenantID(tenantID)
            return next.Handle(ctx)
        })
    }
}
```

## Logging

### Structured Logging

```go
func loggingExamples(ctx *lift.Context) error {
    // Basic logging
    ctx.Logger.Info("Processing request")
    ctx.Logger.Debug("Debug information")
    ctx.Logger.Warn("Warning message")
    ctx.Logger.Error("Error occurred")
    
    // With fields
    ctx.Logger.Info("User action", map[string]interface{}{
        "action":    "login",
        "user_id":   userID,
        "ip":        ctx.ClientIP(),
        "timestamp": time.Now(),
    })
    
    // Error logging
    if err := someOperation(); err != nil {
        ctx.Logger.Error("Operation failed", map[string]interface{}{
            "error":     err.Error(),
            "operation": "someOperation",
            "retry":     true,
        })
        return lift.InternalError("Operation failed")
    }
    
    return ctx.JSON(response)
}
```

### Request Correlation

```go
func correlatedLogging(ctx *lift.Context) error {
    // All logs automatically include request ID
    requestID := ctx.RequestID()
    
    ctx.Logger.Info("Starting process", map[string]interface{}{
        "step": "init",
    })
    
    // Logs across services can be correlated
    result, err := callService(requestID)
    if err != nil {
        ctx.Logger.Error("Service call failed", map[string]interface{}{
            "service": "user-service",
            "error":   err.Error(),
        })
    }
    
    ctx.Logger.Info("Process complete", map[string]interface{}{
        "step":     "complete",
        "duration": time.Since(start),
    })
    
    return ctx.JSON(result)
}
```

## Metrics

### Recording Metrics

```go
func metricsExamples(ctx *lift.Context) error {
    // Count metric
    ctx.Metrics.Count("api.requests", 1, map[string]string{
        "endpoint": ctx.Request.Path,
        "method":   ctx.Request.Method,
    })
    
    // Gauge metric
    activeUsers := getActiveUserCount()
    ctx.Metrics.Gauge("users.active", float64(activeUsers))
    
    // Timing metric
    start := time.Now()
    result := expensiveOperation()
    ctx.Metrics.Timing("operation.duration", time.Since(start), map[string]string{
        "operation": "expensive",
    })
    
    // Custom metric
    ctx.Metrics.Record("custom.metric", map[string]interface{}{
        "value": 123,
        "tags": map[string]string{
            "environment": ctx.Environment(),
            "tenant":      ctx.TenantID(),
        },
    })
    
    return ctx.JSON(result)
}
```

## Request Metadata

### Accessing Lambda Context

```go
func lambdaContext(ctx *lift.Context) error {
    // Lambda deadline
    if lambdaCtx, ok := ctx.Get("lambda_context").(context.Context); ok {
        deadline, exists := lambdaCtx.Deadline()
        if exists {
            remaining := time.Until(deadline)
            ctx.Logger.Info("Time remaining", map[string]interface{}{
                "remaining": remaining,
            })
        }
    }
    
    // Request context
    awsRequestID := ctx.AWSRequestID()
    functionName := ctx.FunctionName()
    
    return ctx.JSON(map[string]interface{}{
        "request_id":    ctx.RequestID(),
        "aws_request_id": awsRequestID,
        "function":      functionName,
    })
}
```

### Event Source Information

```go
func eventSourceInfo(ctx *lift.Context) error {
    // Event trigger type
    trigger := ctx.Request.TriggerType
    
    // Event-specific metadata
    metadata := ctx.Request.Metadata
    
    switch trigger {
    case lift.TriggerAPIGateway:
        stage := metadata["stage"].(string)
        apiID := metadata["apiId"].(string)
        
    case lift.TriggerSQS:
        queueURL := metadata["eventSourceARN"].(string)
        
    case lift.TriggerS3:
        bucket := metadata["bucket"].(string)
    }
    
    return ctx.JSON(map[string]interface{}{
        "trigger":  trigger,
        "metadata": metadata,
    })
}
```

## WebSocket Context

### WebSocket Operations

```go
func websocketHandler(ctx *lift.Context) error {
    // Check if WebSocket
    if !ctx.IsWebSocket() {
        return lift.BadRequest("WebSocket connection required")
    }
    
    // Get WebSocket context
    wsCtx, err := ctx.AsWebSocket()
    if err != nil {
        return err
    }
    
    // Connection info
    connectionID := wsCtx.ConnectionID()
    eventType := wsCtx.EventType()
    
    // Send message to specific connection
    err = wsCtx.SendMessage(targetConnectionID, map[string]interface{}{
        "type":    "notification",
        "message": "Hello!",
    })
    
    // Broadcast to multiple connections
    connections := getActiveConnections()
    err = wsCtx.BroadcastMessage(connections, announcement)
    
    // Reply to sender
    return wsCtx.Reply(map[string]interface{}{
        "status": "message received",
    })
}
```

## Context Utilities

### Client Information

```go
func clientInfo(ctx *lift.Context) error {
    // Client IP
    clientIP := ctx.ClientIP()
    
    // User agent
    userAgent := ctx.Header("User-Agent")
    
    // Geographic information (if available)
    country := ctx.Header("CloudFront-Viewer-Country")
    
    ctx.Logger.Info("Client info", map[string]interface{}{
        "ip":         clientIP,
        "user_agent": userAgent,
        "country":    country,
    })
    
    return ctx.JSON(response)
}
```

### Environment Information

```go
func environmentInfo(ctx *lift.Context) error {
    // Current environment
    env := ctx.Environment() // "development", "staging", "production"
    
    // Feature flags based on environment
    features := map[string]bool{
        "new_feature": env == "development",
        "beta_feature": env != "production",
    }
    
    return ctx.JSON(features)
}
```

### Context Copy

```go
func asyncOperation(ctx *lift.Context) error {
    // Create a copy for goroutines
    asyncCtx := ctx.Copy()
    
    go func() {
        // Safe to use in goroutine
        asyncCtx.Logger.Info("Async operation started")
        
        // Perform async work
        processInBackground(asyncCtx)
        
        asyncCtx.Logger.Info("Async operation completed")
    }()
    
    return ctx.JSON(map[string]string{
        "status": "Processing started",
    })
}
```

## Best Practices

### 1. Use Typed Accessors

```go
// GOOD: Use typed accessors
page := ctx.QueryInt("page", 1)
includeDeleted := ctx.QueryBool("deleted", false)

// AVOID: Manual type conversion
pageStr := ctx.Query("page")
page, _ := strconv.Atoi(pageStr)
```

### 2. Handle Missing Values

```go
// GOOD: Check for existence
userID := ctx.Param("id")
if userID == "" {
    return lift.BadRequest("User ID required")
}

// GOOD: Use defaults
limit := ctx.QueryInt("limit", 20)

// AVOID: Assuming values exist
userID := ctx.Param("id") // Could be empty
```

### 3. Structured Logging

```go
// GOOD: Structured fields
ctx.Logger.Info("Operation completed", map[string]interface{}{
    "duration": duration,
    "records":  count,
    "status":   "success",
})

// AVOID: String concatenation
ctx.Logger.Info(fmt.Sprintf("Processed %d records in %v", count, duration))
```

### 4. Early Returns

```go
// GOOD: Early returns for errors
func handler(ctx *lift.Context) error {
    if !ctx.IsAuthenticated() {
        return lift.Unauthorized("Authentication required")
    }
    
    userID := ctx.Param("id")
    if userID == "" {
        return lift.BadRequest("User ID required")
    }
    
    // Main logic
    return ctx.JSON(data)
}
```

### 5. Consistent Error Handling

```go
// GOOD: Consistent error responses
if err != nil {
    if errors.Is(err, ErrNotFound) {
        return lift.NotFound("Resource not found")
    }
    ctx.Logger.Error("Operation failed", map[string]interface{}{
        "error": err.Error(),
    })
    return lift.InternalError("Operation failed")
}
```

## Summary

The Context object in Lift provides:

- **Unified Interface**: Single object for all request/response operations
- **Type Safety**: Typed accessors with defaults
- **Rich Features**: Logging, metrics, state management
- **Multi-Tenant**: Built-in tenant isolation
- **WebSocket Support**: Specialized WebSocket operations
- **Developer Friendly**: Intuitive API with method chaining

Master the Context API to build efficient, maintainable Lambda functions. 