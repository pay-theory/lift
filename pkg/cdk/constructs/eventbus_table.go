package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"github.com/pay-theory/lift/pkg/naming"
)

// EventBusTableProps defines properties for the EventBus DynamoDB table
type EventBusTableProps struct {
	// Pointer fields (8 bytes each)
	TableName                 *string             // DynamoDB table name - MUST be unique
	ReadCapacity              *float64            // Provisioned read capacity
	WriteCapacity             *float64            // Provisioned write capacity
	EnablePointInTimeRecovery *bool               // Enable automated backups
	TimeToLiveAttribute       *string             // TTL attribute name (default: "ttl")
	EnableStream              *bool               // Enable DynamoDB Streams
	EnableEventIDIndex        *bool               // Add GSI for event ID lookups
	Tags                      *map[string]*string // Resource tags
	EncryptionKey             awskms.IKey         // KMS encryption key (optional)

	// Value types
	BillingMode    awsdynamodb.BillingMode    // Billing mode (default: PAY_PER_REQUEST)
	RemovalPolicy  awscdk.RemovalPolicy       // Removal policy for stack deletion
	StreamViewType awsdynamodb.StreamViewType // Stream data type
}

// EventBusTable represents a DynamoDB table for the EventBus
type EventBusTable struct {
	constructs.Construct

	// Table is the DynamoDB table
	Table awsdynamodb.Table

	// EventIDIndex is the GSI for querying by event ID (if enabled)
	EventIDIndex awsdynamodb.GlobalSecondaryIndexProps

	// StreamArn is the DynamoDB Stream ARN (if enabled)
	StreamArn *string
}

// NewEventBusTable creates a new EventBus DynamoDB table construct
// nolint:gocyclo // complexity is acceptable for a builder function
func NewEventBusTable(scope constructs.Construct, id *string, props *EventBusTableProps) *EventBusTable {
	construct := constructs.NewConstruct(scope, id)

	props = defaultEventBusTableProps(props)

	tableName := resolveEventBusTableName(construct, id, props.TableName)
	billingMode := resolveEventBusBillingMode(props.BillingMode)
	removalPolicy := resolveEventBusRemovalPolicy(props.RemovalPolicy)
	ttlAttribute := resolveEventBusTTLAttribute(props.TimeToLiveAttribute)
	enablePITR := resolveEventBusEnablePITR(props.EnablePointInTimeRecovery)
	enableStream := props.EnableStream != nil && *props.EnableStream
	streamViewType := resolveEventBusStreamViewType(props.StreamViewType, enableStream)

	table := awsdynamodb.NewTable(construct, jsii.String("Table"), buildEventBusTableProps(tableName, billingMode, removalPolicy, ttlAttribute, enablePITR, enableStream, streamViewType, props))

	eventIDIndex := addEventBusEventIDIndexIfEnabled(table, billingMode, props.EnableEventIDIndex)
	addEventBusTenantTimestampIndex(table, billingMode)
	applyEventBusTableTags(table, props)

	streamArn := resolveEventBusStreamArn(table, enableStream)

	eventBusTable := &EventBusTable{
		Construct: construct,
		Table:     table,
		StreamArn: streamArn,
	}
	if eventIDIndex != nil {
		eventBusTable.EventIDIndex = *eventIDIndex
	}

	addEventBusTableOutputs(construct, table, tableName, streamArn)

	return eventBusTable
}

func defaultEventBusTableProps(props *EventBusTableProps) *EventBusTableProps {
	if props == nil {
		return &EventBusTableProps{}
	}
	return props
}

func resolveEventBusTableName(scope constructs.Construct, id *string, tableName *string) *string {
	if tableName != nil {
		return tableName
	}

	// Prefer deterministic names when APP_NAME/STAGE[/PARTNER] are available.
	if resolved, ok := naming.ResourceNameFromEnv("events"); ok {
		return jsii.String(resolved)
	}

	// IMPORTANT: TableName is effectively required to avoid conflicts
	// between multiple applications in the same AWS account.
	// If not provided, we'll use the construct ID with stack name prefix
	// to generate a unique name, but explicit naming is recommended.
	stack := awscdk.Stack_Of(scope)
	stackName := *stack.StackName()
	return jsii.String(stackName + "-" + *id)
}

func resolveEventBusBillingMode(billingMode awsdynamodb.BillingMode) awsdynamodb.BillingMode {
	if billingMode == "" {
		return awsdynamodb.BillingMode_PAY_PER_REQUEST
	}
	return billingMode
}

func resolveEventBusRemovalPolicy(removalPolicy awscdk.RemovalPolicy) awscdk.RemovalPolicy {
	if removalPolicy == "" {
		return awscdk.RemovalPolicy_RETAIN
	}
	return removalPolicy
}

func resolveEventBusTTLAttribute(ttlAttribute *string) *string {
	if ttlAttribute == nil {
		return jsii.String("ttl")
	}
	return ttlAttribute
}

func resolveEventBusEnablePITR(enablePITR *bool) *bool {
	if enablePITR == nil {
		return jsii.Bool(true)
	}
	return enablePITR
}

func resolveEventBusStreamViewType(streamViewType awsdynamodb.StreamViewType, enableStream bool) awsdynamodb.StreamViewType {
	if streamViewType == "" && enableStream {
		return awsdynamodb.StreamViewType_NEW_IMAGE
	}
	return streamViewType
}

