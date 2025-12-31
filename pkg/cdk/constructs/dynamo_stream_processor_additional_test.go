package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/jsii-runtime-go"
)

func TestDynamoStreamProcessor_ConfigAndEventSourceOverrides(t *testing.T) {
	props := &DynamoStreamProcessorProps{
		BatchSize:               jsii.Number(25),
		MaxBatchingWindow:       awscdk.Duration_Seconds(jsii.Number(30)),
		StartingPosition:        awslambda.StartingPosition_TRIM_HORIZON,
		MaxRecordAge:            awscdk.Duration_Hours(jsii.Number(1)),
		BisectBatchOnError:      jsii.Bool(true),
		RetryAttempts:           jsii.Number(5),
		ReportBatchItemFailures: jsii.Bool(false),
		ParallelizationFactor:   jsii.Number(3),
		EnableDeadLetterQueue:   jsii.Bool(false),
		EventSourceProps: &awslambdaeventsources.DynamoEventSourceProps{
			StartingPosition:        awslambda.StartingPosition_LATEST,
			BatchSize:               jsii.Number(5),
			MaxBatchingWindow:       awscdk.Duration_Seconds(jsii.Number(1)),
			MaxRecordAge:            awscdk.Duration_Seconds(jsii.Number(120)),
			BisectBatchOnError:      jsii.Bool(false),
			RetryAttempts:           jsii.Number(2),
			ReportBatchItemFailures: jsii.Bool(true),
			ParallelizationFactor:   jsii.Number(2),
			TumblingWindow:          awscdk.Duration_Seconds(jsii.Number(10)),
			Enabled:                 jsii.Bool(false),
			Filters: &[]*map[string]interface{}{
				{
					"pattern": map[string]interface{}{
						"eventName": []interface{}{"INSERT"},
					},
				},
			},
		},
	}

	config := buildDynamoStreamProcessorConfig(props)
	if config.batchSize != 25 {
		t.Fatalf("expected batchSize 25, got %v", config.batchSize)
	}
	if config.startingPosition != awslambda.StartingPosition_TRIM_HORIZON {
		t.Fatalf("expected startingPosition TRIM_HORIZON, got %s", config.startingPosition)
	}
	if config.enableDLQ {
		t.Fatal("expected dlq disabled")
	}

	esb := &dynamoStreamEventSourceBuilder{
		props:  props,
		config: config,
	}

	eventSourceProps := esb.createBaseEventSourceProps()
	esb.applyUserEventSourceProps(eventSourceProps)

	if eventSourceProps.StartingPosition != awslambda.StartingPosition_LATEST {
		t.Fatalf("expected override StartingPosition LATEST, got %s", eventSourceProps.StartingPosition)
	}
	if eventSourceProps.BatchSize == nil || *eventSourceProps.BatchSize != 5 {
		t.Fatal("expected override BatchSize 5")
	}
	if eventSourceProps.Enabled == nil || *eventSourceProps.Enabled {
		t.Fatal("expected override Enabled false")
	}
	if eventSourceProps.MaxBatchingWindow != props.EventSourceProps.MaxBatchingWindow {
		t.Fatal("expected override MaxBatchingWindow")
	}
	if eventSourceProps.MaxRecordAge != props.EventSourceProps.MaxRecordAge {
		t.Fatal("expected override MaxRecordAge")
	}
	if eventSourceProps.TumblingWindow != props.EventSourceProps.TumblingWindow {
		t.Fatal("expected override TumblingWindow")
	}
	if eventSourceProps.Filters != props.EventSourceProps.Filters {
		t.Fatal("expected override Filters")
	}
}
