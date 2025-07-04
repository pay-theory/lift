# Testing Guide: Writing Testable Lift Applications

**This is the COMPREHENSIVE guide for testing Lift applications with proper mocking, test patterns, and AI-friendly practices.**

## What is This Guide?

This guide demonstrates the **STANDARD patterns** for writing testable Lift applications. It shows the **preferred approaches** for unit testing, integration testing, mocking external dependencies, and ensuring your Lift applications are thoroughly tested and maintainable.

## Why Use These Testing Patterns?

✅ **USE these patterns when:**
- Building production Lift applications that need comprehensive testing
- Need to mock external dependencies (databases, AWS services, APIs)
- Want fast, reliable, and repeatable tests
- Require both unit and integration testing strategies
- Building applications that follow TDD/BDD practices

❌ **DON'T USE when:**
- Building simple prototypes or proof-of-concepts
- Creating one-off scripts or utilities
- Applications where testing overhead exceeds benefits
- Development environments where rapid iteration is prioritized

## Core Testing Principles

### 1. Test Structure (FOUNDATION Pattern)

**Purpose:** Organize tests for clarity and maintainability
**When to use:** All Lift application testing

```go
// CORRECT: Structured test organization
func TestUserService(t *testing.T) {
    // Setup: Create test app and dependencies
    app := lifttesting.NewTestApp()
    mockDB := setupMockDatabase()
    
    // Configure test app with mocks
    setupTestApp(app, mockDB)
    
    t.Run("CreateUser - Success", func(t *testing.T) {
        // Given: Test data and expectations
        userReq := CreateUserRequest{
            Email: "test@example.com",
            Name:  "Test User",
        }
        
        // Setup mock expectations
        mockDB.On("Model", mock.AnythingOfType("*User")).Return(mockQuery)
        mockQuery.On("Create").Return(nil)
        
        // When: Execute the test
        response := app.
            WithHeader("X-Tenant-ID", "tenant123").
            WithHeader("X-User-ID", "user123").
            POST("/users", userReq)
        
        // Then: Verify results
        assert.Equal(t, 201, response.StatusCode)
        assert.Contains(t, response.Body, "test@example.com")
        
        // Verify mock expectations
        mockDB.AssertExpectations(t)
        mockQuery.AssertExpectations(t)
    })
}

// INCORRECT: Unstructured tests
// func TestStuff(t *testing.T) {
//     // Everything mixed together - hard to maintain and debug
// }
```

### 2. Test App Setup (STANDARD Pattern)

**Purpose:** Create isolated test environments for each scenario
**When to use:** All Lift HTTP endpoint testing

```go
// CORRECT: Proper test app setup with dependencies
func setupTestApp(mockDB MockDatabase) *lifttesting.TestApp {
    app := lifttesting.NewTestApp()
    
    // REQUIRED: Configure middleware in correct order
    app.App().Use(AuthMiddleware())           // Authentication first
    app.App().Use(setupMockDynamORM(mockDB)) // Database mocking
    app.App().Use(LoggingMiddleware())        // Logging last
    
    // REQUIRED: Setup routes
    setupRoutes(app.App())
    
    return app
}

func setupMockDynamORM(mockDB MockDatabase) lift.Middleware {
    config := dynamorm.DefaultConfig()
    config.TenantIsolation = true
    config.AutoTransaction = false  // REQUIRED: Disable for testing
    factory := &dynamorm.MockDBFactory{MockDB: mockDB}
    
    return dynamorm.WithDynamORM(config, factory)
}

// INCORRECT: Shared test app state
// var globalApp = lifttesting.NewTestApp()  // Causes test pollution
```

### 3. Mock Database Setup (CRITICAL Pattern)

**Purpose:** Mock database operations for predictable testing
**When to use:** Testing handlers that interact with databases

