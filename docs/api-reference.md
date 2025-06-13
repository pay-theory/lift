# API Reference

Complete API reference for the Lift framework, covering all public types, methods, and interfaces.

## Core Types

### App

The main application container that manages routes, middleware, and request handling.

```go
type App struct {
    // Private fields
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
func (a *App) HEAD(path string, handler Handler)
func (a *App) OPTIONS(path string, handler Handler)

// Generic handler registration
func (a *App) Handle(method, path string, handler Handler)

// Middleware
func (a *App) Use(middleware ...Middleware)

// Group routes
func (a *App) Group(prefix string, middleware ...Middleware) *Group

// Main Lambda handler
func (a *App) HandleRequest(ctx context.Context, event interface{}) (interface{}, error)
```

### Config

Configuration options for the Lift application.

```go
type Config struct {
    AppName        string                 // Application name
    Environment    string                 // Environment (development, staging, production)
    LogLevel       string                 // Log level (debug, info, warn, error)
    Logger         Logger                 // Custom logger implementation
    MetricsCollector MetricsCollector    // Custom metrics collector
    DefaultTimeout time.Duration          // Default request timeout
    EnableMetrics  bool                   // Enable metrics collection
    EnableTracing  bool                   // Enable distributed tracing
    Custom         map[string]interface{} // Custom configuration values
}
```

### Context

The context object passed to all handlers, providing request data and response utilities.

```go
type Context struct {
    Request  *Request           // Request data
    Response *Response          // Response builder
    Logger   Logger             // Request-scoped logger
    Metrics  MetricsCollector   // Metrics collector
    // Private fields for state management
}

// Request data access
func (c *Context) Param(key string) string                    // Path parameters
func (c *Context) Query(key string) string                    // Query parameters
func (c *Context) QueryInt(key string, defaultValue int) int  // Query with default
func (c *Context) QueryBool(key string, defaultValue bool) bool
func (c *Context) QueryArray(key string) []string            // Multiple values
func (c *Context) Header(key string) string                   // Request headers
func (c *Context) Cookie(name string) (string, error)         // Cookies
func (c *Context) Body() []byte                              // Raw body
func (c *Context) ParseJSON(v interface{}) error              // Parse JSON body
func (c *Context) ParseAndValidate(v interface{}) error       // Parse and validate
func (c *Context) ParseForm(v interface{}) error              // Parse form data
func (c *Context) ParseQuery(v interface{}) error             // Parse query into struct

// Response building
func (c *Context) JSON(v interface{}) error                   // JSON response
func (c *Context) Text(text string) error                     // Plain text
func (c *Context) HTML(html string) error                     // HTML response
func (c *Context) XML(v interface{}) error                    // XML response
func (c *Context) Binary(data []byte) error                   // Binary data
func (c *Context) NoContent() error                          // 204 No Content
func (c *Context) Redirect(url string) error                  // 302 redirect
func (c *Context) RedirectPermanent(url string) error         // 301 redirect
func (c *Context) Status(code int) *Context                   // Set status code
func (c *Context) Header(key, value string) *Context          // Set response header

// Multi-tenant support
func (c *Context) TenantID() string                          // Get tenant ID
func (c *Context) SetTenantID(id string)                     // Set tenant ID
func (c *Context) UserID() string                            // Get user ID
func (c *Context) SetUserID(id string)                       // Set user ID

// State management
func (c *Context) Set(key string, value interface{})          // Set value
func (c *Context) Get(key string) interface{}                 // Get value
func (c *Context) GetString(key string, defaultValue string) string
func (c *Context) GetInt(key string, defaultValue int) int
func (c *Context) GetBool(key string, defaultValue bool) bool
func (c *Context) MustGet(key string) interface{}             // Panics if not found

// Utilities
func (c *Context) RequestID() string                          // Request ID
func (c *Context) SetRequestID(id string)                     // Set request ID
func (c *Context) ClientIP() string                           // Client IP address
func (c *Context) IsWebSocket() bool                          // Check if WebSocket
func (c *Context) AsWebSocket() (WebSocketContext, error)     // Get WebSocket context
func (c *Context) Environment() string                        // Current environment
func (c *Context) Copy() *Context                             // Copy for goroutines

// Tracing
func (c *Context) StartSegment(name string) Segment           // Start trace segment
func (c *Context) TraceHeader() string                        // Get trace header
```

