package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDynamORMScaffoldCommand(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	// Change to temp directory
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create a mock go.mod file to simulate being in a Lift project
	goModContent := `module test-project

go 1.21

require (
	github.com/pay-theory/lift v1.0.0
)
`
	err = os.WriteFile("go.mod", []byte(goModContent), 0644)
	require.NoError(t, err)

	cmd := &DynamORMScaffoldCommand{}

	t.Run("basic scaffold", func(t *testing.T) {
		args := []string{"scaffold", "--model", "User", "--table", "users"}

		err := cmd.Execute(context.Background(), args)
		require.NoError(t, err)

		// Check that files were created
		modelFile := filepath.Join("models", "user.go")
		cdkFile := filepath.Join("cdk", "constructs", "user_table.go")
		exampleFile := filepath.Join("examples", "user_example.go")

		assert.FileExists(t, modelFile)
		assert.FileExists(t, cdkFile)
		assert.FileExists(t, exampleFile)

		// Check model file content
		modelContent, err := os.ReadFile(modelFile)
		require.NoError(t, err)
		assert.Contains(t, string(modelContent), "type User struct")
		assert.Contains(t, string(modelContent), "package models")
		assert.Contains(t, string(modelContent), "func NewUser(")

		// Check CDK file content
		cdkContent, err := os.ReadFile(cdkFile)
		require.NoError(t, err)
		assert.Contains(t, string(cdkContent), "type UserTableProps struct")
		assert.Contains(t, string(cdkContent), "func NewUserTable(")
		assert.Contains(t, string(cdkContent), "package constructs")

		// Check example file content
		exampleContent, err := os.ReadFile(exampleFile)
		require.NoError(t, err)
		assert.Contains(t, string(exampleContent), "type UserService struct")
		assert.Contains(t, string(exampleContent), "func main()")
		assert.Contains(t, string(exampleContent), "package main")
	})

	t.Run("multi-tenant scaffold", func(t *testing.T) {
		args := []string{"scaffold", "--model", "Product", "--multi-tenant", "--enable-ttl"}

		err := cmd.Execute(context.Background(), args)
		require.NoError(t, err)

		// Check model file content
		modelFile := filepath.Join("models", "product.go")
		modelContent, err := os.ReadFile(modelFile)
		require.NoError(t, err)

		assert.Contains(t, string(modelContent), "TenantID string")
		assert.Contains(t, string(modelContent), "TTL       int64")
		assert.Contains(t, string(modelContent), "ExpiresAt time.Time")
		assert.Contains(t, string(modelContent), "func NewProduct(tenantID, name string)")
	})

	t.Run("scaffold with GSI", func(t *testing.T) {
		args := []string{"scaffold", "--model", "Order", "--gsi", "Status:Status:CreatedAt", "--gsi", "Customer:CustomerID"}

		err := cmd.Execute(context.Background(), args)
		require.NoError(t, err)

		// Check model file content
		modelFile := filepath.Join("models", "order.go")
		modelContent, err := os.ReadFile(modelFile)
		require.NoError(t, err)

		assert.Contains(t, string(modelContent), "Status string")
		assert.Contains(t, string(modelContent), "CustomerID string")
		assert.Contains(t, string(modelContent), "gsi:Status")
		assert.Contains(t, string(modelContent), "gsi:Customer")

		// Check CDK file content
		cdkFile := filepath.Join("cdk", "constructs", "order_table.go")
		cdkContent, err := os.ReadFile(cdkFile)
		require.NoError(t, err)

		assert.Contains(t, string(cdkContent), `IndexName: jsii.String("Status")`)
		assert.Contains(t, string(cdkContent), `IndexName: jsii.String("Customer")`)
		assert.Contains(t, string(cdkContent), `PartitionKey:   jsii.String("Status")`)
		assert.Contains(t, string(cdkContent), `PartitionKey:   jsii.String("CustomerID")`)
	})

	t.Run("missing model argument", func(t *testing.T) {
		args := []string{"scaffold"}

		err := cmd.Execute(context.Background(), args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--model is required")
	})

	t.Run("not in Lift project", func(t *testing.T) {
		// Remove go.mod to simulate not being in a Lift project
		if err := os.Remove("go.mod"); err != nil {
			t.Fatalf("Failed to remove go.mod: %v", err)
		}

		args := []string{"scaffold", "--model", "Test"}

		err := cmd.Execute(context.Background(), args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in a Lift project directory")
	})
}

func TestParseScaffoldArgs(t *testing.T) {
	cmd := &DynamORMScaffoldCommand{}

	t.Run("basic args", func(t *testing.T) {
		args := []string{"--model", "User", "--table", "custom_users"}

		config, err := cmd.parseScaffoldArgs(args)
		require.NoError(t, err)

		assert.Equal(t, "User", config.ModelName)
		assert.Equal(t, "custom_users", config.TableName)
		assert.False(t, config.MultiTenant)
		assert.False(t, config.EnableTTL)
		assert.False(t, config.EnableStreams)
		assert.Empty(t, config.GSIs)
	})

	t.Run("multi-tenant with TTL", func(t *testing.T) {
		args := []string{"--model", "Product", "--multi-tenant", "--enable-ttl", "--enable-streams"}

		config, err := cmd.parseScaffoldArgs(args)
		require.NoError(t, err)

		assert.Equal(t, "Product", config.ModelName)
		assert.Equal(t, "products", config.TableName) // Default table name
		assert.True(t, config.MultiTenant)
		assert.True(t, config.EnableTTL)
		assert.True(t, config.EnableStreams)
	})

	t.Run("GSI parsing", func(t *testing.T) {
		args := []string{
			"--model", "Order",
			"--gsi", "Status:Status:CreatedAt",
			"--gsi", "Customer:CustomerID",
		}

		config, err := cmd.parseScaffoldArgs(args)
		require.NoError(t, err)

		assert.Len(t, config.GSIs, 2)

		// First GSI
		assert.Equal(t, "Status", config.GSIs[0].IndexName)
		assert.Equal(t, "Status", config.GSIs[0].PartitionKey)
		assert.Equal(t, "CreatedAt", config.GSIs[0].SortKey)

		// Second GSI
		assert.Equal(t, "Customer", config.GSIs[1].IndexName)
		assert.Equal(t, "CustomerID", config.GSIs[1].PartitionKey)
		assert.Empty(t, config.GSIs[1].SortKey)
	})

	t.Run("invalid GSI format", func(t *testing.T) {
		args := []string{"--model", "Order", "--gsi", "InvalidFormat"}

		_, err := cmd.parseScaffoldArgs(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--gsi format should be")
	})

	t.Run("missing model", func(t *testing.T) {
		args := []string{"--table", "users"}

		_, err := cmd.parseScaffoldArgs(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--model is required")
	})
}

func TestTemplateGeneration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	cmd := &DynamORMScaffoldCommand{}

	t.Run("model template", func(t *testing.T) {
		config := &ScaffoldConfig{
			ModelName:     "TestModel",
			TableName:     "test_models",
			MultiTenant:   true,
			EnableTTL:     true,
			EnableStreams: true,
			GSIs: []GSIConfig{
				{IndexName: "Status", PartitionKey: "Status", SortKey: "CreatedAt"},
			},
		}

		err := cmd.createDirectories(config)
		require.NoError(t, err)

		err = cmd.generateModel(config)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join("models", "testmodel.go"))
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "type TestModel struct")
		assert.Contains(t, contentStr, "TenantID string")
		assert.Contains(t, contentStr, "TTL int64")
		assert.Contains(t, contentStr, "Status string")
		assert.Contains(t, contentStr, "gsi:Status")
		assert.Contains(t, contentStr, "func NewTestModel(tenantID, name string)")
	})

	t.Run("CDK template", func(t *testing.T) {
		config := &ScaffoldConfig{
			ModelName:   "TestModel",
			TableName:   "test_models",
			MultiTenant: true,
			GSIs: []GSIConfig{
				{IndexName: "Status", PartitionKey: "Status"},
			},
		}

		err := cmd.generateCDKConstruct(config)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join("cdk", "constructs", "testmodel_table.go"))
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "type TestModelTableProps struct")
		assert.Contains(t, contentStr, "func NewTestModelTable(")
		assert.Contains(t, contentStr, "EnableMultiTenant")
		assert.Contains(t, contentStr, `IndexName: jsii.String("Status")`)
		assert.Contains(t, contentStr, `PartitionKey: jsii.String("Status")`)
	})

	t.Run("example template", func(t *testing.T) {
		config := &ScaffoldConfig{
			ModelName:   "TestModel",
			TableName:   "test_models",
			MultiTenant: true,
		}

		err := cmd.generateExampleUsage(config)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join("examples", "testmodel_example.go"))
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "type TestModelService struct")
		assert.Contains(t, contentStr, "func NewTestModelService()")
		assert.Contains(t, contentStr, "tenantID := ctx.TenantID()")
		assert.Contains(t, contentStr, "/testmodels")
		assert.Contains(t, contentStr, "func main()")
	})
}

func TestCaseConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "user"},
		{"UserProfile", "userprofile"},
		{"Product", "product"},
		{"OrderItem", "orderitem"},
	}

	for _, test := range tests {
		result := strings.ToLower(test.input)
		assert.Equal(t, test.expected, result)
	}
}
