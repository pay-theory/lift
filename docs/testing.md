# Testing

Comprehensive testing is crucial for building reliable serverless applications. This guide covers unit testing, integration testing, and end-to-end testing strategies for Lift applications.

## Overview

Lift provides extensive testing support including:

- **Test Utilities**: Helper functions for creating test contexts and apps
- **Mock Support**: Built-in mocks for AWS services and DynamORM
- **Handler Testing**: Easy testing of individual handlers
- **Middleware Testing**: Test middleware in isolation
- **Integration Testing**: Test complete request flows with TestApp
- **Load Testing**: Performance and scalability testing

## Import Pattern (Avoiding Naming Conflicts)

```go
import (
    "testing"  // Go standard testing
    lifttesting "github.com/pay-theory/lift/pkg/testing"  // Lift testing utilities
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/lift/adapters"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

## Unit Testing

### Testing Handlers - Current Pattern

```go
package handlers_test

import (
    "context"
    "encoding/json"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/lift/adapters"
    lifttesting "github.com/pay-theory/lift/pkg/testing"
)

// Test context helper (copy this pattern from jwt_test.go)
func createTestContext(method, path string, body []byte) *lift.Context {
    adapterReq := &adapters.Request{
        Method:      method,
        Path:        path,
        Headers:     make(map[string]string),
        QueryParams: make(map[string]string),
        PathParams:  make(map[string]string),
        Body:        body,
    }
    req := lift.NewRequest(adapterReq)
    return lift.NewContext(context.Background(), req)
}

func TestGetUserHandler(t *testing.T) {
    // Create test context
    ctx := createTestContext("GET", "/users/123", nil)
    
    // Set path parameters
    ctx.SetParam("id", "123")
    
    // Mock dependencies via ctx.Set()
    mockUserService := &MockUserService{
        GetFunc: func(id string) (*User, error) {
            return &User{
                ID:    id,
                Name:  "Test User",
                Email: "test@example.com",
            }, nil
        },
    }
    ctx.Set("userService", mockUserService)
    
    // Create handler
    handler := NewUserHandler()
    
    // Execute handler
    err := handler.GetUser(ctx)
    
    // Assert results
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    // Check JSON response
    var response UserResponse
    err = json.Unmarshal(ctx.Response.Body.([]byte), &response)
    assert.NoError(t, err)
    assert.Equal(t, "123", response.ID)
    assert.Equal(t, "Test User", response.Name)
}

