# Getting Started with Lift

This guide will help you get up and running with the Lift framework in just a few minutes.

## Prerequisites

- Go 1.21 or higher (for generics support)
- AWS account with Lambda access
- Basic familiarity with AWS Lambda and Go

## Installation

Install Lift using Go modules:

```bash
go get github.com/pay-theory/lift
```

## Your First Lift Application

### 1. Create a New Project

```bash
mkdir my-lift-app
cd my-lift-app
go mod init my-lift-app
```

### 2. Install Dependencies

```bash
go get github.com/pay-theory/lift
go get github.com/aws/aws-lambda-go
```

### 3. Create Your Handler

Create a file named `main.go`:

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
)

func main() {
    // Create a new Lift application
    app := lift.New()
    
    // Add a simple route
    app.GET("/hello", handleHello)
    
    // Start the Lambda handler
    lambda.Start(app.HandleRequest)
}

func handleHello(ctx *lift.Context) error {
    return ctx.JSON(map[string]string{
        "message": "Hello from Lift!",
    })
}
```

### 4. Build for Lambda

```bash
GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
zip function.zip bootstrap
```

### 5. Deploy to AWS Lambda

Using AWS CLI:

```bash
aws lambda create-function \
    --function-name my-lift-function \
    --runtime provided.al2 \
    --role arn:aws:iam::YOUR_ACCOUNT:role/lambda-role \
    --handler bootstrap \
    --zip-file fileb://function.zip
```

## Adding Middleware

Lift comes with a rich set of middleware. Here's how to use them:

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

func main() {
    app := lift.New()
    
    // Add middleware (order matters!)
    app.Use(middleware.Logger())          // Request logging
    app.Use(middleware.Recover())         // Panic recovery
    app.Use(middleware.RequestID())       // Request ID generation
    app.Use(middleware.CORS(corsConfig))  // CORS handling
    
    // Add routes
    app.GET("/users", getUsers)
    app.POST("/users", createUser)
    
    lambda.Start(app.HandleRequest)
}
```

## Type-Safe Handlers

One of Lift's key features is type-safe request handling:

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18"`
}

type UserResponse struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func createUser(ctx *lift.Context) error {
    var req CreateUserRequest
    
    // Parse and validate in one step
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err // Automatic 400 Bad Request with validation errors
    }
    
    // Create user...
    user := UserResponse{
        ID:    generateID(),
        Name:  req.Name,
        Email: req.Email,
    }
    
    // Return JSON response
    return ctx.Status(201).JSON(user)
}

// Or use TypedHandler for even cleaner code
func createUserTyped(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Automatic parsing and validation!
    user := UserResponse{
        ID:    generateID(),
        Name:  req.Name,
        Email: req.Email,
    }
    
    return user, nil
}

// Register typed handler
app.POST("/users", lift.TypedHandler(createUserTyped))
```

## Working with Different Event Sources

Lift automatically detects and handles different Lambda event sources:

### API Gateway HTTP/REST

```go
app.GET("/hello", handleHello)
app.POST("/users", createUser)
app.PUT("/users/:id", updateUser)
app.DELETE("/users/:id", deleteUser)

func updateUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    // Update user...
    return ctx.JSON(updatedUser)
}
```

### SQS Queue Processing

```go
app.Handle("SQS", "/my-queue", handleSQSMessages)

func handleSQSMessages(ctx *lift.Context) error {
    // Access SQS records
    records := ctx.Request.Records
    
    for _, record := range records {
        // Process each message
        messageBody := record["body"].(string)
        // Process message...
    }
    
    return nil
}
```

### S3 Events

```go
app.Handle("S3", "/my-bucket", handleS3Event)

func handleS3Event(ctx *lift.Context) error {
    // Process S3 event
    for _, record := range ctx.Request.Records {
        bucket := record["s3"].(map[string]interface{})["bucket"].(map[string]interface{})["name"].(string)
        key := record["s3"].(map[string]interface{})["object"].(map[string]interface{})["key"].(string)
        
        // Process file...
    }
    
    return nil
}
```

### WebSocket Support

```go
// WebSocket routes use special methods
app.Handle("CONNECT", "/connect", handleConnect)
app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
app.Handle("MESSAGE", "/message", handleMessage)

func handleConnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    connectionID := wsCtx.ConnectionID()
    
    // Store connection...
    
    return ctx.JSON(map[string]string{
        "message": "Connected!",
    })
}
```

## Multi-Tenant Support

Lift has built-in multi-tenant support:

```go
func handleTenantRequest(ctx *lift.Context) error {
    // Access tenant information
    tenantID := ctx.TenantID()
    userID := ctx.UserID()
    
    // Use tenant-scoped database
    db := dynamorm.FromContext(ctx).ForTenant(tenantID)
    
    // Query only returns data for this tenant
    var users []User
    err := db.Query(&users).Execute()
    
    return ctx.JSON(users)
}
```

## Error Handling

Lift provides structured error handling:

```go
func handleRequest(ctx *lift.Context) error {
    // Return Lift errors for automatic status codes
    if !authorized {
        return lift.Unauthorized("Invalid credentials")
    }
    
    user, err := getUser(id)
    if err != nil {
        if err == ErrNotFound {
            return lift.NotFound("User not found")
        }
        // Generic errors become 500 Internal Server Error
        return err
    }
    
    return ctx.JSON(user)
}
```

## Configuration

Configure your Lift application:

```go
config := lift.Config{
    AppName:        "my-service",
    Environment:    "production",
    LogLevel:       "info",
    EnableMetrics:  true,
    EnableTracing:  true,
    DefaultTimeout: 29 * time.Second, // Lambda timeout
}

app := lift.NewWithConfig(config)
```

## Environment Variables

Lift uses these environment variables:

- `LIFT_ENV` - Environment (development, staging, production)
- `LIFT_LOG_LEVEL` - Log level (debug, info, warn, error)
- `LIFT_METRICS_ENABLED` - Enable CloudWatch metrics
- `LIFT_TRACING_ENABLED` - Enable X-Ray tracing

## Next Steps

Now that you have a basic Lift application running:

1. Learn about [Core Concepts](./core-concepts.md)
2. Explore [Middleware](./middleware.md) options
3. Understand [Error Handling](./error-handling.md)
4. Set up [Testing](./testing.md)
5. Review [Production Guide](./production-guide.md)

## Common Patterns

### Health Checks

```go
app.GET("/health", func(ctx *lift.Context) error {
    return ctx.JSON(map[string]string{
        "status": "healthy",
        "version": version,
    })
})
```

### Authentication

```go
// Add JWT middleware
app.Use(middleware.JWT(middleware.JWTConfig{
    SecretKey: os.Getenv("JWT_SECRET"),
    // or PublicKey for RS256
}))

// Access authenticated user
func protectedHandler(ctx *lift.Context) error {
    userID := ctx.UserID()
    claims := ctx.Get("claims").(jwt.MapClaims)
    
    // Handle request...
}
```

### Rate Limiting

```go
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    WindowSize:  time.Minute,
    MaxRequests: 100,
    KeyFunc: func(ctx *lift.Context) string {
        return ctx.TenantID() // Rate limit per tenant
    },
}))
```

## Troubleshooting

### Handler Not Found

Make sure your routes match the incoming request path and method.

### Type Assertion Errors

When working with event metadata, use safe type assertions:

```go
if connectionID, ok := ctx.Request.Metadata["connectionId"].(string); ok {
    // Use connectionID
}
```

### Performance Issues

- Enable connection pooling for databases
- Use middleware selectively
- Profile with X-Ray tracing

## Example Projects

Check out complete examples in the `examples/` directory:

- `hello-world` - Minimal example
- `basic-crud-api` - CRUD operations with DynamoDB
- `jwt-auth` - JWT authentication
- `multi-service-demo` - Microservices architecture
- `websocket-demo` - WebSocket support

Happy coding with Lift! 🚀 