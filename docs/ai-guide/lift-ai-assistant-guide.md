# Lift Framework AI Assistant Guide

## Overview

Lift is a type-safe, Lambda-native serverless framework for Go that eliminates boilerplate while providing production-grade features. This guide provides AI assistants with comprehensive knowledge of Lift's types, patterns, testing utilities, and best practices.

### Key Principles
- **Type Safety First**: Leverage Go's type system for compile-time safety
- **Zero Configuration**: Sensible defaults with optional customization
- **Performance**: <15ms cold start, optimized for Lambda
- **Multi-Tenant Ready**: Built-in tenant isolation and context management
- **Production Grade**: Observability, security, and error handling built-in

## Core Types and Structures

### App Container

The `App` is the central orchestrator of your serverless application:

```go
type App struct {
    router     *Router
    middleware []Middleware
    config     *Config
    
    // Event handling
    adapterRegistry *adapters.AdapterRegistry
    
    // WebSocket support
    wsRoutes  map[string]WebSocketHandler
    wsOptions *WebSocketOptions
    
    // Optional integrations
    db       interface{}
    logger   Logger
    metrics  MetricsCollector
    features map[string]bool
}
```

**Key Methods:**
- `New(options ...AppOption) *App` - Create new app with options
- `GET/POST/PUT/DELETE/PATCH(path, handler)` - Register HTTP routes
- `Use(middleware)` - Add middleware
- `Group(prefix)` - Create route groups
- `HandleRequest(ctx, event)` - Main Lambda handler
- `HandleTestRequest(ctx)` - Testing entry point

### Context

The `Context` is the enhanced request/response hub:

```go
type Context struct {
    context.Context
    
    // Request/Response cycle
    Request  *Request
    Response *Response
    
    // Observability
    Logger  Logger
    Metrics MetricsCollector
    
    // Utilities
    validator Validator
    params    map[string]string
    values    map[string]interface{}
    
    // Optional database connection
    DB interface{}
    
    // Multi-tenant support
    claims          map[string]interface{}
    isAuthenticated bool
}
```

**Key Methods:**
- `Param(key)` / `Query(key)` / `Header(key)` - Access request data
- `Set(key, value)` / `Get(key)` - Context state management
- `UserID()` / `TenantID()` / `AccountID()` - Multi-tenant helpers
- `ParseRequest(v)` - Type-safe request parsing with validation
- `JSON(data)` / `Text(text)` / `HTML(html)` - Response helpers
- `Status(code)` - Set response status
- `OK(data)` / `Created(data)` / `BadRequest()` / `NotFound()` / `Unauthorized()` - HTTP convenience methods

### Request Structure

Unified request format for all Lambda event sources:

```go
type Request struct {
    *adapters.Request
    
    Method      string            `json:"method,omitempty"`
    Path        string            `json:"path,omitempty"`
    Headers     map[string]string `json:"headers,omitempty"`
    QueryParams map[string]string `json:"query_params,omitempty"`
    Body        []byte            `json:"body,omitempty"`
}
```

**TriggerType Enumeration:**
- `TriggerAPIGateway` - API Gateway v1
- `TriggerAPIGatewayV2` - API Gateway v2 (HTTP API)
- `TriggerSQS` - SQS messages
- `TriggerS3` - S3 events
- `TriggerEventBridge` - EventBridge events
- `TriggerScheduled` - CloudWatch Events/EventBridge scheduled
- `TriggerWebSocket` - WebSocket connections
- `TriggerUnknown` - Fallback

### Response Structure

```go
type Response struct {
    StatusCode      int               `json:"statusCode"`
    Body            interface{}       `json:"body"`
    Headers         map[string]string `json:"headers"`
    IsBase64Encoded bool              `json:"isBase64Encoded"`
}
```

## Handler Patterns

### Basic Handler Pattern

```go
func MyHandler(ctx *lift.Context) error {
    // Access request data
    userID := ctx.Query("user_id")
    
    // Business logic
    result := processRequest(userID)
    
    // Return response
    return ctx.OK(result)
}

// Register with app
app.GET("/users", MyHandler)
```

### Type-Safe Handler Pattern

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type CreateUserResponse struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func CreateUserHandler(ctx *lift.Context, req CreateUserRequest) (CreateUserResponse, error) {
    // Request is automatically parsed and validated
    user := createUser(req.Name, req.Email)
    
    return CreateUserResponse{
        ID:    user.ID,
        Name:  user.Name,
        Email: user.Email,
    }, nil
}

