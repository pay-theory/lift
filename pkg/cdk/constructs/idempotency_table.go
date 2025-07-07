package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// IdempotencyTableProps defines properties for creating a DynamORM-compatible idempotency table
type IdempotencyTableProps struct {
	DynamORMTableProps
	// Additional idempotency specific properties can be added here
}

// NewIdempotencyTable creates a DynamoDB table optimized for idempotency with DynamORM
func NewIdempotencyTable(scope constructs.Construct, id *string, props *IdempotencyTableProps) *DynamORMTable {
	// Set default values for idempotency table
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ExpiresAt")
	}
	
	// Set idempotency-specific partition and sort keys
	props.PartitionKey = &awsdynamodb.Attribute{
		Name: jsii.String("IdempotencyKey"),
		Type: awsdynamodb.AttributeType_STRING,
	}
	props.SortKey = &awsdynamodb.Attribute{
		Name: jsii.String("SK"),
		Type: awsdynamodb.AttributeType_STRING,
	}
	
	// Create base DynamORM table with the configured props
	table := NewDynamORMTable(scope, id, &props.DynamORMTableProps)

	// Add GSIs for different query patterns

	// GSI for Function-based queries
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("gsi-function"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("FunctionName"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// GSI for Status-based queries
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("gsi-status"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("Status"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("Timestamp"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// GSI for Tenant-based queries (if multi-tenant)
	if props.EnableMultiTenant != nil && *props.EnableMultiTenant {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("gsi-tenant"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("TenantID"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("Timestamp"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}

	// GSI for Timestamp-based queries
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("gsi-timestamp"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("Timestamp"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	return table
}