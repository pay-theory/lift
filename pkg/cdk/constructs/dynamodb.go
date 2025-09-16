package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// LiftTableProps extends DynamoDB table properties with Lift-specific configuration
//
// This struct contains all configurable properties for creating a Lift-optimized
// DynamoDB table. The properties include basic table configuration, advanced
// features like point-in-time recovery, streams, auto-scaling, and TTL settings.
type LiftTableProps struct {
	TableName                 *string
	PartitionKeyName          *string
	SortKeyName               *string
	EnablePointInTimeRecovery *bool
	EnableStreams             *bool
	TimeToLiveAttribute       *string
	EnableAutoScaling         *bool
	ReadCapacity              *float64
	WriteCapacity             *float64
	StreamViewType            awsdynamodb.StreamViewType
}

// LiftTable is a DynamoDB table construct optimized for Lift applications
//
// This construct creates a DynamoDB table with Lift-optimized defaults including:
// - Point-in-time recovery (if enabled)
// - DynamoDB streams (if enabled)
// - Auto-scaling (if enabled)
// - TTL (if configured)
//
// The table is configured with sensible defaults for production workloads.
type LiftTable struct {
	constructs.Construct
	Table awsdynamodb.Table
}

// NewLiftTable creates a new DynamoDB table with Lift-optimized defaults
//
// This function creates a new DynamoDB table with all Lift-optimized features including:
// - Appropriate billing mode (provisioned or pay-per-request)
// - Point-in-time recovery (if enabled)
// - DynamoDB streams (if enabled)
// - Auto-scaling (if enabled)
// - TTL (if configured)
//
// Parameters:
//   - scope: The CDK construct scope
//   - id: The construct ID
//   - props: Configuration properties
//
// Returns:
//   - A new LiftTable instance
func NewLiftTable(scope constructs.Construct, id *string, props *LiftTableProps) *LiftTable {
	builder := newLiftTableBuilder(scope, id, props)
	return builder.build()
}

// liftTableBuilder builds DynamoDB tables optimized for DynamORM
type liftTableBuilder struct {
	scope       constructs.Construct
	id          *string
	props       *LiftTableProps
	billingMode awsdynamodb.BillingMode
}

// newLiftTableBuilder creates a new lift table builder
func newLiftTableBuilder(scope constructs.Construct, id *string, props *LiftTableProps) *liftTableBuilder {
	return &liftTableBuilder{
		scope: scope,
		id:    id,
		props: props,
	}
}

// build constructs the complete LiftTable
func (b *liftTableBuilder) build() *LiftTable {
	this := constructs.NewConstruct(b.scope, b.id)

	b.determineBillingMode()
	tableProps := b.createTableProps()
	table := b.createTable(this, tableProps)
	b.configureAutoScaling(table)

	return &LiftTable{
		Construct: this,
		Table:     table,
	}
}

// determineBillingMode determines the appropriate billing mode
func (b *liftTableBuilder) determineBillingMode() {
	if b.props.ReadCapacity != nil || b.props.WriteCapacity != nil {
		b.billingMode = awsdynamodb.BillingMode_PROVISIONED
	} else {
		b.billingMode = awsdynamodb.BillingMode_PAY_PER_REQUEST
	}
}

// createTableProps creates the base table properties
func (b *liftTableBuilder) createTableProps() *awsdynamodb.TableProps {
	tableProps := &awsdynamodb.TableProps{
		TableName:     b.props.TableName,
		PartitionKey:  b.createPartitionKey(),
		BillingMode:   b.billingMode,
		RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
	}

	b.configureSortKey(tableProps)
	b.configureCapacity(tableProps)

	return tableProps
}

// createPartitionKey creates the partition key attribute
func (b *liftTableBuilder) createPartitionKey() *awsdynamodb.Attribute {
	if b.props.PartitionKeyName == nil {
		panic("PartitionKeyName is required in LiftTableProps to match DynamORM model field name")
	}

	return &awsdynamodb.Attribute{
		Name: b.props.PartitionKeyName,
		Type: awsdynamodb.AttributeType_STRING,
	}
}

