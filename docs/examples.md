# Examples

This guide provides comprehensive examples of using Lift in various scenarios, from simple APIs to complex enterprise applications.

## Example Directory

The `examples/` directory contains the following complete working examples:

### Core Examples
- [`hello-world/`](../examples/hello-world/) - Minimal Lambda function with type safety
- [`basic-crud-api/`](../examples/basic-crud-api/) - Complete CRUD API with middleware and testing
- [`error-handling/`](../examples/error-handling/) - Structured error handling patterns

### Authentication & Security
- [`jwt-auth/`](../examples/jwt-auth/) - JWT authentication with middleware
- [`jwt-auth-demo/`](../examples/jwt-auth-demo/) - JWT authentication demonstration
- [`rate-limiting/`](../examples/rate-limiting/) - Rate limiting middleware implementation

### Real-time & Event-Driven
- [`websocket-demo/`](../examples/websocket-demo/) - WebSocket support with connection management
- [`websocket-enhanced/`](../examples/websocket-enhanced/) - Advanced WebSocket implementation
- [`event-adapters/`](../examples/event-adapters/) - Multiple AWS event source handlers
- [`multi-event-handler/`](../examples/multi-event-handler/) - Handling multiple event types
- [`eventbridge-wakeup/`](../examples/eventbridge-wakeup/) - EventBridge scheduled events
- [`multiple-scheduled-events/`](../examples/multiple-scheduled-events/) - Multiple scheduled Lambda triggers

### Enterprise Applications
- [`multi-tenant-saas/`](../examples/multi-tenant-saas/) - Multi-tenant SaaS patterns
- [`enterprise-banking/`](../examples/enterprise-banking/) - Banking with SOC2 compliance
- [`enterprise-healthcare/`](../examples/enterprise-healthcare/) - Healthcare with HIPAA compliance
- [`enterprise-ecommerce/`](../examples/enterprise-ecommerce/) - E-commerce platform example

### Production Patterns
- [`production-api/`](../examples/production-api/) - Production-ready API configuration
- [`multi-service-demo/`](../examples/multi-service-demo/) - Microservices with chaos engineering
- [`observability-demo/`](../examples/observability-demo/) - Comprehensive logging and monitoring
- [`health-monitoring/`](../examples/health-monitoring/) - Health check implementation

### Testing & Development
- [`mocking-demo/`](../examples/mocking-demo/) - Testing with mocks
- [`cloudwatch-mocking-demo/`](../examples/cloudwatch-mocking-demo/) - CloudWatch metrics mocking
- [`dynamorm-integration/`](../examples/dynamorm-integration/) - DynamORM database integration
- [`streamer-quickstart/`](../examples/streamer-quickstart/) - Quick start with streaming
- [`sprint6-deployment/`](../examples/sprint6-deployment/) - Deployment patterns
- [`test-event-routing-bug/`](../examples/test-event-routing-bug/) - Event routing test cases
- [`test-scheduled-fix/`](../examples/test-scheduled-fix/) - Scheduled event fixes

## Basic Examples

### Hello World

The simplest Lift application:

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    app.GET("/", func(ctx *lift.Context) error {
        return ctx.JSON(map[string]string{
            "message": "Hello from Lift!",
        })
    })
    
    lambda.Start(app.HandleRequest)
}
```

### Basic CRUD API

A complete CRUD API example:

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

type User struct {
    ID    string `json:"id" dynamodb:"id"`
    Name  string `json:"name" dynamodb:"name"`
    Email string `json:"email" dynamodb:"email"`
    Age   int    `json:"age" dynamodb:"age"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18"`
}

func main() {
    app := lift.New()
    
    // Middleware
    app.Use(middleware.Logger())
    app.Use(middleware.Recover())
    app.Use(middleware.RequestID())
    
    // Routes
    app.GET("/users", listUsers)
    app.GET("/users/:id", getUser)
    app.POST("/users", createUser)
    app.PUT("/users/:id", updateUser)
    app.DELETE("/users/:id", deleteUser)
    
    lambda.Start(app.HandleRequest)
}