### Request

Represents an incoming request from any Lambda event source.

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
```

### Response

Response builder for creating Lambda responses.

```go
type Response struct {
    StatusCode int                     // HTTP status code
    Headers    map[string]string       // Response headers
    Body       interface{}             // Response body
}
```

### TriggerType

Enumeration of supported Lambda event sources.

```go
type TriggerType string

const (
    TriggerUnknown       TriggerType = "unknown"
    TriggerAPIGateway    TriggerType = "api_gateway"
    TriggerAPIGatewayV2  TriggerType = "api_gateway_v2"
    TriggerSQS           TriggerType = "sqs"
    TriggerS3            TriggerType = "s3"
    TriggerEventBridge   TriggerType = "eventbridge"
    TriggerScheduled     TriggerType = "scheduled"
    TriggerDynamoDBStream TriggerType = "dynamodb_stream"
    TriggerKinesis       TriggerType = "kinesis"
    TriggerSNS           TriggerType = "sns"
    TriggerWebSocket     TriggerType = "websocket"
)
```

## Handlers

### Handler Interface

The core interface that all handlers must implement.

```go
type Handler interface {
    Handle(ctx *Context) error
}
```

### HandlerFunc

Function adapter for the Handler interface.

```go
type HandlerFunc func(*Context) error

func (f HandlerFunc) Handle(ctx *Context) error {
    return f(ctx)
}
```

### TypedHandler

Generic handler for type-safe request/response handling.

```go
func TypedHandler[Req any, Resp any](
    handler func(*Context, Req) (Resp, error),
) Handler
```

Example:
```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type UserResponse struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func createUser(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Handler implementation
    return UserResponse{ID: "123", Name: req.Name, Email: req.Email}, nil
}

// Register
app.POST("/users", lift.TypedHandler(createUser))
```

## Middleware

### Middleware Type

Function that wraps a handler to add functionality.

```go
type Middleware func(Handler) Handler
```

### Built-in Middleware

#### Logger
```go
func Logger() Middleware
func LoggerWithConfig(config LoggerConfig) Middleware

type LoggerConfig struct {
    Level            string   // Log level
    SkipPaths        []string // Paths to skip logging
    SensitiveHeaders []string // Headers to redact
    LogLatency       bool     // Log request duration
    LogRequestBody   bool     // Log request body
    LogResponseBody  bool     // Log response body
}
```

#### Recovery
```go
func Recover() Middleware
func RecoverWithConfig(config RecoverConfig) Middleware

type RecoverConfig struct {
    EnableStackTrace bool                                     // Include stack trace
    LogPanics       bool                                     // Log panic details
    PanicHandler    func(ctx *Context, err interface{}) error // Custom handler
}
```

#### RequestID
```go
func RequestID() Middleware
func RequestIDWithConfig(config RequestIDConfig) Middleware

type RequestIDConfig struct {
    Generator  func() string // ID generator function
    HeaderName string        // Header to check/set
}
```

#### CORS
```go
func CORS() Middleware
func CORSWithConfig(config CORSConfig) Middleware

type CORSConfig struct {
    AllowOrigins     []string                      // Allowed origins
    AllowOriginFunc  func(origin string) bool      // Dynamic origin check
    AllowMethods     []string                      // Allowed methods
    AllowHeaders     []string                      // Allowed headers
    ExposeHeaders    []string                      // Exposed headers
    AllowCredentials bool                          // Allow credentials
    MaxAge          int                           // Preflight cache duration
}
```

#### JWT
```go
func JWT(config JWTConfig) Middleware

type JWTConfig struct {
    SecretKey      []byte                          // Secret for HS256
    PublicKey      interface{}                     // Public key for RS256
    TokenLookup    string                          // Where to find token
    AuthScheme     string                          // Auth scheme (Bearer)
    Claims         jwt.Claims                      // Custom claims type
    ValidateFunc   func(claims jwt.MapClaims) error // Custom validation
    ErrorHandler   func(ctx *Context, err error) error
    SkipPaths      []string                        // Paths to skip
}
```

#### RateLimit
```go
func RateLimit(config RateLimitConfig) Middleware

