package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/jsii-runtime-go"
)

func TestNewDynamORMCRUDAPI(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("users"),
	})

	// Create CRUD API with default settings
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable: table,
		EntityName:    jsii.String("User"),
	})

	// Test that the CRUD API was created successfully
	if crudAPI == nil {
		t.Fatal("DynamORMCRUDAPI should not be nil")
	}

	// Test that the table reference is correct
	if crudAPI.Table != table {
		t.Fatal("Table reference should match the provided table")
	}

	// Test that API Gateway was created
	if crudAPI.API == nil {
		t.Fatal("API Gateway should be created")
	}

	// Test that Lambda functions were created for enabled operations
	if crudAPI.CreateFunction == nil {
		t.Fatal("Create function should be created")
	}
	if crudAPI.ReadFunction == nil {
		t.Fatal("Read function should be created")
	}
	if crudAPI.UpdateFunction == nil {
		t.Fatal("Update function should be created")
	}
	if crudAPI.DeleteFunction == nil {
		t.Fatal("Delete function should be created")
	}
	if crudAPI.ListFunction == nil {
		t.Fatal("List function should be created")
	}

	// Test that search function is not created by default
	if crudAPI.SearchFunction != nil {
		t.Fatal("Search function should not be created by default")
	}

	// Test that resources were created
	if crudAPI.EntityResource == nil {
		t.Fatal("Entity resource should be created")
	}
	if crudAPI.ItemResource == nil {
		t.Fatal("Item resource should be created")
	}

	// Test that execution role was created
	if crudAPI.ExecutionRole == nil {
		t.Fatal("Execution role should be created")
	}

	// Test that metrics were created
	if crudAPI.Metrics == nil {
		t.Fatal("Metrics should be created by default")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPICustomOperations(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("orders"),
	})

	// Create CRUD API with custom operations
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable:     table,
		EntityName:        jsii.String("Order"),
		EntityResource:    jsii.String("orders"),
		PrimaryKey:        jsii.String("order_id"),
		SortKey:           jsii.String("created_at"),
		EnabledOperations: []CRUDOperation{
			CRUDOperation_CREATE,
			CRUDOperation_READ,
			CRUDOperation_LIST,
			CRUDOperation_SEARCH,
		},
		EnableSearch:      jsii.Bool(true),
		SearchableFields:  []string{"customer_id", "status", "total"},
		SearchIndexes:     []string{"gsi-customer", "gsi-status"},
	})

	// Test that only enabled functions were created
	if crudAPI.CreateFunction == nil {
		t.Fatal("Create function should be created")
	}
	if crudAPI.ReadFunction == nil {
		t.Fatal("Read function should be created")
	}
	if crudAPI.ListFunction == nil {
		t.Fatal("List function should be created")
	}
	if crudAPI.SearchFunction == nil {
		t.Fatal("Search function should be created when search is enabled")
	}

	// Test that disabled functions were not created
	if crudAPI.UpdateFunction != nil {
		t.Fatal("Update function should not be created when not enabled")
	}
	if crudAPI.DeleteFunction != nil {
		t.Fatal("Delete function should not be created when not enabled")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPIMultiTenant(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a multi-tenant DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName:         jsii.String("multi-tenant-data"),
		EnableMultiTenant: jsii.Bool(true),
		TenantAttribute:   jsii.String("org_id"),
	})

	// Create CRUD API with multi-tenant support
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable:         table,
		EntityName:            jsii.String("Document"),
		EnableMultiTenant:     jsii.Bool(true),
		TenantAttribute:       jsii.String("org_id"),
		TenantFromAuth:        jsii.Bool(true),
		AuthorizationStrategy: AuthorizationStrategy_JWT,
		JWTSecret:            jsii.String("my-jwt-secret"),
		ValidationStrategy:   ValidationStrategy_STRICT,
		RequiredFields:       []string{"title", "content"},
	})

	// Test multi-tenant configuration
	if crudAPI.props.EnableMultiTenant == nil || !*crudAPI.props.EnableMultiTenant {
		t.Fatal("Multi-tenant should be enabled")
	}

	// Test that tenant metrics exist
	if crudAPI.Metrics["TenantOperations"] == nil {
		t.Fatal("Tenant operations metric should exist")
	}

	// Test authorization strategy
	if crudAPI.props.AuthorizationStrategy != AuthorizationStrategy_JWT {
		t.Fatal("Authorization strategy should be JWT")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPIWithCaching(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("products"),
	})

	// Create CRUD API with caching enabled
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable: table,
		EntityName:    jsii.String("Product"),
		EnableCaching: jsii.Bool(true),
		CacheConfig: &DynamORMCacheProps{
			CacheStrategy:        CacheStrategy_IN_MEMORY,
			InvalidationStrategy: CacheInvalidationStrategy_TTL,
			DefaultTTL:          awscdk.Duration_Minutes(jsii.Number(30)),
			MaxCacheSize:        intPtr(5000),
		},
	})

	// Test that cache was created
	if crudAPI.Cache == nil {
		t.Fatal("Cache should be created when caching is enabled")
	}

	// Test cache strategy
	if crudAPI.Cache.props.CacheStrategy != CacheStrategy_IN_MEMORY {
		t.Fatal("Cache strategy should be IN_MEMORY")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPIPagination(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("items"),
	})

	// Create CRUD API with custom pagination settings
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable:    table,
		EntityName:       jsii.String("Item"),
		EnablePagination: jsii.Bool(true),
		DefaultPageSize:  intPtr(50),
		MaxPageSize:      intPtr(200),
		BatchSize:        intPtr(50),
	})

	// Test pagination configuration
	if crudAPI.props.EnablePagination == nil || !*crudAPI.props.EnablePagination {
		t.Fatal("Pagination should be enabled")
	}
	if crudAPI.props.DefaultPageSize == nil || *crudAPI.props.DefaultPageSize != 50 {
		t.Fatal("Default page size should be 50")
	}
	if crudAPI.props.MaxPageSize == nil || *crudAPI.props.MaxPageSize != 200 {
		t.Fatal("Max page size should be 200")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPIRateLimit(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("api-data"),
	})

	// Create CRUD API with rate limiting
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable:   table,
		EntityName:      jsii.String("Data"),
		EnableRateLimit: jsii.Bool(true),
		RateLimit:       intPtr(500),  // 500 requests per minute
		BurstLimit:      intPtr(1000), // 1000 burst limit
		TimeoutSeconds:  intPtr(60),   // 60 second timeout
	})

	// Test rate limiting configuration
	if crudAPI.props.EnableRateLimit == nil || !*crudAPI.props.EnableRateLimit {
		t.Fatal("Rate limiting should be enabled")
	}
	if crudAPI.props.RateLimit == nil || *crudAPI.props.RateLimit != 500 {
		t.Fatal("Rate limit should be 500")
	}
	if crudAPI.props.BurstLimit == nil || *crudAPI.props.BurstLimit != 1000 {
		t.Fatal("Burst limit should be 1000")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPICORS(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("cors-data"),
	})

	// Create CRUD API with custom CORS settings
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable: table,
		EntityName:    jsii.String("Data"),
		EnableCORS:    jsii.Bool(true),
		CORSOrigins:   []string{"https://example.com", "https://app.example.com"},
		CORSMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		CORSHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"},
	})

	// Test CORS configuration
	if crudAPI.props.EnableCORS == nil || !*crudAPI.props.EnableCORS {
		t.Fatal("CORS should be enabled")
	}
	if len(crudAPI.props.CORSOrigins) != 2 {
		t.Fatal("Should have 2 CORS origins")
	}
	if crudAPI.props.CORSOrigins[0] != "https://example.com" {
		t.Fatal("First CORS origin should be https://example.com")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPIEnvironmentVariables(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create CRUD API
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable:         table,
		EntityName:            jsii.String("TestEntity"),
		PrimaryKey:            jsii.String("entity_id"),
		SortKey:               jsii.String("version"),
		EnableMultiTenant:     jsii.Bool(true),
		TenantAttribute:       jsii.String("tenant_id"),
		EnablePagination:      jsii.Bool(true),
		DefaultPageSize:       intPtr(30),
		MaxPageSize:           intPtr(150),
		EnableSearch:          jsii.Bool(true),
		SearchableFields:      []string{"name", "description"},
		SearchIndexes:         []string{"gsi-name", "gsi-description"},
		ValidationStrategy:    ValidationStrategy_STRICT,
		RequiredFields:        []string{"name", "type"},
		AuthorizationStrategy: AuthorizationStrategy_COGNITO,
		CognitoUserPool:       jsii.String("us-east-1_XXXXXXXXX"),
		BatchSize:             intPtr(40),
	})

	// Test that environment variables are properly set
	// Note: In a real test, we would need to access the environment variables differently
	// since they are not directly accessible from the CDK construct

	// Test that the CRUD API was created successfully
	if crudAPI == nil {
		t.Fatal("DynamORMCRUDAPI should not be nil")
	}

	// Test configuration values
	if crudAPI.props.EntityName == nil || *crudAPI.props.EntityName != "TestEntity" {
		t.Fatal("Entity name should be TestEntity")
	}
	if crudAPI.props.PrimaryKey == nil || *crudAPI.props.PrimaryKey != "entity_id" {
		t.Fatal("Primary key should be entity_id")
	}
	if crudAPI.props.SortKey == nil || *crudAPI.props.SortKey != "version" {
		t.Fatal("Sort key should be version")
	}

	// Synth the stack to validate
	app.Synth(nil)
}

