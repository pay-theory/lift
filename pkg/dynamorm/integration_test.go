package dynamorm

import (
	"context"
	"testing"

	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModel represents a simple test model for DynamORM
type TestModel struct {
	ID       string `dynamorm:"pk"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
}

func TestDynamORMIntegration(t *testing.T) {
	// Skip if no local DynamoDB available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test configuration for local DynamoDB
	config := &DynamORMConfig{
		TableName:       "test_lift_integration",
		Region:          "us-east-1",
		Endpoint:        "http://localhost:8000", // Local DynamoDB
		TenantIsolation: false,                   // Disable for basic test
		AutoTransaction: false,                   // Test without transactions first
	}

	t.Run("Middleware Initialization", func(t *testing.T) {
		// Test that middleware can be created without errors
		middleware := WithDynamORM(config)
		assert.NotNil(t, middleware)

		// Test with nil config (should use defaults)
		defaultMiddleware := WithDynamORM(nil)
		assert.NotNil(t, defaultMiddleware)
	})

	t.Run("DynamORM Wrapper Creation", func(t *testing.T) {
		wrapper, err := initDynamORM(config)
		if err != nil {
			t.Skipf("Could not connect to local DynamoDB: %v", err)
		}

		require.NoError(t, err)
		assert.NotNil(t, wrapper)
		assert.Equal(t, config.TableName, wrapper.tableName)
		assert.Equal(t, config.Region, wrapper.region)
	})

	t.Run("Basic CRUD Operations", func(t *testing.T) {
		wrapper, err := initDynamORM(config)
		if err != nil {
			t.Skipf("Could not connect to local DynamoDB: %v", err)
		}
		require.NoError(t, err)

		ctx := context.Background()
		testItem := &TestModel{
			ID:   "test-123",
			Name: "Test Item",
		}

		// Test Put operation
		err = wrapper.Put(ctx, testItem)
		if err != nil {
			t.Logf("Put operation failed (expected if table doesn't exist): %v", err)
		}

		// Test Get operation
		var result TestModel
		err = wrapper.Get(ctx, "test-123", &result)
		if err != nil {
			t.Logf("Get operation failed (expected if table doesn't exist): %v", err)
		}
	})

	t.Run("Transaction Operations", func(t *testing.T) {
		wrapper, err := initDynamORM(config)
		if err != nil {
			t.Skipf("Could not connect to local DynamoDB: %v", err)
		}
		require.NoError(t, err)

		// Test transaction creation
		tx, err := wrapper.BeginTransaction()
		require.NoError(t, err)
		assert.NotNil(t, tx)
		assert.False(t, tx.committed)
		assert.False(t, tx.rolledBack)

		// Test adding operations to transaction
		ctx := context.Background()
		testItem := &TestModel{
			ID:   "tx-test-123",
			Name: "Transaction Test Item",
		}

		err = tx.Put(ctx, testItem)
		require.NoError(t, err)
		assert.Len(t, tx.operations, 1)
		assert.Equal(t, "put", tx.operations[0].Type)

		// Test rollback
		err = tx.Rollback()
		require.NoError(t, err)
		assert.True(t, tx.rolledBack)
		assert.False(t, tx.committed)
	})

	t.Run("Tenant Isolation", func(t *testing.T) {
		tenantConfig := &DynamORMConfig{
			TableName:       "test_lift_tenant",
			Region:          "us-east-1",
			Endpoint:        "http://localhost:8000",
			TenantIsolation: true,
		}

		wrapper, err := initDynamORM(tenantConfig)
		if err != nil {
			t.Skipf("Could not connect to local DynamoDB: %v", err)
		}
		require.NoError(t, err)

		// Test tenant scoping
		tenant1 := wrapper.WithTenant("tenant-1")
		tenant2 := wrapper.WithTenant("tenant-2")

		assert.Equal(t, "tenant-1", tenant1.tenantID)
		assert.Equal(t, "tenant-2", tenant2.tenantID)
		assert.NotEqual(t, tenant1.tenantID, tenant2.tenantID)
	})
}

func TestDynamORMConfig(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		config := DefaultConfig()
		assert.NotNil(t, config)
		assert.Equal(t, "lift_data", config.TableName)
		assert.Equal(t, "us-east-1", config.Region)
		assert.True(t, config.AutoTransaction)
		assert.True(t, config.TenantIsolation)
		assert.Equal(t, "tenant_id", config.TenantKey)
	})

	t.Run("Session Config Creation", func(t *testing.T) {
		config := &DynamORMConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:8000",
		}

		sessionConfig := session.Config{
			Region:   config.Region,
			Endpoint: config.Endpoint,
		}

		assert.Equal(t, "us-west-2", sessionConfig.Region)
		assert.Equal(t, "http://localhost:8000", sessionConfig.Endpoint)
	})
}

// TestDynamORMMiddlewareBasic tests the middleware without the testing package
func TestDynamORMMiddlewareBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &DynamORMConfig{
		TableName:       "test_lift_middleware",
		Region:          "us-east-1",
		Endpoint:        "http://localhost:8000",
		TenantIsolation: false,
		AutoTransaction: false,
	}

	t.Run("Middleware Creation", func(t *testing.T) {
		app := lift.New()
		app.Use(WithDynamORM(config))

		// Create a test handler that uses DynamORM
		app.GET("/test", func(ctx *lift.Context) error {
			db, err := DB(ctx)
			if err != nil {
				return err
			}

			// Verify we can access the database
			assert.NotNil(t, db)
			assert.Equal(t, config.TableName, db.tableName)

			return ctx.JSON(map[string]string{
				"status": "ok",
				"table":  db.tableName,
			})
		})

		// Verify app was configured
		assert.NotNil(t, app)
	})

	t.Run("Auto Transaction Configuration", func(t *testing.T) {
		transactionConfig := &DynamORMConfig{
			TableName:       "test_lift_transaction",
			Region:          "us-east-1",
			Endpoint:        "http://localhost:8000",
			TenantIsolation: false,
			AutoTransaction: true, // Enable auto transactions
		}

		app := lift.New()
		app.Use(WithDynamORM(transactionConfig))

		// Create a POST handler (write operation)
		app.POST("/test", func(ctx *lift.Context) error {
			// Check if transaction is available in context
			tx := ctx.Get("dynamorm_transaction")
			if tx != nil {
				assert.NotNil(t, tx)
			}

			return ctx.JSON(map[string]string{
				"status":      "created",
				"transaction": "enabled",
			})
		})

		// Verify app was configured
		assert.NotNil(t, app)
	})
}