type RateLimitConfig struct {
    WindowSize       time.Duration                   // Time window
    MaxRequests      int                            // Max requests per window
    KeyFunc          func(ctx *Context) string      // Key generator
    Store            RateLimitStore                 // Storage backend
    ExceededHandler  func(ctx *Context) error       // Rate limit exceeded
    SkipPaths        []string                       // Paths to skip
    EndpointLimits   map[string]int                 // Per-endpoint limits
}
```

#### Compress
```go
func Compress() Middleware
func CompressWithConfig(config CompressConfig) Middleware

type CompressConfig struct {
    Level            int      // Compression level
    MinContentLength int      // Minimum size to compress
    SkipPaths        []string // Paths to skip
    ContentTypes     []string // Content types to compress
}
```

#### Security
```go
func SecurityHeaders() Middleware
func SecurityHeadersWithConfig(config SecurityConfig) Middleware

type SecurityConfig struct {
    XSSProtection         string // X-XSS-Protection header
    ContentTypeNosniff    string // X-Content-Type-Options
    XFrameOptions         string // X-Frame-Options
    HSTSMaxAge           int    // HSTS max age
    HSTSIncludeSubdomains bool   // HSTS subdomains
    HSTSPreload          bool   // HSTS preload
    ContentSecurityPolicy string // CSP header
    ReferrerPolicy       string // Referrer-Policy
    PermissionsPolicy    string // Permissions-Policy
}
```

#### Timeout
```go
func Timeout(timeout time.Duration) Middleware
func TimeoutWithConfig(config TimeoutConfig) Middleware

type TimeoutConfig struct {
    Timeout      time.Duration              // Request timeout
    ErrorHandler func(ctx *Context) error   // Timeout handler
}
```

## Error Types

### HTTPError

Interface for errors with HTTP status codes.

```go
type HTTPError interface {
    error
    Status() int    // HTTP status code
    Code() string   // Error code
}
```

### Error Constructors

```go
// Client errors (4xx)
func BadRequest(message string) HTTPError           // 400
func Unauthorized(message string) HTTPError         // 401
func Forbidden(message string) HTTPError            // 403
func NotFound(message string) HTTPError             // 404
func MethodNotAllowed(message string) HTTPError     // 405
func Conflict(message string) HTTPError             // 409
func Gone(message string) HTTPError                 // 410
func UnprocessableEntity(message string) HTTPError  // 422
func TooManyRequests(message string) HTTPError      // 429

// Server errors (5xx)
func InternalError(message string) HTTPError        // 500
func NotImplemented(message string) HTTPError       // 501
func BadGateway(message string) HTTPError           // 502
func ServiceUnavailable(message string) HTTPError   // 503
func GatewayTimeout(message string) HTTPError       // 504
```

## Event Adapters

### EventAdapter Interface

Interface for adapting Lambda events to Lift requests.

```go
type EventAdapter interface {
    CanHandle(event interface{}) bool
    Adapt(event interface{}) (*Request, error)
}
```

### AdapterRegistry

Registry for managing event adapters.

```go
type AdapterRegistry struct {
    // Private fields
}

func NewAdapterRegistry() *AdapterRegistry
func (r *AdapterRegistry) Register(adapter EventAdapter)
func (r *AdapterRegistry) DetectAndAdapt(event interface{}) (*Request, error)
```

## WebSocket Support

### WebSocketContext

Specialized context for WebSocket connections.

```go
type WebSocketContext interface {
    Context
    
    // WebSocket specific
    ConnectionID() string                                      // Connection ID
    RouteKey() string                                         // Route key
    EventType() string                                        // Event type
    RequestID() string                                        // Request ID
    DomainName() string                                       // API domain
    Stage() string                                            // API stage
    
    // Messaging
    SendMessage(connectionID string, message interface{}) error
    BroadcastMessage(connectionIDs []string, message interface{}) error
    Reply(message interface{}) error                          // Reply to sender
}
```

## Logging

### Logger Interface

```go
type Logger interface {
    Debug(message string, fields ...map[string]interface{})
    Info(message string, fields ...map[string]interface{})
    Warn(message string, fields ...map[string]interface{})
    Error(message string, fields ...map[string]interface{})
    With(fields map[string]interface{}) Logger  // Create child logger
}
```

## Metrics

### MetricsCollector Interface

```go
type MetricsCollector interface {
    Count(metric string, value float64, tags map[string]string)
    Gauge(metric string, value float64, tags map[string]string)
    Timing(metric string, duration time.Duration, tags map[string]string)
    Record(metric string, fields map[string]interface{})
}
```

## Testing Utilities

### Test Context Creation

```go
package testing

