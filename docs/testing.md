# Testing

Comprehensive testing is crucial for building reliable serverless applications. This guide covers unit testing, integration testing, and end-to-end testing strategies for Lift applications.

## Overview

Lift provides extensive testing support including:

- **Test Utilities**: Helper functions for creating test contexts
- **Mock Support**: Built-in mocks for AWS services
- **Handler Testing**: Easy testing of individual handlers
- **Middleware Testing**: Test middleware in isolation
- **Integration Testing**: Test complete request flows
- **Load Testing**: Performance and scalability testing

## Unit Testing

### Testing Handlers

```go
package handlers_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/testing"
)

func TestGetUserHandler(t *testing.T) {
    // Create test context
    ctx := testing.NewContext()
    
    // Set up request
    ctx.Request.Method = "GET"
    ctx.Request.Path = "/users/123"
    ctx.SetParam("id", "123")
    
    // Mock dependencies
    userService := &MockUserService{
        GetFunc: func(id string) (*User, error) {
            return &User{
                ID:    id,
                Name:  "Test User",
                Email: "test@example.com",
            }, nil
        },
    }
    
    // Create handler with mocks
    handler := NewUserHandler(userService)
    
    // Execute handler
    err := handler.GetUser(ctx)
    
    // Assert results
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    // Check response body
    var response UserResponse
    err = ctx.ParseResponseJSON(&response)
    assert.NoError(t, err)
    assert.Equal(t, "123", response.ID)
    assert.Equal(t, "Test User", response.Name)
}

func TestGetUserHandler_NotFound(t *testing.T) {
    ctx := testing.NewContext()
    ctx.SetParam("id", "999")
    
    userService := &MockUserService{
        GetFunc: func(id string) (*User, error) {
            return nil, ErrNotFound
        },
    }
    
    handler := NewUserHandler(userService)
    err := handler.GetUser(ctx)
    
    // Should return 404 error
    assert.Error(t, err)
    httpErr, ok := err.(lift.HTTPError)
    assert.True(t, ok)
    assert.Equal(t, 404, httpErr.Status())
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
    
    // Mock service
    userService := &MockUserService{
        CreateFunc: func(req CreateUserRequest) (*User, error) {
            return &User{
                ID:    "new-123",
                Name:  req.Name,
                Email: req.Email,
            }, nil
        },
    }
    
    // Create typed handler
    handler := func(ctx *lift.Context, req CreateUserRequest) (UserResponse, error) {
        user, err := userService.Create(req)
        if err != nil {
            return UserResponse{}, err
        }
        
        return UserResponse{
            ID:    user.ID,
            Name:  user.Name,
            Email: user.Email,
        }, nil
    }
    
    // Test the handler
    ctx := testing.NewContext()
    ctx.Request.Method = "POST"
    ctx.Request.Body = testing.MustMarshalJSON(req)
    
    // Wrap with TypedHandler
    typedHandler := lift.TypedHandler(handler)
    err := typedHandler.Handle(ctx)
    
    assert.NoError(t, err)
    assert.Equal(t, 200, ctx.Response.StatusCode)
    
    var resp UserResponse
    ctx.ParseResponseJSON(&resp)
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
        claims := ctx.Get("claims").(jwt.MapClaims)
        return ctx.JSON(map[string]interface{}{
            "user_id": claims["sub"],
        })
    })
    
    // Wrap handler with middleware
    handler := authMiddleware(testHandler)
    
    t.Run("valid token", func(t *testing.T) {
        ctx := testing.NewContext()
        
        // Create valid token
        token := testing.CreateJWT("user-123", "test-secret")
        ctx.Request.Headers["Authorization"] = "Bearer " + token
        
        err := handler.Handle(ctx)
        assert.NoError(t, err)
        assert.Equal(t, 200, ctx.Response.StatusCode)
    })
    
    t.Run("missing token", func(t *testing.T) {
        ctx := testing.NewContext()
        
        err := handler.Handle(ctx)
        assert.Error(t, err)
        assert.Equal(t, 401, err.(lift.HTTPError).Status())
    })
    
    t.Run("invalid token", func(t *testing.T) {
        ctx := testing.NewContext()
        ctx.Request.Headers["Authorization"] = "Bearer invalid-token"
        
        err := handler.Handle(ctx)
        assert.Error(t, err)
        assert.Equal(t, 401, err.(lift.HTTPError).Status())
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
    ctx := testing.NewContext()
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

## Integration Testing

### Testing Complete Request Flow

```go
func TestCompleteUserFlow(t *testing.T) {
    // Set up test app
    app := createTestApp()
    
    // Test user registration
    t.Run("register user", func(t *testing.T) {
        req := map[string]interface{}{
            "name":     "Jane Doe",
            "email":    "jane@example.com",
            "password": "SecurePass123!",
        }
        
        resp := app.TestRequest("POST", "/register", req)
        assert.Equal(t, 201, resp.StatusCode)
        
        var user UserResponse
        json.Unmarshal(resp.Body, &user)
        assert.NotEmpty(t, user.ID)
        
        // Store for next tests
        t.Setenv("TEST_USER_ID", user.ID)
    })
    
    // Test login
    t.Run("login", func(t *testing.T) {
        req := map[string]interface{}{
            "email":    "jane@example.com",
            "password": "SecurePass123!",
        }
        
        resp := app.TestRequest("POST", "/login", req)
        assert.Equal(t, 200, resp.StatusCode)
        
        var loginResp LoginResponse
        json.Unmarshal(resp.Body, &loginResp)
        assert.NotEmpty(t, loginResp.Token)
        
        // Store token for authenticated requests
        t.Setenv("TEST_TOKEN", loginResp.Token)
    })
    
    // Test authenticated request
    t.Run("get profile", func(t *testing.T) {
        token := os.Getenv("TEST_TOKEN")
        headers := map[string]string{
            "Authorization": "Bearer " + token,
        }
        
        resp := app.TestRequestWithHeaders("GET", "/profile", nil, headers)
        assert.Equal(t, 200, resp.StatusCode)
        
        var profile ProfileResponse
        json.Unmarshal(resp.Body, &profile)
        assert.Equal(t, "jane@example.com", profile.Email)
    })
}
```

### Testing with Real AWS Services

```go
func TestDynamoDBIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Use local DynamoDB or test table
    db := dynamorm.New(dynamorm.Config{
        TableName: "test-users-" + uuid.New().String(),
        Region:    "us-east-1",
        Endpoint:  "http://localhost:8000", // Local DynamoDB
    })
    
    // Create table
    err := db.CreateTable(&User{})
    require.NoError(t, err)
    defer db.DeleteTable(&User{})
    
    // Test operations
    t.Run("create and retrieve user", func(t *testing.T) {
        user := &User{
            ID:    "test-123",
            Name:  "Test User",
            Email: "test@example.com",
        }
        
        // Create
        err := db.Save(user)
        assert.NoError(t, err)
        
        // Retrieve
        var retrieved User
        err = db.Get(&retrieved, "test-123")
        assert.NoError(t, err)
        assert.Equal(t, user.Name, retrieved.Name)
        assert.Equal(t, user.Email, retrieved.Email)
    })
}
```

## Mock Strategies

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
    GetFunc    func(id string) (*User, error)
    CreateFunc func(user *User) error
    UpdateFunc func(user *User) error
    DeleteFunc func(id string) error
    
    // Track calls for assertions
    Calls []MockCall
}

func (m *MockUserService) Get(id string) (*User, error) {
    m.Calls = append(m.Calls, MockCall{Method: "Get", Args: []interface{}{id}})
    if m.GetFunc != nil {
        return m.GetFunc(id)
    }
    return nil, nil
}

// Use in tests
func TestHandler(t *testing.T) {
    mock := &MockUserService{
        GetFunc: func(id string) (*User, error) {
            return &User{ID: id, Name: "Mock User"}, nil
        },
    }
    
    handler := NewHandler(mock)
    // Test handler...
    
    // Assert mock was called correctly
    assert.Len(t, mock.Calls, 1)
    assert.Equal(t, "Get", mock.Calls[0].Method)
}
```

