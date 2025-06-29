# Lift Middleware Examples for Common Use Cases

## 1. Authentication Middleware with JWT

```go
package middleware

import (
    "strings"
    "github.com/golang-jwt/jwt/v5"
    "github.com/pay-theory/lift/pkg/lift"
)

type Claims struct {
    UserID   string `json:"user_id"`
    TenantID string `json:"tenant_id"`
    jwt.RegisteredClaims
}

func JWTAuth(secretKey string) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Get Authorization header
            authHeader := ctx.Header("Authorization")
            if authHeader == "" {
                return lift.NewError(401, "Missing authorization header", nil)
            }
            
            // Extract token
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenString == authHeader {
                return lift.NewError(401, "Invalid authorization format", nil)
            }
            
            // Parse and validate token
            token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
                return []byte(secretKey), nil
            })
            
            if err != nil || !token.Valid {
                return lift.NewError(401, "Invalid token", nil)
            }
            
            // Set user context
            if claims, ok := token.Claims.(*Claims); ok {
                ctx.Set("user_id", claims.UserID)
                ctx.Set("tenant_id", claims.TenantID)
                ctx.Set("authenticated", true)
            }
            
            return next.Handle(ctx)
        })
    }
}

// Usage:
// app.Use(middleware.JWTAuth(os.Getenv("JWT_SECRET")))
```

## 2. Database Connection Middleware

```go
package middleware

import (
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
    "github.com/pay-theory/lift/pkg/lift"
)

// Database connection pool (initialized once)
var db *sql.DB

func InitDB(connectionString string) error {
    var err error
    db, err = sql.Open("postgres", connectionString)
    if err != nil {
        return err
    }
    
    // Configure connection pool
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    
    return db.Ping()
}

func Database() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            if db == nil {
                return lift.NewError(500, "Database not initialized", nil)
            }
            
            // Set database in context
            ctx.Set("db", db)
            
            return next.Handle(ctx)
        })
    }
}

// Helper function to get DB from context
func GetDB(ctx *lift.Context) (*sql.DB, error) {
    dbInterface := ctx.Get("db")
    if dbInterface == nil {
        return nil, fmt.Errorf("database not found in context")
    }
    
    database, ok := dbInterface.(*sql.DB)
    if !ok {
        return nil, fmt.Errorf("invalid database type in context")
    }
    
    return database, nil
}

// Usage in main:
// middleware.InitDB(os.Getenv("DATABASE_URL"))
// app.Use(middleware.Database())
```

## 3. Multi-Tenant Middleware

```go
package middleware

import (
    "github.com/pay-theory/lift/pkg/lift"
)

type TenantService interface {
    ValidateTenant(tenantID string) (*Tenant, error)
}

type Tenant struct {
    ID       string
    Name     string
    IsActive bool
    Plan     string
}

func MultiTenant(tenantService TenantService) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Get tenant ID from header or subdomain
            tenantID := ctx.Header("X-Tenant-ID")
            if tenantID == "" {
                // Try to extract from subdomain
                host := ctx.Header("Host")
                tenantID = extractTenantFromHost(host)
            }
            
            if tenantID == "" {
                return lift.NewError(400, "Tenant ID required", nil)
            }
            
            // Validate tenant
            tenant, err := tenantService.ValidateTenant(tenantID)
            if err != nil {
                return lift.NewError(404, "Tenant not found", nil)
            }
            
            if !tenant.IsActive {
                return lift.NewError(403, "Tenant is inactive", nil)
            }
            
            // Set tenant context
            ctx.Set("tenant", tenant)
            ctx.Set("tenant_id", tenant.ID)
            
            return next.Handle(ctx)
        })
    }
}

// Helper to get tenant from context
func GetTenant(ctx *lift.Context) (*Tenant, error) {
    tenantInterface := ctx.Get("tenant")
    if tenantInterface == nil {
        return nil, fmt.Errorf("tenant not found in context")
    }
    
    tenant, ok := tenantInterface.(*Tenant)
    if !ok {
        return nil, fmt.Errorf("invalid tenant type in context")
    }
    
    return tenant, nil
}
```

## 4. Request ID and Logging Middleware

