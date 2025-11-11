package patterns

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

// EventBusPatternProps defines properties for the EventBus pattern
type EventBusPatternProps struct {
	// Pointer fields (8 bytes each)
	TableName                 *string             // DynamoDB table name (default: "{AppName}-events")
	EnableStream              *bool               // Enable DynamoDB Streams
	ProcessorCodePath         *string             // Path to stream processor Lambda code
	ProcessorHandler          *string             // Lambda handler name (default: "bootstrap")
	ProcessorMemory           *float64            // Lambda memory in MB (default: 256)
	ProcessorEnvironment      *map[string]*string // Additional processor environment variables
	BatchSize                 *float64            // Records per batch (default: 10)
	EnablePointInTimeRecovery *bool               // Enable automated backups
	TTLInDays                 *float64            // Event retention in days (default: 30)
	EnableEventIDIndex        *bool               // Add GSI for event ID lookups
	Tags                      *map[string]*string // Resource tags

	// Value types
	ProcessorTimeout awscdk.Duration         // Lambda timeout (default: 30s)
	BillingMode      awsdynamodb.BillingMode // DynamoDB billing mode

	// String field
	AppName string // Application name prefix (REQUIRED)
}

// EventBusPattern represents a complete EventBus deployment
type EventBusPattern struct {
	constructs.Construct

	// Table is the EventBus DynamoDB table
	Table *liftconstructs.EventBusTable

	// Processor is the stream processor Lambda (if enabled)
	Processor *liftconstructs.LiftFunction

	// StreamArn is the DynamoDB stream ARN
	StreamArn *string
}

// NewEventBusPattern creates a complete EventBus deployment with table and processor
func NewEventBusPattern(scope constructs.Construct, id *string, props *EventBusPatternProps) *EventBusPattern {
	construct := constructs.NewConstruct(scope, id)

	if props == nil {
		props = &EventBusPatternProps{}
	}

	// Validate required fields
	if props.AppName == "" {
		panic("AppName is required and must be unique within your AWS account")
	}

	// Apply defaults
	tableName := props.TableName
	if tableName == nil {
		tableName = jsii.String(props.AppName + "-events")
	}

	enableStream := props.EnableStream
	if enableStream == nil {
		// Enable stream if processor code is provided
		enableStream = jsii.Bool(props.ProcessorCodePath != nil)
	}

	billingMode := props.BillingMode
	if billingMode == "" {
		billingMode = awsdynamodb.BillingMode_PAY_PER_REQUEST
	}

	enablePITR := props.EnablePointInTimeRecovery
	if enablePITR == nil {
		enablePITR = jsii.Bool(true)
	}

	enableEventIDIndex := props.EnableEventIDIndex
	if enableEventIDIndex == nil {
		enableEventIDIndex = jsii.Bool(true)
	}

	// Create EventBus table
	eventBusTable := liftconstructs.NewEventBusTable(construct, jsii.String("EventBusTable"), &liftconstructs.EventBusTableProps{
		TableName:                 tableName,
		BillingMode:               billingMode,
		EnablePointInTimeRecovery: enablePITR,
		EnableStream:              enableStream,
		StreamViewType:            awsdynamodb.StreamViewType_NEW_IMAGE,
		EnableEventIDIndex:        enableEventIDIndex,
		Tags:                      props.Tags,
	})

	pattern := &EventBusPattern{
		Construct: construct,
		Table:     eventBusTable,
		StreamArn: eventBusTable.StreamArn,
	}

	// Create stream processor if code path is provided
	if props.ProcessorCodePath != nil && *enableStream {
		pattern.Processor = pattern.createStreamProcessor(props, eventBusTable)
	}

	// Apply tags
	if props.Tags != nil {
		for key, value := range *props.Tags {
			awscdk.Tags_Of(construct).Add(jsii.String(key), value, nil)
		}
	}

	return pattern
}

// createStreamProcessor creates a Lambda function to process DynamoDB stream events
func (p *EventBusPattern) createStreamProcessor(props *EventBusPatternProps, table *liftconstructs.EventBusTable) *liftconstructs.LiftFunction {
	// Apply defaults
	handler := props.ProcessorHandler
	if handler == nil {
		handler = jsii.String("bootstrap")
	}

	memory := props.ProcessorMemory
	if memory == nil {
		memory = jsii.Number(256)
	}

	timeout := props.ProcessorTimeout
	if timeout == nil {
		timeout = awscdk.Duration_Seconds(jsii.Number(30))
	}

	batchSize := props.BatchSize
	if batchSize == nil {
		batchSize = jsii.Number(10)
	}

	// Build environment variables
	environment := map[string]*string{
		"EVENT_BUS_TABLE_NAME": table.GetTableName(),
		"APP_NAME":             jsii.String(props.AppName),
	}

	if props.ProcessorEnvironment != nil {
		for k, v := range *props.ProcessorEnvironment {
			environment[k] = v
		}
	}

	// Create processor function
	processor := liftconstructs.NewLiftFunction(p.Construct, jsii.String("EventProcessor"), &liftconstructs.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String(props.AppName + "-event-processor"),
			Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
			Code:         awslambda.Code_FromAsset(props.ProcessorCodePath, nil),
			Handler:      handler,
			MemorySize:   memory,
			Timeout:      timeout,
			Environment:  &environment,
		},
		EnableTracing: jsii.Bool(true),
	})

	// Grant permissions
	table.GrantReadWrite(processor.Function)
	table.GrantStreamRead(processor.Function)

	// Add DynamoDB stream as event source
	processor.Function.AddEventSource(awslambdaeventsources.NewDynamoEventSource(table.Table, &awslambdaeventsources.DynamoEventSourceProps{
		StartingPosition:      awslambda.StartingPosition_LATEST,
		BatchSize:             batchSize,
		BisectBatchOnError:    jsii.Bool(true),
		RetryAttempts:         jsii.Number(3),
		MaxRecordAge:          awscdk.Duration_Hours(jsii.Number(1)),
		ParallelizationFactor: jsii.Number(1),
	}))

	return processor
}

// GrantPublish grants permissions to publish events to the bus
func (p *EventBusPattern) GrantPublish(function awslambda.IFunction) {
	p.Table.GrantWrite(function)
}

// GrantQuery grants permissions to query events from the bus
func (p *EventBusPattern) GrantQuery(function awslambda.IFunction) {
	p.Table.GrantRead(function)
}

// GrantFullAccess grants full read/write permissions to the bus
func (p *EventBusPattern) GrantFullAccess(function awslambda.IFunction) {
	p.Table.GrantReadWrite(function)
}

// GetEnvironmentVariables returns environment variables for the EventBus
func (p *EventBusPattern) GetEnvironmentVariables() *map[string]*string {
	return p.Table.GetEnvironmentVariables()
}

// GetTableName returns the EventBus table name
func (p *EventBusPattern) GetTableName() *string {
	return p.Table.GetTableName()
}

// GetTableArn returns the EventBus table ARN
func (p *EventBusPattern) GetTableArn() *string {
	return p.Table.GetTableArn()
}

// GetStreamArn returns the DynamoDB stream ARN
func (p *EventBusPattern) GetStreamArn() *string {
	return p.StreamArn
}

// GetProcessorFunction returns the stream processor function (if enabled)
func (p *EventBusPattern) GetProcessorFunction() *liftconstructs.LiftFunction {
	return p.Processor
}
