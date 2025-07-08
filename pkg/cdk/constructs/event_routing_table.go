package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// EventRoutingTableProps defines properties for the event routing table
type EventRoutingTableProps struct {
	// Table name
	TableName *string
	// TTL attribute name for automatic cleanup
	TimeToLiveAttribute *string
}

// EventRoutingTable is a table for managing event routing
type EventRoutingTable struct {
	construct constructs.Construct
	*LiftTable
}

// NewEventRoutingTable creates a new event routing table
// The table uses standard pk/sk attributes - GSIs should be defined in DynamORM models
func NewEventRoutingTable(scope constructs.Construct, id *string, props *EventRoutingTableProps) *EventRoutingTable {
	// Set defaults
	if props == nil {
		props = &EventRoutingTableProps{}
	}

	// Set event routing table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("event-routing")
	}
	
	// Enable TTL for event cleanup
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ttl")
	}

	// Create the table with field names from EventRoute struct
	liftTable := NewLiftTable(scope, id, &LiftTableProps{
		TableName:                 props.TableName,
		PartitionKeyName:          jsii.String("PK"),
		SortKeyName:               jsii.String("SK"),
		TimeToLiveAttribute:       props.TimeToLiveAttribute,
		EnablePointInTimeRecovery: jsii.Bool(true),
		EnableStreams:             jsii.Bool(true),
	})
	
	return &EventRoutingTable{
		construct: scope,
		LiftTable: liftTable,
	}
}

// GrantEventManagement grants permissions to manage events
func (e *EventRoutingTable) GrantEventManagement(grantee awsiam.IGrantable) {
	// Grant read/write permissions for event management
	e.Table.GrantReadWriteData(grantee)
}

// Example DynamORM model for event routing:
//
// type EventRoute struct {
//     PK         string    `dynamorm:"pk"`                          // event#{event_id}
//     SK         string    `dynamorm:"sk"`                          // timestamp#{timestamp}
//     
//     // Indexes for queries
//     EventSource      string `dynamorm:"index:source-index,pk"`  // event_source
//     Timestamp        string `dynamorm:"index:source-index,sk"`  // ISO timestamp
//     ProcessingStatus string `dynamorm:"index:status-index,pk"`  // processing_status
//     Date             string `dynamorm:"index:date-index,pk"`    // YYYY-MM-DD
//     
//     // Event data
//     EventID    string `json:"event_id"`
//     TTL        int64  `json:"ttl"`
// }