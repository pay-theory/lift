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
type LiftTable struct {
	constructs.Construct
	Table awsdynamodb.Table
}

// NewLiftTable creates a new DynamoDB table with Lift-optimized defaults
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
