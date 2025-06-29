# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Lift is a type-safe, Lambda-native serverless framework for Go that eliminates boilerplate while providing production-grade features. Built by Pay Theory for serverless applications with a focus on security, performance, and developer experience.

## Key Architecture

### Core Components
- **App**: Central orchestrator (`pkg/lift/app.go`)
- **Context**: Enhanced request/response hub with multi-tenant support (`pkg/lift/context.go`)
- **Router**: Path-based routing with middleware chains (`pkg/lift/router.go`)
- **Handlers**: Type-safe generic handlers (`pkg/lift/handlers.go`)
- **Adapters**: Event source adapters for various AWS services (`pkg/adapters/`)

### Design Principles
- Type safety first with Go generics (requires Go 1.21+)
- Zero configuration with sensible defaults
- Performance optimized (<15ms cold start overhead)
- Multi-tenant ready with built-in tenant isolation
- Production-grade with observability and security built-in

## Common Commands

### Testing
```bash
# Run all tests
go test ./pkg/... -v

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run with race detection
go test ./... -race -cover

# Run benchmarks
./benchmarks/run_benchmarks.sh

# Test a specific package
go test ./pkg/lift -v

# Run a specific test
go test ./pkg/lift -v -run TestAppRoutes
```

### Building
```bash
# Build for Lambda
GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
zip function.zip bootstrap

# Local development
go run main.go
```

### Linting and Type Checking
**Note**: This project uses standard Go tooling. There are no specific lint or typecheck commands configured. When implementing features, ensure code compiles cleanly with `go build ./...`.

## Handler Patterns

### Basic Handler
```go
app.GET("/health", func(ctx *lift.Context) error {
    return ctx.JSON(map[string]string{"status": "healthy"})
})
```

### Type-Safe Handler
```go
type CreateUserRequest struct {
    Name string `json:"name" validate:"required"`
    Age  int    `json:"age" validate:"min=0,max=120"`
}

type UserResponse struct {
    UserID   string `json:"user_id"`
    TenantID string `json:"tenant_id,omitempty"`
}

app.POST("/users", lift.SimpleHandler(func(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
    // Automatic parsing and validation
    return UserResponse{
        UserID:   "user_123",
        TenantID: ctx.TenantID(),
    }, nil
}))
```

### Path Parameters
```go
app.GET("/users/:id", func(ctx *lift.Context) error {
    userID := ctx.Param("id")
    return ctx.JSON(map[string]string{"user_id": userID})
})
```

## Testing Patterns

### Creating Test Context
```go
import "github.com/pay-theory/lift/pkg/testing"

// Create test context with request
ctx := testing.NewTestContext(testing.WithRequest(testing.Request{
    Method: "GET",
    Path:   "/users/123",
    Headers: map[string]string{
        "Authorization": "Bearer token",
    },
}))
```

### Testing with TestApp
```go
app := testing.NewTestApp()
app.GET("/users", handler)

// Execute request
ctx := testing.NewTestContext(testing.WithRequest(testing.Request{
    Method: "GET",
    Path:   "/users",
}))

err := app.HandleTestRequest(ctx)
```

## Event Sources

Lift supports multiple AWS event sources through adapters:
- API Gateway (v1 and v2)
- SQS
- S3
- EventBridge
- DynamoDB Streams
- WebSocket

## Error Handling

Use structured errors for consistent error responses:
```go
return lift.NewError(http.StatusBadRequest, "Invalid request", map[string]interface{}{
    "field": "email",
    "error": "invalid format",
})
```

## Key Context Methods

- `ctx.Param(key)` - Get path parameter
- `ctx.Query(key)` - Get query parameter
- `ctx.Header(key)` - Get header value
- `ctx.ParseRequest(&req)` - Parse and validate request body
- `ctx.JSON(data)` - Send JSON response
- `ctx.UserID()` / `ctx.TenantID()` - Multi-tenant helpers
- `ctx.Set(key, value)` / `ctx.Get(key)` - Context state

## Important Files

- **Documentation**: `docs/` directory contains comprehensive guides
- **Examples**: `examples/` directory has 27+ working implementations including:
  - Basic patterns: `hello-world/`, `basic-crud-api/`, `error-handling/`
  - Authentication: `jwt-auth/`, `jwt-auth-demo/`, `rate-limiting/`
  - Event handling: `event-adapters/`, `multi-event-handler/`, `eventbridge-wakeup/`
  - Enterprise apps: `multi-tenant-saas/`, `enterprise-banking/`, `enterprise-healthcare/`
  - Production patterns: `production-api/`, `observability-demo/`, `health-monitoring/`
  - WebSocket support: `websocket-demo/`, `websocket-enhanced/`
- **Tests**: Look for `*_test.go` files for usage patterns
- **AI Guide**: `docs/ai-guide/lift-ai-assistant-guide.md` has detailed framework documentation

## Development Workflow

1. **Before implementing**: Search existing code for similar patterns
2. **Follow conventions**: Match existing code style and patterns
3. **Test thoroughly**: Write tests following existing test patterns
4. **Use type safety**: Leverage Go generics for compile-time safety
5. **Check imports**: Verify libraries are already used in the project

## Performance Considerations

- Router uses efficient pattern matching
- Middleware chains are optimized for minimal overhead
- Context pooling reduces allocations
- Event adapters use lazy parsing

## Security Notes

- JWT validation middleware available
- Built-in CORS support
- Rate limiting middleware
- Tenant isolation built into context
- Never log sensitive data (tokens, passwords)

## Common Patterns

### Middleware
```go
app.Use(middleware.Logger())
app.Use(middleware.Recover())
app.Use(middleware.CORS())
```

### Route Groups
```go
api := app.Group("/api")
api.Use(middleware.Auth())
api.GET("/users", listUsers)
```

### Multi-tenant Context
```go
userID := ctx.UserID()
tenantID := ctx.TenantID()
accountID := ctx.AccountID()
```