func NewContext() *lift.Context
func NewContextWithRequest(req *lift.Request) *lift.Context
```

### Test Helpers

```go
func MustMarshalJSON(v interface{}) []byte
func CreateJWT(subject, secret string) string
func ParseResponseJSON(ctx *lift.Context, v interface{}) error
```

## Type Constraints

### Validation Tags

Lift uses struct tags for validation:

```go
type User struct {
    Name     string `json:"name" validate:"required,min=3,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Age      int    `json:"age" validate:"min=18,max=120"`
    Website  string `json:"website" validate:"omitempty,url"`
    Role     string `json:"role" validate:"required,oneof=admin user guest"`
}
```

Common validation tags:
- `required` - Field must be present
- `email` - Valid email format
- `url` - Valid URL format
- `min=n` - Minimum value/length
- `max=n` - Maximum value/length
- `oneof=a b c` - One of specified values
- `omitempty` - Skip validation if empty

## Constants

### Environment Variables

```go
const (
    EnvAppName       = "LIFT_APP_NAME"
    EnvEnvironment   = "LIFT_ENV"
    EnvLogLevel      = "LIFT_LOG_LEVEL"
    EnvMetricsEnabled = "LIFT_METRICS_ENABLED"
    EnvTracingEnabled = "LIFT_TRACING_ENABLED"
)
```

### Default Values

```go
const (
    DefaultTimeout     = 29 * time.Second
    DefaultLogLevel    = "info"
    DefaultEnvironment = "development"
    DefaultMaxBodySize = 10 * 1024 * 1024 // 10MB
)
```

## Package Structure

```
github.com/pay-theory/lift/
├── pkg/
│   ├── lift/           # Core framework
│   │   ├── app.go
│   │   ├── context.go
│   │   ├── handler.go
│   │   ├── request.go
│   │   ├── response.go
│   │   ├── errors.go
│   │   └── router.go
│   ├── middleware/     # Built-in middleware
│   │   ├── auth.go
│   │   ├── cors.go
│   │   ├── logger.go
│   │   ├── ratelimit.go
│   │   └── security.go
│   ├── adapters/       # Event adapters
│   │   ├── apigateway.go
│   │   ├── sqs.go
│   │   ├── s3.go
│   │   └── websocket.go
│   ├── testing/        # Test utilities
│   │   └── context.go
│   └── observability/  # Logging, metrics, tracing
│       ├── logger.go
│       ├── metrics.go
│       └── tracing.go
```

## Version Compatibility

- Go: 1.21+ (requires generics)
- AWS Lambda Runtime: provided.al2
- AWS SDK: v2

## Migration Guide

### From Standard Lambda Handler

```go
// Before: Standard Lambda
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Manual parsing
    var body RequestBody
    json.Unmarshal([]byte(request.Body), &body)
    
    // Manual response building
    return events.APIGatewayProxyResponse{
        StatusCode: 200,
        Body:       string(responseJSON),
    }, nil
}

// After: Lift
func handler(ctx *lift.Context) error {
    var body RequestBody
    ctx.ParseJSON(&body)
    
    return ctx.JSON(response)
}
```

### From Other Frameworks

```go
// From Gin/Echo/Fiber
// Before:
func handler(c *gin.Context) {
    var req Request
    c.ShouldBindJSON(&req)
    c.JSON(200, response)
}

// After: Lift (similar API)
func handler(ctx *lift.Context) error {
    var req Request
    ctx.ParseJSON(&req)
    return ctx.JSON(response)
}
```