package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// StreamingTableProps defines properties for creating a streaming table
type StreamingTableProps struct {
	// Table name
	TableName *string
	// Stream view type (NEW_IMAGE, OLD_IMAGE, NEW_AND_OLD_IMAGES, KEYS_ONLY)
	StreamViewType awsdynamodb.StreamViewType
	// TTL attribute name for automatic cleanup
	TimeToLiveAttribute *string
	// Enable auto-scaling
	EnableAutoScaling *bool
	// Read capacity (for provisioned mode)
	ReadCapacity *float64
	// Write capacity (for provisioned mode)
	WriteCapacity *float64
}

// StreamingTable is a table with DynamoDB Streams enabled
type StreamingTable struct {
	construct constructs.Construct
	*LiftTable
}

// NewStreamingTable creates a new DynamoDB table with streams
// The table uses standard pk/sk attributes - GSIs should be defined in DynamORM models
func NewStreamingTable(scope constructs.Construct, id *string, props *StreamingTableProps) *StreamingTable {
	// Set defaults
	if props == nil {
		props = &StreamingTableProps{}
	}

	// Set stream view type default
	if props.StreamViewType == "" {
		props.StreamViewType = awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES
	}

	// Set streaming table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("streaming-table")
	}

	// Create the table with streams enabled
	liftTable := NewLiftTable(scope, id, &LiftTableProps{
		TableName:                 props.TableName,
		EnableStreams:             jsii.Bool(true),
		StreamViewType:            props.StreamViewType,
		TimeToLiveAttribute:       props.TimeToLiveAttribute,
		EnableAutoScaling:         props.EnableAutoScaling,
		ReadCapacity:              props.ReadCapacity,
		WriteCapacity:             props.WriteCapacity,
		EnablePointInTimeRecovery: jsii.Bool(true),
	})
	
	return &StreamingTable{
		construct: scope,
		LiftTable: liftTable,
	}
}

// GetStreamArn returns the DynamoDB stream ARN
func (s *StreamingTable) GetStreamArn() *string {
	return s.Table.TableStreamArn()
}

// GrantStreamRead grants stream read permissions
func (s *StreamingTable) GrantStreamRead(grantee awsiam.IGrantable) awsiam.Grant {
	return s.Table.GrantStreamRead(grantee)
}

// GetTableName returns the table name
func (s *StreamingTable) GetTableName() *string {
	return s.Table.TableName()
}

// GetTableArn returns the table ARN
func (s *StreamingTable) GetTableArn() *string {
	return s.Table.TableArn()
}

// GetResourceName returns the resource name for monitoring (implements MonitorableResource interface)
func (s *StreamingTable) GetResourceName() *string {
	return s.Table.TableName()
}

// Example DynamORM model for streaming data:
//
// type StreamRecord struct {
//     PK         string    `dynamorm:"pk"`     // entity#{id}
//     SK         string    `dynamorm:"sk"`     // timestamp#{timestamp}
//     
//     // Your data fields
//     EntityID   string    `json:"entity_id"`
//     EventType  string    `json:"event_type"`
//     Data       string    `json:"data"`
//     TTL        int64     `json:"ttl,omitempty"`
// }