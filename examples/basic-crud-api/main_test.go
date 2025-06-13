package main

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
	lifttesting "github.com/pay-theory/lift/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCRUDAPI(t *testing.T) {
	// Create test app
	app := lifttesting.NewTestApp()

	// Set up official DynamORM mocks
	mockDB := new(mocks.MockDB)
	mockQuery := new(mocks.MockQuery)

	// Configure DynamORM middleware with mock
	dynamormConfig := &dynamorm.DynamORMConfig{
		TableName:       "lift_users",
		Region:          "us-east-1",
		AutoTransaction: true,
		TenantIsolation: true,
		TenantKey:       "tenant_id",
	}

	// Add authentication middleware first (sets tenant_id)
	app.App().Use(AuthMiddleware())

	// Add logging middleware
	app.App().Use(LoggingMiddleware())

	// Add DynamORM middleware (needs tenant_id from auth)
	app.App().Use(dynamorm.WithDynamORM(dynamormConfig))

	// Override DynamORM with mock for testing
	app.App().Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Create a mock wrapper that implements the DynamORMWrapper interface
			mockWrapper := &MockDynamORMWrapper{
				mockDB:    mockDB,
				mockQuery: mockQuery,
			}

			// Replace DynamORM instances with mock
			ctx.Set("dynamorm", mockWrapper)
			ctx.Set("dynamorm_tenant", mockWrapper)
			return next.Handle(ctx)
		})
	})

	// Set up routes
	setupRoutes(app.App())

	t.Run("Health Check", func(t *testing.T) {
		response := app.GET("/health")

		assert.True(t, response.IsSuccess())
		assert.Equal(t, 200, response.StatusCode)
		assert.Contains(t, response.Body, "healthy")
	})

	t.Run("Create User - Success", func(t *testing.T) {
		// Reset mocks for this test
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations for user creation
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
		userReq := CreateUserRequest{
			Email: "test@example.com",
			Name:  "Test User",
		}

		response := app.POST("/users", userReq)

		assert.Equal(t, 401, response.StatusCode)
		assert.Contains(t, response.Body, "Tenant ID is required")
	})

	t.Run("Create User - Invalid Email", func(t *testing.T) {
		userReq := CreateUserRequest{
			Email: "invalid-email",
			Name:  "Test User",
		}

		response := app.
			WithHeader("X-Tenant-ID", "tenant123").
			WithHeader("X-User-ID", "user123").
			POST("/users", userReq)

		assert.Equal(t, 400, response.StatusCode)
		assert.Contains(t, response.Body, "validation")
	})

	t.Run("Get User - Success", func(t *testing.T) {
		// Reset mocks for this test
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations for user retrieval
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
		// Reset mocks for this test
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations for user not found
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery)
		mockQuery.On("Where", "ID", "=", "nonexistent").Return(mockQuery)
		mockQuery.On("First", mock.AnythingOfType("*main.User")).Return(assert.AnError)

		response := app.
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
}

func (m *MockDynamORMWrapper) Get(ctx context.Context, key interface{}, result interface{}) error {
	return m.mockDB.Model(result).Where("ID", "=", key).First(result)
}

func (m *MockDynamORMWrapper) Put(ctx context.Context, item interface{}) error {
	return m.mockDB.Model(item).Create()
}

func (m *MockDynamORMWrapper) Delete(ctx context.Context, key interface{}) error {
	return m.mockDB.Model(&struct{}{}).Where("ID", "=", key).Delete()
}

func (m *MockDynamORMWrapper) Query(ctx context.Context, query interface{}) (interface{}, error) {
	// Simple implementation for testing
	return []interface{}{}, nil
}

func (m *MockDynamORMWrapper) BeginTransaction() (interface{}, error) {
	// Return a mock transaction
	return &struct{}{}, nil
}

func (m *MockDynamORMWrapper) WithTenant(tenantID string) interface{} {
	return m
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
