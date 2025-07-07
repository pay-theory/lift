package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// EventRoutingTableProps defines properties for the event routing table
type EventRoutingTableProps struct {
	DynamORMTableProps
	// Enable source index for querying by event source
	EnableSourceIndex *bool
	// Enable status index for querying by processing status
	EnableStatusIndex *bool
	// Enable date index for time-based queries
	EnableDateIndex *bool
}

// EventRoutingTable is a DynamORM table for managing event routing
type EventRoutingTable struct {
	*DynamORMTable
}

// NewEventRoutingTable creates a new event routing table using DynamORM
func NewEventRoutingTable(scope constructs.Construct, id *string, props *EventRoutingTableProps) *EventRoutingTable {
	// Set defaults
	if props == nil {
		props = &EventRoutingTableProps{}
	}

	// Set event routing table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("event-routing")
	}
	
	// Default partition key for event routing - the event ID
	if props.PartitionKey == nil {
		props.PartitionKey = &awsdynamodb.Attribute{
			Name: jsii.String("event_id"),
			Type: awsdynamodb.AttributeType_STRING,
		}
	}
	
	// Default sort key for event routing - the timestamp
	if props.SortKey == nil {
		props.SortKey = &awsdynamodb.Attribute{
			Name: jsii.String("timestamp"),
			Type: awsdynamodb.AttributeType_STRING,
		}
	}
	
	// Enable TTL for event cleanup
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ttl")
	}
	
	// Enable point-in-time recovery by default for event tracking
	if props.PointInTimeRecovery == nil {
		props.PointInTimeRecovery = jsii.Bool(true)
	}

	// Create the base DynamORM table with the embedded props
	dynamormTable := NewDynamORMTable(scope, id, &props.DynamORMTableProps)
	
	table := &EventRoutingTable{
		DynamORMTable: dynamormTable,
	}
	
	// Add source index if enabled
	enableSourceIndex := true
	if props.EnableSourceIndex != nil {
		enableSourceIndex = *props.EnableSourceIndex
	}
	
	if enableSourceIndex {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("source-index"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("event_source"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("timestamp"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}
	
	// Add status index if enabled
	enableStatusIndex := true
	if props.EnableStatusIndex != nil {
		enableStatusIndex = *props.EnableStatusIndex
	}
	
	if enableStatusIndex {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("status-index"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("processing_status"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("timestamp"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}
	
	// Add date index if enabled
	enableDateIndex := false
	if props.EnableDateIndex != nil {
		enableDateIndex = *props.EnableDateIndex
	}
	
	if enableDateIndex {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("date-index"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("date"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("timestamp"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}
	
	return table
}

// GrantEventManagement grants permissions to manage events
func (e *EventRoutingTable) GrantEventManagement(grantee awsiam.IGrantable) {
	// Grant read/write permissions for event management
	e.GrantReadWrite(grantee)
}

// GetSourceIndexName returns the name of the source index
func (e *EventRoutingTable) GetSourceIndexName() *string {
	return jsii.String("source-index")
}

// GetStatusIndexName returns the name of the status index
func (e *EventRoutingTable) GetStatusIndexName() *string {
	return jsii.String("status-index")
}

// GetDateIndexName returns the name of the date index
func (e *EventRoutingTable) GetDateIndexName() *string {
	return jsii.String("date-index")
}