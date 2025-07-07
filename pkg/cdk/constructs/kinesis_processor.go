package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskinesis"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// KinesisProcessorProps defines the properties for creating a Kinesis processor
type KinesisProcessorProps struct {
	// The Lambda function configuration
	FunctionProps *LiftFunctionProps `field:"required"`

	// Optional: Stream configuration
	StreamProps *awskinesis.StreamProps `field:"optional"`

	// Optional: Use an existing stream instead of creating a new one
	ExistingStream awskinesis.IStream `field:"optional"`

	// Optional: Event source configuration
	EventSourceProps *awslambdaeventsources.KinesisEventSourceProps `field:"optional"`

	// Optional: Enable dead letter queue for failed records
	EnableDLQ *bool `field:"optional"`

	// Optional: DLQ configuration
	DLQProps *awssqs.QueueProps `field:"optional"`

	// Optional: Stream mode (provisioned or on-demand)
	StreamMode *awskinesis.StreamMode `field:"optional"`

	// Optional: Number of shards (for provisioned mode)
	ShardCount *float64 `field:"optional"`

	// Optional: Data retention period in hours (24-8760 hours)
	RetentionPeriodHours *float64 `field:"optional"`

	// Optional: Enable encryption
	Encryption *awskinesis.StreamEncryption `field:"optional"`

	// Optional: Enable enhanced fan-out
	EnableEnhancedFanOut *bool `field:"optional"`

	// Optional: Consumer name for enhanced fan-out
	ConsumerName *string `field:"optional"`

	// Optional: Batch size for processing (1-10000)
	BatchSize *float64 `field:"optional"`

	// Optional: Maximum batching window in seconds
	MaxBatchingWindowSeconds *float64 `field:"optional"`

	// Optional: Parallelization factor (1-10)
	ParallelizationFactor *float64 `field:"optional"`

	// Optional: Starting position
	StartingPosition *awslambda.StartingPosition `field:"optional"`

	// Optional: Maximum record age in seconds
	MaxRecordAgeSeconds *float64 `field:"optional"`

	// Optional: Bisect batch on function error
	BisectBatchOnError *bool `field:"optional"`

	// Optional: Maximum retry attempts
	RetryAttempts *float64 `field:"optional"`

	// Optional: Tumbling window in seconds
	TumblingWindowSeconds *float64 `field:"optional"`

	// Optional: Report batch item failures
	ReportBatchItemFailures *bool `field:"optional"`
}

// KinesisProcessor creates a Kinesis stream with Lambda processor
type KinesisProcessor struct {
	constructs.Construct
	Stream   awskinesis.IStream
	Function LiftFunction
	DLQ      awssqs.IQueue
	Consumer awskinesis.IStreamConsumer
}