// Register with type safety
app.POST("/users", lift.SimpleHandler(CreateUserHandler))
```

### Struct-Based Handler Pattern

```go
type UserService struct {
    db     *dynamorm.DynamORM
    logger lift.Logger
}

func (s *UserService) GetUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    user, err := s.db.Get(ctx, "users", userID, &User{})
    if err != nil {
        return ctx.NotFound("User not found", err)
    }
    
    return ctx.OK(user)
}

// Register struct method
userService := &UserService{db: db, logger: logger}
app.GET("/users/:id", userService.GetUser)
```

## Event Sources and Adapters

### API Gateway Events

```go
// Automatically handled - no special code needed
app.GET("/api/users", func(ctx *lift.Context) error {
    return ctx.OK(map[string]string{"message": "Hello from API Gateway"})
})
```

### SQS Events

```go
func ProcessSQSMessage(ctx *lift.Context) error {
    // Access SQS message data
    if ctx.Request.TriggerType == lift.TriggerSQS {
        // Process SQS records
        for _, record := range ctx.Request.SQSRecords {
            processMessage(record.Body)
        }
    }
    return nil
}
```

### S3 Events

```go
func ProcessS3Event(ctx *lift.Context) error {
    if ctx.Request.TriggerType == lift.TriggerS3 {
        for _, record := range ctx.Request.S3Records {
            bucket := record.S3.Bucket.Name
            key := record.S3.Object.Key
            processS3Object(bucket, key)
        }
    }
    return nil
}
```

### WebSocket Events

```go
func HandleWebSocketConnect(ctx *lift.Context) error {
    connectionID := ctx.Request.ConnectionID
    
    // Store connection
    err := storeConnection(connectionID)
    if err != nil {
        return ctx.Status(500).JSON(map[string]string{"error": "Failed to store connection"})
    }
    
    return ctx.Status(200).JSON(map[string]string{"message": "Connected"})
}

app.WebSocket("$connect", HandleWebSocketConnect)
```

## Middleware System

### Middleware Interface

```go
type Middleware func(Handler) Handler
```

### Built-in Middleware

**Logger Middleware:**
```go
app.Use(middleware.Logger())
```

**Authentication Middleware:**
```go
app.Use(middleware.JWT(middleware.JWTConfig{
    Secret: "your-secret-key",
    Claims: &CustomClaims{},
}))
```

**Rate Limiting Middleware:**
```go
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    RequestsPerMinute: 100,
    BurstSize:        10,
}))
```

### Custom Middleware Pattern

```go
func TenantMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Extract tenant ID from header or JWT
            tenantID := ctx.Header("X-Tenant-ID")
            if tenantID == "" {
                return ctx.BadRequest("Tenant ID required", nil)
            }
            
            // Set in context
            ctx.SetTenantID(tenantID)
            
            // Continue to next handler
            return next.Handle(ctx)
        })
    }
}

app.Use(TenantMiddleware())
```

## Error Handling

### Built-in Error Types

```go
// Create structured errors
err := lift.NewLiftError("VALIDATION_ERROR", "Invalid input", 400)
err = err.WithCause(originalError)
err = err.WithField("field", "email")

// HTTP convenience errors
return ctx.BadRequest("Invalid email format", validationErr)
return ctx.Unauthorized("Invalid token", authErr)
return ctx.NotFound("User not found", nil)
return ctx.InternalError("Database error", dbErr)
```

### Custom Error Types

```go
type BusinessError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

func (e *BusinessError) Error() string {
    return e.Message
}

func HandleBusinessError(ctx *lift.Context, err *BusinessError) error {
    return ctx.Status(422).JSON(map[string]interface{}{
        "error": err.Code,
        "message": err.Message,
        "details": err.Details,
    })
}
```

### Error Response Format

Standard error responses follow this format:
```json
{
    "error": "ERROR_CODE",
    "message": "Human readable message",
    "details": "Additional error details",
    "request_id": "unique-request-id"
}
```

## Testing Framework

### Test Context Creation

```go
import "github.com/pay-theory/lift/pkg/testing"

