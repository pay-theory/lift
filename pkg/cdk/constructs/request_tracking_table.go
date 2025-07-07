package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// RequestTrackingTableProps defines properties for the request tracking table
type RequestTrackingTableProps struct {
	DynamORMTableProps
	// Enable correlation index for querying by correlation ID
	EnableCorrelationIndex *bool
	// Enable status index for querying by request status
	EnableStatusIndex *bool
	// Enable user index for querying by user ID
	EnableUserIndex *bool
	// Enable timestamp index for time-based queries
	EnableTimestampIndex *bool
}

// RequestTrackingTable is a DynamORM table for tracking API requests and their async processing
type RequestTrackingTable struct {
	*DynamORMTable
}

// NewRequestTrackingTable creates a new request tracking table using DynamORM
func NewRequestTrackingTable(scope constructs.Construct, id *string, props *RequestTrackingTableProps) *RequestTrackingTable {
	// Set defaults
	if props == nil {
		props = &RequestTrackingTableProps{}
	}

	// Set request tracking table specific defaults
	if props.TableName == nil {
		props.TableName = jsii.String("request-tracking")
	}
	
	// Default partition key for request tracking
	if props.PartitionKey == nil {
		props.PartitionKey = &awsdynamodb.Attribute{
			Name: jsii.String("request_id"),
			Type: awsdynamodb.AttributeType_STRING,
		}
	}
	
	// Enable TTL for request cleanup
	if props.TimeToLiveAttribute == nil {
		props.TimeToLiveAttribute = jsii.String("ttl")
	}
	
	// Enable point-in-time recovery by default for request tracking
	if props.PointInTimeRecovery == nil {
		props.PointInTimeRecovery = jsii.Bool(true)
	}

	// Create the base DynamORM table
	dynamormTable := NewDynamORMTable(scope, id, &props.DynamORMTableProps)
	
	table := &RequestTrackingTable{
		DynamORMTable: dynamormTable,
	}
	
	// Add correlation index if enabled
	enableCorrelationIndex := true
	if props.EnableCorrelationIndex != nil {
		enableCorrelationIndex = *props.EnableCorrelationIndex
	}
	
	if enableCorrelationIndex {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("correlation-index"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("correlation_id"),
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
				Name: jsii.String("status"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("timestamp"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
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
				Name: jsii.String("timestamp"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}
	
	// Add timestamp index if enabled
	enableTimestampIndex := true
	if props.EnableTimestampIndex != nil {
		enableTimestampIndex = *props.EnableTimestampIndex
	}
	
	if enableTimestampIndex {
		table.AddGSI(&GSIProps{
			IndexName: jsii.String("timestamp-index"),
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
	
	// Add DynamORM-specific permissions
	// Note: These will be granted when Lambda functions need access
	
	return table
}

// GetCorrelationIndexName returns the name of the correlation index
func (r *RequestTrackingTable) GetCorrelationIndexName() *string {
	return jsii.String("correlation-index")
}

// GetStatusIndexName returns the name of the status index
func (r *RequestTrackingTable) GetStatusIndexName() *string {
	return jsii.String("status-index")
}

// GetUserIndexName returns the name of the user index
func (r *RequestTrackingTable) GetUserIndexName() *string {
	return jsii.String("user-index")
}

// GetTimestampIndexName returns the name of the timestamp index
func (r *RequestTrackingTable) GetTimestampIndexName() *string {
	return jsii.String("timestamp-index")
}