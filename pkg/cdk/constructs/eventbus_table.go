package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
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

	// Apply defaults
	if props == nil {
		props = &EventBusTableProps{}
	}

	tableName := props.TableName
	if tableName == nil {
		// IMPORTANT: TableName is effectively required to avoid conflicts
		// between multiple applications in the same AWS account.
		// If not provided, we'll use the construct ID with stack name prefix
		// to generate a unique name, but explicit naming is recommended.
		stack := awscdk.Stack_Of(construct)
		stackName := *stack.StackName()
		tableName = jsii.String(stackName + "-" + *id)
	}

	billingMode := props.BillingMode
	if billingMode == "" {
		billingMode = awsdynamodb.BillingMode_PAY_PER_REQUEST
	}

	removalPolicy := props.RemovalPolicy
	if removalPolicy == "" {
		removalPolicy = awscdk.RemovalPolicy_RETAIN
	}

	ttlAttribute := props.TimeToLiveAttribute
	if ttlAttribute == nil {
		ttlAttribute = jsii.String("ttl")
	}

	enablePITR := props.EnablePointInTimeRecovery
	if enablePITR == nil {
		enablePITR = jsii.Bool(true)
	}

	streamViewType := props.StreamViewType
	if streamViewType == "" && (props.EnableStream != nil && *props.EnableStream) {
		streamViewType = awsdynamodb.StreamViewType_NEW_IMAGE
	}

	// Build table properties
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

	// Add provisioned capacity if specified
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

	// Enable streams if requested
	if props.EnableStream != nil && *props.EnableStream {
		tableProps.Stream = streamViewType
	}

	// Create table
	table := awsdynamodb.NewTable(construct, jsii.String("Table"), tableProps)

	// Add GSI for event ID lookups if requested
	var eventIDIndex *awsdynamodb.GlobalSecondaryIndexProps
	if props.EnableEventIDIndex != nil && *props.EnableEventIDIndex {
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
		eventIDIndex = indexProps
	}

	// Add tenant ID + timestamp GSI for efficient tenant queries
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

	// Apply tags if provided
	if props.Tags != nil {
		for key, value := range *props.Tags {
			awscdk.Tags_Of(table).Add(jsii.String(key), value, nil)
		}
	}

	// Add standard tags
	awscdk.Tags_Of(table).Add(jsii.String("ManagedBy"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(table).Add(jsii.String("Purpose"), jsii.String("EventBus"), nil)

	// Get stream ARN if enabled
	var streamArn *string
	if props.EnableStream != nil && *props.EnableStream {
		streamArn = table.TableStreamArn()
	}

	eventBusTable := &EventBusTable{
		Construct: construct,
		Table:     table,
		StreamArn: streamArn,
	}

	if eventIDIndex != nil {
		eventBusTable.EventIDIndex = *eventIDIndex
	}

	// Output useful values
	awscdk.NewCfnOutput(construct, jsii.String("EventBusTableName"), &awscdk.CfnOutputProps{
		Value:       table.TableName(),
		Description: jsii.String("EventBus DynamoDB table name"),
		ExportName:  jsii.String(*tableName + "-name"),
	})

	awscdk.NewCfnOutput(construct, jsii.String("EventBusTableArn"), &awscdk.CfnOutputProps{
		Value:       table.TableArn(),
		Description: jsii.String("EventBus DynamoDB table ARN"),
		ExportName:  jsii.String(*tableName + "-arn"),
	})

	if streamArn != nil {
		awscdk.NewCfnOutput(construct, jsii.String("EventBusStreamArn"), &awscdk.CfnOutputProps{
			Value:       streamArn,
			Description: jsii.String("EventBus DynamoDB stream ARN"),
			ExportName:  jsii.String(*tableName + "-stream-arn"),
		})
	}

	return eventBusTable
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
