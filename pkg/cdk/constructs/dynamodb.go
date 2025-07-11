package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// LiftTableProps extends DynamoDB table properties with Lift-specific configuration
type LiftTableProps struct {
	// Table name
	TableName *string
	// Partition key attribute name (defaults to field name from DynamORM model)
	PartitionKeyName *string
	// Sort key attribute name (optional, defaults to field name from DynamORM model)
	SortKeyName *string
	// Enable point-in-time recovery
	EnablePointInTimeRecovery *bool
	// Enable DynamoDB Streams
	EnableStreams *bool
	// Stream view type
	StreamViewType awsdynamodb.StreamViewType
	// Time to live attribute name
	TimeToLiveAttribute *string
	// Enable auto-scaling
	EnableAutoScaling *bool
	// Read capacity (for provisioned mode)
	ReadCapacity *float64
	// Write capacity (for provisioned mode)
	WriteCapacity *float64
}

// LiftTable is a DynamoDB table construct optimized for Lift applications
type LiftTable struct {
	constructs.Construct
	Table awsdynamodb.Table
}

// NewLiftTable creates a new DynamoDB table with Lift-optimized defaults
func NewLiftTable(scope constructs.Construct, id *string, props *LiftTableProps) *LiftTable {
	this := constructs.NewConstruct(scope, id)

	// Default to on-demand billing
	billingMode := awsdynamodb.BillingMode_PAY_PER_REQUEST

	// If capacity is specified, use provisioned mode
	if props.ReadCapacity != nil || props.WriteCapacity != nil {
		billingMode = awsdynamodb.BillingMode_PROVISIONED
	}

	// Define partition key - DynamORM expects field names as attribute names
	partitionKeyName := props.PartitionKeyName
	if partitionKeyName == nil {
		// This is incorrect - we should require the key name or detect it from the model
		// For now, we'll require it to be specified
		panic("PartitionKeyName is required in LiftTableProps to match DynamORM model field name")
	}

	partitionKey := &awsdynamodb.Attribute{
		Name: partitionKeyName,
		Type: awsdynamodb.AttributeType_STRING,
	}

	// Define sort key if provided
	var sortKey *awsdynamodb.Attribute
	if props.SortKeyName != nil {
		sortKey = &awsdynamodb.Attribute{
			Name: props.SortKeyName,
			Type: awsdynamodb.AttributeType_STRING,
		}
	}

	// Create table properties matching DynamORM model field names
	tableProps := &awsdynamodb.TableProps{
		TableName:     props.TableName,
		PartitionKey:  partitionKey,
		BillingMode:   billingMode,
		RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
	}

	// Only add sort key if provided
	if sortKey != nil {
		tableProps.SortKey = sortKey
	}

	// Configure capacity for provisioned mode
	if billingMode == awsdynamodb.BillingMode_PROVISIONED {
		if props.ReadCapacity != nil {
			tableProps.ReadCapacity = props.ReadCapacity
		} else {
			tableProps.ReadCapacity = jsii.Number(5)
		}
		if props.WriteCapacity != nil {
			tableProps.WriteCapacity = props.WriteCapacity
		} else {
			tableProps.WriteCapacity = jsii.Number(5)
		}
	}

	// Enable point-in-time recovery
	if props.EnablePointInTimeRecovery != nil && *props.EnablePointInTimeRecovery {
		tableProps.PointInTimeRecoverySpecification = &awsdynamodb.PointInTimeRecoverySpecification{
			PointInTimeRecoveryEnabled: jsii.Bool(true),
		}
	}

	// Enable streams
	if props.EnableStreams != nil && *props.EnableStreams {
		if props.StreamViewType != "" {
			tableProps.Stream = props.StreamViewType
		} else {
			tableProps.Stream = awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES
		}
	}

	// Set TTL attribute
	if props.TimeToLiveAttribute != nil {
		tableProps.TimeToLiveAttribute = props.TimeToLiveAttribute
	}

	// Create the table with only pk/sk - GSIs must be added separately using table.AddGlobalSecondaryIndex()
	table := awsdynamodb.NewTable(this, jsii.String("Table"), tableProps)

	// Configure auto-scaling for provisioned mode
	if billingMode == awsdynamodb.BillingMode_PROVISIONED && props.EnableAutoScaling != nil && *props.EnableAutoScaling {
		readScaling := table.AutoScaleReadCapacity(&awsdynamodb.EnableScalingProps{
			MinCapacity: jsii.Number(5),
			MaxCapacity: jsii.Number(1000),
		})
		readScaling.ScaleOnUtilization(&awsdynamodb.UtilizationScalingProps{
			TargetUtilizationPercent: jsii.Number(70),
		})

		writeScaling := table.AutoScaleWriteCapacity(&awsdynamodb.EnableScalingProps{
			MinCapacity: jsii.Number(5),
			MaxCapacity: jsii.Number(1000),
		})
		writeScaling.ScaleOnUtilization(&awsdynamodb.UtilizationScalingProps{
			TargetUtilizationPercent: jsii.Number(70),
		})
	}

	return &LiftTable{
		Construct: this,
		Table:     table,
	}
}

// GrantReadWrite grants read/write permissions to a Lambda function
func (t *LiftTable) GrantReadWrite(fn awslambda.IFunction) {
	t.Table.GrantReadWriteData(fn)
}

// GetTableName returns the table name
func (t *LiftTable) GetTableName() *string {
	return t.Table.TableName()
}

// GetTableArn returns the table ARN
func (t *LiftTable) GetTableArn() *string {
	return t.Table.TableArn()
}

// GetResourceName returns the resource name for monitoring (implements MonitorableResource interface)
func (t *LiftTable) GetResourceName() *string {
	return t.Table.TableName()
}

// GetStreamArn returns the DynamoDB stream ARN if streams are enabled
func (t *LiftTable) GetStreamArn() *string {
	return t.Table.TableStreamArn()
}