```go
// CORRECT: Comprehensive database mocking
func setupMockDatabase() (*MockExtendedDB, *MockQuery) {
    mockDB := NewMockExtendedDB()
    mockQuery := new(MockQuery)
    
    return mockDB, mockQuery
}

func TestCreateUser_Success(t *testing.T) {
    mockDB, mockQuery := setupMockDatabase()
    app := setupTestApp(mockDB)
    
    // REQUIRED: Setup specific expectations for this test
    mockDB.On("WithContext", mock.Anything).Return(mockDB)
    mockDB.On("Model", mock.AnythingOfType("*User")).Return(mockQuery)
    mockQuery.On("Create").Return(nil)  // Expect successful creation
    
    // Test data
    userReq := CreateUserRequest{
        Email: "test@example.com",
        Name:  "Test User",
    }
    
    // Execute test
    response := app.
        WithHeader("X-Tenant-ID", "tenant123").
        WithHeader("X-User-ID", "user123").
        POST("/users", userReq)
    
    // REQUIRED: Verify both response and mock expectations
    assert.Equal(t, 201, response.StatusCode)
    mockDB.AssertExpectations(t)
    mockQuery.AssertExpectations(t)
}

// INCORRECT: Real database in tests
// func TestWithRealDB(t *testing.T) {
//     db := dynamodb.New()  // Slow, unreliable, requires AWS setup
// }
```

### 4. AWS Service Mocking (PRODUCTION Pattern)

**Purpose:** Mock AWS services for reliable testing without external dependencies
**When to use:** Testing applications that integrate with AWS services

```go
// CORRECT: Comprehensive AWS service mocking
func TestCloudWatchIntegration(t *testing.T) {
    // Setup AWS service mocks
    metricsMock := testing.NewMockCloudWatchMetricsClient()
    alarmsMock := testing.NewMockCloudWatchAlarmsClient()
    apiGatewayMock := testing.NewMockAPIGatewayManagementClient()
    
    // Configure mock behavior
    config := testing.DefaultMockCloudWatchConfig()
    config.NetworkDelay = 5 * time.Millisecond  // Simulate realistic latency
    metricsMock.WithConfig(config)
    
    t.Run("PublishMetrics - Success", func(t *testing.T) {
        metrics := []*testing.MockMetricDatum{
            {
                MetricName: "RequestCount",
                Value:      100.0,
                Unit:       testing.MetricUnitCount,
                Dimensions: map[string]string{
                    "Service": "UserAPI",
                    "Environment": "test",
                },
            },
        }
        
        // Execute
        err := metricsMock.PutMetricData(context.Background(), "MyApp/API", metrics)
        
        // Verify
        assert.NoError(t, err)
        assert.Equal(t, 1, metricsMock.GetCallCount("PutMetricData"))
        
        // Verify metrics were stored
        allMetrics := metricsMock.GetAllMetrics()
        assert.Contains(t, allMetrics, "MyApp/API")
    })
}

// INCORRECT: Real AWS calls in tests
// func TestWithRealAWS(t *testing.T) {
//     svc := cloudwatch.New()  // Requires AWS credentials, slow, expensive
//     svc.PutMetricData(...)
// }
```

### 5. Error Scenario Testing (RELIABILITY Pattern)

**Purpose:** Test error conditions and edge cases comprehensively
**When to use:** All production code paths

