package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/lift/pkg/dynamorm"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/pay-theory/lift/pkg/lift"
	lifttesting "github.com/pay-theory/lift/pkg/testing"
	"github.com/pay-theory/lift/pkg/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCRUDAPI(t *testing.T) {
	// Setup extended mock using the proper factory pattern
	mockDB := liftmocks.NewMockExtendedDB()
	mockQuery := new(mocks.MockQuery)

	// Create test app
	app := lifttesting.NewTestApp()

	// Configure DynamORM middleware with mock factory
	config := dynamorm.DefaultConfig()
	config.TenantIsolation = true
	config.AutoTransaction = false // Disable auto transactions for testing
	factory := &dynamorm.MockDBFactory{MockDB: mockDB}

	// Add authentication middleware first (must run before DynamORM with tenant isolation)
	app.App().Use(AuthMiddleware())

	// Add DynamORM middleware with factory (needs tenant_id from auth middleware)
	app.App().Use(dynamorm.WithDynamORM(config, factory))

	// Add logging middleware last
	app.App().Use(LoggingMiddleware())

	// Set up routes
	setupRoutes(app.App())

	t.Run("Health Check", func(t *testing.T) {
		// Create a separate test setup for health check without tenant isolation
		healthApp := lifttesting.NewTestApp()

		// Configure DynamORM without tenant isolation for health check
		healthConfig := dynamorm.DefaultConfig()
		healthConfig.TenantIsolation = false // Disable for health check
		healthConfig.AutoTransaction = false
		healthFactory := &dynamorm.MockDBFactory{MockDB: mockDB}

		healthApp.App().Use(dynamorm.WithDynamORM(healthConfig, healthFactory))
		healthApp.App().Use(AuthMiddleware())
		healthApp.App().Use(LoggingMiddleware())

		// Set up just the health route
		healthApp.App().GET("/health", HealthCheck)

		response := healthApp.GET("/health")

		t.Logf("Health check response status: %d", response.StatusCode)
		t.Logf("Health check response body: '%s'", response.Body)

		assert.True(t, response.IsSuccess())
		assert.Equal(t, 200, response.StatusCode)
		assert.Contains(t, response.Body, "healthy")
	})

	t.Run("Create User - Success", func(t *testing.T) {
		// Reset mocks for this test
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations for user creation
		mockDB.On("WithContext", mock.Anything).Return(mockDB)
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery)
		mockQuery.On("Create").Return(nil)

		// Test data
		userReq := CreateUserRequest{
			Email: "test@example.com",
			Name:  "Test User",
		}

		response := app.
			WithHeader("X-Tenant-ID", "tenant123").
			WithHeader("X-User-ID", "user123").
			POST("/users", userReq)

		// Assertions
		assert.Equal(t, 201, response.StatusCode)
		assert.Contains(t, response.Body, "test@example.com")
		assert.Contains(t, response.Body, "User created successfully")

		// Parse response
		var userResp UserResponse
		err := response.JSON(&userResp)
		require.NoError(t, err)

		assert.NotNil(t, userResp.User)
		assert.Equal(t, "test@example.com", userResp.User.Email)
		assert.Equal(t, "Test User", userResp.User.Name)
		assert.True(t, userResp.User.Active)
		assert.Equal(t, "tenant123", userResp.User.TenantID)

		// Verify expectations
		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})

	t.Run("Create User - Missing Tenant", func(t *testing.T) {
		// Create a separate app without tenant isolation for testing auth failures
		authTestApp := lifttesting.NewTestApp()

		// Configure DynamORM without tenant isolation for auth error testing
		authConfig := dynamorm.DefaultConfig()
		authConfig.TenantIsolation = false // Disable so auth middleware can handle the error
		authConfig.AutoTransaction = false
		authFactory := &dynamorm.MockDBFactory{MockDB: mockDB}

		authTestApp.App().Use(AuthMiddleware())
		authTestApp.App().Use(dynamorm.WithDynamORM(authConfig, authFactory))
		authTestApp.App().Use(LoggingMiddleware())
		setupRoutes(authTestApp.App())

		userReq := CreateUserRequest{
			Email: "test@example.com",
			Name:  "Test User",
		}

		response := authTestApp.POST("/users", userReq)

		assert.Equal(t, 401, response.StatusCode)
		assert.Contains(t, response.Body, "Tenant ID is required")
	})

	t.Run("Create User - Invalid Email", func(t *testing.T) {
		// Create a separate app for validation testing
		validationTestApp := lifttesting.NewTestApp()

		// Configure DynamORM without tenant isolation for validation testing
		validationConfig := dynamorm.DefaultConfig()
		validationConfig.TenantIsolation = false // Disable for validation testing
		validationConfig.AutoTransaction = false
		validationFactory := &dynamorm.MockDBFactory{MockDB: mockDB}

		// Add validation middleware to set validator in context
		validationTestApp.App().Use(func(next lift.Handler) lift.Handler {
			return lift.HandlerFunc(func(ctx *lift.Context) error {
				ctx.SetValidator(&StructValidator{})
				return next.Handle(ctx)
			})
		})

		validationTestApp.App().Use(AuthMiddleware())
		validationTestApp.App().Use(dynamorm.WithDynamORM(validationConfig, validationFactory))
		validationTestApp.App().Use(LoggingMiddleware())
		setupRoutes(validationTestApp.App())

		userReq := CreateUserRequest{
			Email: "invalid-email",
			Name:  "Test User",
		}

		response := validationTestApp.
			WithHeader("X-Tenant-ID", "tenant123").
			WithHeader("X-User-ID", "user123").
			POST("/users", userReq)

		assert.Equal(t, 400, response.StatusCode)
		assert.Contains(t, response.Body, "VALIDATION_ERROR")
	})

	t.Run("Get User - Success", func(t *testing.T) {
		// Reset mocks for this test
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations for user retrieval
		mockDB.On("WithContext", mock.Anything).Return(mockDB)
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery)
		mockQuery.On("Where", "ID", "=", "user456").Return(mockQuery)
		mockQuery.On("First", mock.AnythingOfType("*main.User")).Run(func(args mock.Arguments) {
			user := args.Get(0).(*User)
			user.ID = "user456"
			user.TenantID = "tenant123"
			user.Email = "existing@example.com"
			user.Name = "Existing User"
			user.Active = true
			user.CreatedAt = time.Now()
			user.UpdatedAt = time.Now()
		}).Return(nil)

		response := app.
			WithHeader("X-Tenant-ID", "tenant123").
			WithHeader("X-User-ID", "user123").
			GET("/users/user456")

		assert.True(t, response.IsSuccess())
		assert.Equal(t, 200, response.StatusCode)

		var userResp UserResponse
		err := response.JSON(&userResp)
		require.NoError(t, err)

		assert.NotNil(t, userResp.User)
		assert.Equal(t, "user456", userResp.User.ID)
		assert.Equal(t, "existing@example.com", userResp.User.Email)

		// Verify expectations
		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})

	t.Run("Get User - Not Found", func(t *testing.T) {
		// Create a separate app for not found testing
		notFoundTestApp := lifttesting.NewTestApp()

		// Configure DynamORM without tenant isolation for not found testing
		notFoundConfig := dynamorm.DefaultConfig()
		notFoundConfig.TenantIsolation = false // Disable for not found testing
		notFoundConfig.AutoTransaction = false
		notFoundFactory := &dynamorm.MockDBFactory{MockDB: mockDB}

		notFoundTestApp.App().Use(AuthMiddleware())
		notFoundTestApp.App().Use(dynamorm.WithDynamORM(notFoundConfig, notFoundFactory))
		notFoundTestApp.App().Use(LoggingMiddleware())
		setupRoutes(notFoundTestApp.App())

		// Reset mocks for this test
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations for user not found - return a record not found error
		mockDB.On("WithContext", mock.Anything).Return(mockDB)
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery)
		mockQuery.On("Where", "ID", "=", "nonexistent").Return(mockQuery)
		mockQuery.On("First", mock.AnythingOfType("*main.User")).Return(
			// Use a simple error to simulate record not found
			fmt.Errorf("record not found"))

		response := notFoundTestApp.
			WithHeader("X-Tenant-ID", "tenant123").
			WithHeader("X-User-ID", "user123").
			GET("/users/nonexistent")

		assert.Equal(t, 404, response.StatusCode)
		assert.Contains(t, response.Body, "User not found")

		// Verify expectations
		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})

}

