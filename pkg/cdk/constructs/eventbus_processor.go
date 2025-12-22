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
type EventBusProcessorProps struct { //nolint:govet // fieldalignment: FunctionProps is large and opaque
	// Lambda function configuration (required).
	FunctionProps awslambda.FunctionProps

	// Existing EventBus table (optional). If omitted, a new EventBusTable is created.
	Table *EventBusTable
	// Properties for creating a new EventBus table (optional).
	TableProps *EventBusTableProps

	// Optional event source overrides.
	EventSourceProps *awslambdaeventsources.DynamoEventSourceProps

	// Optional tuning parameters (defaults are applied when nil/zero).
	BatchSize               *float64
	RetryAttempts           *float64
	ParallelizationFactor   *float64
	EnableDeadLetterQueue   *bool
	BisectBatchOnError      *bool
	ReportBatchItemFailures *bool

	DeadLetterQueueProps *awssqs.QueueProps

	// EventTypes filters stream events by `dynamodb.NewImage.event_type.S`.
	// When empty, no filter is applied (the processor will receive all stream records).
	EventTypes []string

	StartingPosition  awslambda.StartingPosition
	MaxBatchingWindow awscdk.Duration
	MaxRecordAge      awscdk.Duration
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

	applyNonZeroStartingPosition(target, overrides)
	applyPtrOverride(&target.BatchSize, overrides.BatchSize)
	applyPtrOverride(&target.Enabled, overrides.Enabled)
	applyDurationOverride(&target.MaxBatchingWindow, overrides.MaxBatchingWindow)
	applyDurationOverride(&target.MaxRecordAge, overrides.MaxRecordAge)
	applyPtrOverride(&target.BisectBatchOnError, overrides.BisectBatchOnError)
	applyPtrOverride(&target.RetryAttempts, overrides.RetryAttempts)
	applyPtrOverride(&target.ReportBatchItemFailures, overrides.ReportBatchItemFailures)
	applyPtrOverride(&target.ParallelizationFactor, overrides.ParallelizationFactor)
	applyEventSourceDlqOverride(target, overrides)
	applyPtrOverride(&target.Filters, overrides.Filters)
	applyKMSKeyOverride(target, overrides)
	applyPtrOverride(&target.ProvisionedPollerConfig, overrides.ProvisionedPollerConfig)
	applyDurationOverride(&target.TumblingWindow, overrides.TumblingWindow)
	applyPtrOverride(&target.MetricsConfig, overrides.MetricsConfig)
}

func applyNonZeroStartingPosition(target, overrides *awslambdaeventsources.DynamoEventSourceProps) {
	// StartingPosition is required, so only override when explicitly set.
	if overrides.StartingPosition != "" {
		target.StartingPosition = overrides.StartingPosition
	}
}

func applyPtrOverride[T any](dst **T, src *T) {
	if src != nil {
		*dst = src
	}
}

func applyDurationOverride(dst *awscdk.Duration, src awscdk.Duration) {
	if src != nil {
		*dst = src
	}
}

func applyEventSourceDlqOverride(target, overrides *awslambdaeventsources.DynamoEventSourceProps) {
	if overrides.OnFailure != nil {
		target.OnFailure = overrides.OnFailure
	}
}

func applyKMSKeyOverride(target, overrides *awslambdaeventsources.DynamoEventSourceProps) {
	if overrides.FilterEncryption != nil {
		target.FilterEncryption = overrides.FilterEncryption
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
