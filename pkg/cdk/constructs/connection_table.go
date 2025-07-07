package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// ConnectionTableProps defines properties for the WebSocket connection table
type ConnectionTableProps struct {
	DynamORMTableProps
	// Enable user index for user-based queries
	EnableUserIndex *bool
	// Enable tenant index for multi-tenant support
	EnableTenantIndex *bool
}

// ConnectionTable is a DynamORM table for managing WebSocket connections
type ConnectionTable struct {
	*DynamORMTable
}

// NewConnectionTable creates a new connection management table using DynamORM
func NewConnectionTable(scope constructs.Construct, id *string, props *ConnectionTableProps) *ConnectionTable {
	// Set defaults
	if props == nil {
		props = &ConnectionTableProps{}
	}

	// Set connection table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("websocket-connections")
	}
	
	// Default partition key for connections - the connection ID
	if props.PartitionKey == nil {
		props.PartitionKey = &awsdynamodb.Attribute{
			Name: jsii.String("connection_id"),
			Type: awsdynamodb.AttributeType_STRING,
		}
	}
	
	// Enable TTL for connection cleanup
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ttl")
	}
	
	// Enable point-in-time recovery by default for connections
	if props.PointInTimeRecovery == nil {
		props.PointInTimeRecovery = jsii.Bool(true)
	}

	// Create the base DynamORM table with the embedded props
	dynamormTable := NewDynamORMTable(scope, id, &props.DynamORMTableProps)
	
	table := &ConnectionTable{
		DynamORMTable: dynamormTable,
	}
	
	// Add user index if enabled
	enableUserIndex := true
	if props.EnableUserIndex != nil {
		enableUserIndex = *props.EnableUserIndex
	}
	
	if enableUserIndex {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("user-index"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("user_id"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("created_at"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}
	
	// Add tenant index if multi-tenant is enabled
	enableTenantIndex := false
	if props.EnableTenantIndex != nil {
		enableTenantIndex = *props.EnableTenantIndex
	}
	
	if enableTenantIndex || (props.EnableMultiTenant != nil && *props.EnableMultiTenant) {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("tenant-index"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("tenant_id"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("created_at"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}
	
	return table
}

// GrantConnectionManagement grants permissions to manage WebSocket connections
func (c *ConnectionTable) GrantConnectionManagement(grantee awsiam.IGrantable) {
	// Grant read/write permissions for connection management
	c.GrantReadWrite(grantee)
}

// GetUserIndexName returns the name of the user index
func (c *ConnectionTable) GetUserIndexName() *string {
	return jsii.String("user-index")
}

// GetTenantIndexName returns the name of the tenant index
func (c *ConnectionTable) GetTenantIndexName() *string {
	return jsii.String("tenant-index")
}