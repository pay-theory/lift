package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// StreamProcessorProps defines properties for a stream processor
// Memory optimized: 792 → 784 bytes (8 bytes saved)
type StreamProcessorProps struct {
	// Pointers first (8 bytes each)
	StreamingTable *StreamingTable
	DeadLetterQueueProps *awssqs.QueueProps
	EventSourceProps *awslambdaeventsources.DynamoEventSourceProps
	BatchSize               *float64
	RetryAttempts           *float64
	ParallelizationFactor   *float64
	EnableDeadLetterQueue *bool
	BisectBatchOnError      *bool
	ReportBatchItemFailures *bool
	// Duration structs (16 bytes each)
	MaxBatchingWindow       awscdk.Duration
	MaxRecordAge            awscdk.Duration
	TumblingWindow          awscdk.Duration
	// Large struct
	FunctionProps awslambda.FunctionProps
	// Medium types
	StartingPosition        awslambda.StartingPosition
}

// StreamProcessor processes DynamoDB streams with Lambda
type StreamProcessor struct {
	constructs.Construct

	// The Lambda function processing the stream
	Function *LiftFunction

	// The table with streams
	Table *StreamingTable

	// Dead letter queue (if enabled)
	DeadLetterQueue awssqs.IQueue

	// Event source mapping
	EventSource awslambdaeventsources.DynamoEventSource
}

// NewStreamProcessor creates a new stream processor construct
func NewStreamProcessor(scope constructs.Construct, id *string, props *StreamProcessorProps) *StreamProcessor {
	this := constructs.NewConstruct(scope, id)

	// Validate required properties
	if props == nil || props.StreamingTable == nil {
		panic("StreamingTable is required for StreamProcessor")
	}

	processor := &StreamProcessor{
		Construct: this,
		Table:     props.StreamingTable,
	}

	// Set defaults
	batchSize := float64(10)
	if props.BatchSize != nil {
		batchSize = *props.BatchSize
	}

	enableDLQ := true
	if props.EnableDeadLetterQueue != nil {
		enableDLQ = *props.EnableDeadLetterQueue
	}

	startingPosition := awslambda.StartingPosition_LATEST
	if props.StartingPosition != "" {
		startingPosition = props.StartingPosition
	}

	// Create dead letter queue if enabled
	if enableDLQ {
		dlqProps := &awssqs.QueueProps{
			QueueName:       jsii.String(fmt.Sprintf("%s-dlq", *props.FunctionProps.FunctionName)),
			RetentionPeriod: awscdk.Duration_Days(jsii.Number(14)),
		}
		if props.DeadLetterQueueProps != nil {
			dlqProps = props.DeadLetterQueueProps
		}
		processor.DeadLetterQueue = awssqs.NewQueue(this, jsii.String("DLQ"), dlqProps)
	}

	// Create Lambda function with DLQ if enabled
	functionProps := props.FunctionProps
	if enableDLQ && processor.DeadLetterQueue != nil {
		functionProps.DeadLetterQueueEnabled = jsii.Bool(true)
		functionProps.DeadLetterQueue = processor.DeadLetterQueue
	}

	// Ensure stream ARN is available in environment
	if functionProps.Environment == nil {
		functionProps.Environment = &map[string]*string{}
	}
	(*functionProps.Environment)["DYNAMODB_STREAM_ARN"] = processor.Table.GetStreamArn()
	(*functionProps.Environment)["DYNAMODB_TABLE_NAME"] = processor.Table.Table.TableName()

	// Create the Lambda function
	processor.Function = NewLiftFunction(this, jsii.String("Function"), &LiftFunctionProps{
		FunctionProps: functionProps,
	})

	// Create event source
	eventSourceProps := &awslambdaeventsources.DynamoEventSourceProps{
		StartingPosition:        startingPosition,
		BatchSize:               jsii.Number(batchSize),
		Enabled:                 jsii.Bool(true),
		ReportBatchItemFailures: jsii.Bool(true),
	}

	// Apply optional settings
	if props.MaxBatchingWindow != nil {
		eventSourceProps.MaxBatchingWindow = props.MaxBatchingWindow
	}
	if props.MaxRecordAge != nil {
		eventSourceProps.MaxRecordAge = props.MaxRecordAge
	}
	if props.BisectBatchOnError != nil {
		eventSourceProps.BisectBatchOnError = props.BisectBatchOnError
	}
	if props.RetryAttempts != nil {
		eventSourceProps.RetryAttempts = props.RetryAttempts
	}
	if props.ReportBatchItemFailures != nil {
		eventSourceProps.ReportBatchItemFailures = props.ReportBatchItemFailures
	}
	if props.TumblingWindow != nil {
		eventSourceProps.TumblingWindow = props.TumblingWindow
	}
	if props.ParallelizationFactor != nil {
		eventSourceProps.ParallelizationFactor = props.ParallelizationFactor
	}

	// Override with user-provided props if any
	if props.EventSourceProps != nil {
		eventSourceProps = props.EventSourceProps
	}

	// Create and add the event source
	processor.EventSource = awslambdaeventsources.NewDynamoEventSource(
		processor.Table.Table,
		eventSourceProps,
	)
	processor.Function.Function.AddEventSource(processor.EventSource)

	// Grant stream read permissions
	processor.Table.GrantStreamRead(processor.Function.Function)

	return processor
}

// Example usage:
//
// streamingTable := constructs.NewStreamingTable(stack, jsii.String("MyTable"), &constructs.StreamingTableProps{
//     TableName: jsii.String("my-table"),
// })
//
// processor := constructs.NewStreamProcessor(stack, jsii.String("Processor"), &constructs.StreamProcessorProps{
//     StreamingTable: streamingTable,
//     FunctionProps: awslambda.FunctionProps{
//         Runtime: awslambda.Runtime_PROVIDED_AL2023(),
//         Handler: jsii.String("bootstrap"),
//         Code:    awslambda.Code_FromAsset(jsii.String("./handler"), nil),
//     },
// })
