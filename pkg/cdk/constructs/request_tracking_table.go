package constructs

import (
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// RequestTrackingTableProps defines properties for the request tracking table
type RequestTrackingTableProps struct {
	// Table name
	TableName *string
	// TTL attribute name for automatic cleanup
	TimeToLiveAttribute *string
}

// RequestTrackingTable is a table for tracking API requests and their async processing
type RequestTrackingTable struct {
	construct constructs.Construct
	*LiftTable
}

// NewRequestTrackingTable creates a new request tracking table
// The table uses standard pk/sk attributes - GSIs should be defined in DynamORM models
func NewRequestTrackingTable(scope constructs.Construct, id *string, props *RequestTrackingTableProps) *RequestTrackingTable {
	// Set defaults
	if props == nil {
		props = &RequestTrackingTableProps{}
	}

	// Set request tracking table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("request-tracking")
	}

	// Enable TTL for request cleanup
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ttl")
	}

	// Create the table with field names from RequestTracking struct
	liftTable := NewLiftTable(scope, id, &LiftTableProps{
		TableName:                 props.TableName,
		PartitionKeyName:          jsii.String("PK"),
		SortKeyName:               jsii.String("SK"),
		TimeToLiveAttribute:       props.TimeToLiveAttribute,
		EnablePointInTimeRecovery: jsii.Bool(true),
		EnableStreams:             jsii.Bool(true),
	})

	return &RequestTrackingTable{
		construct: scope,
		LiftTable: liftTable,
	}
}

// Example DynamORM model for request tracking:
//
// type RequestTracking struct {
//     PK         string    `dynamorm:"pk"`                             // request#{request_id}
//     SK         string    `dynamorm:"sk"`                             // request#{request_id}
//
//     // Indexes for queries
//     CorrelationID string `dynamorm:"index:correlation-index,pk"`    // correlation_id
//     Timestamp     string `dynamorm:"index:correlation-index,sk"`    // ISO timestamp
//     Status        string `dynamorm:"index:status-index,pk"`         // status
//     UserID        string `dynamorm:"index:user-index,pk"`           // user_id
//     Date          string `dynamorm:"index:timestamp-index,pk"`      // YYYY-MM-DD
//
//     // Request data
//     RequestID     string `json:"request_id"`
//     TTL           int64  `json:"ttl"`
// }
