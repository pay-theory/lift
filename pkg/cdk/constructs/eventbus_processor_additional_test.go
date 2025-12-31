package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/jsii-runtime-go"
)

func TestEventBusProcessor_applyUserDynamoEventSourceProps_Overrides(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	queue := awssqs.NewQueue(stack, jsii.String("DLQ"), nil)
	encryptionKey := awskms.NewKey(stack, jsii.String("Key"), nil)
	var key awskms.IKey = encryptionKey

	filters := &[]*map[string]interface{}{
		{
			"pattern": map[string]interface{}{
				"eventName": []interface{}{"INSERT"},
			},
		},
	}

	target := &awslambdaeventsources.DynamoEventSourceProps{
		StartingPosition: awslambda.StartingPosition_TRIM_HORIZON,
	}

	overrides := &awslambdaeventsources.DynamoEventSourceProps{
		StartingPosition:        awslambda.StartingPosition_LATEST,
		BatchSize:               jsii.Number(5),
		Enabled:                 jsii.Bool(false),
		MaxBatchingWindow:       awscdk.Duration_Seconds(jsii.Number(10)),
		MaxRecordAge:            awscdk.Duration_Seconds(jsii.Number(120)),
		BisectBatchOnError:      jsii.Bool(true),
		RetryAttempts:           jsii.Number(2),
		ReportBatchItemFailures: jsii.Bool(true),
		ParallelizationFactor:   jsii.Number(2),
		TumblingWindow:          awscdk.Duration_Seconds(jsii.Number(30)),
		OnFailure:               awslambdaeventsources.NewSqsDlq(queue),
		Filters:                 filters,
		FilterEncryption:        key,
		ProvisionedPollerConfig: &awslambdaeventsources.ProvisionedPollerConfig{
			MinimumPollers: jsii.Number(1),
			MaximumPollers: jsii.Number(2),
		},
		MetricsConfig: &awslambda.MetricsConfig{},
	}

	applyUserDynamoEventSourceProps(nil, overrides)
	applyUserDynamoEventSourceProps(target, nil)
	applyUserDynamoEventSourceProps(target, overrides)

	if target.StartingPosition != awslambda.StartingPosition_LATEST {
		t.Fatalf("expected StartingPosition LATEST, got %s", target.StartingPosition)
	}
	if target.BatchSize == nil || *target.BatchSize != 5 {
		t.Fatal("expected BatchSize 5")
	}
	if target.Enabled == nil || *target.Enabled {
		t.Fatal("expected Enabled false")
	}
	if target.MaxBatchingWindow != overrides.MaxBatchingWindow {
		t.Fatal("expected MaxBatchingWindow override")
	}
	if target.MaxRecordAge != overrides.MaxRecordAge {
		t.Fatal("expected MaxRecordAge override")
	}
	if target.TumblingWindow != overrides.TumblingWindow {
		t.Fatal("expected TumblingWindow override")
	}
	if target.OnFailure == nil {
		t.Fatal("expected OnFailure override")
	}
	if target.Filters != filters {
		t.Fatal("expected Filters override")
	}
	if target.FilterEncryption != key {
		t.Fatal("expected FilterEncryption override")
	}
	if target.ProvisionedPollerConfig == nil || target.ProvisionedPollerConfig != overrides.ProvisionedPollerConfig {
		t.Fatal("expected ProvisionedPollerConfig override")
	}
	if target.MetricsConfig == nil || target.MetricsConfig != overrides.MetricsConfig {
		t.Fatal("expected MetricsConfig override")
	}

	emptyStart := &awslambdaeventsources.DynamoEventSourceProps{
		StartingPosition: awslambda.StartingPosition_TRIM_HORIZON,
	}
	applyUserDynamoEventSourceProps(emptyStart, &awslambdaeventsources.DynamoEventSourceProps{})
	if emptyStart.StartingPosition != awslambda.StartingPosition_TRIM_HORIZON {
		t.Fatal("did not expect StartingPosition to be overridden by empty override")
	}
}

func TestEventBusProcessor_buildEventBusEventTypeFilters_Empty(t *testing.T) {
	filters, err := buildEventBusEventTypeFilters([]string{""})
	if err == nil {
		t.Fatal("expected error")
	}
	if filters != nil {
		t.Fatal("expected nil filters on error")
	}
}
