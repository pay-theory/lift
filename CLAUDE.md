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
- **CDK**: Infrastructure as code constructs for deploying Lift apps (`pkg/cdk/`)

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

### Rate Limiting
Lift provides two rate limiting approaches:

1. **Built-in middleware** (requires DynamORMWrapper setup - complex)
2. **Limited library integration** (recommended - simple and functional)

#### Using the Limited Library (Recommended)
```go
// Simple IP-based rate limiting
rateLimiter, err := middleware.LimitedRateLimit(middleware.LimitedConfig{
    Region:    "us-east-1",
    TableName: "rate-limits",
    Window:    time.Hour,
    Limit:     1000,
})
if err != nil {
    panic(err)
}

app.Use(rateLimiter)
```

#### Pre-built Rate Limiters
```go
// IP-based (1000 req/hour)
ipLimiter, _ := middleware.IPRateLimitWithLimited(1000, time.Hour)
app.Use(ipLimiter)

// User-based (100 req/15min)
userLimiter, _ := middleware.UserRateLimitWithLimited(100, 15*time.Minute)
api.Use(userLimiter)

// Tenant-based (500 req/hour)
tenantLimiter, _ := middleware.TenantRateLimitWithLimited(500, time.Hour)
api.Use(tenantLimiter)
```

### Building
```bash
# Build for Lambda
GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
zip function.zip bootstrap

# Build for Lambda ARM64 (recommended)
GOOS=linux GOARCH=arm64 go build -o bootstrap main.go
zip function.zip bootstrap

# Local development
go run main.go
```

### CDK Deployment
```bash
# Synthesize CDK stack
make cdk-synth

# Deploy with CDK
make cdk-deploy

# Show deployment changes
make cdk-diff

# Destroy CDK stack
make cdk-destroy
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

## DynamoDB Initialization

### Basic Initialization
```go
import (
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/dynamorm/pkg/core"
    "github.com/pay-theory/dynamorm/pkg/session"
)

// Initialize DynamoDB connection
var db core.DB
db, err := dynamorm.NewBasic(session.Config{
    Region: "us-east-1",
    // For local testing:
    // Endpoint: "http://localhost:8000",
})
if err != nil {
    panic(fmt.Sprintf("Failed to initialize DynamoDB: %v", err))
}
```

### Extended Functionality
```go
// If you need extended features, use dynamorm.New()
var db core.ExtendedDB
db, err := dynamorm.New(session.Config{
    Region: "us-east-1",
})
if err != nil {
    panic(fmt.Sprintf("Failed to initialize DynamoDB: %v", err))
}
```

### Usage with Limited Rate Limiter
```go
// The initialized db can be used with rate limiters
rateLimiter := limited.NewDynamoRateLimiter(
    db,  // Pass the core.DB
    nil, // Use default config
    strategy,
    logger,
)
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

## CDK Patterns

Lift provides AWS CDK constructs for deploying applications with infrastructure as code.

### Basic CDK App
```go
import "github.com/pay-theory/lift/pkg/cdk/patterns"

app := awscdk.NewApp(nil)

patterns.NewLiftApp(app, jsii.String("MyApp"), &patterns.LiftAppProps{
    AppName:           jsii.String("my-app"),
    CodeAssetPath:     jsii.String("./dist"),
    EnableMultiTenant: jsii.Bool(true),
    EnableDatabase:    jsii.Bool(true),
})

app.Synth(nil)
```

### DynamORM Integration

All Lift tables now use a standardized structure compatible with DynamORM:
- Primary key: `pk` (partition key)
- Sort key: `sk` (sort key)
- GSIs must be defined in CDK during table creation
- Composite keys for entity identification and multi-tenancy

#### 1. Define GSIs in CDK (Infrastructure)
```go
// Create the table with LiftTable
liftTable := constructs.NewLiftTable(stack, jsii.String("UserTable"), &constructs.LiftTableProps{
    TableName:        jsii.String("my-app-users"),
    PartitionKeyName: jsii.String("PK"),
    SortKeyName:      jsii.String("SK"),
})

// Add GSIs to the underlying DynamoDB table
liftTable.Table.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
    IndexName: jsii.String("email-index"),
    PartitionKey: &awsdynamodb.Attribute{
        Name: jsii.String("Email"),
        Type: awsdynamodb.AttributeType_STRING,
    },
})

liftTable.Table.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
    IndexName: jsii.String("tenant-index"),
    PartitionKey: &awsdynamodb.Attribute{
        Name: jsii.String("TenantID"),
        Type: awsdynamodb.AttributeType_STRING,
    },
    SortKey: &awsdynamodb.Attribute{
        Name: jsii.String("Created"),
        Type: awsdynamodb.AttributeType_STRING,
    },
})
```

#### 2. Match GSIs in your Go models
```go
type User struct {
    // Keys - only dynamorm tags needed
    PK string `dynamorm:"pk" `  // tenant#{tenant_id} or user#{user_id}
    SK string `dynamorm:"sk" `  // user#{user_id} or hierarchical data
    
    // GSI attributes - must match CDK definition
    Email    string `dynamorm:"index:email-index,pk" `
    TenantID string `dynamorm:"index:tenant-index,pk" `
    Created  string `dynamorm:"index:tenant-index,sk" `
    
    // Business fields
    UserID   string    `json:"user_id" `
    Name     string    `json:"name" `
    Status   string    `json:"status" `
    TTL      int64     `json:"ttl,omitempty" dynamorm:"ttl"`
}
```

#### 3. Query using GSIs
```go
// Query by email (using email-index GSI)
users, err := dynamorm.Query[User](ctx, db).
    WithTable(tableName).
    WithIndex("email-index").
    WithPK("user@example.com").
    Execute()

// Query by tenant with date range (using tenant-index GSI)
users, err := dynamorm.Query[User](ctx, db).
    WithTable(tableName).
    WithIndex("tenant-index").
    WithPK("tenant-123").
    WithSKBetween("2024-01-01", "2024-12-31").
    Execute()
```

**Important**: 
- GSIs must be defined in CDK during table creation
- DynamORM cannot add GSIs to existing tables at runtime (this would be a slow, expensive DynamoDB operation)
- The struct tags in your models tell DynamORM how to use the GSIs for queries, but don't create them
- GSI attribute names in CDK must match the Go struct field names exactly

For detailed DynamORM integration patterns, see `docs/dynamorm-integration.md`.

### CDK Constructs
- **LiftFunction**: Optimized Lambda with ARM64, tracing, multi-tenant support
- **LiftAPI**: API Gateway with CORS, custom domains, rate limiting
- **LiftTable**: DynamoDB with single-table design, GSI, auto-scaling
- **LiftApp**: Complete application pattern with all components

### Pre-built Stacks
```go
import "github.com/pay-theory/lift/pkg/cdk/stacks"

// Microservice stack
stacks.NewMicroserviceStack(app, "Service", &stacks.MicroserviceStackProps{
    ServiceName:    "user-service",
    CodePath:       "./dist/bootstrap",
    EnableDatabase: true,
})

// Multi-tenant SaaS stack
stacks.NewMultiTenantSaaSStack(app, "SaaS", &stacks.MultiTenantSaaSStackProps{
    AppName:           "my-saas",
    CodePath:          "./dist/bootstrap",
    EnableAuth:        true,
    EnableFileStorage: true,
})

// Event-driven stack
stacks.NewEventDrivenStack(app, "Events", &stacks.EventDrivenStackProps{
    AppName:                "order-system",
    ApiCodePath:            "./dist/api/bootstrap",
    EventProcessorCodePath: "./dist/processor/bootstrap",
})
```