func TestMyHandler(t *testing.T) {
    // Create test context
    ctx := testing.NewTestContext()
    ctx.Request.Method = "GET"
    ctx.Request.Path = "/users/123"
    ctx.SetParam("id", "123")
    
    // Call handler
    err := MyHandler(ctx)
    
    // Assert results
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
}
```

### TestApp for Integration Testing

```go
func TestUserAPI(t *testing.T) {
    // Create test app
    app := lift.New()
    app.GET("/users/:id", GetUserHandler)
    
    // Create test app wrapper
    testApp := testing.NewTestApp(app)
    
    // Make test request
    resp := testApp.GET("/users/123")
    
    // Assert response
    resp.AssertStatus(200)
    resp.AssertJSON(map[string]interface{}{
        "id": "123",
        "name": "John Doe",
    })
}
```

### TestResponse Utilities

```go
type TestResponse struct {
    StatusCode int
    Body       interface{}
    Headers    map[string]string
}

// Assertion helpers
func (r *TestResponse) AssertStatus(expected int)
func (r *TestResponse) AssertJSON(expected interface{})
func (r *TestResponse) AssertContains(substring string)
func (r *TestResponse) AssertHeader(key, value string)
```

## Mock Utilities

### Mock Interfaces

**Logger Mock:**
```go
mockLogger := testing.NewMockLogger()
mockLogger.WithLevel("DEBUG")
app.WithLogger(mockLogger)
```

**Metrics Mock:**
```go
mockMetrics := testing.NewMockMetricsCollector()
app.WithMetrics(mockMetrics)

// Verify metrics were recorded
assert.Equal(t, 1, mockMetrics.GetCallCount("PutMetricData"))
```

**Database Mock:**
```go
mockDB := testing.NewMockDynamORM()
mockDB.WithData("users", "123", User{ID: "123", Name: "John"})
app.WithDatabase(mockDB)
```

### AWS Service Mocks

**S3 Mock:**
```go
mockS3 := testing.NewMockAWSService()
mockS3.WithResponse("GetObject", &s3.GetObjectOutput{
    Body: strings.NewReader("file content"),
})
```

**DynamoDB Mock:**
```go
mockDynamoDB := testing.NewMockDynamORM()
mockDynamoDB.WithData("table", "key", item)
mockDynamoDB.WithFailure("put", errors.New("simulated error"))
```

**API Gateway Management Mock:**
```go
mockAPIGW := testing.NewMockAPIGatewayManagementClient()
mockAPIGW.WithConnection("conn-123", &testing.MockConnection{
    ID:    "conn-123",
    State: testing.ConnectionStateConnected,
})
```

### Test Helpers

```go
// Create test scenarios
scenario := testing.NewScenario("User Creation")
scenario.WithRequest("POST", "/users", CreateUserRequest{
    Name:  "John Doe",
    Email: "john@example.com",
})
scenario.ExpectStatus(201)
scenario.ExpectJSON(CreateUserResponse{
    ID:    "generated-id",
    Name:  "John Doe",
    Email: "john@example.com",
})

// Run scenario
scenario.Run(t, app)
```

## Implementation Patterns

### Basic CRUD API Pattern

```go
type UserAPI struct {
    db     *dynamorm.DynamORM
    logger lift.Logger
}

func (api *UserAPI) SetupRoutes(app *lift.App) {
    users := app.Group("/users")
    users.GET("", api.ListUsers)
    users.POST("", api.CreateUser)
    users.GET("/:id", api.GetUser)
    users.PUT("/:id", api.UpdateUser)
    users.DELETE("/:id", api.DeleteUser)
}

func (api *UserAPI) CreateUser(ctx *lift.Context) error {
    var req CreateUserRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return err
    }
    
    user := &User{
        ID:    generateID(),
        Name:  req.Name,
        Email: req.Email,
    }
    
    if err := api.db.Put(ctx, "users", user.ID, user); err != nil {
        return ctx.InternalError("Failed to create user", err)
    }
    
    return ctx.Created(user)
}
```

### Multi-Tenant SaaS Pattern

```go
func TenantAwareHandler(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    if tenantID == "" {
        return ctx.BadRequest("Tenant ID required", nil)
    }
    
    // Use tenant-scoped database operations
    tenantDB := ctx.DB.(*dynamorm.DynamORM).WithTenant(tenantID)
    
    var items []Item
    err := tenantDB.Query(ctx, &dynamorm.Query{
        TableName: "items",
        // Query is automatically scoped to tenant
    }, &items)
    
    if err != nil {
        return ctx.InternalError("Query failed", err)
    }
    
    return ctx.OK(items)
}
```

### Event Processing Pattern

```go
func ProcessEvents(ctx *lift.Context) error {
    switch ctx.Request.TriggerType {
    case lift.TriggerSQS:
        return processSQSEvents(ctx)
    case lift.TriggerS3:
        return processS3Events(ctx)
    case lift.TriggerEventBridge:
        return processEventBridgeEvents(ctx)
    default:
        return ctx.BadRequest("Unsupported event type", nil)
    }
}