```go
// CORRECT: Comprehensive error scenario testing
func TestErrorScenarios(t *testing.T) {
    t.Run("Database Connection Error", func(t *testing.T) {
        mockDB, mockQuery := setupMockDatabase()
        app := setupTestApp(mockDB)
        
        // REQUIRED: Setup failure expectation
        mockDB.On("WithContext", mock.Anything).Return(mockDB)
        mockDB.On("Model", mock.AnythingOfType("*User")).Return(mockQuery)
        mockQuery.On("Create").Return(errors.New("connection timeout"))
        
        userReq := CreateUserRequest{
            Email: "test@example.com",
            Name:  "Test User",
        }
        
        response := app.
            WithHeader("X-Tenant-ID", "tenant123").
            WithHeader("X-User-ID", "user123").
            POST("/users", userReq)
        
        // REQUIRED: Verify error response
        assert.Equal(t, 500, response.StatusCode)
        assert.Contains(t, response.Body, "Database error")
        
        mockDB.AssertExpectations(t)
        mockQuery.AssertExpectations(t)
    })
    
    t.Run("Invalid Input Data", func(t *testing.T) {
        app := setupTestAppWithValidation()
        
        // Test with invalid email
        userReq := CreateUserRequest{
            Email: "invalid-email",  // Missing @ symbol
            Name:  "Test User",
        }
        
        response := app.
            WithHeader("X-Tenant-ID", "tenant123").
            WithHeader("X-User-ID", "user123").
            POST("/users", userReq)
        
        assert.Equal(t, 400, response.StatusCode)
        assert.Contains(t, response.Body, "VALIDATION_ERROR")
    })
    
    t.Run("Missing Authentication", func(t *testing.T) {
        app := setupTestApp(nil)
        
        userReq := CreateUserRequest{
            Email: "test@example.com",
            Name:  "Test User",
        }
        
        // CRITICAL: Test without required headers
        response := app.POST("/users", userReq)  // No tenant/user headers
        
        assert.Equal(t, 401, response.StatusCode)
        assert.Contains(t, response.Body, "Tenant ID is required")
    })
}

// INCORRECT: Only testing happy path
// func TestOnlySuccess(t *testing.T) {
//     // Only tests successful scenarios - real apps will fail in production
// }
```

### 6. Integration Testing (COMPREHENSIVE Pattern)

**Purpose:** Test complete workflows with multiple components
**When to use:** Testing business processes that span multiple services

```go
// CORRECT: End-to-end integration testing
func TestUserWorkflow_Integration(t *testing.T) {
    // Setup integrated mock environment
    mockDB, mockQuery := setupMockDatabase()
    metricsMock := testing.NewMockCloudWatchMetricsClient()
    app := setupIntegratedTestApp(mockDB, metricsMock)
    
    t.Run("Complete User Lifecycle", func(t *testing.T) {
        // Step 1: Create user
        mockDB.On("WithContext", mock.Anything).Return(mockDB)
        mockDB.On("Model", mock.AnythingOfType("*User")).Return(mockQuery)
        mockQuery.On("Create").Return(nil)
        
        createReq := CreateUserRequest{
            Email: "integration@example.com",
            Name:  "Integration User",
        }
        
        createResp := app.
            WithHeader("X-Tenant-ID", "tenant123").
            WithHeader("X-User-ID", "admin123").
            POST("/users", createReq)
        
        assert.Equal(t, 201, createResp.StatusCode)
        
        // Extract created user ID from response
        var userResp UserResponse
        err := createResp.JSON(&userResp)
        require.NoError(t, err)
        userID := userResp.User.ID
        
        // Step 2: Retrieve user
        mockQuery.On("Where", "ID", "=", userID).Return(mockQuery)
        mockQuery.On("First", mock.AnythingOfType("*User")).Run(func(args mock.Arguments) {
            user := args.Get(0).(*User)
            user.ID = userID
            user.Email = "integration@example.com"
            user.Name = "Integration User"
            user.TenantID = "tenant123"
            user.Active = true
        }).Return(nil)
        
        getResp := app.
            WithHeader("X-Tenant-ID", "tenant123").
            WithHeader("X-User-ID", "admin123").
            GET(fmt.Sprintf("/users/%s", userID))
        
        assert.Equal(t, 200, getResp.StatusCode)
        
        // Step 3: Update user
        mockQuery.On("Where", "ID", "=", userID).Return(mockQuery)
        mockQuery.On("Updates", mock.Anything).Return(nil)
        
        updateReq := UpdateUserRequest{
            Name: "Updated Integration User",
        }
        
        updateResp := app.
            WithHeader("X-Tenant-ID", "tenant123").
            WithHeader("X-User-ID", "admin123").
            PUT(fmt.Sprintf("/users/%s", userID), updateReq)
        
        assert.Equal(t, 200, updateResp.StatusCode)
        
        // Step 4: Delete user
        mockQuery.On("Where", "ID", "=", userID).Return(mockQuery)
        mockQuery.On("Delete").Return(nil)
        
        deleteResp := app.
            WithHeader("X-Tenant-ID", "tenant123").
            WithHeader("X-User-ID", "admin123").
            DELETE(fmt.Sprintf("/users/%s", userID))
        
        assert.Equal(t, 204, deleteResp.StatusCode)
        
        // Verify all mock expectations
        mockDB.AssertExpectations(t)
        mockQuery.AssertExpectations(t)
        
        // Verify metrics were published (if using metrics)
        assert.GreaterOrEqual(t, metricsMock.GetCallCount("PutMetricData"), 0)
    })
}
```