### AWS Service Mocks

```go
// Mock S3 client
type MockS3Client struct {
    s3iface.S3API
    GetObjectFunc func(*s3.GetObjectInput) (*s3.GetObjectOutput, error)
    PutObjectFunc func(*s3.PutObjectInput) (*s3.PutObjectOutput, error)
}

func (m *MockS3Client) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
    if m.GetObjectFunc != nil {
        return m.GetObjectFunc(input)
    }
    return &s3.GetObjectOutput{
        Body: ioutil.NopCloser(strings.NewReader("mock content")),
    }, nil
}

// Use in tests
func TestS3Handler(t *testing.T) {
    mockS3 := &MockS3Client{
        GetObjectFunc: func(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
            assert.Equal(t, "my-bucket", *input.Bucket)
            assert.Equal(t, "test.txt", *input.Key)
            
            return &s3.GetObjectOutput{
                Body: ioutil.NopCloser(strings.NewReader("file content")),
            }, nil
        },
    }
    
    handler := NewS3Handler(mockS3)
    // Test handler...
}
```

## Test Utilities

### Context Helpers

```go
// Create context with common setup
func createAuthenticatedContext(userID string) *lift.Context {
    ctx := testing.NewContext()
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
    ctx := testing.NewContext()
    ctx.Request.TriggerType = lift.TriggerSQS
    
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

### Basic Load Test

```go
func TestHandlerPerformance(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test")
    }
    
    handler := createHandler()
    
    // Warm up
    for i := 0; i < 100; i++ {
        ctx := testing.NewContext()
        handler.Handle(ctx)
    }
    
    // Measure performance
    start := time.Now()
    iterations := 10000
    
    for i := 0; i < iterations; i++ {
        ctx := testing.NewContext()
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

### Concurrent Load Test

```go
func TestConcurrentRequests(t *testing.T) {
    handler := createHandler()
    
    // Test concurrent requests
    concurrency := 100
    requestsPerWorker := 100
    
    var wg sync.WaitGroup
    errors := make(chan error, concurrency*requestsPerWorker)
    
    start := time.Now()
    
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for j := 0; j < requestsPerWorker; j++ {
                ctx := testing.NewContext()
                ctx.SetRequestID(fmt.Sprintf("req-%d-%d", workerID, j))
                
                if err := handler.Handle(ctx); err != nil {
                    errors <- err
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    duration := time.Since(start)
    totalRequests := concurrency * requestsPerWorker
    rps := float64(totalRequests) / duration.Seconds()
    
    // Check for errors
    var errorCount int
    for err := range errors {
        errorCount++
        t.Logf("Error: %v", err)
    }
    
    assert.Equal(t, 0, errorCount, "No errors expected")
    t.Logf("Processed %d requests in %v (%.2f req/s)", totalRequests, duration, rps)
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

### API Gateway Testing

```go
func TestAPIGateway(t *testing.T) {
    if os.Getenv("RUN_E2E") != "true" {
        t.Skip("Skipping E2E test")
    }
    
    baseURL := os.Getenv("API_GATEWAY_URL")
    client := &http.Client{Timeout: 10 * time.Second}
    
    // Test endpoint
    req, _ := http.NewRequest("GET", baseURL+"/users", nil)
    req.Header.Set("Authorization", "Bearer "+getTestToken())
    
    resp, err := client.Do(req)
    assert.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, 200, resp.StatusCode)
    
    var users []User
    json.NewDecoder(resp.Body).Decode(&users)
    assert.NotEmpty(t, users)
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
    db := createTestDB()
    defer db.Close()
    
    handler := NewHandler(db)
    // Test...
}