func buildEventBusTableProps(
	tableName *string,
	billingMode awsdynamodb.BillingMode,
	removalPolicy awscdk.RemovalPolicy,
	ttlAttribute *string,
	enablePITR *bool,
	enableStream bool,
	streamViewType awsdynamodb.StreamViewType,
	props *EventBusTableProps,
) *awsdynamodb.TableProps {
	tableProps := &awsdynamodb.TableProps{
		TableName:   tableName,
		BillingMode: billingMode,
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("sk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		TimeToLiveAttribute: ttlAttribute,
		RemovalPolicy:       removalPolicy,
		PointInTimeRecovery: enablePITR,
		Encryption:          awsdynamodb.TableEncryption_AWS_MANAGED,
	}

	// Add provisioned capacity if specified.
	if billingMode == awsdynamodb.BillingMode_PROVISIONED {
		tableProps.ReadCapacity = props.ReadCapacity
		if tableProps.ReadCapacity == nil {
			tableProps.ReadCapacity = jsii.Number(5)
		}
		tableProps.WriteCapacity = props.WriteCapacity
		if tableProps.WriteCapacity == nil {
			tableProps.WriteCapacity = jsii.Number(5)
		}
	}

	// Enable streams if requested.
	if enableStream {
		tableProps.Stream = streamViewType
	}

	return tableProps
}

func addEventBusEventIDIndexIfEnabled(table awsdynamodb.Table, billingMode awsdynamodb.BillingMode, enabled *bool) *awsdynamodb.GlobalSecondaryIndexProps {
	if enabled == nil || !*enabled {
		return nil
	}

	indexProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("event-id-index"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}

	if billingMode == awsdynamodb.BillingMode_PROVISIONED {
		indexProps.ReadCapacity = jsii.Number(5)
		indexProps.WriteCapacity = jsii.Number(5)
	}

	table.AddGlobalSecondaryIndex(indexProps)
	return indexProps
}

func addEventBusTenantTimestampIndex(table awsdynamodb.Table, billingMode awsdynamodb.BillingMode) {
	tenantIndexProps := &awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("tenant-timestamp-index"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("tenant_id"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("published_at"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	}

	if billingMode == awsdynamodb.BillingMode_PROVISIONED {
		tenantIndexProps.ReadCapacity = jsii.Number(5)
		tenantIndexProps.WriteCapacity = jsii.Number(5)
	}

	table.AddGlobalSecondaryIndex(tenantIndexProps)
}

func applyEventBusTableTags(table awsdynamodb.Table, props *EventBusTableProps) {
	if props != nil && props.Tags != nil {
		for key, value := range *props.Tags {
			awscdk.Tags_Of(table).Add(jsii.String(key), value, nil)
		}
	}

	awscdk.Tags_Of(table).Add(jsii.String("ManagedBy"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(table).Add(jsii.String("Purpose"), jsii.String("EventBus"), nil)
}

func resolveEventBusStreamArn(table awsdynamodb.Table, enableStream bool) *string {
	if !enableStream {
		return nil
	}
	return table.TableStreamArn()
}

func addEventBusTableOutputs(scope constructs.Construct, table awsdynamodb.Table, tableName *string, streamArn *string) {
	awscdk.NewCfnOutput(scope, jsii.String("EventBusTableName"), &awscdk.CfnOutputProps{
		Value:       table.TableName(),
		Description: jsii.String("EventBus DynamoDB table name"),
		ExportName:  jsii.String(*tableName + "-name"),
	})

	awscdk.NewCfnOutput(scope, jsii.String("EventBusTableArn"), &awscdk.CfnOutputProps{
		Value:       table.TableArn(),
		Description: jsii.String("EventBus DynamoDB table ARN"),
		ExportName:  jsii.String(*tableName + "-arn"),
	})

	if streamArn != nil {
		awscdk.NewCfnOutput(scope, jsii.String("EventBusStreamArn"), &awscdk.CfnOutputProps{
			Value:       streamArn,
			Description: jsii.String("EventBus DynamoDB stream ARN"),
			ExportName:  jsii.String(*tableName + "-stream-arn"),
		})
	}
}

// GrantReadWrite grants read and write permissions to a Lambda function
func (e *EventBusTable) GrantReadWrite(function awslambda.IFunction) {
	e.Table.GrantReadWriteData(function)
}

// GrantRead grants read-only permissions to a Lambda function
func (e *EventBusTable) GrantRead(function awslambda.IFunction) {
	e.Table.GrantReadData(function)
}

// GrantWrite grants write-only permissions to a Lambda function
func (e *EventBusTable) GrantWrite(function awslambda.IFunction) {
	e.Table.GrantWriteData(function)
}

// GrantStreamRead grants permissions to read from the DynamoDB stream
func (e *EventBusTable) GrantStreamRead(function awslambda.IFunction) {
	if e.StreamArn == nil {
		return
	}

	function.GrantPrincipal().AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: jsii.Strings(
			"dynamodb:GetRecords",
			"dynamodb:GetShardIterator",
			"dynamodb:DescribeStream",
			"dynamodb:ListStreams",
		),
		Resources: jsii.Strings(*e.StreamArn),
	}))
}

// GetTableName returns the table name
func (e *EventBusTable) GetTableName() *string {
	return e.Table.TableName()
}

// GetTableArn returns the table ARN
func (e *EventBusTable) GetTableArn() *string {
	return e.Table.TableArn()
}

// GetStreamArn returns the stream ARN (if enabled)
func (e *EventBusTable) GetStreamArn() *string {
	return e.StreamArn
}

// GetEnvironmentVariables returns EventBus environment variables as a map
func (e *EventBusTable) GetEnvironmentVariables() *map[string]*string {
	env := map[string]*string{
		"EVENT_BUS_TABLE_NAME": e.Table.TableName(),
	}
	if e.StreamArn != nil {
		env["EVENT_BUS_STREAM_ARN"] = e.StreamArn
	}
	return &env
}