func listUsers(ctx *lift.Context) error {
    // Query parameters
    limit := ctx.QueryInt("limit", 20)
    offset := ctx.QueryInt("offset", 0)
    
    users, err := getUsersFromDB(limit, offset)
    if err != nil {
        return lift.InternalError("Failed to fetch users")
    }
    
    return ctx.JSON(users)
}

func getUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    user, err := getUserByID(userID)
    if err != nil {
        return lift.NotFound("User not found")
    }
    
    return ctx.JSON(user)
}

func createUser(ctx *lift.Context) error {
    var req CreateUserRequest
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err
    }
    
    user := &User{
        ID:    generateID(),
        Name:  req.Name,
        Email: req.Email,
        Age:   req.Age,
    }
    
    if err := saveUser(user); err != nil {
        return lift.InternalError("Failed to create user")
    }
    
    return ctx.Status(201).JSON(user)
}

func updateUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    var updates map[string]interface{}
    if err := ctx.ParseJSON(&updates); err != nil {
        return lift.BadRequest("Invalid request body")
    }
    
    user, err := updateUserByID(userID, updates)
    if err != nil {
        return lift.NotFound("User not found")
    }
    
    return ctx.JSON(user)
}

func deleteUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    if err := deleteUserByID(userID); err != nil {
        return lift.NotFound("User not found")
    }
    
    return ctx.NoContent()
}
```

## Authentication Examples

### JWT Authentication

```go
package main

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

type LoginRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

type AuthResponse struct {
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
    User      UserInfo  `json:"user"`
}

func main() {
    app := lift.New()
    
    // Public routes
    app.POST("/auth/login", login)
    app.POST("/auth/register", register)
    app.POST("/auth/refresh", refreshToken)
    
    // Protected routes
    protected := app.Group("/api", middleware.JWT(middleware.JWTConfig{
        SecretKey: []byte(os.Getenv("JWT_SECRET")),
        ErrorHandler: func(ctx *lift.Context, err error) error {
            return lift.Unauthorized("Invalid or expired token")
        },
    }))
    
    protected.GET("/profile", getProfile)
    protected.PUT("/profile", updateProfile)
    protected.POST("/logout", logout)
    
    lambda.Start(app.HandleRequest)
}

func login(ctx *lift.Context) error {
    var req LoginRequest
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err
    }
    
    // Authenticate user
    user, err := authenticateUser(req.Email, req.Password)
    if err != nil {
        return lift.Unauthorized("Invalid credentials")
    }
    
    // Generate token
    token, expiresAt, err := generateToken(user)
    if err != nil {
        return lift.InternalError("Failed to generate token")
    }
    
    // Log successful login
    ctx.Logger.Info("User logged in", map[string]interface{}{
        "user_id": user.ID,
        "email":   user.Email,
    })
    
    return ctx.JSON(AuthResponse{
        Token:     token,
        ExpiresAt: expiresAt,
        User: UserInfo{
            ID:    user.ID,
            Email: user.Email,
            Name:  user.Name,
        },
    })
}