// configureSortKey configures the sort key if provided
func (b *liftTableBuilder) configureSortKey(tableProps *awsdynamodb.TableProps) {
	if b.props.SortKeyName != nil {
		tableProps.SortKey = &awsdynamodb.Attribute{
			Name: b.props.SortKeyName,
			Type: awsdynamodb.AttributeType_STRING,
		}
	}
}

// configureCapacity configures capacity for provisioned billing
func (b *liftTableBuilder) configureCapacity(tableProps *awsdynamodb.TableProps) {
	if b.billingMode != awsdynamodb.BillingMode_PROVISIONED {
		return
	}

	if b.props.ReadCapacity != nil {
		tableProps.ReadCapacity = b.props.ReadCapacity
	} else {
		tableProps.ReadCapacity = jsii.Number(5)
	}

	if b.props.WriteCapacity != nil {
		tableProps.WriteCapacity = b.props.WriteCapacity
	} else {
		tableProps.WriteCapacity = jsii.Number(5)
	}
}

// createTable creates the DynamoDB table with advanced features
func (b *liftTableBuilder) createTable(construct constructs.Construct, tableProps *awsdynamodb.TableProps) awsdynamodb.Table {
	b.configureAdvancedFeatures(tableProps)
	return awsdynamodb.NewTable(construct, jsii.String("Table"), tableProps)
}

// configureAdvancedFeatures configures advanced DynamoDB features
func (b *liftTableBuilder) configureAdvancedFeatures(tableProps *awsdynamodb.TableProps) {
	if b.props.EnablePointInTimeRecovery != nil && *b.props.EnablePointInTimeRecovery {
		tableProps.PointInTimeRecoverySpecification = &awsdynamodb.PointInTimeRecoverySpecification{
			PointInTimeRecoveryEnabled: jsii.Bool(true),
		}
	}

	if b.props.EnableStreams != nil && *b.props.EnableStreams {
		if b.props.StreamViewType != "" {
			tableProps.Stream = b.props.StreamViewType
		} else {
			tableProps.Stream = awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES
		}
	}

	if b.props.TimeToLiveAttribute != nil {
		tableProps.TimeToLiveAttribute = b.props.TimeToLiveAttribute
	}
}

// configureAutoScaling configures auto-scaling for provisioned billing
func (b *liftTableBuilder) configureAutoScaling(table awsdynamodb.Table) {
	if b.billingMode != awsdynamodb.BillingMode_PROVISIONED {
		return
	}

	if b.props.EnableAutoScaling == nil || !*b.props.EnableAutoScaling {
		return
	}

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

// GrantReadWrite grants read/write permissions to a Lambda function
//
// This method grants the specified Lambda function read and write permissions
// to the DynamoDB table. This is typically used to allow Lambda functions to
// perform CRUD operations on the table.
//
// Parameters:
//   - fn: The Lambda function to grant permissions to
func (t *LiftTable) GrantReadWrite(fn awslambda.IFunction) {
	t.Table.GrantReadWriteData(fn)
}

// GetTableName returns the table name
//
// This method returns the name of the DynamoDB table. This is useful for
// configuration and when setting up environment variables for applications
// that need to access the table.
//
// Returns:
//   - The table name
func (t *LiftTable) GetTableName() *string {
	return t.Table.TableName()
}

// GetTableArn returns the table ARN
//
// This method returns the ARN (Amazon Resource Name) of the DynamoDB table.
// This is useful for cross-service integrations and IAM permissions.
//
// Returns:
//   - The table ARN
func (t *LiftTable) GetTableArn() *string {
	return t.Table.TableArn()
}

// GetResourceName returns the resource name for monitoring
//
// This method returns the resource name for monitoring purposes. It implements
// the MonitorableResource interface.
//
// Returns:
//   - The resource name (table name)
func (t *LiftTable) GetResourceName() *string {
	return t.Table.TableName()
}

// GetStreamArn returns the DynamoDB stream ARN if streams are enabled
//
// This method returns the ARN of the DynamoDB stream if streams are enabled
// on the table. This is useful for setting up event-driven architectures.
//
// Returns:
//   - The stream ARN, or nil if streams are not enabled
func (t *LiftTable) GetStreamArn() *string {
	return t.Table.TableStreamArn()
}