```go
package middleware

import (
    "log"
    "time"
    "github.com/google/uuid"
    "github.com/pay-theory/lift/pkg/lift"
)

func RequestID() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Generate or extract request ID
            requestID := ctx.Header("X-Request-ID")
            if requestID == "" {
                requestID = uuid.New().String()
            }
            
            // Set in context
            ctx.Set("request_id", requestID)
            
            // Add to response headers
            ctx.SetHeader("X-Request-ID", requestID)
            
            return next.Handle(ctx)
        })
    }
}

func StructuredLogger() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            start := time.Now()
            
            // Get request ID
            requestID, _ := ctx.Get("request_id").(string)
            
            // Log request start
            log.Printf("[%s] START %s %s", requestID, ctx.Method(), ctx.Path())
            
            // Process request
            err := next.Handle(ctx)
            
            // Calculate duration
            duration := time.Since(start)
            
            // Log completion
            if err != nil {
                log.Printf("[%s] ERROR %s %s - %v (took %v)", 
                    requestID, ctx.Method(), ctx.Path(), err, duration)
            } else {
                log.Printf("[%s] COMPLETED %s %s - %d (took %v)", 
                    requestID, ctx.Method(), ctx.Path(), ctx.StatusCode(), duration)
            }
            
            return err
        })
    }
}
```

## 5. Rate Limiting Middleware

```go
package middleware

import (
    "fmt"
    "sync"
    "time"
    "github.com/pay-theory/lift/pkg/lift"
)

type RateLimiter struct {
    requests map[string][]time.Time
    mu       sync.RWMutex
    rate     int
    window   time.Duration
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
    return &RateLimiter{
        requests: make(map[string][]time.Time),
        rate:     rate,
        window:   window,
    }
}

func (rl *RateLimiter) Allow(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    windowStart := now.Add(-rl.window)
    
    // Get existing requests
    requests, exists := rl.requests[key]
    if !exists {
        rl.requests[key] = []time.Time{now}
        return true
    }
    
    // Remove old requests
    validRequests := []time.Time{}
    for _, req := range requests {
        if req.After(windowStart) {
            validRequests = append(validRequests, req)
        }
    }
    
    // Check rate limit
    if len(validRequests) >= rl.rate {
        rl.requests[key] = validRequests
        return false
    }
    
    // Add new request
    validRequests = append(validRequests, now)
    rl.requests[key] = validRequests
    return true
}

func RateLimit(limiter *RateLimiter) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Get rate limit key (IP, user ID, etc.)
            key := ctx.Header("X-Forwarded-For")
            if key == "" {
                key = ctx.Get("user_id").(string)
            }
            
            if !limiter.Allow(key) {
                return lift.NewError(429, "Rate limit exceeded", map[string]interface{}{
                    "retry_after": limiter.window.Seconds(),
                })
            }
            
            return next.Handle(ctx)
        })
    }
}

// Usage:
// limiter := middleware.NewRateLimiter(100, time.Minute)
// app.Use(middleware.RateLimit(limiter))
```

## 6. Service Injection Middleware

```go
package middleware

import (
    "github.com/pay-theory/lift/pkg/lift"
    "your-app/services"
)

type ServiceContainer struct {
    UserService     services.UserService
    ProductService  services.ProductService
    OrderService    services.OrderService
    EmailService    services.EmailService
}

func ServiceInjection(container *ServiceContainer) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Inject all services into context
            ctx.Set("services", container)
            
            // Or inject individually
            ctx.Set("user_service", container.UserService)
            ctx.Set("product_service", container.ProductService)
            ctx.Set("order_service", container.OrderService)
            ctx.Set("email_service", container.EmailService)
            
            return next.Handle(ctx)
        })
    }
}

// Helper functions to retrieve services
func GetServices(ctx *lift.Context) (*ServiceContainer, error) {
    servicesInterface := ctx.Get("services")
    if servicesInterface == nil {
        return nil, fmt.Errorf("services not found in context")
    }
    
    services, ok := servicesInterface.(*ServiceContainer)
    if !ok {
        return nil, fmt.Errorf("invalid services type in context")
    }
    
    return services, nil
}

func GetUserService(ctx *lift.Context) (services.UserService, error) {
    serviceInterface := ctx.Get("user_service")
    if serviceInterface == nil {
        return nil, fmt.Errorf("user service not found in context")
    }
    
    service, ok := serviceInterface.(services.UserService)
    if !ok {
        return nil, fmt.Errorf("invalid user service type in context")
    }
    
    return service, nil
}
```

