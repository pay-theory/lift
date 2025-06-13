# Lift Framework - Technical Architecture

## Architecture Overview

Lift is designed as a type-safe, Lambda-native framework that eliminates boilerplate while providing production-grade features. The architecture is inspired by modern web frameworks but optimized for serverless environments.

```
┌─────────────────────────────────────────────────────────────┐
│                    Lift Framework                           │
├─────────────────────────────────────────────────────────────┤
│  Developer API Layer                                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │   Routing   │ │ Middleware  │ │   Testing   │          │
│  │   System    │ │   Stack     │ │  Utilities  │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│  Core Framework Layer                                       │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │   Context   │ │  Type Safe  │ │    Error    │          │
│  │  Enhanced   │ │  Handlers   │ │  Handling   │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│  Event Processing Layer                                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │ API Gateway │ │     SQS     │ │     S3      │          │
│  │  Adapter    │ │   Adapter   │ │   Adapter   │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│  Infrastructure Layer                                       │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │   Logging   │ │   Metrics   │ │   Tracing   │          │
│  │   System    │ │ Collection  │ │   System    │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   AWS Lambda Runtime                        │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Application Container

**Lesson from Streamer**: Centralized configuration and dependency injection simplifies testing and deployment.

```go
// pkg/lift/app.go
type App struct {
    // Core components
    router     *Router
    middleware []Middleware
    config     *Config
    
    // Optional integrations
    db         DatabaseClient
    logger     Logger
    metrics    MetricsCollector
    tracer     Tracer
    
    // Runtime state
    started    bool
    mu         sync.RWMutex
}

type Config struct {
    // Performance settings
    MaxRequestSize  int64         `json:"max_request_size"`
    MaxResponseSize int64         `json:"max_response_size"`
    Timeout         time.Duration `json:"timeout"`
    
    // Observability
    LogLevel        string `json:"log_level"`
    MetricsEnabled  bool   `json:"metrics_enabled"`
    TracingEnabled  bool   `json:"tracing_enabled"`
    
    // Security
    CORSEnabled     bool     `json:"cors_enabled"`
    AllowedOrigins  []string `json:"allowed_origins"`
    
    // Database (optional)
    DatabaseConfig  *DatabaseConfig `json:"database_config,omitempty"`
}
```

### 2. Enhanced Context System

**Inspired by**: Streamer's context patterns but with more utilities built-in.

```go
// pkg/context/context.go
type Context struct {
    context.Context
    
    // Request/Response cycle
    Request    *Request
    Response   *Response
    
    // Observability
    Logger     Logger
    Metrics    MetricsCollector
    Tracer     Tracer
    
    // Utilities
    validator  Validator
    params     map[string]string
    values     map[string]interface{}
    
    // Optional database
    DB interface{}
    
    // Lambda-specific
    LambdaContext *lambdacontext.LambdaContext
    RequestID     string
    
    // Performance tracking
    startTime time.Time
}

// Context utilities inspired by Streamer's patterns
func (c *Context) Param(key string) string {
    return c.params[key]
}

func (c *Context) Query(key string) string {
    return c.Request.QueryParams[key]
}

func (c *Context) Header(key string) string {
    return c.Request.Headers[key]
}

func (c *Context) UserID() string {
    if userID, ok := c.values["user_id"].(string); ok {
        return userID
    }
    return ""
}

func (c *Context) TenantID() string {
    if tenantID, ok := c.values["tenant_id"].(string); ok {
        return tenantID
    }
    return ""
}