func generateToken(user *User) (string, time.Time, error) {
    expiresAt := time.Now().Add(24 * time.Hour)
    
    claims := jwt.MapClaims{
        "sub":       user.ID,
        "email":     user.Email,
        "tenant_id": user.TenantID,
        "roles":     user.Roles,
        "exp":       expiresAt.Unix(),
        "iat":       time.Now().Unix(),
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
    
    return tokenString, expiresAt, err
}

func getProfile(ctx *lift.Context) error {
    claims := ctx.Get("claims").(jwt.MapClaims)
    userID := claims["sub"].(string)
    
    user, err := getUserByID(userID)
    if err != nil {
        return lift.NotFound("User not found")
    }
    
    return ctx.JSON(user)
}
```

### API Key Authentication

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
)

func APIKeyMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            apiKey := ctx.Header("X-API-Key")
            if apiKey == "" {
                return lift.Unauthorized("API key required")
            }
            
            // Validate API key
            keyInfo, err := validateAPIKey(apiKey)
            if err != nil {
                return lift.Unauthorized("Invalid API key")
            }
            
            // Check rate limits
            if err := checkRateLimit(keyInfo); err != nil {
                return lift.TooManyRequests("Rate limit exceeded")
            }
            
            // Set context
            ctx.Set("api_key_info", keyInfo)
            ctx.SetTenantID(keyInfo.TenantID)
            
            return next.Handle(ctx)
        })
    }
}

func main() {
    app := lift.New()
    
    // API key protected routes
    api := app.Group("/api/v1", APIKeyMiddleware())
    
    api.GET("/data", getData)
    api.POST("/webhook", handleWebhook)
    
    lambda.Start(app.HandleRequest)
}
```

## Event Processing Examples

### SQS Message Processing

```go
package main

import (
    "encoding/json"
    "github.com/pay-theory/lift/pkg/lift"
)

type OrderMessage struct {
    OrderID    string  `json:"order_id"`
    CustomerID string  `json:"customer_id"`
    Amount     float64 `json:"amount"`
    Status     string  `json:"status"`
}

func main() {
    app := lift.New()
    
    // SQS handler
    app.Handle("SQS", "/process-orders", processOrderQueue)
    
    lambda.Start(app.HandleRequest)
}

func processOrderQueue(ctx *lift.Context) error {
    var successCount, errorCount int
    
    // Process batch of messages
    for _, record := range ctx.Request.Records {
        sqsRecord := record.(map[string]interface{})
        body := sqsRecord["body"].(string)
        
        var order OrderMessage
        if err := json.Unmarshal([]byte(body), &order); err != nil {
            ctx.Logger.Error("Failed to parse message", map[string]interface{}{
                "error": err.Error(),
                "body":  body,
            })
            errorCount++
            continue
        }
        
        // Process order
        if err := processOrder(ctx, order); err != nil {
            ctx.Logger.Error("Failed to process order", map[string]interface{}{
                "order_id": order.OrderID,
                "error":    err.Error(),
            })
            errorCount++
            // Return error to retry message
            return err
        }
        
        successCount++
    }
    
    ctx.Logger.Info("Batch processing complete", map[string]interface{}{
        "success_count": successCount,
        "error_count":   errorCount,
    })
    
    return nil
}

func processOrder(ctx *lift.Context, order OrderMessage) error {
    // Start trace segment
    segment := ctx.StartSegment("process_order")
    defer segment.End()
    
    segment.AddMetadata("order", map[string]interface{}{
        "id":     order.OrderID,
        "amount": order.Amount,
    })
    
    // Business logic
    switch order.Status {
    case "pending":
        return processPendingOrder(order)
    case "confirmed":
        return processConfirmedOrder(order)
    case "cancelled":
        return processCancelledOrder(order)
    default:
        return fmt.Errorf("unknown order status: %s", order.Status)
    }
}
```

### S3 Event Processing

```go
package main

import (
    "strings"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
    app := lift.New()
    
    // S3 event handler
    app.Handle("S3", "/process-uploads", processS3Upload)
    
    lambda.Start(app.HandleRequest)
}

func processS3Upload(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        s3Record := record.(map[string]interface{})
        eventName := s3Record["eventName"].(string)
        
        // Only process new objects
        if !strings.HasPrefix(eventName, "s3:ObjectCreated:") {
            continue
        }
        
        s3Data := s3Record["s3"].(map[string]interface{})
        bucket := s3Data["bucket"].(map[string]interface{})["name"].(string)
        object := s3Data["object"].(map[string]interface{})
        key := object["key"].(string)
        size := int64(object["size"].(float64))
        
        ctx.Logger.Info("Processing S3 object", map[string]interface{}{
            "bucket": bucket,
            "key":    key,
            "size":   size,
        })
        
        // Process based on file type
        switch {
        case strings.HasSuffix(key, ".jpg") || strings.HasSuffix(key, ".png"):
            return processImage(ctx, bucket, key)
        case strings.HasSuffix(key, ".pdf"):
            return processPDF(ctx, bucket, key)
        case strings.HasSuffix(key, ".csv"):
            return processCSV(ctx, bucket, key)
        default:
            ctx.Logger.Warn("Unsupported file type", map[string]interface{}{
                "key": key,
            })
        }
    }
    
    return nil
}

func processImage(ctx *lift.Context, bucket, key string) error {
    // Generate thumbnails
    sizes := []int{100, 200, 400}
    
    for _, size := range sizes {
        thumbnailKey := fmt.Sprintf("thumbnails/%dx%d/%s", size, size, key)
        
        if err := generateThumbnail(bucket, key, thumbnailKey, size); err != nil {
            return fmt.Errorf("failed to generate %dx%d thumbnail: %w", size, size, err)
        }
    }
    
    // Update metadata
    return updateImageMetadata(bucket, key)
}
```

### EventBridge Custom Events

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/mitchellh/mapstructure"
)

