package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
)

func TestLiftTable_CreatesTableWithFieldNames(t *testing.T) {
	// Test default behavior (pk/sk)
	t.Run("Default pk/sk attributes", func(t *testing.T) {
		app := awscdk.NewApp(nil)
		stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)
		table := NewLiftTable(stack, jsii.String("DefaultTable"), &LiftTableProps{
			TableName:        jsii.String("test-table"),
			PartitionKeyName: jsii.String("pk"),
			SortKeyName:      jsii.String("sk"),
		})

		assert.NotNil(t, table)
		assert.NotNil(t, table.Table)

		// Synthesize and check CloudFormation
		template := assertions.Template_FromStack(stack, nil)
		
		// Check that table has pk and sk as attribute names
		template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
			"KeySchema": []interface{}{
				map[string]interface{}{
					"AttributeName": "pk",
					"KeyType":       "HASH",
				},
				map[string]interface{}{
					"AttributeName": "sk",
					"KeyType":       "RANGE",
				},
			},
			"AttributeDefinitions": assertions.Match_ArrayWith(&[]interface{}{
				map[string]interface{}{
					"AttributeName": "pk",
					"AttributeType": "S",
				},
				map[string]interface{}{
					"AttributeName": "sk",
					"AttributeType": "S",
				},
			}),
		})
	})

	// Test custom field names
	t.Run("Custom field name attributes", func(t *testing.T) {
		app := awscdk.NewApp(nil)
		stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)
		
		table := NewLiftTable(stack, jsii.String("CustomTable"), &LiftTableProps{
			TableName:        jsii.String("custom-table"),
			PartitionKeyName: jsii.String("ID"),
			SortKeyName:      jsii.String("Version"),
		})

		assert.NotNil(t, table)
		assert.NotNil(t, table.Table)

		// Synthesize and check CloudFormation
		template := assertions.Template_FromStack(stack, nil)
		
		// Check that table uses custom attribute names
		template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
			"KeySchema": []interface{}{
				map[string]interface{}{
					"AttributeName": "ID",
					"KeyType":       "HASH",
				},
				map[string]interface{}{
					"AttributeName": "Version",
					"KeyType":       "RANGE",
				},
			},
			"AttributeDefinitions": assertions.Match_ArrayWith(&[]interface{}{
				map[string]interface{}{
					"AttributeName": "ID",
					"AttributeType": "S",
				},
				map[string]interface{}{
					"AttributeName": "Version",
					"AttributeType": "S",
				},
			}),
		})
	})

	// Test single key table
	t.Run("Single key table", func(t *testing.T) {
		app := awscdk.NewApp(nil)
		stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)
		
		table := NewLiftTable(stack, jsii.String("SingleKeyTable"), &LiftTableProps{
			TableName:        jsii.String("single-key-table"),
			PartitionKeyName: jsii.String("PK"),
			// No SortKeyName specified
		})

		assert.NotNil(t, table)
		assert.NotNil(t, table.Table)

		// Synthesize and check CloudFormation
		template := assertions.Template_FromStack(stack, nil)
		
		// Check that table only has partition key
		template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
			"KeySchema": []interface{}{
				map[string]interface{}{
					"AttributeName": "PK",
					"KeyType":       "HASH",
				},
			},
			"AttributeDefinitions": assertions.Match_ArrayWith(&[]interface{}{
				map[string]interface{}{
					"AttributeName": "PK",
					"AttributeType": "S",
				},
			}),
		})
	})
}

func TestWrapperConstructs_UseCorrectFieldNames(t *testing.T) {
	t.Run("ConnectionTable uses PK/SK", func(t *testing.T) {
		app := awscdk.NewApp(nil)
		stack := awscdk.NewStack(app, jsii.String("ConnectionStack"), nil)
		
		table := NewConnectionTable(stack, jsii.String("Connections"), &ConnectionTableProps{
			TableName: jsii.String("connections"),
		})

		assert.NotNil(t, table)

		template := assertions.Template_FromStack(stack, nil)
		
		// Verify it uses PK/SK as field names
		template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
			"KeySchema": []interface{}{
				map[string]interface{}{
					"AttributeName": "PK",
					"KeyType":       "HASH",
				},
				map[string]interface{}{
					"AttributeName": "SK",
					"KeyType":       "RANGE",
				},
			},
		})
	})

	t.Run("RateLimitTable uses PK/SK", func(t *testing.T) {
		app := awscdk.NewApp(nil)
		stack := awscdk.NewStack(app, jsii.String("RateLimitStack"), nil)
		
		table := NewRateLimitTable(stack, jsii.String("RateLimits"), &RateLimitTableProps{
			TableName: jsii.String("rate-limits"),
		})

		assert.NotNil(t, table)

		template := assertions.Template_FromStack(stack, nil)
		
		// Verify it uses PK/SK as field names
		template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
			"KeySchema": []interface{}{
				map[string]interface{}{
					"AttributeName": "PK",
					"KeyType":       "HASH",
				},
				map[string]interface{}{
					"AttributeName": "SK",
					"KeyType":       "RANGE",
				},
			},
		})
	})
}