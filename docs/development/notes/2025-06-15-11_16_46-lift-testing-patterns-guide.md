# Lift Testing Patterns - Complete Guide

**Date**: 2025-06-15  
**Issue**: Developer needs correct patterns for testing Lift handlers and applications

## The Issue with `app.ServeHTTP()`

❌ **This doesn't exist in Lift**: Unlike standard Go HTTP servers, Lift apps don't have a `ServeHTTP()` method because they're designed for Lambda environments, not direct HTTP serving.

## Solution: Proper Lift Testing Patterns

### 1. Import Pattern (Avoiding Naming Conflicts)

```go
import (
    "testing"  // Go standard testing
    lifttesting "github.com/pay-theory/lift/pkg/testing"  // Lift testing utilities
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/lift/adapters"
    "github.com/stretchr/testify/assert"
)
```

### 2. Unit Testing Individual Handlers

**Pattern from `pkg/lift/jwt_test.go`:**

```go
// Helper function to create test context (COPY THIS PATTERN)
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

func TestHealthHandler(t *testing.T) {
    // 1. Create test context
    ctx := createTestContext("GET", "/health", nil)
    
    // 2. Set dependencies via ctx.Set()
    mockDB := &MockDB{connected: true}
    ctx.Set("db", mockDB)
    
    // 3. Set any required headers/params
    ctx.Request.Headers["X-Tenant-ID"] = "test-tenant"
    ctx.SetParam("id", "123")  // If needed
    
    // 4. Create and call handler
    handler := handlers.NewHealthHandler()
    err := handler.Health(ctx)
    
    // 5. Assert on results
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    // 6. Check JSON response
    var response map[string]interface{}
    err = json.Unmarshal(ctx.Response.Body.([]byte), &response)
    assert.NoError(t, err)
    assert.Equal(t, "healthy", response["status"])
}
```

### 3. Integration Testing with TestApp

**Pattern from `pkg/testing/scenarios.go`:**

```go
func TestHealthEndpointIntegration(t *testing.T) {
    // 1. Create test app
    testApp := lifttesting.NewTestApp()
    
    // 2. Configure your routes on the underlying app
    app := testApp.App()
    
    // 3. Add your handlers
    healthHandler := handlers.NewHealthHandler()
    app.GET("/health", healthHandler.Health)
    
    // 4. Start the test server
    err := testApp.Start()
    assert.NoError(t, err)
    defer testApp.Stop()
    
    // 5. Make HTTP requests
    resp := testApp.GET("/health", nil)
    
    // 6. Use rich assertions
    resp.AssertStatus(200).
        AssertJSONPath("$.status", "healthy").
        AssertJSONPath("$.service", "migrant").
        AssertHeaderExists("Content-Type")
}
```

### 4. Testing with Dependencies and Middleware

```go
func TestHealthHandlerWithDatabase(t *testing.T) {
    // Create mock database
    mockDB := lifttesting.NewMockDynamORM()
    
    // Create test context
    ctx := createTestContext("GET", "/health", nil)
    
    // Inject dependencies (this is the key pattern!)
    ctx.Set("db", mockDB)
    ctx.Set("dynamorm", mockDB)
    
    // Set up mock expectations
    mockDB.On("Ping").Return(nil)
    
    // Create handler
    handler := handlers.NewHealthHandler()
    
    // Call handler
    err := handler.Health(ctx)
    
    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    // Verify mock was called
    mockDB.AssertExpectations(t)
}
```

### 5. Testing with Middleware Chain

```go
func TestHealthWithMiddleware(t *testing.T) {
    testApp := lifttesting.NewTestApp()
    app := testApp.App()
    
    // Add middleware (this runs during integration tests)
    app.Use(middleware.RequestID())
    app.Use(dynamorm.WithDynamORM(&dynamorm.DynamORMConfig{
        TableName: "test-health",
    }))
    
    // Add handler
    app.GET("/health", handlers.NewHealthHandler().Health)
    
    // Start and test
    testApp.Start()
    defer testApp.Stop()
    
    resp := testApp.GET("/health", nil)
    resp.AssertStatus(200).
        AssertHeaderExists("X-Request-ID")
}
```

### 6. Missing `NewTestContext()` - Create Your Own