type UserEvent struct {
    UserID    string                 `json:"user_id"`
    EventType string                 `json:"event_type"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
}

func main() {
    app := lift.New()
    
    // EventBridge handler
    app.Handle("EventBridge", "/user-events", handleUserEvents)
    
    lambda.Start(app.HandleRequest)
}

func handleUserEvents(ctx *lift.Context) error {
    detail := ctx.Request.Metadata["detail"].(map[string]interface{})
    detailType := ctx.Request.Metadata["detail-type"].(string)
    
    var event UserEvent
    if err := mapstructure.Decode(detail, &event); err != nil {
        return lift.BadRequest("Invalid event format")
    }
    
    ctx.Logger.Info("Processing user event", map[string]interface{}{
        "user_id":     event.UserID,
        "event_type":  detailType,
        "detail_type": detailType,
    })
    
    switch detailType {
    case "UserRegistered":
        return handleUserRegistered(ctx, event)
    case "UserUpdated":
        return handleUserUpdated(ctx, event)
    case "UserDeleted":
        return handleUserDeleted(ctx, event)
    case "UserLoginFailed":
        return handleLoginFailed(ctx, event)
    default:
        ctx.Logger.Warn("Unknown event type", map[string]interface{}{
            "detail_type": detailType,
        })
        return nil
    }
}

func handleUserRegistered(ctx *lift.Context, event UserEvent) error {
    // Send welcome email
    if err := sendWelcomeEmail(event.UserID); err != nil {
        return err
    }
    
    // Create default settings
    if err := createDefaultSettings(event.UserID); err != nil {
        return err
    }
    
    // Update analytics
    ctx.Metrics.Count("users.registered", 1, map[string]string{
        "source": event.Data["source"].(string),
    })
    
    return nil
}
```

## WebSocket Examples

### Real-time Chat Application

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

type Message struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    Username  string    `json:"username"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
    Type      string    `json:"type"`
}

func main() {
    app := lift.New()
    
    // WebSocket routes
    app.Handle("CONNECT", "/connect", handleConnect)
    app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
    app.Handle("MESSAGE", "/message", handleMessage)
    app.Handle("MESSAGE", "/typing", handleTyping)
    app.Handle("MESSAGE", "/presence", handlePresence)
    
    lambda.Start(app.HandleRequest)
}

func handleConnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    connectionID := wsCtx.ConnectionID()
    
    // Validate auth token from query params
    token := ctx.Query("token")
    userID, username, err := validateWebSocketToken(token)
    if err != nil {
        return lift.Unauthorized("Invalid token")
    }
    
    // Store connection
    if err := storeConnection(connectionID, userID, username); err != nil {
        return lift.InternalError("Failed to store connection")
    }
    
    // Notify other users
    connections, _ := getActiveConnections()
    wsCtx.BroadcastMessage(connections, Message{
        Type:      "user_joined",
        UserID:    userID,
        Username:  username,
        Timestamp: time.Now(),
    })
    
    ctx.Logger.Info("User connected", map[string]interface{}{
        "connection_id": connectionID,
        "user_id":       userID,
    })
    
    return ctx.JSON(map[string]interface{}{
        "message": "Connected successfully",
        "user_id": userID,
    })
}

func handleMessage(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    connectionID := wsCtx.ConnectionID()
    
    // Get user info
    userInfo, err := getConnectionInfo(connectionID)
    if err != nil {
        return lift.Unauthorized("Connection not found")
    }
    
    // Parse message
    var msg struct {
        Content string `json:"content" validate:"required,max=1000"`
        To      string `json:"to,omitempty"` // Optional: for direct messages
    }
    if err := ctx.ParseAndValidate(&msg); err != nil {
        return err
    }
    
    message := Message{
        ID:        generateID(),
        UserID:    userInfo.UserID,
        Username:  userInfo.Username,
        Content:   msg.Content,
        Timestamp: time.Now(),
        Type:      "message",
    }
    
    // Store message
    if err := storeMessage(message); err != nil {
        return lift.InternalError("Failed to store message")
    }
    
    // Send message
    if msg.To != "" {
        // Direct message
        targetConn, err := getConnectionByUserID(msg.To)
        if err != nil {
            return lift.NotFound("User not connected")
        }
        
        message.Type = "direct_message"
        return wsCtx.SendMessage(targetConn, message)
    } else {
        // Broadcast to all
        connections, _ := getActiveConnections()
        return wsCtx.BroadcastMessage(connections, message)
    }
}

func handleDisconnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    connectionID := wsCtx.ConnectionID()
    
    // Get user info before removing
    userInfo, _ := getConnectionInfo(connectionID)
    
    // Remove connection
    removeConnection(connectionID)
    
    // Notify others
    if userInfo != nil {
        connections, _ := getActiveConnections()
        wsCtx.BroadcastMessage(connections, Message{
            Type:      "user_left",
            UserID:    userInfo.UserID,
            Username:  userInfo.Username,
            Timestamp: time.Now(),
        })
    }
    
    ctx.Logger.Info("User disconnected", map[string]interface{}{
        "connection_id": connectionID,
        "user_id":       userInfo.UserID,
    })
    
    return nil
}
```

## Advanced Examples

### Multi-Tenant SaaS Application

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/dynamorm"
)