func TestGetUserHandler_NotFound(t *testing.T) {
    ctx := createTestContext("GET", "/users/999", nil)
    ctx.SetParam("id", "999")
    
    mockUserService := &MockUserService{
        GetFunc: func(id string) (*User, error) {
            return nil, ErrNotFound
        },
    }
    ctx.Set("userService", mockUserService)
    
    handler := NewUserHandler()
    err := handler.GetUser(ctx)
    
    // Should handle error internally and set status
    assert.NoError(t, err)
    assert.Equal(t, 404, ctx.Response.StatusCode)
}
```

### Testing with Dependencies and Mocks

```go
func TestUserHandlerWithDatabase(t *testing.T) {
    // Create mock database using Lift's built-in mocks
    mockDB := lifttesting.NewMockDynamORM()
    
    // Create test context
    ctx := createTestContext("GET", "/users/123", nil)
    ctx.SetParam("id", "123")
    
    // Inject dependencies (this is the key pattern!)
    ctx.Set("db", mockDB)
    ctx.Set("dynamorm", mockDB)
    
    // Set up mock expectations
    expectedUser := &User{ID: "123", Name: "Test User"}
    mockDB.On("Get", mock.Anything, "123").Return(expectedUser, nil)
    
    // Create handler
    handler := NewUserHandler()
    
    // Call handler
    err := handler.GetUser(ctx)
    
    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    // Verify mock was called
    mockDB.AssertExpectations(t)
}
```

### Testing TypedHandlers

```go
func TestCreateUserTypedHandler(t *testing.T) {
    // Test request
    req := CreateUserRequest{
        Name:  "John Doe",
        Email: "john@example.com",
        Age:   30,
    }
    
    // Create context with JSON body
    reqBody, _ := json.Marshal(req)
    ctx := createTestContext("POST", "/users", reqBody)
    ctx.Request.Headers["Content-Type"] = "application/json"
    
    // Mock service
    mockUserService := &MockUserService{
        CreateFunc: func(req CreateUserRequest) (*User, error) {
            return &User{
                ID:    "new-123",
                Name:  req.Name,
                Email: req.Email,
            }, nil
        },
    }
    ctx.Set("userService", mockUserService)
    
    // Create typed handler
    handler := func(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
        service := ctx.Get("userService").(*MockUserService)
        user, err := service.Create(req)
        if err != nil {
            return UserResponse{}, err
        }
        
        return UserResponse{
            ID:    user.ID,
            Name:  user.Name,
            Email: user.Email,
        }, nil
    }
    
    // Wrap with TypedHandler
    typedHandler := lift.TypedHandler(handler)
    err := typedHandler.Handle(ctx)
    
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    var resp UserResponse
    json.Unmarshal(ctx.Response.Body.([]byte), &resp)
    assert.Equal(t, "new-123", resp.ID)
}
```

### Testing with Table-Driven Tests

```go
func TestValidateUser(t *testing.T) {
    tests := []struct {
        name    string
        user    User
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid user",
            user: User{
                Name:  "John Doe",
                Email: "john@example.com",
                Age:   25,
            },
            wantErr: false,
        },
        {
            name: "missing name",
            user: User{
                Email: "john@example.com",
                Age:   25,
            },
            wantErr: true,
            errMsg:  "name is required",
        },
        {
            name: "invalid email",
            user: User{
                Name:  "John Doe",
                Email: "invalid-email",
                Age:   25,
            },
            wantErr: true,
            errMsg:  "invalid email format",
        },
        {
            name: "age too young",
            user: User{
                Name:  "John Doe",
                Email: "john@example.com",
                Age:   12,
            },
            wantErr: true,
            errMsg:  "must be at least 13 years old",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateUser(tt.user)
            
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Integration Testing with TestApp

### Testing Complete Request Flow

```go
func TestUserAPIIntegration(t *testing.T) {
    // Create test app using Lift's TestApp
    testApp := lifttesting.NewTestApp()
    app := testApp.App()
    
    // Configure your routes
    userHandler := NewUserHandler()
    app.GET("/users/:id", userHandler.GetUser)
    app.POST("/users", userHandler.CreateUser)
    
    // Start the test server
    err := testApp.Start()
    require.NoError(t, err)
    defer testApp.Stop()
    
    // Test user creation
    t.Run("create user", func(t *testing.T) {
        req := map[string]interface{}{
            "name":  "Jane Doe",
            "email": "jane@example.com",
        }
        
        resp := testApp.POST("/users", req)
        resp.AssertStatus(201).
            AssertJSONPath("$.name", "Jane Doe").
            AssertJSONPath("$.email", "jane@example.com").
            AssertHeaderExists("Content-Type")
        
        // Extract ID for next test
        userID := resp.GetJSONPath("$.id").(string)
        t.Setenv("TEST_USER_ID", userID)
    })
    
    // Test user retrieval
    t.Run("get user", func(t *testing.T) {
        userID := os.Getenv("TEST_USER_ID")
        
        resp := testApp.GET("/users/"+userID, nil)
        resp.AssertStatus(200).
            AssertJSONPath("$.id", userID).
            AssertJSONPath("$.name", "Jane Doe")
    })
}
```

### Testing with Authentication

```go
func TestProtectedEndpoints(t *testing.T) {
    testApp := lifttesting.NewTestApp()
    app := testApp.App()
    
    // Add JWT auth middleware
    app.Use(middleware.JWT(middleware.JWTConfig{
        SecretKey: []byte("test-secret"),
    }))
    
    // Protected route
    app.GET("/profile", func(ctx *lift.Context) error {
        userID := ctx.UserID()
        return ctx.JSON(map[string]string{"user_id": userID})
    })
    
    testApp.Start()
    defer testApp.Stop()
    
    // Test without auth - should fail
    t.Run("unauthenticated", func(t *testing.T) {
        resp := testApp.GET("/profile", nil)
        resp.AssertStatus(401)
    })
    
    // Test with auth - should succeed
    t.Run("authenticated", func(t *testing.T) {
        testApp.WithAuth(&lifttesting.AuthConfig{
            Token:    createValidJWT("user-123", "test-secret"),
            UserID:   "user-123",
            TenantID: "tenant-456",
        })
        
        resp := testApp.GET("/profile", nil)
        resp.AssertStatus(200).
            AssertJSONPath("$.user_id", "user-123")
    })
}
```

### Testing with Database Integration

```go
func TestDatabaseIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    testApp := lifttesting.NewTestApp()
    app := testApp.App()
    
    // Add DynamORM middleware
    app.Use(dynamorm.WithDynamORM(&dynamorm.DynamORMConfig{
        TableName: "test-users-" + uuid.New().String(),
        Region:    "us-east-1",
        Endpoint:  "http://localhost:8000", // Local DynamoDB
    }))
    
    // Add handlers
    app.POST("/users", handlers.CreateUser)
    app.GET("/users/:id", handlers.GetUser)
    
    testApp.Start()
    defer testApp.Stop()
    
    // Test create and retrieve
    t.Run("create and get user", func(t *testing.T) {
        // Create user
        createReq := map[string]interface{}{
            "name":  "Integration Test User",
            "email": "integration@example.com",
        }
        
        createResp := testApp.POST("/users", createReq)
        createResp.AssertStatus(201)
        
        userID := createResp.GetJSONPath("$.id").(string)
        
        // Retrieve user
        getResp := testApp.GET("/users/"+userID, nil)
        getResp.AssertStatus(200).
            AssertJSONPath("$.name", "Integration Test User")
    })
}
```

## Testing Middleware

### Basic Middleware Testing

```go
func TestAuthMiddleware(t *testing.T) {
    // Create middleware
    authMiddleware := middleware.JWT(middleware.JWTConfig{
        SecretKey: []byte("test-secret"),
    })
    
    // Create test handler
    testHandler := lift.HandlerFunc(func(ctx *lift.Context) error {
        // This should only be called if auth succeeds
        userID := ctx.UserID()
        return ctx.JSON(map[string]interface{}{
            "user_id": userID,
        })
    })
    
    // Wrap handler with middleware
    handler := authMiddleware(testHandler)
    
    t.Run("valid token", func(t *testing.T) {
        ctx := createTestContext("GET", "/protected", nil)
        
        // Create valid token
        token := createValidJWT("user-123", "test-secret")
        ctx.Request.Headers["Authorization"] = "Bearer " + token
        
        err := handler.Handle(ctx)
        assert.NoError(t, err)
        assert.Equal(t, 200, ctx.Response.StatusCode)
    })
    
    t.Run("missing token", func(t *testing.T) {
        ctx := createTestContext("GET", "/protected", nil)
        
        err := handler.Handle(ctx)
        assert.NoError(t, err) // Middleware handles error internally
        assert.Equal(t, 401, ctx.Response.StatusCode)
    })
    
    t.Run("invalid token", func(t *testing.T) {
        ctx := createTestContext("GET", "/protected", nil)
        ctx.Request.Headers["Authorization"] = "Bearer invalid-token"
        
        err := handler.Handle(ctx)
        assert.NoError(t, err) // Middleware handles error internally
        assert.Equal(t, 401, ctx.Response.StatusCode)
    })
}
```

### Testing Middleware Chain

```go
func TestMiddlewareChain(t *testing.T) {
    var executionOrder []string
    
    // Create test middleware
    createMiddleware := func(name string) lift.Middleware {
        return func(next lift.Handler) lift.Handler {
            return lift.HandlerFunc(func(ctx *lift.Context) error {
                executionOrder = append(executionOrder, name+"-before")
                err := next.Handle(ctx)
                executionOrder = append(executionOrder, name+"-after")
                return err
            })
        }
    }
    
    // Create handler
    handler := lift.HandlerFunc(func(ctx *lift.Context) error {
        executionOrder = append(executionOrder, "handler")
        return ctx.JSON(map[string]string{"status": "ok"})
    })
    
    // Build chain
    chain := handler
    chain = createMiddleware("third")(chain)
    chain = createMiddleware("second")(chain)
    chain = createMiddleware("first")(chain)
    
    // Execute
    ctx := createTestContext("GET", "/test", nil)
    err := chain.Handle(ctx)
    
    // Verify order
    assert.NoError(t, err)
    assert.Equal(t, []string{
        "first-before",
        "second-before",
        "third-before",
        "handler",
        "third-after",
        "second-after",
        "first-after",
    }, executionOrder)
}
```

## Mock Strategies

### Using Lift's Built-in Mocks

```go
func TestWithMockDynamORM(t *testing.T) {
    // Use Lift's built-in DynamORM mock
    mockDB := lifttesting.NewMockDynamORM()
    
    // Set up expectations
    expectedUser := &User{ID: "123", Name: "Test User"}
    mockDB.On("Get", mock.Anything, "123").Return(expectedUser, nil)
    mockDB.On("Save", mock.AnythingOfType("*User")).Return(nil)
    
    // Create context with mock
    ctx := createTestContext("GET", "/users/123", nil)
    ctx.Set("dynamorm", mockDB)
    
    // Test your handler
    handler := NewUserHandler()
    err := handler.GetUser(ctx)
    
    assert.NoError(t, err)
    mockDB.AssertExpectations(t)
}
```

### Interface-Based Mocks

```go
// Define interfaces for dependencies
type UserService interface {
    Get(id string) (*User, error)
    Create(user *User) error
    Update(user *User) error
    Delete(id string) error
}

// Mock implementation
type MockUserService struct {
    mock.Mock
}

func (m *MockUserService) Get(id string) (*User, error) {
    args := m.Called(id)
    return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserService) Create(user *User) error {
    args := m.Called(user)
    return args.Error(0)
}

// Use in tests
func TestHandlerWithMock(t *testing.T) {
    mockService := &MockUserService{}
    mockService.On("Get", "123").Return(&User{ID: "123", Name: "Test"}, nil)
    
    ctx := createTestContext("GET", "/users/123", nil)
    ctx.Set("userService", mockService)
    
    handler := NewUserHandler()
    err := handler.GetUser(ctx)
    
    assert.NoError(t, err)
    mockService.AssertExpectations(t)
}
```

### AWS Service Mocks

```go
func TestWithAWSMocks(t *testing.T) {
    // Use Lift's CloudWatch mock
    mockCloudWatch := lifttesting.NewMockCloudWatchClient()
    
    // Set up expectations
    mockCloudWatch.On("PutMetricData", 
        mock.Anything, 
        mock.AnythingOfType("*cloudwatch.PutMetricDataInput"),
        mock.Anything,
    ).Return(&cloudwatch.PutMetricDataOutput{}, nil)
    
    // Test your handler that uses CloudWatch
    ctx := createTestContext("POST", "/metrics", nil)
    ctx.Set("cloudwatch", mockCloudWatch)
    
    handler := NewMetricsHandler()
    err := handler.PublishMetrics(ctx)
    
    assert.NoError(t, err)
    mockCloudWatch.AssertExpectations(t)
}
```

## Test Utilities

### Context Helpers

```go
// Create context with common setup
func createAuthenticatedContext(userID string) *lift.Context {
    ctx := createTestContext("GET", "/", nil)
    ctx.SetUserID(userID)
    ctx.SetTenantID("tenant-123")
    ctx.Set("claims", jwt.MapClaims{
        "sub": userID,
        "exp": time.Now().Add(time.Hour).Unix(),
    })
    return ctx
}

// Create context for specific event types
func createSQSContext(messages []string) *lift.Context {
    ctx := createTestContext("POST", "/", nil)
    ctx.Request.TriggerType = adapters.TriggerSQS
    
    records := make([]interface{}, len(messages))
    for i, msg := range messages {
        records[i] = map[string]interface{}{
            "body":      msg,
            "messageId": fmt.Sprintf("msg-%d", i),
        }
    }
    ctx.Request.Records = records
    
    return ctx
}

// Helper for creating JWT tokens
func createValidJWT(userID, secret string) string {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": userID,
        "exp": time.Now().Add(time.Hour).Unix(),
    })
    tokenString, _ := token.SignedString([]byte(secret))
    return tokenString
}
```

### Response Assertions

```go
// Custom assertions for common patterns
func AssertJSONResponse(t *testing.T, ctx *lift.Context, expected interface{}) {
    t.Helper()
    
    assert.Equal(t, 200, ctx.Response.StatusCode)
    assert.Equal(t, "application/json", ctx.Response.Headers["Content-Type"])
    
    var actual interface{}
    err := json.Unmarshal(ctx.Response.Body.([]byte), &actual)
    assert.NoError(t, err)
    assert.Equal(t, expected, actual)
}

func AssertErrorResponse(t *testing.T, ctx *lift.Context, statusCode int, errorCode string) {
    t.Helper()
    
    assert.Equal(t, statusCode, ctx.Response.StatusCode)
    
    var errResp ErrorResponse
    err := json.Unmarshal(ctx.Response.Body.([]byte), &errResp)
    assert.NoError(t, err)
    assert.Equal(t, errorCode, errResp.Error.Code)
}
```

## Load Testing

### Using Lift's Load Testing Framework

```go
func TestHandlerPerformance(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test")
    }
    
    // Create test app
    testApp := lifttesting.NewTestApp()
    app := testApp.App()
    app.GET("/users/:id", handlers.GetUser)
    
    // Create load tester
    loadTester := lifttesting.NewLoadTester(testApp, &lifttesting.LoadTestConfig{
        ConcurrentUsers: 10,
        Duration:        30 * time.Second,
        ErrorThreshold:  0.01, // 1% error rate
    })
    
    // Run load test
    result, err := loadTester.RunLoadTest(context.Background(), func(app *lifttesting.TestApp) *lifttesting.TestResponse {
        return app.GET("/users/123", nil)
    })
    
    require.NoError(t, err)
    assert.Less(t, result.ErrorRate, 0.01)
    assert.Greater(t, result.RequestsPerSecond, 100.0)
    
    t.Logf("Load test results: %.2f req/s, %.2f%% error rate", 
        result.RequestsPerSecond, result.ErrorRate*100)
}
```

### Basic Performance Testing

```go
func TestHandlerPerformance(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test")
    }
    
    handler := createHandler()
    
    // Warm up
    for i := 0; i < 100; i++ {
        ctx := createTestContext("GET", "/test", nil)
        handler.Handle(ctx)
    }
    
    // Measure performance
    start := time.Now()
    iterations := 10000
    
    for i := 0; i < iterations; i++ {
        ctx := createTestContext("GET", "/test", nil)
        err := handler.Handle(ctx)
        assert.NoError(t, err)
    }
    
    duration := time.Since(start)
    avgDuration := duration / time.Duration(iterations)
    
    // Assert performance requirements
    assert.Less(t, avgDuration, 100*time.Microsecond, 
        "Handler should complete in less than 100μs")
    
    t.Logf("Average duration: %v", avgDuration)
}
```

## End-to-End Testing

### Lambda Function Testing

```go
func TestLambdaFunction(t *testing.T) {
    if os.Getenv("RUN_E2E") != "true" {
        t.Skip("Skipping E2E test")
    }
    
    // Invoke actual Lambda function
    lambdaClient := lambda.New(session.Must(session.NewSession()))
    
    payload := map[string]interface{}{
        "httpMethod": "GET",
        "path":       "/users/123",
        "headers": map[string]string{
            "Authorization": "Bearer " + getTestToken(),
        },
    }
    
    payloadBytes, _ := json.Marshal(payload)
    
    result, err := lambdaClient.Invoke(&lambda.InvokeInput{
        FunctionName: aws.String("my-function"),
        Payload:      payloadBytes,
    })
    
    assert.NoError(t, err)
    assert.Equal(t, int64(200), *result.StatusCode)
    
    var response APIGatewayResponse
    json.Unmarshal(result.Payload, &response)
    assert.Equal(t, 200, response.StatusCode)
}
```

### Testing with App.HandleTestRequest

```go
func TestDirectHandlerCall(t *testing.T) {
    app := lift.New()
    app.GET("/test", func(ctx *lift.Context) error {
        return ctx.JSON(map[string]string{"result": "ok"})
    })
    
    // Start app to configure routes
    err := app.Start()
    require.NoError(t, err)
    
    // Create test context
    ctx := createTestContext("GET", "/test", nil)
    
    // Call directly through app
    err = app.HandleTestRequest(ctx)
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    var response map[string]string
    json.Unmarshal(ctx.Response.Body.([]byte), &response)
    assert.Equal(t, "ok", response["result"])
}
```

## Test Organization

### Project Structure

```
myapp/
├── handlers/
│   ├── user.go
│   ├── user_test.go
│   ├── order.go
│   └── order_test.go
├── services/
│   ├── user_service.go
│   ├── user_service_test.go
│   └── mocks/
│       └── user_service_mock.go
├── middleware/
│   ├── auth.go
│   └── auth_test.go
└── integration/
    ├── api_test.go
    └── e2e_test.go
```

### Test Naming Conventions

```go
// Unit tests
func TestUserHandler_Get(t *testing.T) {}
func TestUserHandler_Create(t *testing.T) {}
func TestUserHandler_Update(t *testing.T) {}

// Edge cases
func TestUserHandler_Get_NotFound(t *testing.T) {}
func TestUserHandler_Create_InvalidInput(t *testing.T) {}

// Integration tests
func TestUserAPI_CompleteFlow(t *testing.T) {}

// Benchmarks
func BenchmarkUserHandler_Get(b *testing.B) {}
```

## Testing Best Practices

### 1. Test Isolation

```go
// GOOD: Each test is independent
func TestHandlerA(t *testing.T) {
    // Setup specific to this test
    ctx := createTestContext("GET", "/test", nil)
    mockDB := lifttesting.NewMockDynamORM()
    ctx.Set("db", mockDB)
    
    // Test handler...
}

// AVOID: Tests depend on shared state
var sharedDB *DB // Don't do this

func TestHandlerB(t *testing.T) {
    // Uses shared state - bad!
}
```

### 2. Use Test Tables

```go
// GOOD: Table-driven tests
func TestCalculatePrice(t *testing.T) {
    tests := []struct {
        name     string
        quantity int
        price    float64
        want     float64
    }{
        {"single item", 1, 10.0, 10.0},
        {"bulk discount", 10, 10.0, 90.0},
        {"zero quantity", 0, 10.0, 0.0},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := calculatePrice(tt.quantity, tt.price)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### 3. Mock External Dependencies

```go
// GOOD: Mock external services using Lift's patterns
func TestHandlerWithDependencies(t *testing.T) {
    ctx := createTestContext("GET", "/test", nil)
    
    // Use Lift's built-in mocks
    mockDB := lifttesting.NewMockDynamORM()
    mockS3 := lifttesting.NewMockS3Client()
    
    // Inject via context
    ctx.Set("db", mockDB)
    ctx.Set("s3", mockS3)
    
    // Set expectations
    mockDB.On("Get", mock.Anything, "123").Return(&User{}, nil)
    mockS3.On("GetObject", mock.Anything).Return(&s3.GetObjectOutput{}, nil)
    
    // Test handler
    handler := NewHandler()
    err := handler.Handle(ctx)
    
    assert.NoError(t, err)
    mockDB.AssertExpectations(t)
    mockS3.AssertExpectations(t)
}
```

### 4. Test Error Cases

```go
// GOOD: Test both success and failure
func TestService(t *testing.T) {
    t.Run("success", func(t *testing.T) {
        // Test happy path
    })
    
    t.Run("network error", func(t *testing.T) {
        // Test network failure
    })
    
    t.Run("invalid input", func(t *testing.T) {
        // Test validation
    })
    
    t.Run("timeout", func(t *testing.T) {
        // Test timeout handling
    })
}
```

### 5. Use Subtests

```go
// GOOD: Organized subtests
func TestUserCRUD(t *testing.T) {
    testApp := lifttesting.NewTestApp()
    // Configure app...
    testApp.Start()
    defer testApp.Stop()
    
    var userID string
    
    t.Run("create", func(t *testing.T) {
        resp := testApp.POST("/users", createUserReq)
        resp.AssertStatus(201)
        userID = resp.GetJSONPath("$.id").(string)
    })
    
    t.Run("read", func(t *testing.T) {
        resp := testApp.GET("/users/"+userID, nil)
        resp.AssertStatus(200)
    })
    
    t.Run("update", func(t *testing.T) {
        resp := testApp.PUT("/users/"+userID, updateUserReq)
        resp.AssertStatus(200)
    })
    
    t.Run("delete", func(t *testing.T) {
        resp := testApp.DELETE("/users/"+userID, nil)
        resp.AssertStatus(204)
    })
}
```

## Test Coverage

### Measuring Coverage

```bash
# Run tests with coverage
go test -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Check coverage percentage
go test -cover ./...
```

### Coverage Goals

```go
// Aim for high coverage of critical paths
// handlers/user.go - Target: 80%+ coverage
func (h *UserHandler) CreateUser(ctx *lift.Context) error {
    // Critical path - must be tested
    var req CreateUserRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return err // Test this error case
    }
    
    // Business logic - test thoroughly
    user, err := h.service.Create(req)
    if err != nil {
        if errors.Is(err, ErrDuplicateEmail) {
            return ctx.Status(409).JSON(map[string]string{
                "error": "Email already exists",
            }) // Test this
        }
        return ctx.Status(500).JSON(map[string]string{
            "error": "Failed to create user",
        }) // Test this
    }
    
    // Success path - test
    return ctx.Status(201).JSON(user)
}
```

## Key Takeaways

1. **Use `createTestContext()` for unit tests** - this is the established pattern in the codebase
2. **Use `lifttesting.NewTestApp()` for integration tests** - tests full HTTP flow with real server
3. **Import lift testing as `lifttesting`** - avoids conflicts with Go's standard testing package
4. **Inject dependencies with `ctx.Set()`** - standard dependency injection pattern
5. **Use Lift's built-in mocks** - `NewMockDynamORM()`, `NewMockCloudWatchClient()`, etc.
6. **Access response via `ctx.Response`** - status code, headers, body are all available
7. **Use TestApp's rich assertions** - `AssertStatus()`, `AssertJSONPath()`, etc.

## Summary

Effective testing in Lift includes:

- **Unit Tests**: Test individual handlers using `createTestContext()`
- **Integration Tests**: Test complete flows using `TestApp`
- **Mock Strategies**: Use Lift's built-in mocks and dependency injection via `ctx.Set()`
- **Test Utilities**: Helpers for creating contexts and assertions
- **Load Testing**: Performance testing with Lift's load testing framework

The key difference from standard Go HTTP testing is that Lift uses context-based dependency injection and Lambda-style event processing, not direct HTTP serving. Use the patterns shown here for reliable testing of your Lift applications. 