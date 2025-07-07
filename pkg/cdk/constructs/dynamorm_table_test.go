package constructs

import (
	"fmt"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
)

func TestNewDynamORMTable(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test cases
	tests := []struct {
		name      string
		props     *DynamORMTableProps
		shouldErr bool
		check     func(t *testing.T, table *DynamORMTable)
	}{
		{
			name: "creates table with default settings",
			props: &DynamORMTableProps{
				TableName: jsii.String("test-table"),
				PartitionKey: &awsdynamodb.Attribute{
					Name: jsii.String("pk"),
					Type: awsdynamodb.AttributeType_STRING,
				},
			},
			check: func(t *testing.T, table *DynamORMTable) {
				assert.NotNil(t, table)
				assert.NotNil(t, table.Table)
				assert.Equal(t, awsdynamodb.BillingMode_PAY_PER_REQUEST, table.props.BillingMode)
				assert.True(t, *table.props.PointInTimeRecovery)
				assert.True(t, *table.props.DeletionProtection)
			},
		},
		{
			name: "creates table with sort key",
			props: &DynamORMTableProps{
				TableName: jsii.String("test-table-with-sort"),
				PartitionKey: &awsdynamodb.Attribute{
					Name: jsii.String("pk"),
					Type: awsdynamodb.AttributeType_STRING,
				},
				SortKey: &awsdynamodb.Attribute{
					Name: jsii.String("sk"),
					Type: awsdynamodb.AttributeType_STRING,
				},
			},
			check: func(t *testing.T, table *DynamORMTable) {
				assert.NotNil(t, table)
				assert.NotNil(t, table.Table)
			},
		},
		{
			name: "creates table with custom billing mode",
			props: &DynamORMTableProps{
				TableName:   jsii.String("test-table-provisioned"),
				BillingMode: awsdynamodb.BillingMode_PROVISIONED,
				ReadCapacity: jsii.Number(10),
				WriteCapacity: jsii.Number(5),
				PartitionKey: &awsdynamodb.Attribute{
					Name: jsii.String("pk"),
					Type: awsdynamodb.AttributeType_STRING,
				},
			},
			check: func(t *testing.T, table *DynamORMTable) {
				assert.NotNil(t, table)
				assert.Equal(t, awsdynamodb.BillingMode_PROVISIONED, table.props.BillingMode)
			},
		},
		{
			name: "creates table with TTL",
			props: &DynamORMTableProps{
				TableName:           jsii.String("test-table-ttl"),
				TimeToLiveAttribute: jsii.String("ExpiresAt"),
				PartitionKey: &awsdynamodb.Attribute{
					Name: jsii.String("pk"),
					Type: awsdynamodb.AttributeType_STRING,
				},
			},
			check: func(t *testing.T, table *DynamORMTable) {
				assert.NotNil(t, table)
				assert.Equal(t, "ExpiresAt", *table.props.TimeToLiveAttribute)
			},
		},
		{
			name: "creates table with streams enabled",
			props: &DynamORMTableProps{
				TableName: jsii.String("test-table-streams"),
				Stream:    awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
				PartitionKey: &awsdynamodb.Attribute{
					Name: jsii.String("pk"),
					Type: awsdynamodb.AttributeType_STRING,
				},
			},
			check: func(t *testing.T, table *DynamORMTable) {
				assert.NotNil(t, table)
				assert.NotNil(t, table.GetStreamArn())
			},
		},
		{
			name: "creates multi-tenant table",
			props: &DynamORMTableProps{
				TableName:         jsii.String("test-table-multitenant"),
				EnableMultiTenant: jsii.Bool(true),
				PartitionKey: &awsdynamodb.Attribute{
					Name: jsii.String("pk"),
					Type: awsdynamodb.AttributeType_STRING,
				},
			},
			check: func(t *testing.T, table *DynamORMTable) {
				assert.NotNil(t, table)
				assert.True(t, *table.props.EnableMultiTenant)
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use unique ID for each table
			tableId := fmt.Sprintf("TestTable%d", i)
			if tt.shouldErr {
				assert.Panics(t, func() {
					NewDynamORMTable(stack, jsii.String(tableId), tt.props)
				})
			} else {
				// Create table
				table := NewDynamORMTable(stack, jsii.String(tableId), tt.props)
				
				// Run specific checks
				tt.check(t, table)
			}
		})
	}
}

func TestNewDynamORMTable_RequiresPartitionKey(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Should panic when no partition key
	assert.Panics(t, func() {
		NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
			TableName: jsii.String("test-table-no-pk"),
		})
	})
}

func TestDynamORMTable_Synthesis(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create DynamORMTable
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName: jsii.String("test-create-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Verify table was created
	assert.NotNil(t, table.Table)
	
	// Test synthesized CloudFormation
	template := assertions.Template_FromStack(stack, nil)
	
	// Check table properties
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"TableName": "test-create-table",
		"BillingMode": "PAY_PER_REQUEST",
		"KeySchema": []interface{}{
			map[string]interface{}{
				"AttributeName": "pk",
				"KeyType": "HASH",
			},
			map[string]interface{}{
				"AttributeName": "sk",
				"KeyType": "RANGE",
			},
		},
		"SSESpecification": map[string]interface{}{
			"SSEEnabled": true,
		},
		"PointInTimeRecoverySpecification": map[string]interface{}{
			"PointInTimeRecoveryEnabled": true,
		},
		"DeletionProtectionEnabled": true,
	})
}