func main() {
    app := lift.New()
    
    // Global middleware
    app.Use(middleware.Logger())
    app.Use(middleware.Recover())
    app.Use(middleware.RequestID())
    app.Use(middleware.SecurityHeaders())
    
    // Tenant extraction middleware
    app.Use(TenantMiddleware())
    
    // Public routes
    app.POST("/auth/login", login)
    app.POST("/tenants", createTenant)
    
    // Tenant-scoped routes
    api := app.Group("/api", middleware.JWT(jwtConfig))
    
    // Users
    api.GET("/users", listTenantUsers)
    api.POST("/users", createTenantUser)
    api.GET("/users/:id", getTenantUser)
    
    // Resources
    api.GET("/resources", listTenantResources)
    api.POST("/resources", createTenantResource)
    
    // Admin routes
    admin := api.Group("/admin", RequireRole("admin"))
    admin.GET("/settings", getTenantSettings)
    admin.PUT("/settings", updateTenantSettings)
    admin.GET("/billing", getTenantBilling)
    
    lambda.Start(app.HandleRequest)
}

func TenantMiddleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Extract tenant from subdomain
            host := ctx.Header("Host")
            tenantID := extractTenantFromHost(host)
            
            // Or from JWT claims
            if tenantID == "" {
                if claims, ok := ctx.Get("claims").(jwt.MapClaims); ok {
                    tenantID = claims["tenant_id"].(string)
                }
            }
            
            // Or from header
            if tenantID == "" {
                tenantID = ctx.Header("X-Tenant-ID")
            }
            
            if tenantID == "" && !isPublicRoute(ctx.Request.Path) {
                return lift.BadRequest("Tenant identification required")
            }
            
            ctx.SetTenantID(tenantID)
            
            // Add tenant context to logger
            ctx.Logger = ctx.Logger.With(map[string]interface{}{
                "tenant_id": tenantID,
            })
            
            return next.Handle(ctx)
        })
    }
}