## 7. Complete Application Example

```go
package main

import (
    "log"
    "os"
    "time"
    
    "github.com/pay-theory/lift/pkg/lift"
    "your-app/middleware"
    "your-app/handlers"
    "your-app/services"
)

func main() {
    // Initialize services
    err := middleware.InitDB(os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatal("Failed to initialize database:", err)
    }
    
    // Create service container
    container := &middleware.ServiceContainer{
        UserService:    services.NewUserService(),
        ProductService: services.NewProductService(),
        OrderService:   services.NewOrderService(),
        EmailService:   services.NewEmailService(),
    }
    
    // Create rate limiter
    rateLimiter := middleware.NewRateLimiter(100, time.Minute)
    
    // Create app
    app := lift.New()
    
    // Global middleware (order matters!)
    app.Use(lift.Recover())                              // Panic recovery
    app.Use(middleware.RequestID())                      // Request tracking
    app.Use(middleware.StructuredLogger())               // Logging
    app.Use(middleware.Database())                       // Database connection
    app.Use(middleware.ServiceInjection(container))      // Service injection
    
    // Public routes
    public := app.Group("/api/v1")
    public.POST("/auth/login", handlers.Login)
    public.POST("/auth/register", handlers.Register)
    
    // Protected routes
    protected := app.Group("/api/v1")
    protected.Use(middleware.JWTAuth(os.Getenv("JWT_SECRET")))
    protected.Use(middleware.RateLimit(rateLimiter))
    
    // User routes
    protected.GET("/users/me", handlers.GetCurrentUser)
    protected.PUT("/users/me", handlers.UpdateCurrentUser)
    
    // Multi-tenant routes
    tenant := protected.Group("")
    tenant.Use(middleware.MultiTenant(container.TenantService))
    
    tenant.GET("/products", handlers.ListProducts)
    tenant.POST("/products", handlers.CreateProduct)
    tenant.GET("/orders", handlers.ListOrders)
    tenant.POST("/orders", handlers.CreateOrder)
    
    // Admin routes
    admin := protected.Group("/admin")
    admin.Use(middleware.RequireRole("admin"))
    
    admin.GET("/users", handlers.ListAllUsers)
    admin.DELETE("/users/:id", handlers.DeleteUser)
    
    // Start the application
    log.Println("Starting application...")
    app.Start()
}
```

## Testing Middleware

```go
package middleware_test

import (
    "testing"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/testing"
    "your-app/middleware"
)

func TestJWTAuthMiddleware(t *testing.T) {
    app := lift.New()
    
    // Add middleware
    app.Use(middleware.JWTAuth("test-secret"))
    
    // Add test handler
    app.GET("/protected", func(ctx *lift.Context) error {
        userID := ctx.Get("user_id")
        if userID == nil {
            t.Error("User ID not set by middleware")
        }
        return ctx.JSON(map[string]string{"user_id": userID.(string)})
    })
    
    // Create test app
    testApp := testing.NewTestApp()
    testApp.ConfigureFromApp(app)
    
    // Test without token
    ctx := testing.NewTestContext(testing.WithRequest(testing.Request{
        Method: "GET",
        Path:   "/protected",
    }))
    
    err := testApp.HandleTestRequest(ctx)
    if err == nil {
        t.Error("Expected error without token")
    }
    
    // Test with valid token
    validToken := generateTestToken("user123", "tenant456")
    ctx = testing.NewTestContext(testing.WithRequest(testing.Request{
        Method: "GET",
        Path:   "/protected",
        Headers: map[string]string{
            "Authorization": "Bearer " + validToken,
        },
    }))
    
    err = testApp.HandleTestRequest(ctx)
    if err != nil {
        t.Errorf("Expected success with valid token: %v", err)
    }
}
```

## Best Practices

1. **Order Matters**: Place error recovery first, logging early, auth before business logic
2. **Use Groups**: Organize routes with common middleware using groups
3. **Error Handling**: Always handle nil checks when retrieving from context
4. **Performance**: Initialize expensive resources (DB, services) once and reuse
5. **Testing**: Test middleware in isolation and as part of the full chain
6. **Security**: Never log sensitive data (tokens, passwords) in middleware