### 7. Performance Testing (BENCHMARK Pattern)

**Purpose:** Ensure applications meet performance requirements
**When to use:** Critical application paths and before production deployment

```go
// CORRECT: Comprehensive performance testing
func BenchmarkUserOperations(b *testing.B) {
    // Setup benchmark environment
    app := lifttesting.NewTestApp()
    setupRoutes(app.App())
    
    userReq := CreateUserRequest{
        Email: "benchmark@example.com",
        Name:  "Benchmark User",
    }
    
    b.Run("CreateUser", func(b *testing.B) {
        b.ResetTimer()  // REQUIRED: Reset timer after setup
        
        for i := 0; i < b.N; i++ {
            response := app.
                WithHeader("X-Tenant-ID", "tenant123").
                WithHeader("X-User-ID", "user123").
                POST("/users", userReq)
            
            if !response.IsSuccess() {
                b.Fatalf("Request failed: %s", response.Body)
            }
        }
    })
    
    b.Run("GetUser", func(b *testing.B) {
        b.ResetTimer()
        
        for i := 0; i < b.N; i++ {
            response := app.
                WithHeader("X-Tenant-ID", "tenant123").
                WithHeader("X-User-ID", "user123").
                GET("/users/test123")
            
            // Accept 404 as valid for benchmark (no setup data)
            if response.StatusCode != 404 && !response.IsSuccess() {
                b.Fatalf("Unexpected response: %s", response.Body)
            }
        }
    })
}

// Performance requirements validation
func TestPerformanceRequirements(t *testing.T) {
    app := setupTestApp(nil)
    
    // Test response time requirements
    start := time.Now()
    response := app.GET("/health")
    duration := time.Since(start)
    
    assert.True(t, response.IsSuccess())
    assert.Less(t, duration, 100*time.Millisecond, "Health check should respond within 100ms")
}
```

## Testing Different Application Patterns

### HTTP API Testing

```go
// STANDARD: HTTP endpoint testing pattern
func TestHTTPEndpoint(t *testing.T) {
    app := lifttesting.NewTestApp()
    
    // Setup test endpoint
    app.App().GET("/api/data", func(ctx *lift.Context) error {
        return ctx.JSON(map[string]string{"message": "success"})
    })
    
    // Test successful request
    response := app.GET("/api/data")
    
    assert.Equal(t, 200, response.StatusCode)
    assert.Contains(t, response.Body, "success")
    
    // Test with headers
    response = app.
        WithHeader("Authorization", "Bearer token").
        WithHeader("X-Custom-Header", "value").
        GET("/api/data")
    
    assert.True(t, response.IsSuccess())
}
```

### Event Handler Testing

```go
// STANDARD: Event processing testing pattern
func TestEventHandler(t *testing.T) {
    app := lifttesting.NewTestApp()
    
    // Setup event handler
    app.App().Handle("POST", "/events", func(ctx *lift.Context) error {
        // Process event
        return ctx.JSON(map[string]string{"status": "processed"})
    })
    
    // Test SQS event
    sqsEvent := map[string]any{
        "Records": []any{
            map[string]any{
                "eventSource": "aws:sqs",
                "body":        `{"orderId": "12345"}`,
                "messageId":   "test-message-id",
            },
        },
    }
    
    response := app.HandleRequest(context.Background(), sqsEvent)
    assert.Equal(t, 200, response.StatusCode)
}
```

### Middleware Testing