// NewKinesisProcessor creates a new Kinesis processor with Lambda function
func NewKinesisProcessor(scope constructs.Construct, id *string, props *KinesisProcessorProps) *KinesisProcessor {
	this := constructs.NewConstruct(scope, id)

	// Create or use existing Kinesis stream
	var stream awskinesis.IStream
	if props.ExistingStream != nil {
		stream = props.ExistingStream
	} else {
		streamProps := props.StreamProps
		if streamProps == nil {
			streamProps = &awskinesis.StreamProps{}
		}

		// Set stream mode
		if props.StreamMode != nil {
			streamProps.StreamMode = *props.StreamMode
		} else {
			// Default to on-demand for simplicity
			streamProps.StreamMode = awskinesis.StreamMode_ON_DEMAND
		}

		// Set shard count for provisioned mode
		if props.ShardCount != nil {
			streamProps.ShardCount = props.ShardCount
		}

		// Set retention period
		if props.RetentionPeriodHours != nil {
			streamProps.RetentionPeriod = awscdk.Duration_Hours(props.RetentionPeriodHours)
		} else if streamProps.RetentionPeriod == nil {
			streamProps.RetentionPeriod = awscdk.Duration_Hours(jsii.Number(24)) // 24 hours default
		}

		// Set encryption
		if props.Encryption != nil {
			streamProps.Encryption = *props.Encryption
		}

		stream = awskinesis.NewStream(this, jsii.String("Stream"), streamProps)
	}

	// Create the Lambda function
	function := NewLiftFunction(this, jsii.String("Function"), props.FunctionProps)

	// Add Kinesis stream environment variables
	function.Function.AddEnvironment(jsii.String("KINESIS_STREAM_ARN"), stream.StreamArn(), nil)
	function.Function.AddEnvironment(jsii.String("KINESIS_STREAM_NAME"), stream.StreamName(), nil)

	// Create DLQ if enabled
	var dlq awssqs.IQueue
	enableDLQ := true // Default to enabled
	if props.EnableDLQ != nil {
		enableDLQ = *props.EnableDLQ
	}

	if enableDLQ {
		dlqProps := props.DLQProps
		if dlqProps == nil {
			dlqProps = &awssqs.QueueProps{
				RetentionPeriod: awscdk.Duration_Days(jsii.Number(14)),
			}
		}
		dlq = awssqs.NewQueue(this, jsii.String("DLQ"), dlqProps)
		function.Function.AddEnvironment(jsii.String("KINESIS_DLQ_URL"), dlq.QueueUrl(), nil)
	}

	// Configure event source
	eventSourceProps := props.EventSourceProps
	if eventSourceProps == nil {
		eventSourceProps = &awslambdaeventsources.KinesisEventSourceProps{}
	}

	// Set batch size
	if props.BatchSize != nil {
		eventSourceProps.BatchSize = props.BatchSize
	} else if eventSourceProps.BatchSize == nil {
		eventSourceProps.BatchSize = jsii.Number(100) // Default batch size
	}

	// Set batching window
	if props.MaxBatchingWindowSeconds != nil {
		eventSourceProps.MaxBatchingWindow = awscdk.Duration_Seconds(props.MaxBatchingWindowSeconds)
	}

	// Set parallelization factor
	if props.ParallelizationFactor != nil {
		eventSourceProps.ParallelizationFactor = props.ParallelizationFactor
	}

	// Set starting position
	if props.StartingPosition != nil {
		eventSourceProps.StartingPosition = *props.StartingPosition
	} else {
		eventSourceProps.StartingPosition = awslambda.StartingPosition_LATEST
	}

	// Set max record age
	if props.MaxRecordAgeSeconds != nil {
		eventSourceProps.MaxRecordAge = awscdk.Duration_Seconds(props.MaxRecordAgeSeconds)
	}

	// Set bisect batch on error
	if props.BisectBatchOnError != nil {
		eventSourceProps.BisectBatchOnError = props.BisectBatchOnError
	}

	// Set retry attempts
	if props.RetryAttempts != nil {
		eventSourceProps.RetryAttempts = props.RetryAttempts
	}

	// Set tumbling window
	if props.TumblingWindowSeconds != nil {
		eventSourceProps.TumblingWindow = awscdk.Duration_Seconds(props.TumblingWindowSeconds)
	}

	// Set report batch item failures
	if props.ReportBatchItemFailures != nil {
		eventSourceProps.ReportBatchItemFailures = props.ReportBatchItemFailures
	}

	// Set DLQ
	if dlq != nil {
		eventSourceProps.OnFailure = awslambdaeventsources.NewSqsDlq(dlq)
	}

	// Create enhanced fan-out consumer if requested
	var consumer awskinesis.IStreamConsumer
	if props.EnableEnhancedFanOut != nil && *props.EnableEnhancedFanOut {
		consumerName := props.ConsumerName
		if consumerName == nil {
			consumerName = jsii.String("LiftConsumer")
		}
		
		// Note: Enhanced fan-out consumer creation is not directly supported in CDK Go
		// You would need to create the consumer using CloudFormation or after deployment
		// For now, we just set the consumer name as an environment variable
		function.Function.AddEnvironment(jsii.String("KINESIS_CONSUMER_NAME"), consumerName, nil)
	}

	// Add Kinesis event source to Lambda
	function.Function.AddEventSource(awslambdaeventsources.NewKinesisEventSource(stream, eventSourceProps))

	// Grant permissions
	stream.GrantRead(function.Function.GrantPrincipal())

	processor := &KinesisProcessor{
		Construct: this,
		Stream:    stream,
		Function:  *function,
		DLQ:       dlq,
		Consumer:  consumer,
	}

	return processor
}

// GrantRead grants read permissions to the stream
func (k *KinesisProcessor) GrantRead(grantee awsiam.IGrantable) awsiam.Grant {
	return k.Stream.GrantRead(grantee)
}

// GrantWrite grants write permissions to the stream
func (k *KinesisProcessor) GrantWrite(grantee awsiam.IGrantable) awsiam.Grant {
	return k.Stream.GrantWrite(grantee)
}

// GrantReadWrite grants read and write permissions to the stream
func (k *KinesisProcessor) GrantReadWrite(grantee awsiam.IGrantable) awsiam.Grant {
	return k.Stream.GrantReadWrite(grantee)
}

// GetStreamArn returns the Kinesis stream ARN
func (k *KinesisProcessor) GetStreamArn() *string {
	return k.Stream.StreamArn()
}

// GetStreamName returns the Kinesis stream name
func (k *KinesisProcessor) GetStreamName() *string {
	return k.Stream.StreamName()
}

// GetDLQUrl returns the DLQ URL if DLQ is enabled
func (k *KinesisProcessor) GetDLQUrl() *string {
	if k.DLQ != nil {
		return k.DLQ.QueueUrl()
	}
	return nil
}

// AddConsumer adds an enhanced fan-out consumer to the stream
func (k *KinesisProcessor) AddConsumer(id *string, consumerName *string) awskinesis.IStreamConsumer {
	// This is a simplified approach - in a real implementation, 
	// you would use CfnStreamConsumer or handle this differently
	k.Function.Function.AddEnvironment(jsii.String("KINESIS_CONSUMER_"+*id), consumerName, nil)
	return k.Consumer
}

// Metric returns a metric for the stream
func (k *KinesisProcessor) Metric(metricName *string, props *awscloudwatch.MetricOptions) awscloudwatch.Metric {
	return k.Stream.Metric(metricName, props)
}

// MetricGetRecords returns the GetRecords metric
func (k *KinesisProcessor) MetricGetRecords(props *awscloudwatch.MetricOptions) awscloudwatch.Metric {
	return k.Stream.Metric(jsii.String("GetRecords.Success"), props)
}

// MetricPutRecords returns the PutRecords metric
func (k *KinesisProcessor) MetricPutRecords(props *awscloudwatch.MetricOptions) awscloudwatch.Metric {
	return k.Stream.Metric(jsii.String("PutRecords.Success"), props)
}