func processSQSEvents(ctx *lift.Context) error {
    for _, record := range ctx.Request.SQSRecords {
        var event BusinessEvent
        if err := json.Unmarshal([]byte(record.Body), &event); err != nil {
            ctx.Logger.Error("Failed to parse event", "error", err)
            continue
        }
        
        if err := handleBusinessEvent(ctx, &event); err != nil {
            ctx.Logger.Error("Failed to process event", "error", err)
            // Depending on requirements, might want to return error to trigger retry
        }
    }
    return nil
}
```

### WebSocket Real-time Pattern

```go
type WebSocketHandler struct {
    connections *ConnectionStore
    broadcaster *MessageBroadcaster
}

func (h *WebSocketHandler) HandleConnect(ctx *lift.Context) error {
    connectionID := ctx.Request.ConnectionID
    userID := ctx.GetClaim("user_id").(string)
    
    conn := &Connection{
        ID:     connectionID,
        UserID: userID,
        ConnectedAt: time.Now(),
    }
    
    if err := h.connections.Store(ctx, conn); err != nil {
        return ctx.InternalError("Failed to store connection", err)
    }
    
    return ctx.OK(map[string]string{"status": "connected"})
}

func (h *WebSocketHandler) HandleMessage(ctx *lift.Context) error {
    connectionID := ctx.Request.ConnectionID
    
    var msg IncomingMessage
    if err := ctx.ParseRequest(&msg); err != nil {
        return err
    }
    
    // Process message and broadcast to relevant connections
    response := processMessage(&msg)
    return h.broadcaster.SendToConnection(ctx, connectionID, response)
}
```

## Best Practices

### Handler Design

**DO:**
- Use type-safe handlers with `lift.SimpleHandler()` for complex request/response types
- Validate input using struct tags and the built-in validator
- Return structured errors with appropriate HTTP status codes
- Use context values for request-scoped data (user ID, tenant ID, etc.)
- Keep handlers focused on HTTP concerns, delegate business logic to services

**DON'T:**
- Perform heavy computation directly in handlers
- Ignore error handling or return generic errors
- Access database directly without proper error handling
- Mix business logic with HTTP handling code

### Error Handling

**DO:**
- Use structured errors with codes and messages
- Log errors with appropriate context
- Return user-friendly error messages
- Use appropriate HTTP status codes
- Handle partial failures gracefully in batch operations

**DON'T:**
- Expose internal error details to clients
- Use generic error messages
- Ignore errors or fail silently
- Return 500 errors for client mistakes

### Middleware Usage

**DO:**
- Use middleware for cross-cutting concerns (auth, logging, metrics)
- Order middleware appropriately (auth before business logic)
- Keep middleware focused and composable
- Use built-in middleware when available

**DON'T:**
- Put business logic in middleware
- Create overly complex middleware chains
- Ignore middleware order dependencies

### Testing Strategy

**DO:**
- Write unit tests for individual handlers
- Use integration tests for complete request flows
- Mock external dependencies (databases, APIs)
- Test error conditions and edge cases
- Use table-driven tests for multiple scenarios

**DON'T:**
- Test only happy paths
- Use real AWS services in unit tests
- Write tests that depend on external state
- Ignore test coverage for error handling

### Performance Optimization

**DO:**
- Use connection pooling for database connections
- Implement proper caching strategies
- Monitor cold start times and optimize initialization
- Use appropriate timeout values
- Batch operations when possible

**DON'T:**
- Initialize heavy resources in handler functions
- Make unnecessary database calls
- Ignore Lambda memory and timeout limits
- Perform synchronous operations that could be async

### Security Practices

**DO:**
- Validate all input data
- Use proper authentication and authorization
- Implement rate limiting for public endpoints
- Log security events
- Use HTTPS and secure headers

**DON'T:**
- Trust client-provided data without validation
- Log sensitive information (passwords, tokens)
- Use weak authentication mechanisms
- Ignore CORS configuration
- Expose internal system details in error messages

## Common Code Examples

### Complete CRUD Handler Set

```go
type ItemService struct {
    db     *dynamorm.DynamORM
    logger lift.Logger
}