func listTenantUsers(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    
    // DynamORM automatically scopes queries by tenant
    db := dynamorm.FromContext(ctx)
    
    var users []User
    err := db.Query(&users).
        Where("tenant_id = ?", tenantID).
        Where("active = ?", true).
        Execute()
    
    if err != nil {
        return lift.InternalError("Failed to fetch users")
    }
    
    // Check permissions
    if !ctx.HasPermission("users:list") {
        // Filter to only show limited fields
        users = filterUsersForPermission(users, ctx.Permissions())
    }
    
    return ctx.JSON(users)
}

func createTenantResource(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    
    // Check tenant limits
    plan, err := getTenantPlan(tenantID)
    if err != nil {
        return lift.InternalError("Failed to get tenant plan")
    }
    
    count, err := getResourceCount(tenantID)
    if err != nil {
        return lift.InternalError("Failed to get resource count")
    }
    
    if count >= plan.ResourceLimit {
        return lift.PaymentRequired("Resource limit exceeded. Please upgrade your plan.")
    }
    
    // Create resource
    var req CreateResourceRequest
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err
    }
    
    resource := &Resource{
        ID:        generateID(),
        TenantID:  tenantID,
        Name:      req.Name,
        CreatedBy: ctx.UserID(),
        CreatedAt: time.Now(),
    }
    
    db := dynamorm.FromContext(ctx)
    if err := db.Save(resource); err != nil {
        return lift.InternalError("Failed to create resource")
    }
    
    // Update metrics
    ctx.Metrics.Count("resources.created", 1, map[string]string{
        "tenant_id": tenantID,
        "plan":      plan.Name,
    })
    
    return ctx.Status(201).JSON(resource)
}
```

### Microservices Communication

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

// Service mesh pattern with circuit breaker
type ServiceClient struct {
    name           string
    baseURL        string
    circuitBreaker *CircuitBreaker
    client         *http.Client
}

func main() {
    app := lift.New()
    
    // Service discovery
    services := &ServiceRegistry{
        UserService:  NewServiceClient("user-service", os.Getenv("USER_SERVICE_URL")),
        OrderService: NewServiceClient("order-service", os.Getenv("ORDER_SERVICE_URL")),
        PaymentService: NewServiceClient("payment-service", os.Getenv("PAYMENT_SERVICE_URL")),
    }
    
    // Inject services
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            ctx.Set("services", services)
            return next.Handle(ctx)
        })
    })
    
    // API Gateway routes
    app.POST("/checkout", handleCheckout)
    app.GET("/orders/:id", getOrderWithDetails)
    
    lambda.Start(app.HandleRequest)
}

func handleCheckout(ctx *lift.Context) error {
    services := ctx.Get("services").(*ServiceRegistry)
    
    var req CheckoutRequest
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err
    }
    
    // Start distributed trace
    segment := ctx.StartSegment("checkout")
    defer segment.End()
    
    // 1. Validate user
    user, err := services.UserService.GetUser(ctx, req.UserID)
    if err != nil {
        return lift.BadRequest("Invalid user")
    }
    
    // 2. Create order
    order, err := services.OrderService.CreateOrder(ctx, CreateOrderRequest{
        UserID: user.ID,
        Items:  req.Items,
    })
    if err != nil {
        return lift.InternalError("Failed to create order")
    }
    
    // 3. Process payment
    payment, err := services.PaymentService.ProcessPayment(ctx, ProcessPaymentRequest{
        OrderID: order.ID,
        Amount:  order.Total,
        Method:  req.PaymentMethod,
    })
    if err != nil {
        // Compensate: cancel order
        services.OrderService.CancelOrder(ctx, order.ID)
        return lift.BadRequest("Payment failed: " + err.Error())
    }
    
    // 4. Confirm order
    order, err = services.OrderService.ConfirmOrder(ctx, order.ID, payment.ID)
    if err != nil {
        // Compensate: refund payment
        services.PaymentService.RefundPayment(ctx, payment.ID)
        return lift.InternalError("Failed to confirm order")
    }
    
    // Record metrics
    ctx.Metrics.Count("checkout.completed", 1, map[string]string{
        "payment_method": req.PaymentMethod,
    })
    ctx.Metrics.Record("checkout.value", map[string]interface{}{
        "amount": order.Total,
        "items":  len(order.Items),
    })
    
    return ctx.JSON(CheckoutResponse{
        OrderID:   order.ID,
        PaymentID: payment.ID,
        Status:    "completed",
    })
}

func (s *ServiceClient) GetUser(ctx *lift.Context, userID string) (*User, error) {
    // Circuit breaker protection
    var result *User
    err := s.circuitBreaker.Call(func() error {
        // Build request with trace propagation
        req, _ := http.NewRequest("GET", s.baseURL+"/users/"+userID, nil)
        req.Header.Set("X-Amzn-Trace-Id", ctx.TraceHeader())
        req.Header.Set("X-Request-ID", ctx.RequestID())
        
        resp, err := s.client.Do(req)
        if err != nil {
            return err
        }
        defer resp.Body.Close()
        
        if resp.StatusCode != 200 {
            return fmt.Errorf("service returned %d", resp.StatusCode)
        }
        
        return json.NewDecoder(resp.Body).Decode(&result)
    })
    
    return result, err
}
```