func TestDynamORMTable_AddGSI(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create and configure table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName: jsii.String("test-gsi-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Add GSI
	table.AddGSI(&GSIProps{
		IndexName:    jsii.String("test-gsi"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("gsi-pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("gsi-sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// Test synthesized CloudFormation
	template := assertions.Template_FromStack(stack, nil)
	
	// Check GSI properties
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"GlobalSecondaryIndexes": []interface{}{
			map[string]interface{}{
				"IndexName": "test-gsi",
				"KeySchema": []interface{}{
					map[string]interface{}{
						"AttributeName": "gsi-pk",
						"KeyType": "HASH",
					},
					map[string]interface{}{
						"AttributeName": "gsi-sk",
						"KeyType": "RANGE",
					},
				},
				"Projection": map[string]interface{}{
					"ProjectionType": "ALL",
				},
			},
		},
	})
}

func TestDynamORMTable_AddDynamORMIndex(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName: jsii.String("test-dynamorm-index"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Add DynamORM index
	table.AddDynamORMIndex("status", 
		&awsdynamodb.Attribute{
			Name: jsii.String("status"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		&awsdynamodb.Attribute{
			Name: jsii.String("created_at"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	)

	// Test synthesized CloudFormation
	template := assertions.Template_FromStack(stack, nil)
	
	// Check GSI with DynamORM naming convention
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"GlobalSecondaryIndexes": []interface{}{
			map[string]interface{}{
				"IndexName": "gsi-status",
			},
		},
	})
}

func TestDynamORMTable_Permissions(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName: jsii.String("test-permissions-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Test table getters
	assert.NotNil(t, table.GetTable())
	assert.NotNil(t, table.GetTableName())
	assert.NotNil(t, table.GetTableArn())
}

func TestDynamORMTable_EnvironmentVariables(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName: jsii.String("test-env-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Get environment variables
	envVars := table.GetEnvironmentVariables()
	assert.NotNil(t, envVars)
	assert.Contains(t, *envVars, "DYNAMODB_TABLE_NAME")
	assert.Contains(t, *envVars, "DYNAMORM_REGION")
}

func TestDynamORMTable_ModelCompatibility(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create table
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName: jsii.String("test-model-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Test compatible model
	err := table.ValidateModelCompatibility(DynamORMModelSpec{
		ModelName:    "TestModel",
		PartitionKey: "pk",
		SortKey:      "sk",
	})
	assert.NoError(t, err)

	// Test incompatible model
	err = table.ValidateModelCompatibility(DynamORMModelSpec{
		ModelName:    "TestModel",
		PartitionKey: "wrong_pk",
		SortKey:      "sk",
	})
	assert.Error(t, err)
}

func TestDynamORMTable_AutoScaling(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create table with provisioned capacity
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName:         jsii.String("test-autoscaling-table"),
		BillingMode:       awsdynamodb.BillingMode_PROVISIONED,
		ReadCapacity:      jsii.Number(5),
		WriteCapacity:     jsii.Number(5),
		EnableAutoScaling: jsii.Bool(true),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// The auto-scaling should be enabled automatically
	assert.NotNil(t, table)
}

func TestDynamORMTable_Tags(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create table with tags
	tags := &map[string]*string{
		"Environment": jsii.String("test"),
		"Application": jsii.String("dynamorm"),
	}

	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName: jsii.String("test-tags-table"),
		Tags:      tags,
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Add additional tags
	table.AddTags(&map[string]*string{
		"Team": jsii.String("backend"),
	})

	// Verify construct exists
	assert.NotNil(t, table)
	
	// Check that standard tags are added
	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"Tags": assertions.Match_ArrayWith(&[]interface{}{
			map[string]interface{}{
				"Key":   "Framework",
				"Value": "DynamORM",
			},
			map[string]interface{}{
				"Key":   "ManagedBy",
				"Value": "CDK",
			},
		}),
	})
}

func TestDynamORMTable_ConfigureForDynamORM(t *testing.T) {
	// Create a test stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create table with timestamps enabled
	table := NewDynamORMTable(stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName:        jsii.String("test-dynamorm-config"),
		EnableTimestamps: jsii.Bool(true),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})

	// Configure for DynamORM
	table.ConfigureForDynamORM()

	// Should have added GSI for created_at queries
	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"GlobalSecondaryIndexes": assertions.Match_ArrayWith(&[]interface{}{
			map[string]interface{}{
				"IndexName": "gsi-created-at",
				"KeySchema": assertions.Match_AnyValue(),
				"Projection": map[string]interface{}{
					"ProjectionType": "ALL",
				},
			},
		}),
	})
}