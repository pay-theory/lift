package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskinesis"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
)

func TestKinesisProcessor_DefaultConfiguration(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// When
	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
	})

	// Then
	template := assertions.Template_FromStack(stack, nil)

	// Verify Kinesis stream is created with default on-demand mode
	template.HasResourceProperties(jsii.String("AWS::Kinesis::Stream"), &map[string]interface{}{
		"StreamModeDetails": map[string]interface{}{
			"StreamMode": "ON_DEMAND",
		},
		"RetentionPeriodHours": 24, // Default 24 hours
	})

	// Verify Lambda function is created
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"Runtime": "provided.al2023",
		"Handler": "bootstrap",
	})

	// Verify DLQ is created by default
	template.HasResourceProperties(jsii.String("AWS::SQS::Queue"), &map[string]interface{}{
		"MessageRetentionPeriod": 1209600, // 14 days in seconds
	})

	// Verify event source mapping
	template.HasResourceProperties(jsii.String("AWS::Lambda::EventSourceMapping"), &map[string]interface{}{
		"BatchSize":        100, // Default batch size
		"StartingPosition": "LATEST",
	})

	// Verify processor properties
	assert.NotNil(t, processor.Stream)
	assert.NotNil(t, processor.Function)
	assert.NotNil(t, processor.DLQ)
}

func TestKinesisProcessor_ProvisionedMode(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// When
	streamMode := awskinesis.StreamMode_PROVISIONED
	encryption := awskinesis.StreamEncryption_KMS
	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
		StreamMode:           &streamMode,
		ShardCount:           jsii.Number(5),
		RetentionPeriodHours: jsii.Number(72), // 3 days
		Encryption:           &encryption,
	})

	// Then
	template := assertions.Template_FromStack(stack, nil)

	// Verify provisioned stream configuration
	template.HasResourceProperties(jsii.String("AWS::Kinesis::Stream"), &map[string]interface{}{
		"StreamModeDetails": map[string]interface{}{
			"StreamMode": "PROVISIONED",
		},
		"ShardCount":           5,
		"RetentionPeriodHours": 72,
		"StreamEncryption": map[string]interface{}{
			"EncryptionType": "KMS",
		},
	})

	assert.NotNil(t, processor)
}

func TestKinesisProcessor_EnhancedFanOut(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// When
	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
		EnableEnhancedFanOut: jsii.Bool(true),
		ConsumerName:         jsii.String("MyConsumer"),
	})

	// Then
	template := assertions.Template_FromStack(stack, nil)

	// Verify Lambda function exists
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"Runtime": "provided.al2023",
	})

	assert.NotNil(t, processor)
}

func TestKinesisProcessor_CustomEventSourceConfiguration(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// When
	startingPos := awslambda.StartingPosition_TRIM_HORIZON
	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
		BatchSize:                jsii.Number(50),
		MaxBatchingWindowSeconds: jsii.Number(5),
		ParallelizationFactor:    jsii.Number(5),
		StartingPosition:         &startingPos,
		MaxRecordAgeSeconds:      jsii.Number(3600),
		BisectBatchOnError:       jsii.Bool(true),
		RetryAttempts:            jsii.Number(3),
		TumblingWindowSeconds:    jsii.Number(30),
		ReportBatchItemFailures:  jsii.Bool(true),
	})

	// Then
	template := assertions.Template_FromStack(stack, nil)

	// Verify custom event source mapping configuration
	template.HasResourceProperties(jsii.String("AWS::Lambda::EventSourceMapping"), &map[string]interface{}{
		"BatchSize":                      50,
		"MaximumBatchingWindowInSeconds": 5,
		"ParallelizationFactor":          5,
		"StartingPosition":               "TRIM_HORIZON",
		"MaximumRecordAgeInSeconds":      3600,
		"BisectBatchOnFunctionError":     true,
		"MaximumRetryAttempts":           3,
		"TumblingWindowInSeconds":        30,
		"FunctionResponseTypes":          []interface{}{"ReportBatchItemFailures"},
	})

	assert.NotNil(t, processor)
}