func TestDynamORMCRUDAPINilProps(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test that creating with nil props panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic when props is nil")
		}
	}()

	NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), nil)
}

func TestDynamORMCRUDAPINilTable(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test that creating with nil table panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic when DynamORMTable is nil")
		}
	}()

	NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable: nil,
	})
}

func TestDynamORMCRUDAPIGetters(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create a DynamORM table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TableName: jsii.String("test-table"),
	})

	// Create CRUD API with caching enabled
	crudAPI := NewDynamORMCRUDAPI(stack, jsii.String("TestCRUDAPI"), &DynamORMCRUDAPIProps{
		DynamORMTable: table,
		EntityName:    jsii.String("Item"),
		EnableCaching: jsii.Bool(true),
	})

	// Test getters
	if crudAPI.GetTable() != table {
		t.Fatal("GetTable should return the correct table")
	}

	if crudAPI.GetAPI() == nil {
		t.Fatal("GetAPI should not return nil")
	}

	if crudAPI.GetCache() == nil {
		t.Fatal("GetCache should not return nil when caching is enabled")
	}

	if crudAPI.GetCRUDMetrics() == nil {
		t.Fatal("GetCRUDMetrics should not return nil")
	}

	if crudAPI.GetExecutionRole() == nil {
		t.Fatal("GetExecutionRole should not return nil")
	}

	if crudAPI.GetAPIURL() == nil {
		t.Fatal("GetAPIURL should not return nil")
	}

	if crudAPI.GetEntityEndpoint() == nil {
		t.Fatal("GetEntityEndpoint should not return nil")
	}

	// Synth the stack to validate
	app.Synth(nil)
}