## Performance Optimization Examples

### Caching Strategy

```go
package main

import (
    "sync"
    "time"
    "github.com/pay-theory/lift/pkg/lift"
)

// Multi-level cache
type CacheManager struct {
    memory *MemoryCache
    redis  *RedisCache
}

func main() {
    app := lift.New()
    
    // Initialize cache
    cache := &CacheManager{
        memory: NewMemoryCache(5 * time.Minute),
        redis:  NewRedisCache(os.Getenv("REDIS_URL")),
    }
    
    // Cache middleware
    app.Use(CacheMiddleware(cache))
    
    // Cached endpoints
    app.GET("/products", getProducts)
    app.GET("/products/:id", getProduct)
    
    lambda.Start(app.HandleRequest)
}

func CacheMiddleware(cache *CacheManager) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Only cache GET requests
            if ctx.Request.Method != "GET" {
                return next.Handle(ctx)
            }
            
            // Generate cache key
            key := fmt.Sprintf("%s:%s:%s", ctx.TenantID(), ctx.Request.Method, ctx.Request.Path)
            
            // Check memory cache first
            if cached, ok := cache.memory.Get(key); ok {
                ctx.Header("X-Cache", "HIT-MEMORY")
                return ctx.JSON(cached)
            }
            
            // Check Redis cache
            if cached, ok := cache.redis.Get(ctx, key); ok {
                // Store in memory for next request
                cache.memory.Set(key, cached)
                ctx.Header("X-Cache", "HIT-REDIS")
                return ctx.JSON(cached)
            }
            
            // Execute handler
            ctx.Header("X-Cache", "MISS")
            err := next.Handle(ctx)
            if err != nil {
                return err
            }
            
            // Cache successful responses
            if ctx.Response.StatusCode == 200 && ctx.Response.Body != nil {
                cache.memory.Set(key, ctx.Response.Body)
                cache.redis.Set(ctx, key, ctx.Response.Body)
            }
            
            return nil
        })
    }
}

func getProducts(ctx *lift.Context) error {
    // This will be cached
    products, err := loadProducts(ctx.TenantID())
    if err != nil {
        return lift.InternalError("Failed to load products")
    }
    
    return ctx.JSON(products)
}
```

### Batch Processing

