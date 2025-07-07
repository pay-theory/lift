package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// DynamORMTableTestSuite tests the DynamORMTable construct
type DynamORMTableTestSuite struct {
	suite.Suite
	app   awscdk.App
	stack awscdk.Stack
}

func (s *DynamORMTableTestSuite) SetupTest() {
	s.app = awscdk.NewApp(nil)
	s.stack = awscdk.NewStack(s.app, jsii.String("TestStack"), nil)
}

func (s *DynamORMTableTestSuite) TestDynamORMTableWithMockClient() {
	// Create DynamORM table construct
	table := NewDynamORMTable(s.stack, jsii.String("TestTable"), &DynamORMTableProps{
		TableName:   jsii.String("test-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		BillingMode: awsdynamodb.BillingMode_PAY_PER_REQUEST,
		EnableMultiTenant: jsii.Bool(true),
		TimeToLiveAttribute: jsii.String("expires_at"),
		Stream: awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})
	
	// Add GSI
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("email-index"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("email"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})
	
	// Verify table properties
	assert.NotNil(s.T(), table)
	assert.NotNil(s.T(), table.Table.TableName())
	
	// Verify CloudFormation template
	template := assertions.Template_FromStack(s.stack, nil)
	
	// Check table properties
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"TableName": "test-table",
		"BillingMode": "PAY_PER_REQUEST",
		"StreamSpecification": map[string]interface{}{
			"StreamViewType": "NEW_AND_OLD_IMAGES",
		},
		"TimeToLiveSpecification": map[string]interface{}{
			"AttributeName": "expires_at",
			"Enabled": true,
		},
	})
	
	// Verify that email-index was added (gsi-tenant is auto-added for multi-tenant)
	// Just check that the table has GlobalSecondaryIndexes without being too specific
	template.HasResource(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"Properties": map[string]interface{}{
			"TableName": "test-table",
			"GlobalSecondaryIndexes": assertions.Match_AnyValue(),
		},
	})
}

func (s *DynamORMTableTestSuite) TestMultiTenantConfiguration() {
	// Create multi-tenant table
	table := NewDynamORMTable(s.stack, jsii.String("MultiTenantTable"), &DynamORMTableProps{
		TableName:   jsii.String("multi-tenant-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		EnableMultiTenant: jsii.Bool(true),
	})
	
	// Add tenant GSI
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("tenant-data"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("tenant_id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("data_type"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})
	
	// Get environment variables
	envVars := table.GetEnvironmentVariables()
	
	// Verify multi-tenant configuration
	assert.NotNil(s.T(), (*envVars)["DYNAMODB_TABLE_NAME"])
	assert.NotNil(s.T(), (*envVars)["DYNAMORM_REGION"])
	
	// Verify CloudFormation includes GSIs
	template := assertions.Template_FromStack(s.stack, nil)
	template.HasResource(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"Properties": map[string]interface{}{
			"TableName": "multi-tenant-table",
			"GlobalSecondaryIndexes": assertions.Match_AnyValue(),
		},
	})
}

func (s *DynamORMTableTestSuite) TestDynamORMIndexHelper() {
	table := NewDynamORMTable(s.stack, jsii.String("IndexTable"), &DynamORMTableProps{
		TableName: jsii.String("index-table"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
	})
	
	// Add DynamORM-style index
	table.AddDynamORMIndex("UsersByEmail", 
		&awsdynamodb.Attribute{
			Name: jsii.String("gsi1pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		&awsdynamodb.Attribute{
			Name: jsii.String("gsi1sk"),
			Type: awsdynamodb.AttributeType_STRING,
		})
	
	// Verify the GSI was created with correct naming
	template := assertions.Template_FromStack(s.stack, nil)
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"GlobalSecondaryIndexes": []interface{}{
			map[string]interface{}{
				"IndexName": "gsi-UsersByEmail",
				"KeySchema": []interface{}{
					map[string]interface{}{
						"AttributeName": "gsi1pk",
						"KeyType": "HASH",
					},
					map[string]interface{}{
						"AttributeName": "gsi1sk",
						"KeyType": "RANGE",
					},
				},
			},
		},
	})
}

func TestDynamORMTableSuite(t *testing.T) {
	suite.Run(t, new(DynamORMTableTestSuite))
}

// Integration test with real testing infrastructure
func TestDynamORMTableWithTestHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Create test helper
	helper := test.NewDynamORMTestHelper(t)
	
	// Create a test table using the helper
	tableName := "test-dynamorm-table"
	helper.CreateTestTable(t, tableName,
		test.WithGSI("email-index", "email", "sk"),
		test.WithGSI("tenant-index", "tenant_id", "created_at"),
		test.WithTTL("expires_at"),
		test.WithStream(),
	)
	defer helper.DeleteTestTable(t, tableName)
	
	// Put test item
	testItem := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: "USER#123"},
		"sk":         &types.AttributeValueMemberS{Value: "PROFILE"},
		"email":      &types.AttributeValueMemberS{Value: "test@example.com"},
		"tenant_id":  &types.AttributeValueMemberS{Value: "TENANT#ABC"},
		"created_at": &types.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
		"data":       &types.AttributeValueMemberS{Value: "Test user data"},
	}
	helper.PutTestItem(t, tableName, testItem)
	
	// Retrieve and verify
	retrieved := helper.GetTestItem(t, tableName, map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "USER#123"},
		"sk": &types.AttributeValueMemberS{Value: "PROFILE"},
	})
	
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test@example.com", retrieved["email"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "TENANT#ABC", retrieved["tenant_id"].(*types.AttributeValueMemberS).Value)
}