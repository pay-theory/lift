# Basic CRUD API Example

This example demonstrates how to build a production-ready CRUD API using the Lift framework with DynamORM integration, multi-tenant support, and comprehensive testing.

## Features

- ✅ Full CRUD operations (Create, Read, Update, Delete, List)
- ✅ Multi-tenant data isolation
- ✅ DynamORM integration with automatic transactions
- ✅ Type-safe request/response handling
- ✅ Structured error responses
- ✅ Request logging and metrics
- ✅ Comprehensive test suite

## API Endpoints

### Health Check
```
GET /health
```

### User Management

#### Create User
```
POST /users
Headers:
  X-Tenant-ID: <tenant-id>
  X-User-ID: <user-id>
Body:
{
  "email": "user@example.com",
  "name": "John Doe"
}
```

#### Get User
```
GET /users/:id
Headers:
  X-Tenant-ID: <tenant-id>
  X-User-ID: <user-id>
```

#### List Users
```
GET /users
Headers:
  X-Tenant-ID: <tenant-id>
  X-User-ID: <user-id>
```

#### Update User
```
PUT /users/:id
Headers:
  X-Tenant-ID: <tenant-id>
  X-User-ID: <user-id>
Body:
{
  "email": "newemail@example.com",  // optional
  "name": "New Name",                // optional
  "active": false                    // optional
}
```

#### Delete User
```
DELETE /users/:id
Headers:
  X-Tenant-ID: <tenant-id>
  X-User-ID: <user-id>
```

## Code Structure

### Main Application (`main.go`)

The main file demonstrates:

1. **App Setup**: Creating a Lift application with middleware
2. **DynamORM Integration**: Configuring DynamORM with tenant isolation
3. **Route Handlers**: Type-safe handlers with validation
4. **Error Handling**: Consistent error responses
5. **Middleware**: Authentication and logging

### Key Components

#### User Entity
```go
type User struct {
    ID        string    `json:"id" dynamodb:"id,hash"`
    TenantID  string    `json:"tenant_id" dynamodb:"tenant_id"`
    Email     string    `json:"email" validate:"required,email"`
    Name      string    `json:"name" validate:"required,min=1,max=100"`
    Active    bool      `json:"active"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

#### DynamORM Configuration
```go
dynamormConfig := &dynamorm.DynamORMConfig{
    TableName:       "lift_users",
    Region:          "us-east-1",
    AutoTransaction: true,        // Automatic transactions for writes
    TenantIsolation: true,       // Enable multi-tenant isolation
    TenantKey:       "tenant_id",
}
```

### Testing (`main_test.go`)

The test file demonstrates:

1. **TestApp Usage**: Using the Lift testing framework
2. **Mock Systems**: MockDynamORM for database testing
3. **Comprehensive Tests**: All CRUD operations tested
4. **Edge Cases**: Error handling, cross-tenant access
5. **Benchmarks**: Performance testing

#### Example Test
```go
func TestCreateUser(t *testing.T) {
    app := lifttesting.NewTestApp()
    setupRoutes(app.App())
    
    response := app.
        WithHeader("X-Tenant-ID", "tenant123").
        WithHeader("X-User-ID", "user123").
        POST("/users", CreateUserRequest{
            Email: "test@example.com",
            Name:  "Test User",
        })
    
    assert.Equal(t, 201, response.StatusCode)
    assert.Contains(t, response.Body, "test@example.com")
}
```

## Running the Example

### Prerequisites

1. Add testify to your go.mod:
```bash
go get github.com/stretchr/testify
```

2. Build the application:
```bash
go build -o crud-api main.go
```

3. Deploy to AWS Lambda (requires AWS setup)

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...
```

## Key Learnings

1. **Minimal Boilerplate**: ~300 lines for a complete CRUD API
2. **Type Safety**: Compile-time checking for requests/responses
3. **Testing First**: Tests are as easy to write as the code
4. **Multi-Tenant Ready**: Tenant isolation built-in
5. **Production Ready**: Error handling, logging, metrics included

## Extending the Example

### Add Custom Validation
```go
type CreateUserRequest struct {
    Email    string `json:"email" validate:"required,email,endswith=@paytheory.com"`
    Name     string `json:"name" validate:"required,min=1,max=100"`
    Password string `json:"password" validate:"required,min=8,containsany=!@#$%"`
}
```

### Add Custom Middleware
```go
func RateLimitMiddleware(limit int) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Rate limiting logic
            return next.Handle(ctx)
        })
    }
}
```

### Add DynamoDB Indexes
```go
type User struct {
    // ... existing fields ...
    OrgID string `json:"org_id" dynamodb:"org_id,gsi:org-index"`
}
```

## Performance

With the Lift framework:
- Cold starts: <15ms overhead
- Request processing: <5ms for simple operations
- DynamORM operations: <2ms overhead
- Memory usage: Minimal allocations 