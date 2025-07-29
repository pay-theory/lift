package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
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
	// Create base props
	baseProps := &BaseManagementTableProps{
		DefaultTableName: "websocket-connections",
	}
	
	if props != nil {
		baseProps.TableName = props.TableName
		baseProps.TimeToLiveAttribute = props.TimeToLiveAttribute
	}

	// Create the table using common function
	liftTable := createManagementTable(scope, id, baseProps)

	return &ConnectionTable{
		construct: scope,
		LiftTable: liftTable,
	}
}

// GrantConnectionManagement grants permissions to manage WebSocket connections
func (c *ConnectionTable) GrantConnectionManagement(grantee awsiam.IGrantable) {
	grantManagementPermissions(c.LiftTable, grantee)
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
