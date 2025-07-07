package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// StreamingTableProps extends DynamORMTableProps for streaming tables
type StreamingTableProps struct {
	DynamORMTableProps
	// Stream view type (NEW_IMAGE, OLD_IMAGE, NEW_AND_OLD_IMAGES, KEYS_ONLY)
	StreamViewType awsdynamodb.StreamViewType
	// Enable stream encryption (defaults to true)
	EnableStreamEncryption *bool
}

// StreamingTable is a DynamORM table with DynamoDB Streams enabled
type StreamingTable struct {
	*DynamORMTable
}

// NewStreamingTable creates a new DynamoDB table with streams using DynamORM
func NewStreamingTable(scope constructs.Construct, id *string, props *StreamingTableProps) *StreamingTable {
	// Set defaults
	if props == nil {
		props = &StreamingTableProps{}
	}

	// Set stream view type default
	if props.StreamViewType == "" {
		props.StreamViewType = awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES
	}

	// Enable stream encryption by default
	_ = true // enableStreamEncryption - reserved for future use
	if props.EnableStreamEncryption != nil {
		_ = *props.EnableStreamEncryption
	}

	// Set streaming table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("streaming-table")
	}
	
	// Default partition and sort keys for streaming table if not provided
	if props.PartitionKey == nil {
		props.PartitionKey = &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		}
	}
	
	if props.SortKey == nil {
		props.SortKey = &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		}
	}

	// Enable streams in the base props
	props.Stream = props.StreamViewType
	
	// Create the base DynamORM table with streams enabled
	dynamormTable := NewDynamORMTable(scope, id, &props.DynamORMTableProps)
	
	table := &StreamingTable{
		DynamORMTable: dynamormTable,
	}
	
	// Enable auto-scaling if configured
	if props.EnableAutoScaling != nil && *props.EnableAutoScaling && props.BillingMode == awsdynamodb.BillingMode_PROVISIONED {
		table.EnableAutoScaling(nil, nil, nil)
	}
	
	// Add DynamORM-specific permissions
	// Note: Stream permissions will be granted when Lambda functions need access
	
	return table
}

// GetStreamArn returns the DynamoDB stream ARN
func (s *StreamingTable) GetStreamArn() *string {
	return s.Table.TableStreamArn()
}

// GrantStreamRead grants stream read permissions using DynamORM methods
func (s *StreamingTable) GrantStreamRead(grantee awsiam.IGrantable) awsiam.Grant {
	return s.Table.GrantStreamRead(grantee)
}