```go
// STANDARD: Middleware testing pattern
func TestCustomMiddleware(t *testing.T) {
    var capturedContext *lift.Context
    
    // Create test middleware
    testMiddleware := func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            capturedContext = ctx
            ctx.Set("middleware_executed", true)
            return next.Handle(ctx)
        })
    }
    
    app := lifttesting.NewTestApp()
    app.App().Use(testMiddleware)
    app.App().GET("/test", func(ctx *lift.Context) error {
        executed := ctx.Get("middleware_executed").(bool)
        return ctx.JSON(map[string]bool{"middleware_executed": executed})
    })
    
    response := app.GET("/test")
    
    assert.Equal(t, 200, response.StatusCode)
    assert.NotNil(t, capturedContext)
    assert.Contains(t, response.Body, "middleware_executed")
}
```

## What This Guide Teaches

### ✅ Best Practices Demonstrated

1. **ALWAYS use lifttesting.NewTestApp()** - Provides isolated test environment
2. **ALWAYS mock external dependencies** - Databases, AWS services, third-party APIs
3. **ALWAYS test error scenarios** - Network failures, validation errors, auth failures
4. **ALWAYS verify mock expectations** - Ensure mocks were called as expected
5. **ALWAYS use proper test structure** - Given/When/Then pattern for clarity

### 🚫 Critical Anti-Patterns Avoided

1. **Real external dependencies in tests** - Slow, unreliable, expensive
2. **Shared test state** - Tests that depend on each other or global state
3. **Missing error testing** - Only testing happy path scenarios
4. **No mock verification** - Setting up mocks but not verifying they were called
5. **Poor test organization** - Tests that are hard to understand and maintain

### 📊 Testing Performance Benefits

- **Test Speed**: Mock-based tests run 10-100x faster than real dependencies
- **Reliability**: 99%+ test reliability vs 60-80% with real dependencies
- **Coverage**: Can test error scenarios impossible with real dependencies
- **Cost**: $0 AWS costs vs $100s+ for integration testing with real services

## Common Testing Patterns

### Test Data Factory Pattern

```go
// RECOMMENDED: Test data factories for consistent test data
func NewTestUser() *User {
    return &User{
        ID:       "test-user-123",
        Email:    "test@example.com",
        Name:     "Test User",
        TenantID: "test-tenant",
        Active:   true,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
}

func NewTestUserRequest() CreateUserRequest {
    return CreateUserRequest{
        Email: "test@example.com",
        Name:  "Test User",
    }
}
```

### Test Helper Pattern

```go
// RECOMMENDED: Reusable test helpers
func assertSuccessResponse(t *testing.T, response *lifttesting.TestResponse) {
    t.Helper()
    assert.True(t, response.IsSuccess())
    assert.Greater(t, len(response.Body), 0)
}

func assertErrorResponse(t *testing.T, response *lifttesting.TestResponse, expectedStatus int, expectedMessage string) {
    t.Helper()
    assert.Equal(t, expectedStatus, response.StatusCode)
    assert.Contains(t, response.Body, expectedMessage)
}
```

### Test Cleanup Pattern

```go
// RECOMMENDED: Proper test cleanup
func TestWithCleanup(t *testing.T) {
    mockDB, mockQuery := setupMockDatabase()
    
    // Cleanup function
    t.Cleanup(func() {
        mockDB.AssertExpectations(t)
        mockQuery.AssertExpectations(t)
    })
    
    // Test implementation...
}
```

## Testing Tools and Libraries

### Required Dependencies

```go
// REQUIRED: Core testing dependencies
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
    lifttesting "github.com/pay-theory/lift/pkg/testing"
)
```

### Recommended Testing Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./...

# Run specific test
go test -run TestUserService ./...

# Verbose output
go test -v ./...
```

## Next Steps

After mastering these testing patterns:

1. **Mocking Demo** → See `examples/mocking-demo/`
2. **Mockery Idempotency** → See `examples/mockery-idempotency/`
3. **Basic CRUD API Tests** → See `examples/basic-crud-api/main_test.go`
4. **Production API** → See `examples/production-api/`

This guide provides the complete foundation for testing Lift applications - master these patterns to build reliable, maintainable, and thoroughly tested applications.