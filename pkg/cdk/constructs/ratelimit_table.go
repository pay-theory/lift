package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// RateLimitTableProps defines properties for creating a DynamORM-compatible rate limit table
type RateLimitTableProps struct {
	DynamORMTableProps
	// Additional rate limit specific properties can be added here
}

// NewRateLimitTable creates a DynamoDB table optimized for rate limiting with DynamORM
func NewRateLimitTable(scope constructs.Construct, id *string, props *RateLimitTableProps) *DynamORMTable {
	// Set default values for rate limit table
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ExpiresAt")
	}
	
	// Set rate limit-specific partition and sort keys
	props.PartitionKey = &awsdynamodb.Attribute{
		Name: jsii.String("Identifier"),
		Type: awsdynamodb.AttributeType_STRING,
	}
	props.SortKey = &awsdynamodb.Attribute{
		Name: jsii.String("WindowTime"),
		Type: awsdynamodb.AttributeType_STRING,
	}
	
	// Create base DynamORM table with the configured props
	table := NewDynamORMTable(scope, id, &props.DynamORMTableProps)

	// Add GSIs for different query patterns
	
	// GSI for IP-based queries
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("gsi-ip"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("IPAddress"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// GSI for User-based queries
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("gsi-user"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("UserID"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// GSI for Tenant-based queries
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("gsi-tenant"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("TenantID"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// GSI for Bucket-based queries (for Limited library)
	table.AddGSI(&GSIProps{
		IndexName: jsii.String("gsi-bucket"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("BucketKey"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	return table
}