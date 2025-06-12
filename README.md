# Lift - Type-Safe Lambda Handler Framework for Go

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/doc/install)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-latest-green.svg)](docs/)
[![Go Report Card](https://goreportcard.com/badge/github.com/pay-theory/lift)](https://goreportcard.com/report/github.com/pay-theory/lift)

Lift is a **type-safe**, **Lambda-native** handler framework for Go that eliminates boilerplate and lets you focus on business logic. Write 80% less code while building more reliable serverless applications.

## ✨ Why Lift?

Writing Lambda handlers shouldn't be painful. Traditional Lambda development requires:
- Manual request parsing and validation
- Repetitive error handling
- Complex response formatting
- Boilerplate for every single handler

Lift solves these problems with a clean, type-safe API inspired by modern web frameworks.

## 🚀 Quick Start

```go
package main

import (
    "github.com/pay-theory/lift"
)

type CreateUserRequest struct {
    Email string `json:"email" validate:"required,email"`
    Name  string `json:"name" validate:"required"`
}

func main() {
    app := lift.New()
    
    app.POST("/users", CreateUser)
    app.GET("/users/:id", GetUser)
    
    app.Start()
}

func CreateUser(ctx *lift.Context, req CreateUserRequest) (*User, error) {
    // Request is automatically parsed and validated
    user := &User{
        ID:    lift.GenerateID("usr"),
        Email: req.Email,
        Name:  req.Name,
    }
    
    if err := ctx.DB.Create(user); err != nil {
        return nil, lift.Conflict("User already exists")
    }
    
    return user, nil
}

func GetUser(ctx *lift.Context) (*User, error) {
    userID := ctx.Param("id")
    
    user, err := ctx.DB.Find(userID)
    if err != nil {
        return nil, lift.NotFound("User not found")
    }
    
    return user, nil
}
```

That's it! Lift handles:
- ✅ Request parsing and validation
- ✅ Error responses with proper status codes
- ✅ JSON marshaling/unmarshaling
- ✅ Logging and metrics
- ✅ Path parameter extraction

## 📊 Before & After

### Before (Traditional Lambda) - 50+ lines
```go
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var input CreateUserRequest
    if err := json.Unmarshal([]byte(request.Body), &input); err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: 400,
            Body:       `{"error":"Invalid request"}`,
        }, nil
    }
    // ... 40+ more lines of boilerplate ...
}
```

### After (With Lift) - 10 lines
```go
func CreateUser(ctx *lift.Context, req CreateUserRequest) (*User, error) {
    user := &User{Email: req.Email, Name: req.Name}
    if err := ctx.DB.Create(user); err != nil {
        return nil, lift.Conflict("User already exists")
    }
    return user, nil
}
```

## 🎯 Key Features

### 🔒 Type-Safe Handlers
```go
// Lift ensures type safety at compile time
func ProcessPayment(ctx *lift.Context, req PaymentRequest) (*PaymentResponse, error) {
    // req is guaranteed to be valid and properly typed
}
```

### ⚡ Automatic Validation
```go
type PaymentRequest struct {
    Amount   float64 `json:"amount" validate:"required,min=0.01"`
    Currency string  `json:"currency" validate:"required,oneof=USD EUR GBP"`
}
// Validation happens automatically before your handler is called
```

### 🛡️ Built-in Middleware
```go
app.Use(lift.Logger())       // Structured logging
app.Use(lift.Recover())      // Panic recovery
app.Use(lift.CORS())         // CORS handling
app.Use(lift.RateLimit(100)) // Rate limiting
app.Use(lift.Cache())        // Response caching
```

### 🧪 Easy Testing
```go
func TestCreateUser(t *testing.T) {
    app := lift.NewTestApp()
    
    resp := app.POST("/users", CreateUserRequest{
        Email: "test@example.com",
        Name:  "Test User",
    })
    
    assert.Equal(t, 201, resp.StatusCode)
}
```

### 🔄 Multiple Trigger Types
```go
// API Gateway
app.POST("/users", CreateUser)

// SQS
app.SQS("user-queue", ProcessUserEvent)

// S3
app.S3("uploads", ProcessUpload)

// EventBridge
app.EventBridge("orders", ProcessOrder)

// Scheduled
app.Schedule("0 9 * * *", DailyReport)
```

## 📦 Installation

```bash
go get github.com/pay-theory/lift
```

## 📚 Documentation

- [**Getting Started**](docs/getting-started.md) - Set up your first Lift application
- [**API Reference**](docs/api-reference.md) - Complete API documentation
- [**Examples**](examples/) - Real-world examples and patterns
- [**Migration Guide**](docs/migration.md) - Migrate from raw Lambda handlers
- [**Best Practices**](docs/best-practices.md) - Production-ready patterns

## 🏗️ More Examples

### Middleware & Authentication
```go
// Global middleware
app.Use(lift.Logger())
app.Use(lift.Auth(authConfig))

// Route-specific middleware
app.POST("/admin/users", CreateAdminUser, lift.RequireRole("admin"))

// Custom middleware
func APIKeyAuth(validKeys map[string]bool) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return func(ctx *lift.Context) error {
            key := ctx.Header("X-API-Key")
            if !validKeys[key] {
                return lift.Unauthorized("Invalid API key")
            }
            return next(ctx)
        }
    }
}
```

### Error Handling
```go
func GetOrder(ctx *lift.Context) (*Order, error) {
    orderID := ctx.Param("id")
    
    order, err := findOrder(orderID)
    if err != nil {
        switch err {
        case ErrNotFound:
            return nil, lift.NotFound("Order not found")
        case ErrCancelled:
            return nil, lift.Gone("Order cancelled")
        default:
            return nil, err // 500 Internal Server Error
        }
    }
    
    if order.UserID != ctx.UserID {
        return nil, lift.Forbidden("Access denied")
    }
    
    return order, nil
}
```

### Context Utilities
```go
func ProcessOrder(ctx *lift.Context, req OrderRequest) error {
    // Logging
    ctx.Logger.Info("Processing order", "orderID", req.ID)
    
    // Metrics
    ctx.Metrics.Increment("orders.processed")
    
    // Distributed tracing
    span := ctx.StartSpan("process-payment")
    defer span.End()
    
    // Timeouts
    result, err := ctx.WithTimeout(5*time.Second, func() (any, error) {
        return processPayment(req)
    })
    
    return nil
}
```

## 🚀 Performance

Lift is optimized for Lambda cold starts and high throughput:

| Metric | Performance |
|--------|-------------|
| Cold Start Overhead | <15ms |
| Request Routing | <1ms |
| Memory Overhead | <5MB |
| Throughput | 50,000+ req/sec |

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## 🗺️ Roadmap

- [ ] GraphQL support
- [ ] WebSocket handlers
- [ ] OpenAPI generation
- [ ] More middleware options
- [ ] Plugin system

## 💬 Community

- **GitHub Discussions**: [Join the conversation](https://github.com/pay-theory/lift/discussions)
- **Discord**: [Chat with us](https://discord.gg/lift)
- **Twitter**: [@PayTheory](https://twitter.com/paytheory)

## 📄 License

Lift is licensed under the [Apache License 2.0](LICENSE).

---

<p align="center">
  Built with ❤️ by <a href="https://paytheory.com">Pay Theory</a>
</p> # lift