func TestKinesisProcessor_ExistingStream(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)
	existingStream := awskinesis.NewStream(stack, jsii.String("ExistingStream"), &awskinesis.StreamProps{
		StreamName: jsii.String("existing-stream"),
		StreamMode: awskinesis.StreamMode_ON_DEMAND,
	})

	// When
	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
		ExistingStream: existingStream,
	})

	// Then
	template := assertions.Template_FromStack(stack, nil)

	// Should have the existing stream
	template.HasResourceProperties(jsii.String("AWS::Kinesis::Stream"), &map[string]interface{}{
		"Name": "existing-stream",
	})

	// Should not create another stream
	template.ResourceCountIs(jsii.String("AWS::Kinesis::Stream"), jsii.Number(1))

	// Verify event source mapping exists
	template.HasResourceProperties(jsii.String("AWS::Lambda::EventSourceMapping"), &map[string]interface{}{
		"BatchSize": 100,
	})

	assert.Equal(t, existingStream, processor.Stream)
}

func TestKinesisProcessor_DisabledDLQ(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// When
	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
		EnableDLQ: jsii.Bool(false),
	})

	// Then
	template := assertions.Template_FromStack(stack, nil)

	// Should not have Kinesis DLQ (but may have Lambda DLQ)
	// We can't check the exact count because LiftFunction might create its own DLQ

	// Verify Lambda function exists
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"Runtime": "provided.al2023",
	})

	assert.Nil(t, processor.DLQ)
}

func TestKinesisProcessor_CustomStreamProps(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// When
	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
		StreamProps: &awskinesis.StreamProps{
			StreamName:      jsii.String("custom-stream"),
			StreamMode:      awskinesis.StreamMode_ON_DEMAND,
			RetentionPeriod: awscdk.Duration_Days(jsii.Number(2)),
		},
		DLQProps: &awssqs.QueueProps{
			QueueName:       jsii.String("custom-dlq"),
			RetentionPeriod: awscdk.Duration_Days(jsii.Number(7)),
		},
	})

	// Then
	template := assertions.Template_FromStack(stack, nil)

	// Verify custom stream configuration
	template.HasResourceProperties(jsii.String("AWS::Kinesis::Stream"), &map[string]interface{}{
		"Name":                 "custom-stream",
		"RetentionPeriodHours": 48, // 2 days in hours
	})

	// Verify custom DLQ configuration
	template.HasResourceProperties(jsii.String("AWS::SQS::Queue"), &map[string]interface{}{
		"QueueName":              "custom-dlq",
		"MessageRetentionPeriod": 604800, // 7 days in seconds
	})

	assert.NotNil(t, processor)
}

func TestKinesisProcessor_HelperMethods(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
		FunctionProps: &LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Runtime: awslambda.Runtime_PROVIDED_AL2023(),
				Handler: jsii.String("bootstrap"),
				Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
			},
		},
	})

	// Create a test role
	testRole := awsiam.NewRole(stack, jsii.String("TestRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
	})

	// When & Then
	// Test grant methods
	readGrant := processor.GrantRead(testRole)
	assert.NotNil(t, readGrant)

	writeGrant := processor.GrantWrite(testRole)
	assert.NotNil(t, writeGrant)

	readWriteGrant := processor.GrantReadWrite(testRole)
	assert.NotNil(t, readWriteGrant)

	// Test getter methods
	streamArn := processor.GetStreamArn()
	assert.NotNil(t, streamArn)

	streamName := processor.GetStreamName()
	assert.NotNil(t, streamName)

	dlqUrl := processor.GetDLQUrl()
	assert.NotNil(t, dlqUrl)

	// Test metrics
	getRecordsMetric := processor.MetricGetRecords(nil)
	assert.NotNil(t, getRecordsMetric)

	putRecordsMetric := processor.MetricPutRecords(nil)
	assert.NotNil(t, putRecordsMetric)
}

func TestKinesisProcessor_NilProps(t *testing.T) {
	// Given
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// When & Then - should not panic
	assert.NotPanics(t, func() {
		processor := NewKinesisProcessor(stack, jsii.String("TestProcessor"), &KinesisProcessorProps{
			FunctionProps: &LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Runtime: awslambda.Runtime_PROVIDED_AL2023(),
					Handler: jsii.String("bootstrap"),
					Code:    awslambda.Code_FromAsset(jsii.String("../test"), nil),
				},
			},
			StreamProps:      nil,
			EventSourceProps: nil,
			DLQProps:         nil,
		})
		assert.NotNil(t, processor)
	})
}
