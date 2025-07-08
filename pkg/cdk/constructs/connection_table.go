package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// ConnectionTableProps defines properties for the WebSocket connection table
type ConnectionTableProps struct {
	// Table name
	TableName *string
	// Enable TTL for automatic connection cleanup
	TimeToLiveAttribute *string
}

// ConnectionTable is a table for managing WebSocket connections
type ConnectionTable struct {
	construct constructs.Construct
	*LiftTable
}

// NewConnectionTable creates a new connection management table
// The table uses pk/sk for connection_id and metadata storage
// GSIs should be defined in your DynamORM model structs
func NewConnectionTable(scope constructs.Construct, id *string, props *ConnectionTableProps) *ConnectionTable {
	// Set defaults
	if props == nil {
		props = &ConnectionTableProps{}
	}

	// Set connection table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("websocket-connections")
	}
	
	// Enable TTL for connection cleanup
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ttl")
	}

	// Create the table with standard pk/sk attributes
	liftTable := NewLiftTable(scope, id, &LiftTableProps{
		TableName:                 props.TableName,
		TimeToLiveAttribute:       props.TimeToLiveAttribute,
		EnablePointInTimeRecovery: jsii.Bool(true),
		EnableStreams:             jsii.Bool(true),
	})
	
	return &ConnectionTable{
		construct: scope,
		LiftTable: liftTable,
	}
}

// GrantConnectionManagement grants permissions to manage WebSocket connections
func (c *ConnectionTable) GrantConnectionManagement(grantee awsiam.IGrantable) {
	// Grant read/write permissions for connection management
	c.Table.GrantReadWriteData(grantee)
}

// Example DynamORM model for connections:
//
// type Connection struct {
//     PK         string    `dynamorm:"pk"`                       // connection#{connection_id}
//     SK         string    `dynamorm:"sk"`                       // connection#{connection_id}
//     
//     // Indexes for queries
//     UserID     string    `dynamorm:"index:user-index,pk"`      // user_id
//     CreatedAt  string    `dynamorm:"index:user-index,sk"`      // ISO timestamp
//     TenantID   string    `dynamorm:"index:tenant-index,pk"`    // tenant_id (if multi-tenant)
//     
//     // Connection data
//     ConnectionID string  `json:"connection_id"`
//     Endpoint     string  `json:"endpoint"`
//     TTL          int64   `json:"ttl"`
// }