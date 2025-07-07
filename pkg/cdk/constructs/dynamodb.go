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
	// Enable multi-tenant partitioning
	EnableMultiTenant *bool
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

	// Define partition key
	partitionKey := &awsdynamodb.Attribute{
		Name: jsii.String("pk"),
		Type: awsdynamodb.AttributeType_STRING,
	}

	// Define sort key
	sortKey := &awsdynamodb.Attribute{
		Name: jsii.String("sk"),
		Type: awsdynamodb.AttributeType_STRING,
	}

	// Multi-tenant tables use standard 'pk' naming for DynamORM compatibility
	// Tenant isolation is achieved through key values, not attribute names

	// Create table properties
	tableProps := &awsdynamodb.TableProps{
		TableName:    props.TableName,
		PartitionKey: partitionKey,
		SortKey:      sortKey,
		BillingMode:  billingMode,
		RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
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
		tableProps.PointInTimeRecovery = jsii.Bool(true)
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

	// Create the table
	table := awsdynamodb.NewTable(this, jsii.String("Table"), tableProps)

	// Add global secondary index for queries by type
	table.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("gsi1"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("gsi1pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("gsi1sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

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