// Timeout utility from Streamer's connection manager patterns
func (c *Context) WithTimeout(duration time.Duration, fn func() (interface{}, error)) (interface{}, error) {
    ctx, cancel := context.WithTimeout(c.Context, duration)
    defer cancel()
    
    type result struct {
        value interface{}
        err   error
    }
    
    ch := make(chan result, 1)
    go func() {
        value, err := fn()
        ch <- result{value, err}
    }()
    
    select {
    case res := <-ch:
        return res.value, res.err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### 3. Type-Safe Handler System

**Lesson from DynamORM**: Type safety eliminates entire classes of runtime errors.

```go
// pkg/lift/handler.go
type Handler interface {
    Handle(ctx *Context) error
}

type HandlerFunc func(ctx *Context) error

// Type-safe handlers with automatic parsing/validation
type TypedHandler[Req, Resp any] interface {
    Handle(ctx *Context, req Req) (Resp, error)
}

// Convenience function for simple handlers
func SimpleHandler[Req, Resp any](handler func(ctx *Context, req Req) (Resp, error)) Handler {
    return TypedHandlerAdapter[Req, Resp](
        TypedHandlerFunc[Req, Resp](handler),
    )
}

type TypedHandlerFunc[Req, Resp any] func(ctx *Context, req Req) (Resp, error)

func (h TypedHandlerFunc[Req, Resp]) Handle(ctx *Context, req Req) (Resp, error) {
    return h(ctx, req)
}
```

### 4. Request/Response System

**Based on**: Streamer's event handling patterns but generalized for all Lambda triggers.

```go
// pkg/lift/request.go
type Request struct {
    // Common fields across all triggers
    Body            []byte
    Headers         map[string]string
    QueryParams     map[string]string
    PathParams      map[string]string
    
    // HTTP-specific (API Gateway)
    Method          string
    Path            string
    IsBase64Encoded bool
    
    // Lambda context
    RequestContext  map[string]interface{}
    
    // Trigger-specific data
    TriggerType     TriggerType
    RawEvent        interface{}
}

// pkg/lift/response.go
type Response struct {
    StatusCode      int                    `json:"statusCode"`
    Body            interface{}            `json:"body"`
    Headers         map[string]string      `json:"headers"`
    IsBase64Encoded bool                   `json:"isBase64Encoded"`
    
    // Internal state
    written bool
}

func (r *Response) JSON(data interface{}) error {
    if r.written {
        return errors.New("response already written")
    }
    
    r.Body = data
    r.Headers["Content-Type"] = "application/json"
    r.written = true
    return nil
}

func (r *Response) Status(code int) *Response {
    r.StatusCode = code
    return r
}
```

### 5. Routing System

**Inspired by**: Modern web frameworks but optimized for Lambda's single-handler model.

```go
// pkg/lift/router.go
type Router struct {
    routes     map[string]map[string]Handler // method -> path -> handler
    middleware []Middleware
    
    // Path parameter extraction
    paramRoutes map[string]*paramRoute
}

type paramRoute struct {
    pattern string
    handler Handler
    params  []string
}

func (r *Router) addRoute(method, path string, handler Handler) {
    if r.routes[method] == nil {
        r.routes[method] = make(map[string]Handler)
    }
    
    // Check for path parameters
    if strings.Contains(path, ":") {
        r.addParamRoute(method, path, handler)
    } else {
        r.routes[method][path] = handler
    }
}

func (r *Router) findHandler(method, path string) (Handler, map[string]string) {
    // Exact match first
    if handler, exists := r.routes[method][path]; exists {
        return handler, nil
    }
    
    // Parameter matching
    return r.findParamHandler(method, path)
}
```

### 6. Middleware System

**Lesson from Streamer**: Middleware should be composable and easy to test.

```go
// pkg/middleware/middleware.go
type Middleware func(Handler) Handler

// Chain multiple middleware
func Chain(middlewares ...Middleware) Middleware {
    return func(handler Handler) Handler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            handler = middlewares[i](handler)
        }
        return handler
    }
}

// Built-in middleware based on Streamer patterns
func Logger() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            start := time.Now()
            
            // Add request ID to logger
            ctx.Logger = ctx.Logger.WithField("request_id", ctx.RequestID)
            
            err := next.Handle(ctx)
            
            // Log completion
            ctx.Logger.WithFields(map[string]interface{}{
                "method":     ctx.Request.Method,
                "path":       ctx.Request.Path,
                "status":     ctx.Response.StatusCode,
                "duration":   time.Since(start),
                "error":      err,
            }).Info("Request completed")
            
            return err
        })
    }
}

func Recover() Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            defer func() {
                if r := recover(); r != nil {
                    ctx.Logger.WithField("panic", r).Error("Handler panicked")
                    ctx.Response.Status(500).JSON(map[string]string{
                        "error": "Internal server error",
                    })
                }
            }()
            
            return next.Handle(ctx)
        })
    }
}
```

### 7. Event Source Adapters

**Lesson from Streamer**: Different Lambda triggers need different handling but common patterns.

```go
// pkg/lift/adapters.go
type EventAdapter interface {
    Adapt(rawEvent interface{}) (*Request, error)
    GetTriggerType() TriggerType
}

// API Gateway adapter
type APIGatewayAdapter struct{}

func (a *APIGatewayAdapter) Adapt(rawEvent interface{}) (*Request, error) {
    event, ok := rawEvent.(events.APIGatewayProxyRequest)
    if !ok {
        return nil, errors.New("invalid API Gateway event")
    }
    
    return &Request{
        Body:            []byte(event.Body),
        Headers:         event.Headers,
        QueryParams:     event.QueryStringParameters,
        PathParams:      event.PathParameters,
        Method:          event.HTTPMethod,
        Path:            event.Path,
        IsBase64Encoded: event.IsBase64Encoded,
        RequestContext:  map[string]interface{}{"apiGateway": event.RequestContext},
        TriggerType:     TriggerAPIGateway,
        RawEvent:        rawEvent,
    }, nil
}

// SQS adapter
type SQSAdapter struct{}

func (a *SQSAdapter) Adapt(rawEvent interface{}) (*Request, error) {
    event, ok := rawEvent.(events.SQSEvent)
    if !ok {
        return nil, errors.New("invalid SQS event")
    }
    
    // For SQS, we'll process all records as a batch
    records := make([]map[string]interface{}, len(event.Records))
    for i, record := range event.Records {
        records[i] = map[string]interface{}{
            "messageId":     record.MessageId,
            "body":          record.Body,
            "attributes":    record.Attributes,
            "eventSource":   record.EventSource,
            "eventSourceARN": record.EventSourceARN,
        }
    }
    
    body, _ := json.Marshal(map[string]interface{}{
        "records": records,
    })
    
    return &Request{
        Body:        body,
        TriggerType: TriggerSQS,
        RawEvent:    rawEvent,
    }, nil
}
```

### 8. Error Handling System

**Based on**: Streamer's structured error handling.

```go
// pkg/errors/errors.go
type LiftError struct {
    Code       string                 `json:"code"`
    Message    string                 `json:"message"`
    Details    map[string]interface{} `json:"details,omitempty"`
    StatusCode int                    `json:"-"`
    Cause      error                  `json:"-"`
    
    // Observability
    RequestID  string `json:"request_id,omitempty"`
    Timestamp  int64  `json:"timestamp"`
}

func (e *LiftError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *LiftError) Unwrap() error {
    return e.Cause
}

// HTTP error constructors
func BadRequest(message string) *LiftError {
    return &LiftError{
        Code:       "BAD_REQUEST",
        Message:    message,
        StatusCode: 400,
        Timestamp:  time.Now().Unix(),
    }
}

func Unauthorized(message string) *LiftError {
    return &LiftError{
        Code:       "UNAUTHORIZED",
        Message:    message,
        StatusCode: 401,
        Timestamp:  time.Now().Unix(),
    }
}

func NotFound(message string) *LiftError {
    return &LiftError{
        Code:       "NOT_FOUND",
        Message:    message,
        StatusCode: 404,
        Timestamp:  time.Now().Unix(),
    }
}

func InternalError(message string) *LiftError {
    return &LiftError{
        Code:       "INTERNAL_ERROR",
        Message:    message,
        StatusCode: 500,
        Timestamp:  time.Now().Unix(),
    }
}

// Validation error with field details
func ValidationError(field, message string) *LiftError {
    return &LiftError{
        Code:       "VALIDATION_ERROR",
        Message:    "Validation failed",
        StatusCode: 400,
        Details: map[string]interface{}{
            "field":   field,
            "message": message,
        },
        Timestamp: time.Now().Unix(),
    }
}
```

### 9. Database Integration

**Lesson from DynamORM**: Database integration should be optional but seamless when needed.

```go
// pkg/database/database.go
type DatabaseClient interface {
    Query(ctx context.Context, query string, args ...interface{}) (interface{}, error)
    Execute(ctx context.Context, query string, args ...interface{}) error
    Close() error
}

// DynamoDB integration using DynamORM
type DynamoDBClient struct {
    db *dynamorm.DB
}

func NewDynamoDBClient(config DynamoDBConfig) (*DynamoDBClient, error) {
    db, err := dynamorm.New(session.Config{
        Region: config.Region,
    })
    if err != nil {
        return nil, err
    }
    
    return &DynamoDBClient{db: db}, nil
}

func (d *DynamoDBClient) Query(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
    // Implement DynamORM query logic
    return nil, nil
}

// PostgreSQL integration
type PostgreSQLClient struct {
    db *sql.DB
}

func NewPostgreSQLClient(config PostgreSQLConfig) (*PostgreSQLClient, error) {
    db, err := sql.Open("postgres", config.ConnectionString)
    if err != nil {
        return nil, err
    }
    
    return &PostgreSQLClient{db: db}, nil
}
```

### 10. Observability System

**Based on**: Streamer's comprehensive monitoring approach.

```go
// pkg/observability/logging.go
type Logger interface {
    Debug(message string, fields ...map[string]interface{})
    Info(message string, fields ...map[string]interface{})
    Warn(message string, fields ...map[string]interface{})
    Error(message string, fields ...map[string]interface{})
    WithField(key string, value interface{}) Logger
    WithFields(fields map[string]interface{}) Logger
}

// CloudWatch Logs integration
type CloudWatchLogger struct {
    serviceName string
    requestID   string
}

func (l *CloudWatchLogger) Info(message string, fields ...map[string]interface{}) {
    entry := map[string]interface{}{
        "level":      "INFO",
        "message":    message,
        "service":    l.serviceName,
        "request_id": l.requestID,
        "timestamp":  time.Now().Unix(),
    }
    
    if len(fields) > 0 {
        for k, v := range fields[0] {
            entry[k] = v
        }
    }
    
    data, _ := json.Marshal(entry)
    fmt.Println(string(data))
}

// pkg/observability/metrics.go
type MetricsCollector interface {
    Counter(name string, tags ...map[string]string) Counter
    Histogram(name string, tags ...map[string]string) Histogram
    Gauge(name string, tags ...map[string]string) Gauge
}

type Counter interface {
    Inc()
    Add(value float64)
}

type Histogram interface {
    Observe(value float64)
}

type Gauge interface {
    Set(value float64)
}

// CloudWatch Metrics integration
type CloudWatchMetrics struct {
    namespace string
    client    *cloudwatch.Client
}

func (m *CloudWatchMetrics) Counter(name string, tags ...map[string]string) Counter {
    return &CloudWatchCounter{
        name:      name,
        namespace: m.namespace,
        client:    m.client,
        tags:      mergeTags(tags...),
    }
}
```

## Performance Optimizations

### 1. Cold Start Optimization

**Target**: Sub-15ms overhead for cold starts.

```go
// pkg/lift/optimization.go
type OptimizationConfig struct {
    // Connection pooling
    PrewarmConnections bool `json:"prewarm_connections"`
    MaxConnections     int  `json:"max_connections"`
    
    // Memory management
    LazyInitialization bool `json:"lazy_initialization"`
    MemoryOptimization bool `json:"memory_optimization"`
    
    // Request processing
    StreamingEnabled   bool  `json:"streaming_enabled"`
    MaxRequestSize     int64 `json:"max_request_size"`
}

// Connection pool for database connections
type ConnectionPool struct {
    connections chan interface{}
    factory     func() (interface{}, error)
    mu          sync.Mutex
    closed      bool
}

func NewConnectionPool(size int, factory func() (interface{}, error)) *ConnectionPool {
    pool := &ConnectionPool{
        connections: make(chan interface{}, size),
        factory:     factory,
    }
    
    // Pre-warm connections
    for i := 0; i < size; i++ {
        if conn, err := factory(); err == nil {
            pool.connections <- conn
        }
    }
    
    return pool
}

func (p *ConnectionPool) Get() (interface{}, error) {
    select {
    case conn := <-p.connections:
        return conn, nil
    default:
        return p.factory()
    }
}

func (p *ConnectionPool) Put(conn interface{}) error {
    if p.closed {
        return errors.New("pool is closed")
    }
    
    select {
    case p.connections <- conn:
        return nil
    default:
        // Pool is full, discard connection
        return nil
    }
}
```

### 2. Memory Management

**Lesson from Streamer**: Lambda memory usage directly impacts cost and performance.

```go
// pkg/lift/memory.go
type MemoryManager struct {
    maxRequestSize  int64
    maxResponseSize int64
    bufferPool      sync.Pool
}

func NewMemoryManager(config MemoryConfig) *MemoryManager {
    return &MemoryManager{
        maxRequestSize:  config.MaxRequestSize,
        maxResponseSize: config.MaxResponseSize,
        bufferPool: sync.Pool{
            New: func() interface{} {
                return make([]byte, 0, 1024) // 1KB initial capacity
            },
        },
    }
}

func (m *MemoryManager) GetBuffer() []byte {
    return m.bufferPool.Get().([]byte)
}

func (m *MemoryManager) PutBuffer(buf []byte) {
    if cap(buf) <= 64*1024 { // Don't pool buffers larger than 64KB
        buf = buf[:0] // Reset length but keep capacity
        m.bufferPool.Put(buf)
    }
}

// Streaming request parser for large payloads
func (c *Context) ParseRequestStream(v interface{}) error {
    if len(c.Request.Body) > c.app.memoryManager.maxRequestSize {
        return errors.New("request too large")
    }
    
    // Use streaming JSON decoder for large requests
    decoder := json.NewDecoder(bytes.NewReader(c.Request.Body))
    return decoder.Decode(v)
}
```

## Testing Architecture

**Lesson from Streamer**: Testing Lambda handlers should be as easy as testing HTTP handlers.

```go
// pkg/testing/testing.go
type TestApp struct {
    *App
    recorder *ResponseRecorder
}

func NewTestApp() *TestApp {
    app := New()
    return &TestApp{
        App:      app,
        recorder: &ResponseRecorder{},
    }
}

func (t *TestApp) Request(method, path string, body interface{}) *TestResponse {
    // Create test request
    var bodyBytes []byte
    if body != nil {
        bodyBytes, _ = json.Marshal(body)
    }
    
    req := &Request{
        Method:      method,
        Path:        path,
        Body:        bodyBytes,
        Headers:     make(map[string]string),
        QueryParams: make(map[string]string),
        PathParams:  make(map[string]string),
    }
    
    // Create test context
    ctx := &Context{
        Context:   context.Background(),
        Request:   req,
        Response:  &Response{Headers: make(map[string]string)},
        Logger:    &TestLogger{},
        Metrics:   &TestMetrics{},
        RequestID: "test-request-id",
        startTime: time.Now(),
    }
    
    // Execute request
    handler, params := t.router.findHandler(method, path)
    if handler == nil {
        ctx.Response.Status(404).JSON(map[string]string{
            "error": "Not found",
        })
    } else {
        ctx.params = params
        if err := handler.Handle(ctx); err != nil {
            if liftErr, ok := err.(*LiftError); ok {
                ctx.Response.Status(liftErr.StatusCode).JSON(liftErr)
            } else {
                ctx.Response.Status(500).JSON(map[string]string{
                    "error": err.Error(),
                })
            }
        }
    }
    
    return &TestResponse{
        StatusCode: ctx.Response.StatusCode,
        Body:       ctx.Response.Body,
        Headers:    ctx.Response.Headers,
    }
}

type TestResponse struct {
    StatusCode int
    Body       interface{}
    Headers    map[string]string
}

func (r *TestResponse) JSON(v interface{}) error {
    if bodyBytes, ok := r.Body.([]byte); ok {
        return json.Unmarshal(bodyBytes, v)
    }
    
    bodyBytes, err := json.Marshal(r.Body)
    if err != nil {
        return err
    }
    
    return json.Unmarshal(bodyBytes, v)
}
```

## Security Considerations

### 1. Input Validation

```go
// pkg/validation/validator.go
type Validator interface {
    Validate(interface{}) error
}

type StructValidator struct {
    validator *validator.Validate
}

func NewStructValidator() *StructValidator {
    v := validator.New()
    
    // Register custom validation rules
    v.RegisterValidation("tenant_id", validateTenantID)
    v.RegisterValidation("user_id", validateUserID)
    
    return &StructValidator{validator: v}
}

func validateTenantID(fl validator.FieldLevel) bool {
    tenantID := fl.Field().String()
    // Validate tenant ID format (e.g., UUID)
    return regexp.MustCompile(`^[a-fA-F0-9-]{36}$`).MatchString(tenantID)
}
```

### 2. Authentication Integration

```go
// pkg/middleware/auth.go
type AuthConfig struct {
    JWTSecret     string   `json:"jwt_secret"`
    JWTIssuer     string   `json:"jwt_issuer"`
    RequiredRoles []string `json:"required_roles"`
    SkipPaths     []string `json:"skip_paths"`
}

func JWT(config AuthConfig) Middleware {
    return func(next Handler) Handler {
        return HandlerFunc(func(ctx *Context) error {
            // Skip authentication for certain paths
            for _, path := range config.SkipPaths {
                if ctx.Request.Path == path {
                    return next.Handle(ctx)
                }
            }
            
            // Extract token
            token := extractToken(ctx.Request)
            if token == "" {
                return Unauthorized("Missing authorization token")
            }
            
            // Validate JWT
            claims, err := validateJWT(token, config)
            if err != nil {
                return Unauthorized("Invalid token")
            }
            
            // Check required roles
            if len(config.RequiredRoles) > 0 {
                if !hasRequiredRole(claims.Roles, config.RequiredRoles) {
                    return Forbidden("Insufficient permissions")
                }
            }
            
            // Add claims to context
            ctx.Set("user_id", claims.Subject)
            ctx.Set("tenant_id", claims.TenantID)
            ctx.Set("roles", claims.Roles)
            
            return next.Handle(ctx)
        })
    }
}
```

## Deployment and Operations

### 1. Configuration Management

```go
// pkg/config/config.go
type Config struct {
    // Application settings
    Environment     string        `env:"ENVIRONMENT" default:"development"`
    LogLevel        string        `env:"LOG_LEVEL" default:"INFO"`
    Timeout         time.Duration `env:"TIMEOUT" default:"30s"`
    
    // Database settings
    DatabaseURL     string `env:"DATABASE_URL"`
    DatabaseType    string `env:"DATABASE_TYPE" default:"dynamodb"`
    
    // Observability
    MetricsEnabled  bool   `env:"METRICS_ENABLED" default:"true"`
    TracingEnabled  bool   `env:"TRACING_ENABLED" default:"true"`
    
    // Security
    JWTSecret       string   `env:"JWT_SECRET"`
    JWTIssuer       string   `env:"JWT_ISSUER"`
    AllowedOrigins  []string `env:"ALLOWED_ORIGINS"`
}

func LoadConfig() (*Config, error) {
    var config Config
    if err := env.Parse(&config); err != nil {
        return nil, err
    }
    return &config, nil
}
```

### 2. Health Checks and Monitoring

```go
// pkg/health/health.go
type HealthChecker struct {
    checks map[string]HealthCheck
}

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) error
}

type DatabaseHealthCheck struct {
    db DatabaseClient
}

func (h *DatabaseHealthCheck) Name() string {
    return "database"
}

func (h *DatabaseHealthCheck) Check(ctx context.Context) error {
    return h.db.Query(ctx, "SELECT 1")
}

// Health endpoint
func (a *App) GET("/health", func(ctx *Context) error {
    results := make(map[string]interface{})
    
    for name, check := range a.healthChecker.checks {
        if err := check.Check(ctx.Context); err != nil {
            results[name] = map[string]interface{}{
                "status": "unhealthy",
                "error":  err.Error(),
            }
        } else {
            results[name] = map[string]interface{}{
                "status": "healthy",
            }
        }
    }
    
    return ctx.JSON(map[string]interface{}{
        "status": "ok",
        "checks": results,
        "timestamp": time.Now().Unix(),
    })
})
```

This technical architecture provides a solid foundation for the Lift framework, incorporating lessons learned from both DynamORM and Streamer while providing a modern, type-safe development experience for Lambda functions. 