# Lift Framework - AI Assistant Guide

*A comprehensive guide for AI assistants working with the Lift serverless framework*

## Table of Contents

1. [Framework Overview](#framework-overview)
2. [Core Types and Structures](#core-types-and-structures)
3. [Handler Patterns](#handler-patterns)
4. [Event Sources and Adapters](#event-sources-and-adapters)
5. [Middleware System](#middleware-system)
6. [Error Handling](#error-handling)
7. [Testing Framework](#testing-framework)
8. [Mock Utilities](#mock-utilities)
9. [Implementation Patterns](#implementation-patterns)
10. [Best Practices](#best-practices)
11. [Common Code Examples](#common-code-examples)

## Framework Overview

Lift is a type-safe, Lambda-native serverless framework for Go that eliminates boilerplate while providing production-grade features. It follows a middleware-based architecture similar to popular web frameworks but optimized for serverless environments.

### Key Design Principles
- **Type Safety First**: Leverage Go generics for compile-time type checking
- **Zero Configuration**: Automatic event detection and routing
- **Performance Optimized**: <15ms cold start overhead target
- **Multi-Tenant Ready**: Built-in tenant isolation
- **Production Grade**: Comprehensive observability and security

### Architecture Flow
```
Lambda Event → Event Adapter → Router → Middleware Chain → Handler → Response
```

## Core Types and Structures

### App Container

The central orchestrator for your Lambda function:

```go
type App struct {
    // Core components (private)
    router     *Router
    middleware []Middleware
    config     *Config
    
    // Optional integrations
    db         DatabaseClient
    logger     Logger
    metrics    MetricsCollector
}

// Constructor
func New() *App
func NewWithConfig(config Config) *App

// HTTP Methods
func (a *App) GET(path string, handler Handler)
func (a *App) POST(path string, handler Handler)
func (a *App) PUT(path string, handler Handler)
func (a *App) DELETE(path string, handler Handler)
func (a *App) PATCH(path string, handler Handler)

// Generic handler registration
func (a *App) Handle(method, path string, handler Handler)

// Middleware
func (a *App) Use(middleware ...Middleware)

// Main Lambda handler
func (a *App) HandleRequest(ctx context.Context, event interface{}) (interface{}, error)
```

### Context - The Request/Response Hub

The enhanced context provides utilities for every request:

```go
type Context struct {
    context.Context
    Request    *Request
    Response   *Response
    Logger     Logger
    Metrics    MetricsCollector
    
    // Private fields for state management
    params     map[string]string
    values     map[string]interface{}
    validator  Validator
}

// Request data access
func (c *Context) Param(key string) string                    // Path parameters
func (c *Context) Query(key string) string                    // Query parameters
func (c *Context) QueryInt(key string, defaultValue int) int
func (c *Context) QueryArray(key string) []string
func (c *Context) Header(key string) string                   // Request headers
func (c *Context) Body() []byte                              // Raw body
func (c *Context) ParseJSON(v interface{}) error              // Parse JSON body
func (c *Context) ParseAndValidate(v interface{}) error       // Parse and validate

// Response building
func (c *Context) JSON(v interface{}) error                   // JSON response
func (c *Context) Text(text string) error                     // Plain text
func (c *Context) Status(code int) *Context                   // Set status code
func (c *Context) Header(key, value string) *Context          // Set response header
func (c *Context) NoContent() error                          // 204 No Content
func (c *Context) Redirect(url string) error                  // 302 redirect

// Multi-tenant support
func (c *Context) TenantID() string
func (c *Context) SetTenantID(id string)
func (c *Context) UserID() string
func (c *Context) SetUserID(id string)

// State management
func (c *Context) Set(key string, value interface{})
func (c *Context) Get(key string) interface{}
func (c *Context) MustGet(key string) interface{}

// Utilities
func (c *Context) RequestID() string
func (c *Context) ClientIP() string
func (c *Context) Copy() *Context                             // Copy for goroutines
```

### Request Structure

Unified request format for all Lambda event sources:

```go
type Request struct {
    TriggerType TriggerType             // Event source type
    Method      string                  // HTTP method
    Path        string                  // Request path
    Headers     map[string]string       // Request headers
    Query       map[string]string       // Query parameters
    Body        []byte                  // Request body
    TenantID    string                  // Tenant identifier
    UserID      string                  // User identifier
    Records     []interface{}           // Batch event records
    Metadata    map[string]interface{}  // Event-specific metadata
}

// TriggerType enumeration
type TriggerType string

const (
    TriggerUnknown       TriggerType = "unknown"
    TriggerAPIGateway    TriggerType = "api_gateway"
    TriggerAPIGatewayV2  TriggerType = "api_gateway_v2"
    TriggerSQS           TriggerType = "sqs"
    TriggerS3            TriggerType = "s3"
    TriggerEventBridge   TriggerType = "eventbridge"
    TriggerScheduled     TriggerType = "scheduled"
    TriggerWebSocket     TriggerType = "websocket"
)
```

### Response Structure

```go
type Response struct {
    StatusCode int                    `json:"statusCode"`
    Body       interface{}            `json:"body"`
    Headers    map[string]string      `json:"headers"`
    
    // Internal state
    written bool
}
```

## Handler Patterns

### Handler Interface

The core interface all handlers implement:

```go
type Handler interface {
    Handle(ctx *Context) error
}

type HandlerFunc func(*Context) error

func (f HandlerFunc) Handle(ctx *Context) error {
    return f(ctx)
}
```

### Basic Handler Pattern

```go
func handleRequest(ctx *lift.Context) error {
    // Your business logic
    data := processRequest(ctx)
    return ctx.JSON(data)
}

// Register
app.GET("/users", handleRequest)
```

### Type-Safe Handler Pattern

The most powerful pattern with automatic parsing and validation:

```go
// Define request/response types
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18,max=120"`
}

type UserResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

// Type-safe handler
func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // req is already parsed and validated!
    user, err := userService.Create(req)
    if err != nil {
        return UserResponse{}, err
    }
    
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

### Struct-Based Handler Pattern

For complex handlers with dependencies:

```go
type UserHandler struct {
    userService UserService
    logger      Logger
    metrics     MetricsCollector
}

func (h *UserHandler) GetUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    user, err := h.userService.Get(userID)
    if err != nil {
        h.logger.Error("Failed to get user", map[string]interface{}{
            "user_id": userID,
            "error":   err.Error(),
        })
        return lift.NotFound("User not found")
    }
    
    h.metrics.Counter("user.retrieved", map[string]string{
        "tenant_id": ctx.TenantID(),
    })
    
    return ctx.JSON(user)
}

// Register
handler := &UserHandler{userService, logger, metrics}
app.GET("/users/:id", handler.GetUser)
```

## Event Sources and Adapters

### Event Adapter Interface

```go
type EventAdapter interface {
    CanHandle(event interface{}) bool
    Adapt(event interface{}) (*Request, error)
}
```

### Supported Event Sources

#### API Gateway (HTTP/REST)
```go
// Automatic routing for HTTP methods
app.GET("/users", getUsers)
app.POST("/users", createUser)
app.PUT("/users/:id", updateUser)
app.DELETE("/users/:id", deleteUser)
```

#### SQS (Queue Processing)
```go
app.Handle("SQS", "/process-orders", processOrders)

func processOrders(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        sqsRecord := record.(map[string]interface{})
        body := sqsRecord["body"].(string)
        
        var order Order
        json.Unmarshal([]byte(body), &order)
        
        if err := processOrder(order); err != nil {
            return err // Will retry message
        }
    }
    return nil
}
```

#### S3 Events
```go
app.Handle("S3", "/process-upload", handleS3Upload)

func handleS3Upload(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        s3Record := record.(map[string]interface{})
        s3Data := s3Record["s3"].(map[string]interface{})
        
        bucket := s3Data["bucket"].(map[string]interface{})["name"].(string)
        key := s3Data["object"].(map[string]interface{})["key"].(string)
        
        err := processFile(bucket, key)
        if err != nil {
            return err
        }
    }
    return nil
}
```

#### WebSocket
```go
app.Handle("CONNECT", "/connect", handleConnect)
app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
app.Handle("MESSAGE", "/message", handleMessage)

func handleMessage(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    
    var msg ChatMessage
    ctx.ParseJSON(&msg)
    
    // Send to specific connection
    return wsCtx.SendMessage(targetConnectionID, response)
}
```

## Middleware System

### Middleware Interface

```go
type Middleware func(Handler) Handler
```

### Built-in Middleware

#### Logger Middleware
```go
func Logger() Middleware
func LoggerWithConfig(config LoggerConfig) Middleware

type LoggerConfig struct {
    Level            string
    SkipPaths        []string
    SensitiveHeaders []string
    LogLatency       bool
    LogRequestBody   bool
    LogResponseBody  bool
}
```

#### Authentication Middleware
```go
func JWT(config JWTConfig) Middleware

type JWTConfig struct {
    SecretKey      []byte
    PublicKey      interface{}
    TokenLookup    string
    AuthScheme     string
    Claims         jwt.Claims
    ValidateFunc   func(claims jwt.MapClaims) error
    ErrorHandler   func(ctx *Context, err error) error
    SkipPaths      []string
}
```

#### Rate Limiting Middleware
```go
func RateLimit(config RateLimitConfig) Middleware

type RateLimitConfig struct {
    WindowSize       time.Duration
    MaxRequests      int
    KeyFunc          func(ctx *Context) string
    Store            RateLimitStore
    ExceededHandler  func(ctx *Context) error
    SkipPaths        []string
}
```

### Custom Middleware Pattern

```go
func CustomMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Before handler
            start := time.Now()
            
            // Call next handler
            err := next.Handle(ctx)
            
            // After handler
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

## Error Handling

### Built-in Error Types

```go
// HTTP error constructors
func BadRequest(message string) HTTPError           // 400
func Unauthorized(message string) HTTPError         // 401
func Forbidden(message string) HTTPError            // 403
func NotFound(message string) HTTPError             // 404
func Conflict(message string) HTTPError             // 409
func TooManyRequests(message string) HTTPError      // 429
func InternalError(message string) HTTPError        // 500
func ServiceUnavailable(message string) HTTPError   // 503

// Validation error with details
func ValidationError(field, message string) HTTPError
```

### Custom Error Types

```go
type APIError struct {
    StatusCode int                    `json:"-"`
    Code       string                 `json:"code"`
    Message    string                 `json:"message"`
    Details    map[string]interface{} `json:"details,omitempty"`
    RequestID  string                 `json:"request_id,omitempty"`
    Timestamp  int64                  `json:"timestamp"`
}

func (e *APIError) Error() string {
    return e.Message
}

func (e *APIError) Status() int {
    return e.StatusCode
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

## Testing Framework

### Test Context Creation

```go
package testing

// Create basic test context
func NewContext() *lift.Context

// Create context with request data
func NewContextWithRequest(req *lift.Request) *lift.Context

// Create authenticated context
func NewAuthenticatedContext(userID, tenantID string) *lift.Context
```

### TestApp for Integration Testing

```go
type TestApp struct {
    *lift.App
    recorder *ResponseRecorder
}

func NewTestApp() *TestApp

// Make test requests
func (t *TestApp) GET(path string) *TestResponse
func (t *TestApp) POST(path string, body interface{}) *TestResponse
func (t *TestApp) PUT(path string, body interface{}) *TestResponse
func (t *TestApp) DELETE(path string) *TestResponse

// With headers
func (t *TestApp) Request(method, path string, body interface{}, headers map[string]string) *TestResponse
```

### TestResponse Utilities

```go
type TestResponse struct {
    StatusCode int
    Body       interface{}
    Headers    map[string]string
}

// Assertion helpers
func (r *TestResponse) ExpectStatus(code int) *TestResponse
func (r *TestResponse) ExpectHeader(key, value string) *TestResponse
func (r *TestResponse) ExpectJSON(expected interface{}) *TestResponse
func (r *TestResponse) ExpectContains(text string) *TestResponse
func (r *TestResponse) JSON(v interface{}) error
```

## Mock Utilities

### Mock Interfaces

```go
// Mock Logger
type MockLogger struct {
    Logs []LogEntry
}

func (m *MockLogger) Info(message string, fields ...map[string]interface{}) {
    m.Logs = append(m.Logs, LogEntry{
        Level:   "info",
        Message: message,
        Fields:  mergeFields(fields...),
    })
}

// Mock Metrics Collector
type MockMetricsCollector struct {
    Metrics []Metric
}

func (m *MockMetricsCollector) Counter(name string, tags ...map[string]string) {
    m.Metrics = append(m.Metrics, Metric{
        Type: "counter",
        Name: name,
        Tags: mergeTags(tags...),
    })
}

// Mock Database
type MockDB struct {
    Data map[string]interface{}
    Calls []DBCall
}

func (m *MockDB) Get(ctx context.Context, key string, result interface{}) error {
    m.Calls = append(m.Calls, DBCall{Method: "Get", Key: key})
    if data, exists := m.Data[key]; exists {
        // Copy data to result
        return nil
    }
    return ErrNotFound
}
```

### AWS Service Mocks

```go
// Mock S3 Client
type MockS3Client struct {
    GetObjectFunc func(*s3.GetObjectInput) (*s3.GetObjectOutput, error)
    PutObjectFunc func(*s3.PutObjectInput) (*s3.PutObjectOutput, error)
}

// Mock DynamoDB Client
type MockDynamoDBClient struct {
    GetItemFunc func(*dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
    PutItemFunc func(*dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error)
}

// Mock API Gateway Management Client (WebSocket)
type MockAPIGatewayManagementClient struct {
    PostToConnectionFunc func(*apigatewaymanagementapi.PostToConnectionInput) error
}
```

### Test Helpers

```go
// Create test JWT token
func CreateJWT(subject, secret string) string

// Marshal JSON for tests
func MustMarshalJSON(v interface{}) []byte

// Create test events
func CreateAPIGatewayEvent(method, path string, body interface{}) events.APIGatewayProxyRequest
func CreateSQSEvent(messages []string) events.SQSEvent
func CreateS3Event(bucket, key string) events.S3Event
```

## Implementation Patterns

### Basic CRUD API Pattern

```go
func main() {
    app := lift.New()
    
    // Middleware
    app.Use(middleware.Logger())
    app.Use(middleware.Recover())
    app.Use(middleware.CORS())
    
    // Routes
    app.GET("/users", getUsers)
    app.GET("/users/:id", getUser)
    app.POST("/users", createUser)
    app.PUT("/users/:id", updateUser)
    app.DELETE("/users/:id", deleteUser)
    
    lambda.Start(app.HandleRequest)
}

func getUsers(ctx *lift.Context) error {
    users, err := userService.List(ctx.TenantID())
    if err != nil {
        return lift.InternalError("Failed to retrieve users")
    }
    return ctx.JSON(users)
}

func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    user, err := userService.Create(req)
    if err != nil {
        return UserResponse{}, lift.BadRequest("Invalid user data")
    }
    return mapToResponse(user), nil
}
```

### Multi-Tenant SaaS Pattern

```go
func main() {
    app := lift.New()
    
    // Multi-tenant middleware
    app.Use(middleware.JWT(jwtConfig))
    app.Use(middleware.TenantIsolation())
    app.Use(middleware.RateLimit(rateLimitConfig))
    
    // Tenant-scoped routes
    app.GET("/api/v1/data", getTenantData)
    app.POST("/api/v1/data", createTenantData)
    
    lambda.Start(app.HandleRequest)
}

func getTenantData(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    userID := ctx.UserID()
    
    // Automatically scoped to tenant
    data, err := dataService.GetForTenant(tenantID, userID)
    if err != nil {
        return lift.NotFound("Data not found")
    }
    
    return ctx.JSON(data)
}
```

### Event Processing Pattern

```go
func main() {
    app := lift.New()
    
    // HTTP API
    app.GET("/api/status", getStatus)
    
    // Event processing
    app.Handle("SQS", "/process-orders", processOrders)
    app.Handle("S3", "/process-files", processFiles)
    app.Handle("Scheduled", "/daily-report", generateReport)
    
    lambda.Start(app.HandleRequest)
}

func processOrders(ctx *lift.Context) error {
    var errors []error
    
    for _, record := range ctx.Request.Records {
        if err := processSingleOrder(record); err != nil {
            errors = append(errors, err)
            ctx.Logger.Error("Failed to process order", map[string]interface{}{
                "error": err.Error(),
            })
        }
    }
    
    // Partial batch failure
    if len(errors) > 0 {
        return lift.PartialBatchFailure(errors)
    }
    
    return nil
}
```

### WebSocket Real-time Pattern

```go
func main() {
    app := lift.New()
    
    // WebSocket handlers
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
    app.Handle("MESSAGE", "/message", handleMessage)
    app.Handle("MESSAGE", "/chat", handleChat)
    
    lambda.Start(app.HandleRequest)
}

func handleConnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    connectionID := wsCtx.ConnectionID()
    
    // Authenticate via query params
    token := ctx.Query("token")
    userID, err := validateToken(token)
    if err != nil {
        return lift.Unauthorized("Invalid token")
    }
    
    // Store connection
    connectionStore.Save(connectionID, userID)
    
    return ctx.JSON(map[string]string{
        "message": "Connected successfully",
    })
}

func handleMessage(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    
    var msg ChatMessage
    ctx.ParseJSON(&msg)
    
    // Broadcast to room
    connections := connectionStore.GetByRoom(msg.RoomID)
    return wsCtx.BroadcastMessage(connections, msg)
}
```

## Best Practices

### 1. Handler Design

**DO:**
```go
// Use TypedHandler for automatic validation
func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Business logic only
    return userService.Create(req)
}

// Keep handlers focused on single responsibility
func getUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    user, err := userService.Get(userID)
    if err != nil {
        return lift.NotFound("User not found")
    }
    return ctx.JSON(user)
}
```

**DON'T:**
```go
// Avoid manual parsing when TypedHandler can do it
func createUser(ctx *lift.Context) error {
    var req CreateUserRequest
    if err := ctx.ParseJSON(&req); err != nil {
        return lift.BadRequest("Invalid JSON")
    }
    if err := validate(req); err != nil {
        return lift.BadRequest(err.Error())
    }
    // ... business logic
}
```

### 2. Error Handling

**DO:**
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
        // Log internal errors but return generic message
        ctx.Logger.Error("Service error", map[string]interface{}{
            "error": err.Error(),
        })
        return lift.InternalError("Failed to retrieve resource")
    }
    return ctx.JSON(resource)
}
```

### 3. Middleware Usage

**DO:**
```go
// Use middleware for cross-cutting concerns
app.Use(middleware.Logger())
app.Use(middleware.Auth())
app.Use(middleware.RateLimit())

// Keep handlers clean
app.GET("/users", getUsers) // No auth/logging logic here
```

**DON'T:**
```go
// Avoid repeating logic in handlers
func getUsers(ctx *lift.Context) error {
    // Log request
    // Check auth
    // Check rate limit
    // Actual logic
}
```

### 4. Testing Strategy

**DO:**
```go
// Test handlers in isolation
func TestGetUser(t *testing.T) {
    // Arrange
    mockService := &MockUserService{
        GetFunc: func(id string) (*User, error) {
            return &User{ID: id}, nil
        },
    }
    handler := NewUserHandler(mockService)
    
    // Act
    ctx := testing.NewContext()
    ctx.SetParam("id", "123")
    err := handler.GetUser(ctx)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
}

// Use table-driven tests for multiple scenarios
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   CreateUserRequest
        wantErr bool
    }{
        {"valid", CreateUserRequest{Name: "John", Email: "john@example.com"}, false},
        {"missing name", CreateUserRequest{Email: "john@example.com"}, true},
        {"invalid email", CreateUserRequest{Name: "John", Email: "invalid"}, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateUser(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 5. Performance Optimization

**DO:**
```go
// Use context copying for goroutines
func asyncHandler(ctx *lift.Context) error {
    asyncCtx := ctx.Copy()
    
    go func() {
        // Use copied context
        processInBackground(asyncCtx)
    }()
    
    return ctx.JSON(map[string]string{"status": "processing"})
}

// Pool expensive resources
var dbPool = &ConnectionPool{
    factory: createDBConnection,
    maxSize: 10,
}

// Use efficient JSON handling
func handler(ctx *lift.Context) error {
    // Stream large responses
    return ctx.Stream(largeDataStream)
}
```

### 6. Security Best Practices

**DO:**
```go
// Always validate input
func updateUser(ctx *lift.Context, req UpdateUserRequest) (UserResponse, error) {
    // Validation happens automatically with TypedHandler
    
    // Check permissions
    if !ctx.HasPermission("user:update") {
        return UserResponse{}, lift.Forbidden("Insufficient permissions")
    }
    
    // Tenant isolation
    if req.TenantID != ctx.TenantID() {
        return UserResponse{}, lift.Forbidden("Cross-tenant access denied")
    }
    
    return userService.Update(req)
}

// Use structured logging for security events
func authMiddleware(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        token := ctx.Header("Authorization")
        
        user, err := validateToken(token)
        if err != nil {
            ctx.Logger.Warn("Authentication failed", map[string]interface{}{
                "ip":         ctx.ClientIP(),
                "user_agent": ctx.Header("User-Agent"),
                "path":       ctx.Request.Path,
            })
            return lift.Unauthorized("Invalid token")
        }
        
        ctx.SetUserID(user.ID)
        return next.Handle(ctx)
    })
}
```

## Common Code Examples

### Complete CRUD Handler Set

```go
type UserHandler struct {
    service UserService
    logger  Logger
}

func (h *UserHandler) List(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    
    // Parse query parameters
    page := ctx.QueryInt("page", 1)
    limit := ctx.QueryInt("limit", 20)
    search := ctx.Query("search")
    
    users, total, err := h.service.List(tenantID, page, limit, search)
    if err != nil {
        return lift.InternalError("Failed to retrieve users")
    }
    
    return ctx.JSON(map[string]interface{}{
        "users": users,
        "total": total,
        "page":  page,
        "limit": limit,
    })
}

func (h *UserHandler) Get(ctx *lift.Context) error {
    userID := ctx.Param("id")
    tenantID := ctx.TenantID()
    
    user, err := h.service.Get(tenantID, userID)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return lift.NotFound("User not found")
        }
        return lift.InternalError("Failed to retrieve user")
    }
    
    return ctx.JSON(user)
}

func (h *UserHandler) Create(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    req.TenantID = ctx.TenantID()
    req.CreatedBy = ctx.UserID()
    
    user, err := h.service.Create(req)
    if err != nil {
        if errors.Is(err, ErrDuplicateEmail) {
            return UserResponse{}, lift.Conflict("Email already exists")
        }
        return UserResponse{}, lift.InternalError("Failed to create user")
    }
    
    h.logger.Info("User created", map[string]interface{}{
        "user_id":   user.ID,
        "tenant_id": user.TenantID,
        "created_by": req.CreatedBy,
    })
    
    return mapToUserResponse(user), nil
}

func (h *UserHandler) Update(ctx *lift.Context, req UpdateUserRequest) (UserResponse, error) {
    userID := ctx.Param("id")
    req.ID = userID
    req.TenantID = ctx.TenantID()
    req.UpdatedBy = ctx.UserID()
    
    user, err := h.service.Update(req)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return UserResponse{}, lift.NotFound("User not found")
        }
        return UserResponse{}, lift.InternalError("Failed to update user")
    }
    
    return mapToUserResponse(user), nil
}

func (h *UserHandler) Delete(ctx *lift.Context) error {
    userID := ctx.Param("id")
    tenantID := ctx.TenantID()
    
    err := h.service.Delete(tenantID, userID)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return lift.NotFound("User not found")
        }
        return lift.InternalError("Failed to delete user")
    }
    
    return ctx.NoContent()
}
```

### Complete Test Suite Example

```go
func TestUserHandler(t *testing.T) {
    // Setup
    mockService := &MockUserService{}
    handler := &UserHandler{
        service: mockService,
        logger:  &MockLogger{},
    }
    
    t.Run("List", func(t *testing.T) {
        mockService.ListFunc = func(tenantID string, page, limit int, search string) ([]User, int, error) {
            return []User{{ID: "1", Name: "John"}}, 1, nil
        }
        
        ctx := testing.NewAuthenticatedContext("user1", "tenant1")
        ctx.Request.Query["page"] = "1"
        ctx.Request.Query["limit"] = "10"
        
        err := handler.List(ctx)
        assert.NoError(t, err)
        assert.Equal(t, 200, ctx.Response.StatusCode)
        
        var resp map[string]interface{}
        ctx.ParseResponseJSON(&resp)
        assert.Len(t, resp["users"], 1)
    })
    
    t.Run("Create", func(t *testing.T) {
        mockService.CreateFunc = func(req CreateUserRequest) (*User, error) {
            return &User{
                ID:       "new-123",
                Name:     req.Name,
                Email:    req.Email,
                TenantID: req.TenantID,
            }, nil
        }
        
        req := CreateUserRequest{
            Name:  "Jane Doe",
            Email: "jane@example.com",
        }
        
        ctx := testing.NewAuthenticatedContext("user1", "tenant1")
        ctx.Request.Body = testing.MustMarshalJSON(req)
        
        typedHandler := lift.TypedHandler(handler.Create)
        err := typedHandler.Handle(ctx)
        
        assert.NoError(t, err)
        assert.Equal(t, 200, ctx.Response.StatusCode)
    })
    
    t.Run("Get_NotFound", func(t *testing.T) {
        mockService.GetFunc = func(tenantID, userID string) (*User, error) {
            return nil, ErrNotFound
        }
        
        ctx := testing.NewAuthenticatedContext("user1", "tenant1")
        ctx.SetParam("id", "999")
        
        err := handler.Get(ctx)
        assert.Error(t, err)
        
        httpErr, ok := err.(lift.HTTPError)
        assert.True(t, ok)
        assert.Equal(t, 404, httpErr.Status())
    })
}
```

This comprehensive guide provides AI assistants with all the essential information needed to work effectively with the Lift framework, including types, patterns, testing utilities, and best practices for building production-ready serverless applications.