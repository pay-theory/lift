package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// BaseManagementTableProps defines common properties for management tables
type BaseManagementTableProps struct {
	// Table name
	TableName *string
	// TTL attribute name for automatic cleanup
	TimeToLiveAttribute *string
	// Default table name if not provided
	DefaultTableName string
}

// createManagementTable creates a standard management table with common settings
func createManagementTable(scope constructs.Construct, id *string, props *BaseManagementTableProps) *LiftTable {
	// Set defaults
	if props == nil {
		props = &BaseManagementTableProps{}
	}

	// Set table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String(props.DefaultTableName)
	}

	// Enable TTL for cleanup
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ttl")
	}

	// Create the table with standard settings
	return NewLiftTable(scope, id, &LiftTableProps{
		TableName:                 props.TableName,
		PartitionKeyName:          jsii.String("PK"),
		SortKeyName:               jsii.String("SK"),
		TimeToLiveAttribute:       props.TimeToLiveAttribute,
		EnablePointInTimeRecovery: jsii.Bool(true),
		EnableStreams:             jsii.Bool(true),
	})
}

// grantManagementPermissions grants read/write permissions for management operations
func grantManagementPermissions(table *LiftTable, grantee awsiam.IGrantable) {
	table.Table.GrantReadWriteData(grantee)
}