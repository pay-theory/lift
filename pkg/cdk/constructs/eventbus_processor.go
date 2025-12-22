package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// EventBusProcessorProps defines properties for an EventBus DynamoDB stream processor.
type EventBusProcessorProps struct {
	// Existing EventBus table (optional). If omitted, a new EventBusTable is created.
	Table *EventBusTable
	// Properties for creating a new EventBus table (optional).
	TableProps *EventBusTableProps

	// Lambda function configuration (required).
	FunctionProps awslambda.FunctionProps

	// EventTypes filters stream events by `dynamodb.NewImage.event_type.S`.
	// When empty, no filter is applied (the processor will receive all stream records).
	EventTypes []string

	// Optional event source overrides.
	EventSourceProps *awslambdaeventsources.DynamoEventSourceProps

	// Optional tuning parameters (defaults are applied when nil/zero).
	BatchSize               *float64
	RetryAttempts           *float64
	ParallelizationFactor   *float64
	EnableDeadLetterQueue   *bool
	BisectBatchOnError      *bool
	ReportBatchItemFailures *bool

	MaxBatchingWindow awscdk.Duration
	MaxRecordAge      awscdk.Duration
	StartingPosition  awslambda.StartingPosition

	DeadLetterQueueProps *awssqs.QueueProps
}

// EventBusProcessor wires an EventBus table stream to a Lambda function with sensible defaults.
type EventBusProcessor struct {
	constructs.Construct

	Function *LiftFunction
	Table    *EventBusTable

	DeadLetterQueue awssqs.IQueue
	EventSource     awslambdaeventsources.DynamoEventSource
}

// NewEventBusProcessor creates a new EventBus stream processor.
func NewEventBusProcessor(scope constructs.Construct, id *string, props *EventBusProcessorProps) *EventBusProcessor {
	construct := constructs.NewConstruct(scope, id)

	if props == nil {
		props = &EventBusProcessorProps{}
	}

	processor := &EventBusProcessor{
		Construct: construct,
	}

	processor.Table = resolveEventBusProcessorTable(construct, props)
	if processor.Table.GetStreamArn() == nil {
		panic("EventBusProcessor requires DynamoDB Streams enabled on the EventBus table")
	}

	processor.DeadLetterQueue = resolveEventBusProcessorDLQ(construct, props)

	processor.Function = NewLiftFunction(construct, jsii.String("Function"), &LiftFunctionProps{
		FunctionProps: props.FunctionProps,
	})

	// Permissions: processors commonly need read/write for publishing and checkpoints.
	processor.Table.GrantStreamRead(processor.Function.Function)
	processor.Table.GrantReadWrite(processor.Function.Function)

	eventSourceProps := buildEventBusProcessorEventSourceProps(props, processor.DeadLetterQueue)
	processor.EventSource = awslambdaeventsources.NewDynamoEventSource(processor.Table.Table, eventSourceProps)
	processor.Function.Function.AddEventSource(processor.EventSource)

	return processor
}

func resolveEventBusProcessorTable(scope constructs.Construct, props *EventBusProcessorProps) *EventBusTable {
	if props.Table != nil {
		return props.Table
	}

	tableProps := props.TableProps
	if tableProps == nil {
		tableProps = &EventBusTableProps{}
	}
	if tableProps.EnableStream == nil {
		tableProps.EnableStream = jsii.Bool(true)
	}
	if tableProps.StreamViewType == "" {
		// NEW_IMAGE is sufficient for typical EventBus processors (INSERT/MODIFY).
		// Consumers that need REMOVE processing can override to NEW_AND_OLD_IMAGES.
		tableProps.StreamViewType = awsdynamodb.StreamViewType_NEW_IMAGE
	}

	return NewEventBusTable(scope, jsii.String("EventBusTable"), tableProps)
}

func resolveEventBusProcessorDLQ(scope constructs.Construct, props *EventBusProcessorProps) awssqs.IQueue {
	enableDLQ := true
	if props.EnableDeadLetterQueue != nil {
		enableDLQ = *props.EnableDeadLetterQueue
	}
	if !enableDLQ {
		return nil
	}

	queueProps := props.DeadLetterQueueProps
	if queueProps == nil {
		queueProps = &awssqs.QueueProps{
			RetentionPeriod: awscdk.Duration_Days(jsii.Number(14)),
		}
	}

	return awssqs.NewQueue(scope, jsii.String("DLQ"), queueProps)
}