// List items with pagination
func (s *ItemService) ListItems(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    limit := ctx.Query("limit")
    cursor := ctx.Query("cursor")
    
    query := &dynamorm.Query{
        TableName: "items",
        Limit:     parseLimit(limit, 20),
        Cursor:    cursor,
    }
    
    result, err := s.db.Query(ctx, query)
    if err != nil {
        return ctx.InternalError("Failed to query items", err)
    }
    
    return ctx.OK(map[string]interface{}{
        "items":      result.Items,
        "next_cursor": result.NextCursor,
        "count":      result.Count,
    })
}

// Create new item
func (s *ItemService) CreateItem(ctx *lift.Context, req CreateItemRequest) (CreateItemResponse, error) {
    item := &Item{
        ID:        generateID(),
        TenantID:  ctx.TenantID(),
        Name:      req.Name,
        Category:  req.Category,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    
    if err := s.db.Put(ctx, "items", item.ID, item); err != nil {
        return CreateItemResponse{}, fmt.Errorf("failed to create item: %w", err)
    }
    
    return CreateItemResponse{
        ID:        item.ID,
        Name:      item.Name,
        Category:  item.Category,
        CreatedAt: item.CreatedAt,
    }, nil
}

// Get single item
func (s *ItemService) GetItem(ctx *lift.Context) error {
    itemID := ctx.Param("id")
    tenantID := ctx.TenantID()
    
    var item Item
    err := s.db.Get(ctx, "items", buildKey(tenantID, itemID), &item)
    if err != nil {
        if isDynamoNotFoundError(err) {
            return ctx.NotFound("Item not found", nil)
        }
        return ctx.InternalError("Failed to get item", err)
    }
    
    return ctx.OK(item)
}

// Update item
func (s *ItemService) UpdateItem(ctx *lift.Context, req UpdateItemRequest) (UpdateItemResponse, error) {
    itemID := ctx.Param("id")
    tenantID := ctx.TenantID()
    
    // Get existing item
    var item Item
    err := s.db.Get(ctx, "items", buildKey(tenantID, itemID), &item)
    if err != nil {
        if isDynamoNotFoundError(err) {
            return UpdateItemResponse{}, lift.NewLiftError("NOT_FOUND", "Item not found", 404)
        }
        return UpdateItemResponse{}, fmt.Errorf("failed to get item: %w", err)
    }
    
    // Update fields
    if req.Name != "" {
        item.Name = req.Name
    }
    if req.Category != "" {
        item.Category = req.Category
    }
    item.UpdatedAt = time.Now()
    
    // Save updated item
    if err := s.db.Put(ctx, "items", buildKey(tenantID, itemID), &item); err != nil {
        return UpdateItemResponse{}, fmt.Errorf("failed to update item: %w", err)
    }
    
    return UpdateItemResponse{
        ID:        item.ID,
        Name:      item.Name,
        Category:  item.Category,
        UpdatedAt: item.UpdatedAt,
    }, nil
}

// Delete item
func (s *ItemService) DeleteItem(ctx *lift.Context) error {
    itemID := ctx.Param("id")
    tenantID := ctx.TenantID()
    
    err := s.db.Delete(ctx, "items", buildKey(tenantID, itemID))
    if err != nil {
        if isDynamoNotFoundError(err) {
            return ctx.NotFound("Item not found", nil)
        }
        return ctx.InternalError("Failed to delete item", err)
    }
    
    return ctx.Status(204).JSON(nil)
}
```

### Comprehensive Test Suite

```go
func TestItemService(t *testing.T) {
    // Setup
    mockDB := testing.NewMockDynamORM()
    service := &ItemService{
        db:     mockDB,
        logger: testing.NewMockLogger(),
    }
    
    app := lift.New()
    app.WithDatabase(mockDB)
    
    // Register routes
    items := app.Group("/items")
    items.GET("", service.ListItems)
    items.POST("", lift.SimpleHandler(service.CreateItem))
    items.GET("/:id", service.GetItem)
    items.PUT("/:id", lift.SimpleHandler(service.UpdateItem))
    items.DELETE("/:id", service.DeleteItem)
    
    testApp := testing.NewTestApp(app)
    
    t.Run("CreateItem", func(t *testing.T) {
        req := CreateItemRequest{
            Name:     "Test Item",
            Category: "test",
        }
        
        resp := testApp.POST("/items", req)
        resp.AssertStatus(201)
        
        var result CreateItemResponse
        resp.ParseJSON(&result)
        assert.Equal(t, "Test Item", result.Name)
        assert.NotEmpty(t, result.ID)
    })
    
    t.Run("GetItem", func(t *testing.T) {
        // Pre-populate mock data
        item := &Item{
            ID:       "item-123",
            TenantID: "tenant-1",
            Name:     "Existing Item",
            Category: "existing",
        }
        mockDB.WithData("items", "tenant-1#item-123", item)
        
        // Set tenant context
        ctx := testing.NewTestContext()
        ctx.SetTenantID("tenant-1")
        ctx.SetParam("id", "item-123")
        
        err := service.GetItem(ctx)
        assert.NoError(t, err)
        assert.Equal(t, 200, ctx.Response.StatusCode)
    })
    
    t.Run("GetItem_NotFound", func(t *testing.T) {
        ctx := testing.NewTestContext()
        ctx.SetTenantID("tenant-1")
        ctx.SetParam("id", "nonexistent")
        
        err := service.GetItem(ctx)
        assert.NoError(t, err) // Handler handles error internally
        assert.Equal(t, 404, ctx.Response.StatusCode)
    })
    
    t.Run("ListItems_WithPagination", func(t *testing.T) {
        // Setup mock query result
        mockDB.WithQueryResult("items", &dynamorm.QueryResult{
            Items: []interface{}{
                &Item{ID: "1", Name: "Item 1"},
                &Item{ID: "2", Name: "Item 2"},
            },
            Count:      2,
            NextCursor: "cursor-123",
        })
        
        resp := testApp.GET("/items?limit=10&cursor=start")
        resp.AssertStatus(200)
        
        var result map[string]interface{}
        resp.ParseJSON(&result)
        assert.Equal(t, 2, int(result["count"].(float64)))
        assert.Equal(t, "cursor-123", result["next_cursor"])
    })
    
    t.Run("UpdateItem", func(t *testing.T) {
        // Pre-populate existing item
        existingItem := &Item{
            ID:       "item-123",
            TenantID: "tenant-1",
            Name:     "Old Name",
            Category: "old",
        }
        mockDB.WithData("items", "tenant-1#item-123", existingItem)
        
        req := UpdateItemRequest{
            Name:     "New Name",
            Category: "updated",
        }
        
        // Create context with tenant and param
        ctx := testing.NewTestContext()
        ctx.SetTenantID("tenant-1")
        ctx.SetParam("id", "item-123")
        ctx.Request.Body, _ = json.Marshal(req)
        
        result, err := service.UpdateItem(ctx, req)
        assert.NoError(t, err)
        assert.Equal(t, "New Name", result.Name)
        assert.Equal(t, "updated", result.Category)
    })
    
    t.Run("DeleteItem", func(t *testing.T) {
        ctx := testing.NewTestContext()
        ctx.SetTenantID("tenant-1")
        ctx.SetParam("id", "item-123")
        
        err := service.DeleteItem(ctx)
        assert.NoError(t, err)
        assert.Equal(t, 204, ctx.Response.StatusCode)
    })
}

func TestItemService_ErrorHandling(t *testing.T) {
    mockDB := testing.NewMockDynamORM()
    service := &ItemService{db: mockDB, logger: testing.NewMockLogger()}
    
    t.Run("DatabaseError", func(t *testing.T) {
        // Configure mock to return error
        mockDB.WithFailure("get", errors.New("database connection failed"))
        
        ctx := testing.NewTestContext()
        ctx.SetTenantID("tenant-1")
        ctx.SetParam("id", "item-123")
        
        err := service.GetItem(ctx)
        assert.NoError(t, err) // Handler handles error internally
        assert.Equal(t, 500, ctx.Response.StatusCode)
    })
    
    t.Run("ValidationError", func(t *testing.T) {
        req := CreateItemRequest{
            Name:     "", // Invalid - empty name
            Category: "test",
        }
        
        ctx := testing.NewTestContext()
        ctx.Request.Body, _ = json.Marshal(req)
        
        _, err := service.CreateItem(ctx, req)
        assert.Error(t, err)
        // Validation error should be handled by the framework
    })
}
```

This guide provides comprehensive coverage of the Lift framework for AI assistants, including all major types, patterns, testing utilities, and best practices needed to effectively work with the framework.
This comprehensive guide provides AI assistants with all the essential information needed to work effectively with the Lift framework, including types, patterns, testing utilities, and best practices for building production-ready serverless applications.