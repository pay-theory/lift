# Lift Framework 🚀

A type-safe, Lambda-native framework for Go that eliminates boilerplate while providing production-grade features. Built by Pay Theory for serverless applications with a focus on security, performance, and developer experience.

## Quick Start

```go
package main

import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

type UserRequest struct {
    Name string `json:"name" validate:"required"`
    Age  int    `json:"age" validate:"min=0,max=120"`
}

type UserResponse struct {
    Message  string `json:"message"`
    UserID   string `json:"user_id"`
    TenantID string `json:"tenant_id,omitempty"`
}

func main() {
    app := lift.New()

    // Add middleware
    app.Use(middleware.RequestID())
    app.Use(middleware.Logger())
    app.Use(middleware.Recover())
    app.Use(middleware.ErrorHandler())

    // Health check
    app.GET("/health", func(ctx *lift.Context) error {
        return ctx.JSON(map[string]string{
            "status": "healthy",
            "service": "my-service",
        })
    })

    // Type-safe handler
    app.POST("/users", lift.SimpleHandler(func(ctx *lift.Context, req UserRequest) (UserResponse, error) {
        return UserResponse{
            Message:  fmt.Sprintf("User %s created", req.Name),
            UserID:   "user_123",
            TenantID: ctx.TenantID(),
        }, nil
    }))

    // Path parameters
    app.GET("/users/:id", func(ctx *lift.Context) error {
        userID := ctx.Param("id")
        return ctx.JSON(map[string]interface{}{
            "user_id": userID,
            "tenant":  ctx.TenantID(),
        })
    })

    // Start the application
    app.Start()

    // In Lambda, use: lambda.Start(app.HandleRequest)
}
```

## Foundation Status ✅

**Sprint 1-2 Foundation: COMPLETE**

We have successfully implemented the core foundation of the Lift framework:

### ✅ Core Components
- **Type-Safe Handlers**: Generic handlers with compile-time type checking
- **Enhanced Context**: Rich context with multi-tenant support
- **Routing Engine**: Path parameters, exact matching, middleware chains
- **Request/Response**: Unified structure supporting multiple Lambda triggers
- **Error Handling**: Structured errors with HTTP status codes
- **Middleware System**: Essential middleware (Logger, Recover, CORS, etc.)

### ✅ Key Features
- 🔒 **Type Safety**: Compile-time type checking with Go generics
- 🏗️ **Zero Boilerplate**: From 50+ lines to ~10 lines per handler
- 🚀 **Performance**: Designed for <15ms cold start overhead
- 🏢 **Multi-Tenant**: Built-in tenant/user context support
- 🔧 **Middleware**: Composable middleware system
- 📊 **Observability**: Request ID tracking, structured logging
- ✅ **Testing**: Comprehensive test coverage

### ✅ Testing Results
```
=== PASS: TestNew (0.00s)
=== PASS: TestAppRoutes (0.00s)
=== PASS: TestAppStart (0.00s)
=== PASS: TestAppWithConfig (0.00s)
=== PASS: TestDefaultConfig (0.00s)
=== PASS: TestAppHandleRequest (0.00s)
=== PASS: TestNewRouter (0.00s)
=== PASS: TestRouterAddRoute (0.00s)
=== PASS: TestExtractParams (0.00s)
=== PASS: TestMatchPattern (0.00s)
=== PASS: TestRouterFindHandler (0.00s)
=== PASS: TestRouterHandle (0.00s)
PASS
```

## Architecture

```
┌─────────────────────────────────────────┐
│              Lift Framework             │
├─────────────────────────────────────────┤
│  Type-Safe Handlers                     │
│  ┌─────────────┐ ┌─────────────────────┐│
│  │ SimpleHandler│ │ Generic Handlers   ││
│  │   Function   │ │ TypedHandler<T,R>  ││
│  └─────────────┘ └─────────────────────┘│
├─────────────────────────────────────────┤
│  Enhanced Context & Routing             │
│  ┌─────────────┐ ┌─────────────────────┐│
│  │   Context   │ │      Router         ││
│  │  Multi-Tenant│ │  Path Parameters   ││
│  └─────────────┘ └─────────────────────┘│
├─────────────────────────────────────────┤
│  Middleware & Error Handling            │
│  ┌─────────────┐ ┌─────────────────────┐│
│  │ Composable  │ │   Structured        ││
│  │ Middleware  │ │     Errors          ││
│  └─────────────┘ └─────────────────────┘│
└─────────────────────────────────────────┘
```

## Examples

Check the `examples/` directory for working examples:
- `examples/hello-world/` - Basic Lambda handler with type safety
- `examples/basic-crud-api/` - Complete CRUD API with middleware
- `examples/jwt-auth/` - JWT authentication example
- `examples/websocket-enhanced/` - WebSocket implementation
- `examples/observability-demo/` - Comprehensive observability setup

## Documentation

### 📚 Main Documentation
- [`docs/getting-started.md`](docs/getting-started.md) - Getting started guide
- [`docs/api-reference.md`](docs/api-reference.md) - Complete API reference
- [`docs/middleware.md`](docs/middleware.md) - Middleware configuration
- [`docs/security.md`](docs/security.md) - Security features and JWT
- [`docs/observability.md`](docs/observability.md) - Logging and monitoring
- [`docs/testing.md`](docs/testing.md) - Testing utilities and patterns

### 🏗️ Architecture & Planning
- [`docs/architecture/`](docs/architecture/) - Technical and security architecture
- [`docs/planning/`](docs/planning/) - Development plans and roadmaps
- [`docs/LESSONS_LEARNED.md`](docs/LESSONS_LEARNED.md) - Key insights and learnings

### 🗃️ Historical Documentation
- [`docs/archive/`](docs/archive/) - Archived sprint history and decisions
- [`docs/development/`](docs/development/) - Active development notes

## Roadmap

### 🚧 Next: Sprint 3-4 (Type Safety Enhancement)
- [ ] Enhanced validation with struct tags
- [ ] Event source adapters (SQS, S3, EventBridge)
- [ ] AWS integration utilities
- [ ] Advanced middleware (Auth, Rate Limiting)
- [ ] DynamORM integration

### 🔮 Future Sprints
- Multi-trigger support (SQS, S3, EventBridge)
- Advanced security features
- Performance optimizations
- Testing utilities
- CLI tooling

## Development

```bash
# Clone and setup
git clone https://github.com/pay-theory/lift
cd lift
go mod tidy

# Run tests
go test ./pkg/... -v

# Build example
cd examples/hello-world
go build .
```

## Contributing

This is a Pay Theory internal project. See our development documentation in `docs/` for architecture decisions and implementation notes.

## License

Internal Pay Theory License
