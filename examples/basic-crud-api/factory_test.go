package main

import (
	"testing"

	"github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/lift/pkg/dynamorm"
	liftmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/pay-theory/lift/pkg/lift"
	lifttesting "github.com/pay-theory/lift/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFactoryPattern(t *testing.T) {
	// Setup extended mock
	mockDB := liftmocks.NewMockExtendedDB()
	mockQuery := new(mocks.MockQuery)

	// Create test app
	testApp := lifttesting.NewTestApp()
	app := testApp.App()

	// Configure DynamORM middleware with mock factory
	config := dynamorm.DefaultConfig()
	config.TenantIsolation = false // Disable for simple test
	config.AutoTransaction = false // Disable auto transactions for testing
	factory := &dynamorm.MockDBFactory{MockDB: mockDB}
	app.Use(dynamorm.WithDynamORM(config, factory))

	// Register routes
	app.GET("/health", HealthCheck)
	app.POST("/users", CreateUser)
	app.GET("/users/:id", GetUser)

	t.Run("HealthCheck", func(t *testing.T) {
		resp := testApp.GET("/health")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, resp.Body, "healthy")
	})

	t.Run("CreateUser", func(t *testing.T) {
		// Reset mocks
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations
		mockDB.On("WithContext", mock.Anything).Return(mockDB).Once()
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery).Once()
		mockQuery.On("Create").Return(nil).Once()

		// Test request
		resp := testApp.POST("/users", map[string]any{
			"name":  "John Doe",
			"email": "john@example.com",
		})

		// Assertions
		assert.Equal(t, 201, resp.StatusCode)
		assert.Contains(t, resp.Body, "John Doe")
		assert.Contains(t, resp.Body, "john@example.com")

		// Verify mock expectations
		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})

	t.Run("GetUser", func(t *testing.T) {
		// Reset mocks
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations
		mockDB.On("WithContext", mock.Anything).Return(mockDB).Once()
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery).Once()
		mockQuery.On("Where", "ID", "=", "123").Return(mockQuery).Once()
		mockQuery.On("First", mock.AnythingOfType("*main.User")).Run(func(args mock.Arguments) {
			user := args.Get(0).(*User)
			user.ID = "123"
			user.Name = "John Doe"
			user.Email = "john@example.com"
		}).Return(nil).Once()

		// Test request
		resp := testApp.GET("/users/123")

		// Assertions
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, resp.Body, "123")
		assert.Contains(t, resp.Body, "John Doe")
		assert.Contains(t, resp.Body, "john@example.com")

		// Verify mock expectations
		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})

	t.Run("UserNotFound", func(t *testing.T) {
		// Reset mocks
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Import the error type
		var errNotFound = assert.AnError // Use a generic error for now

		// Setup expectations
		mockDB.On("WithContext", mock.Anything).Return(mockDB).Once()
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery).Once()
		mockQuery.On("Where", "ID", "=", "999").Return(mockQuery).Once()
		mockQuery.On("First", mock.AnythingOfType("*main.User")).Return(errNotFound).Once()

		// Test request
		resp := testApp.GET("/users/999")

		// Assertions
		assert.Equal(t, 404, resp.StatusCode)
		assert.Contains(t, resp.Body, "User not found")

		// Verify mock expectations
		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})
}

func TestFactoryPatternWithTenantIsolation(t *testing.T) {
	// Setup extended mock
	mockDB := liftmocks.NewMockExtendedDB()
	mockQuery := new(mocks.MockQuery)

	// Create test app
	testApp := lifttesting.NewTestApp()
	app := testApp.App()

	// Add simple auth middleware that sets tenant ID
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			tenantID := ctx.Header("X-Tenant-ID")
			if tenantID != "" {
				ctx.Set("tenant_id", tenantID)
			}
			return next.Handle(ctx)
		})
	})

	// Configure DynamORM middleware with tenant isolation
	config := dynamorm.DefaultConfig()
	config.TenantIsolation = true
	config.AutoTransaction = false // Disable auto transactions for testing
	factory := &dynamorm.MockDBFactory{MockDB: mockDB}
	app.Use(dynamorm.WithDynamORM(config, factory))

	// Register routes
	app.GET("/users/:id", GetUser)

	t.Run("RequiresTenantID", func(t *testing.T) {
		// Test request without tenant ID
		resp := testApp.GET("/users/123")

		// Should fail with unauthorized
		assert.Equal(t, 401, resp.StatusCode)
		assert.Contains(t, resp.Body, "Tenant ID required")
	})

	t.Run("WithTenantID", func(t *testing.T) {
		// Reset mocks
		mockDB.ExpectedCalls = nil
		mockQuery.ExpectedCalls = nil

		// Setup expectations
		mockDB.On("WithContext", mock.Anything).Return(mockDB).Once()
		mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery).Once()
		mockQuery.On("Where", "ID", "=", "123").Return(mockQuery).Once()
		mockQuery.On("First", mock.AnythingOfType("*main.User")).Run(func(args mock.Arguments) {
			user := args.Get(0).(*User)
			user.ID = "123"
			user.Name = "Tenant User"
			user.Email = "user@tenant.com"
			user.TenantID = "tenant-abc"
		}).Return(nil).Once()

		// Test request with tenant ID
		resp := testApp.
			WithHeader("X-Tenant-ID", "tenant-abc").
			GET("/users/123")

		// Assertions
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, resp.Body, "Tenant User")

		// Verify mock expectations
		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})
}