// MockDynamORMWrapper adapts the official DynamORM mocks to our wrapper interface
type MockDynamORMWrapper struct {
	mockDB    *mocks.MockDB
	mockQuery *mocks.MockQuery
	tenantID  string
}

func (m *MockDynamORMWrapper) Get(ctx context.Context, key any, result any) error {
	return m.mockDB.Model(result).Where("ID", "=", key).First(result)
}

func (m *MockDynamORMWrapper) Put(ctx context.Context, item any) error {
	return m.mockDB.Model(item).Create()
}

func (m *MockDynamORMWrapper) Delete(ctx context.Context, key any) error {
	return m.mockDB.Model(&struct{}{}).Where("ID", "=", key).Delete()
}

func (m *MockDynamORMWrapper) Query(ctx context.Context, query any) (any, error) {
	// Simple implementation for testing
	return []any{}, nil
}

func (m *MockDynamORMWrapper) BeginTransaction() (any, error) {
	// Return a mock transaction
	return &struct{}{}, nil
}

func (m *MockDynamORMWrapper) WithTenant(tenantID string) any {
	return &MockDynamORMWrapper{
		mockDB:    m.mockDB,
		mockQuery: m.mockQuery,
		tenantID:  tenantID,
	}
}

// setupRoutes configures the application routes (extracted from main for testing)
func setupRoutes(app *lift.App) {
	// Routes only - middleware is added separately in tests
	app.POST("/users", CreateUser)
	app.GET("/users/:id", GetUser)
	app.GET("/users", ListUsers)
	app.PUT("/users/:id", UpdateUser)
	app.DELETE("/users/:id", DeleteUser)
	app.GET("/health", HealthCheck)
}

// BenchmarkCRUDOperations benchmarks the CRUD operations
func BenchmarkCRUDOperations(b *testing.B) {
	app := lifttesting.NewTestApp()
	setupRoutes(app.App())

	userReq := CreateUserRequest{
		Email: "benchmark@example.com",
		Name:  "Benchmark User",
	}

	b.Run("CreateUser", func(b *testing.B) {
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
		for i := 0; i < b.N; i++ {
			response := app.
				WithHeader("X-Tenant-ID", "tenant123").
				WithHeader("X-User-ID", "user123").
				GET("/users/test123")

			// 404 is expected since we're not setting up data
			if response.StatusCode != 404 && !response.IsSuccess() {
				b.Fatalf("Unexpected response: %s", response.Body)
			}
		}
	})
}

// StructValidator implements the Lift Validator interface using the validation package
type StructValidator struct{}

func (v *StructValidator) Validate(i any) error {
	return validation.Validate(i)
}