func buildEventBusProcessorEventSourceProps(props *EventBusProcessorProps, dlq awssqs.IQueue) *awslambdaeventsources.DynamoEventSourceProps {
	startingPosition := props.StartingPosition
	if startingPosition == "" {
		startingPosition = awslambda.StartingPosition_LATEST
	}

	batchSize := props.BatchSize
	if batchSize == nil {
		batchSize = jsii.Number(10)
	}

	retryAttempts := props.RetryAttempts
	if retryAttempts == nil {
		retryAttempts = jsii.Number(10000)
	}

	parallelizationFactor := props.ParallelizationFactor
	if parallelizationFactor == nil {
		parallelizationFactor = jsii.Number(1)
	}

	bisectBatchOnError := props.BisectBatchOnError
	if bisectBatchOnError == nil {
		bisectBatchOnError = jsii.Bool(false)
	}

	reportBatchItemFailures := props.ReportBatchItemFailures
	if reportBatchItemFailures == nil {
		reportBatchItemFailures = jsii.Bool(true)
	}

	maxBatchingWindow := props.MaxBatchingWindow
	if maxBatchingWindow == nil {
		maxBatchingWindow = awscdk.Duration_Seconds(jsii.Number(5))
	}

	maxRecordAge := props.MaxRecordAge
	if maxRecordAge == nil {
		maxRecordAge = awscdk.Duration_Hours(jsii.Number(24))
	}

	eventSourceProps := &awslambdaeventsources.DynamoEventSourceProps{
		StartingPosition:        startingPosition,
		BatchSize:               batchSize,
		MaxBatchingWindow:       maxBatchingWindow,
		MaxRecordAge:            maxRecordAge,
		BisectBatchOnError:      bisectBatchOnError,
		RetryAttempts:           retryAttempts,
		ReportBatchItemFailures: reportBatchItemFailures,
		ParallelizationFactor:   parallelizationFactor,
	}

	if dlq != nil {
		eventSourceProps.OnFailure = awslambdaeventsources.NewSqsDlq(dlq)
	}

	if len(props.EventTypes) > 0 {
		filters, err := buildEventBusEventTypeFilters(props.EventTypes)
		if err != nil {
			panic(err)
		}
		eventSourceProps.Filters = filters
	}

	applyUserDynamoEventSourceProps(eventSourceProps, props.EventSourceProps)

	return eventSourceProps
}

func applyUserDynamoEventSourceProps(target *awslambdaeventsources.DynamoEventSourceProps, overrides *awslambdaeventsources.DynamoEventSourceProps) {
	if target == nil || overrides == nil {
		return
	}

	// StartingPosition is required, so only override when explicitly set.
	if overrides.StartingPosition != "" {
		target.StartingPosition = overrides.StartingPosition
	}
	if overrides.BatchSize != nil {
		target.BatchSize = overrides.BatchSize
	}
	if overrides.MaxBatchingWindow != nil {
		target.MaxBatchingWindow = overrides.MaxBatchingWindow
	}
	if overrides.MaxRecordAge != nil {
		target.MaxRecordAge = overrides.MaxRecordAge
	}
	if overrides.BisectBatchOnError != nil {
		target.BisectBatchOnError = overrides.BisectBatchOnError
	}
	if overrides.RetryAttempts != nil {
		target.RetryAttempts = overrides.RetryAttempts
	}
	if overrides.ReportBatchItemFailures != nil {
		target.ReportBatchItemFailures = overrides.ReportBatchItemFailures
	}
	if overrides.ParallelizationFactor != nil {
		target.ParallelizationFactor = overrides.ParallelizationFactor
	}
	if overrides.Enabled != nil {
		target.Enabled = overrides.Enabled
	}
	if overrides.OnFailure != nil {
		target.OnFailure = overrides.OnFailure
	}
	if overrides.Filters != nil {
		target.Filters = overrides.Filters
	}
	if overrides.FilterEncryption != nil {
		target.FilterEncryption = overrides.FilterEncryption
	}
	if overrides.ProvisionedPollerConfig != nil {
		target.ProvisionedPollerConfig = overrides.ProvisionedPollerConfig
	}
	if overrides.TumblingWindow != nil {
		target.TumblingWindow = overrides.TumblingWindow
	}
	if overrides.MetricsConfig != nil {
		target.MetricsConfig = overrides.MetricsConfig
	}
}

func buildEventBusEventTypeFilters(eventTypes []string) (*[]*map[string]interface{}, error) {
	values := make([]*string, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		if eventType == "" {
			continue
		}
		values = append(values, jsii.String(eventType))
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("event types filter is empty")
	}

	filter := awslambda.FilterCriteria_Filter(&map[string]interface{}{
		// Process publish-time events by default.
		"eventName": awslambda.FilterRule_Or(
			jsii.String("INSERT"),
			jsii.String("MODIFY"),
		),
		"dynamodb": map[string]interface{}{
			"NewImage": map[string]interface{}{
				// Guard against non-event rows co-located in the table (e.g., schedules/checkpoints).
				// Real EventBus events always include an "id" attribute.
				"id": map[string]interface{}{
					"S": awslambda.FilterRule_Exists(),
				},
				"event_type": map[string]interface{}{
					"S": awslambda.FilterRule_Or(values...),
				},
			},
		},
	})

	filters := []*map[string]interface{}{filter}
	return &filters, nil
}