```go
package main

import (
    "sync"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    app := lift.New()
    
    // Batch processing endpoint
    app.POST("/batch/users/update", batchUpdateUsers)
    
    // SQS batch processor
    app.Handle("SQS", "/batch-processor", processBatchQueue)
    
    lambda.Start(app.HandleRequest)
}

func batchUpdateUsers(ctx *lift.Context) error {
    var req BatchUpdateRequest
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err
    }
    
    // Process in parallel with worker pool
    workers := 10
    jobs := make(chan UpdateJob, len(req.Updates))
    results := make(chan UpdateResult, len(req.Updates))
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go updateWorker(ctx, &wg, jobs, results)
    }
    
    // Queue jobs
    for _, update := range req.Updates {
        jobs <- update
    }
    close(jobs)
    
    // Wait for completion
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Collect results
    var response BatchUpdateResponse
    for result := range results {
        if result.Error != nil {
            response.Failed = append(response.Failed, FailedUpdate{
                UserID: result.UserID,
                Error:  result.Error.Error(),
            })
        } else {
            response.Succeeded = append(response.Succeeded, result.UserID)
        }
    }
    
    // Return results
    statusCode := 200
    if len(response.Failed) > 0 {
        statusCode = 207 // Multi-status
    }
    
    return ctx.Status(statusCode).JSON(response)
}

func updateWorker(ctx *lift.Context, wg *sync.WaitGroup, jobs <-chan UpdateJob, results chan<- UpdateResult) {
    defer wg.Done()
    
    for job := range jobs {
        result := UpdateResult{UserID: job.UserID}
        
        // Process update
        err := updateUser(ctx, job)
        if err != nil {
            result.Error = err
        }
        
        results <- result
    }
}
```

## Testing Examples

### Unit Testing Handlers

```go
package main

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/testing"
)

func TestCreateUser(t *testing.T) {
    // Create test context
    ctx := testing.NewContext()
    ctx.Request.Method = "POST"
    ctx.Request.Body = []byte(`{
        "name": "John Doe",
        "email": "john@example.com",
        "age": 25
    }`)
    
    // Mock service
    mockService := &MockUserService{
        CreateFunc: func(user *User) error {
            assert.Equal(t, "John Doe", user.Name)
            assert.Equal(t, "john@example.com", user.Email)
            return nil
        },
    }
    
    // Create handler with mock
    handler := NewUserHandler(mockService)
    
    // Execute
    err := handler.CreateUser(ctx)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 201, ctx.Response.StatusCode)
    
    var response User
    err = testing.ParseResponseJSON(ctx, &response)
    assert.NoError(t, err)
    assert.NotEmpty(t, response.ID)
}

func TestCreateUser_ValidationError(t *testing.T) {
    ctx := testing.NewContext()
    ctx.Request.Method = "POST"
    ctx.Request.Body = []byte(`{
        "name": "Jo",
        "email": "invalid-email"
    }`)
    
    handler := NewUserHandler(nil)
    err := handler.CreateUser(ctx)
    
    assert.Error(t, err)
    httpErr, ok := err.(lift.HTTPError)
    assert.True(t, ok)
    assert.Equal(t, 400, httpErr.Status())
}
```

### Integration Testing

```go
package main

import (
    "testing"
    "github.com/pay-theory/lift/pkg/lift"
)

func TestUserAPI_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Set up test app
    app := createTestApp()
    
    // Test user creation
    t.Run("create user", func(t *testing.T) {
        resp := app.TestRequest("POST", "/users", map[string]interface{}{
            "name":  "Test User",
            "email": "test@example.com",
            "age":   30,
        })
        
        assert.Equal(t, 201, resp.StatusCode)
        
        var user User
        json.Unmarshal(resp.Body, &user)
        assert.NotEmpty(t, user.ID)
        
        // Store for next tests
        t.Setenv("TEST_USER_ID", user.ID)
    })
    
    // Test user retrieval
    t.Run("get user", func(t *testing.T) {
        userID := os.Getenv("TEST_USER_ID")
        resp := app.TestRequest("GET", "/users/"+userID, nil)
        
        assert.Equal(t, 200, resp.StatusCode)
        
        var user User
        json.Unmarshal(resp.Body, &user)
        assert.Equal(t, userID, user.ID)
        assert.Equal(t, "test@example.com", user.Email)
    })
}
```

## Summary

These examples demonstrate:

- **Basic APIs**: CRUD operations and routing
- **Authentication**: JWT and API key patterns
- **Event Processing**: SQS, S3, EventBridge handlers
- **WebSocket**: Real-time communication
- **Advanced Patterns**: Multi-tenancy, microservices
- **Performance**: Caching and batch processing
- **Testing**: Unit and integration testing

Use these as starting points for your own Lift applications! 