// AVOID: Tests depend on shared state
var sharedDB *DB // Don't do this

func TestHandlerB(t *testing.T) {
    // Uses shared state
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
// GOOD: Mock external services
type EmailService interface {
    Send(to, subject, body string) error
}

type MockEmailService struct {
    SendFunc func(to, subject, body string) error
    Calls    []EmailCall
}

func (m *MockEmailService) Send(to, subject, body string) error {
    m.Calls = append(m.Calls, EmailCall{To: to, Subject: subject})
    if m.SendFunc != nil {
        return m.SendFunc(to, subject, body)
    }
    return nil
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
    var userID string
    
    t.Run("create", func(t *testing.T) {
        // Create user
        userID = createdUser.ID
    })
    
    t.Run("read", func(t *testing.T) {
        // Read user using userID
    })
    
    t.Run("update", func(t *testing.T) {
        // Update user
    })
    
    t.Run("delete", func(t *testing.T) {
        // Delete user
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
    if err := ctx.ParseAndValidate(&req); err != nil {
        return err // Test this error case
    }
    
    // Business logic - test thoroughly
    user, err := h.service.Create(req)
    if err != nil {
        if errors.Is(err, ErrDuplicateEmail) {
            return lift.Conflict("Email already exists") // Test this
        }
        return lift.InternalError("Failed to create user") // Test this
    }
    
    // Success path - test
    return ctx.Status(201).JSON(user)
}
```

## Summary

Effective testing in Lift includes:

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions
- **E2E Tests**: Test complete user flows
- **Mock Strategies**: Isolate external dependencies
- **Test Utilities**: Helpers for common test scenarios
- **Load Testing**: Ensure performance requirements

Comprehensive testing gives confidence in your serverless application's reliability and performance. 