**The `testing.NewTestContext()` referenced in the AI guide doesn't exist yet. Create this helper:**

```go
// Add this to your test files or create pkg/testing/context.go
func NewTestContext() *lift.Context {
    return createTestContext("GET", "/", nil)
}

func NewTestContextWithMethod(method, path string) *lift.Context {
    return createTestContext(method, path, nil)
}

func NewTestContextWithBody(method, path string, body interface{}) *lift.Context {
    var bodyBytes []byte
    if body != nil {
        bodyBytes, _ = json.Marshal(body)
    }
    return createTestContext(method, path, bodyBytes)
}
```

### 7. Complete Health Handler Test Example

```go
package handlers_test

import (
    "context"
    "encoding/json"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/lift/adapters"
    lifttesting "github.com/pay-theory/lift/pkg/testing"
    
    "your-app/handlers"
)

// Test context helper
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

// Mock database
type MockDB struct {
    mock.Mock
    connected bool
}

func (m *MockDB) Ping() error {
    args := m.Called()
    return args.Error(0)
}

func TestHealthHandler_WithDatabase(t *testing.T) {
    tests := []struct {
        name           string
        dbConnected    bool
        expectedStatus int
        expectedHealth string
    }{
        {
            name:           "healthy database",
            dbConnected:    true,
            expectedStatus: 200,
            expectedHealth: "healthy",
        },
        {
            name:           "unhealthy database",
            dbConnected:    false,
            expectedStatus: 503,
            expectedHealth: "unhealthy",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup
            ctx := createTestContext("GET", "/health", nil)
            mockDB := &MockDB{connected: tt.dbConnected}
            
            // Set dependencies
            ctx.Set("db", mockDB)
            
            // Set mock expectations
            if tt.dbConnected {
                mockDB.On("Ping").Return(nil)
            } else {
                mockDB.On("Ping").Return(errors.New("connection failed"))
            }
            
            // Create and call handler
            handler := handlers.NewHealthHandler()
            err := handler.Health(ctx)
            
            // Assertions
            assert.NoError(t, err)
            assert.Equal(t, tt.expectedStatus, ctx.Response.StatusCode)
            
            // Check response body
            var response map[string]interface{}
            err = json.Unmarshal(ctx.Response.Body.([]byte), &response)
            assert.NoError(t, err)
            assert.Equal(t, tt.expectedHealth, response["status"])
            
            mockDB.AssertExpectations(t)
        })
    }
}

func TestHealthHandler_Integration(t *testing.T) {
    // Integration test using TestApp
    testApp := lifttesting.NewTestApp()
    app := testApp.App()
    
    // Add real middleware and handlers
    app.GET("/health", handlers.NewHealthHandler().Health)
    
    testApp.Start()
    defer testApp.Stop()
    
    // Test the full HTTP flow
    resp := testApp.GET("/health", nil)
    resp.AssertStatus(200).
        AssertJSONPath("$.status", "healthy").
        AssertJSONPath("$.service", "migrant").
        AssertHeaderExists("Content-Type")
}
```

## Key Takeaways

1. **Use `createTestContext()` for unit tests** - this is the established pattern
2. **Use `lifttesting.NewTestApp()` for integration tests** - tests full HTTP flow
3. **Inject dependencies with `ctx.Set()`** - standard dependency injection pattern
4. **Access response via `ctx.Response`** - status code, headers, body are all available
5. **Import lift testing as `lifttesting`** - avoids conflicts with Go's testing package
6. **`testing.NewTestContext()` doesn't exist yet** - use the `createTestContext()` pattern instead

## Working Health Handler Test

```go
func TestHealth_WithDatabase(t *testing.T) {
    // ✅ This works - proper Lift testing pattern
    ctx := createTestContext("GET", "/health", nil)
    
    // ✅ Set up mock database
    mockDB := &MockDB{connected: true}
    ctx.Set("db", mockDB)
    mockDB.On("Ping").Return(nil)

    // ✅ Create handler and call it
    handler := handlers.NewHealthHandler()
    err := handler.Health(ctx)  // This now works!
    
    // ✅ Assert on response
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    var response map[string]interface{}
    json.Unmarshal(ctx.Response.Body.([]byte), &response)
    assert.Equal(t, "healthy", response["status"])
}
```

This should solve your Sprint 1 testing issues! 