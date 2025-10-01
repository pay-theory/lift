package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// LiftSQSQueueProps defines properties for attaching an SQS queue to an existing Lambda function
type LiftSQSQueueProps struct {
	// Required: Existing Lambda function to attach this queue to
	Function awslambda.Function

	// Queue configuration
	QueueName              *string
	VisibilityTimeout      awscdk.Duration // Default: 5 minutes
	MessageRetentionPeriod awscdk.Duration // Default: 14 days
	ReceiveMessageWaitTime awscdk.Duration // For long polling, default: 20 seconds

	// Dead letter queue configuration
	EnableDeadLetterQueue *bool           // Default: true
	DeadLetterQueueName   *string         // Default: {QueueName}-dlq
	MaxReceiveCount       *float64        // Default: 3
	DLQRetentionPeriod    awscdk.Duration // Default: 14 days

	// Encryption configuration
	EncryptionMasterKey awskms.IKey        // Required for K3 - partner-specific KMS key
	DataKeyReuse        awscdk.Duration    // Default: 300 seconds

	// Event source configuration
	EnableEventSource       *bool           // Default: true
	BatchSize               *float64        // Default: 10
	MaxBatchingWindow       awscdk.Duration // Default: 5 seconds
	ReportBatchItemFailures *bool           // Default: true
	MaxConcurrency          *float64        // Default: 5

	// Environment variable configuration
	QueueUrlEnvVar    *string // Custom env var name for queue URL (e.g., "K3_PROCESSOR_INSTRUMENT_QUEUE_URL")
	DLQUrlEnvVar      *string // Custom env var name for DLQ URL (optional)

	// SSM Parameter Store configuration
	EnableSSMParameter *bool   // Default: false
	SSMParameterName   *string // SSM parameter name to store queue URL
	SSMDescription     *string // SSM parameter description

	// FIFO queue configuration
	FifoQueue                       *bool
	EnableContentBasedDeduplication *bool

	// Additional permissions
	GrantSendMessages    *bool // Default: true - grant Lambda permission to send messages
	GrantConsumeMessages *bool // Default: true - grant Lambda permission to consume messages
}

// LiftSQSQueue represents an SQS queue attached to an existing Lambda function
type LiftSQSQueue struct {
	constructs.Construct

	// The SQS queue
	Queue awssqs.Queue

	// Dead letter queue (if enabled)
	DeadLetterQueue awssqs.Queue

	// Event source mapping (if enabled)
	EventSource awslambdaeventsources.SqsEventSource

	// SSM Parameter (if enabled)
	SSMParameter awsssm.StringParameter
}

// NewLiftSQSQueue creates a new SQS queue and attaches it to an existing Lambda function
func NewLiftSQSQueue(scope constructs.Construct, id *string, props *LiftSQSQueueProps) *LiftSQSQueue {
	this := &LiftSQSQueue{}
	constructs.NewConstruct_Override(this, scope, id)

	if props.Function == nil {
		panic("Function is required - use existing Lambda function")
	}

	// Set defaults
	if props.EnableDeadLetterQueue == nil {
		props.EnableDeadLetterQueue = jsii.Bool(true)
	}
	if props.MaxReceiveCount == nil {
		props.MaxReceiveCount = jsii.Number(3)
	}
	if props.VisibilityTimeout == nil {
		props.VisibilityTimeout = awscdk.Duration_Minutes(jsii.Number(5))
	}
	if props.MessageRetentionPeriod == nil {
		props.MessageRetentionPeriod = awscdk.Duration_Days(jsii.Number(14))
	}
	if props.ReceiveMessageWaitTime == nil {
		props.ReceiveMessageWaitTime = awscdk.Duration_Seconds(jsii.Number(20))
	}
	if props.DLQRetentionPeriod == nil {
		props.DLQRetentionPeriod = awscdk.Duration_Days(jsii.Number(14))
	}
	if props.DataKeyReuse == nil {
		props.DataKeyReuse = awscdk.Duration_Seconds(jsii.Number(300))
	}
	if props.EnableEventSource == nil {
		props.EnableEventSource = jsii.Bool(true)
	}
	if props.BatchSize == nil {
		props.BatchSize = jsii.Number(10)
	}
	if props.MaxBatchingWindow == nil {
		props.MaxBatchingWindow = awscdk.Duration_Seconds(jsii.Number(5))
	}
	if props.ReportBatchItemFailures == nil {
		props.ReportBatchItemFailures = jsii.Bool(true)
	}
	if props.MaxConcurrency == nil {
		props.MaxConcurrency = jsii.Number(5)
	}
	if props.GrantSendMessages == nil {
		props.GrantSendMessages = jsii.Bool(true)
	}
	if props.GrantConsumeMessages == nil {
		props.GrantConsumeMessages = jsii.Bool(true)
	}
	if props.EnableSSMParameter == nil {
		props.EnableSSMParameter = jsii.Bool(false)
	}

	// Create dead letter queue if enabled
	if *props.EnableDeadLetterQueue {
		dlqName := props.DeadLetterQueueName
		if dlqName == nil && props.QueueName != nil {
			dlqName = jsii.String(*props.QueueName + "-dlq")
		}

		dlqProps := &awssqs.QueueProps{
			QueueName:       dlqName,
			RetentionPeriod: props.DLQRetentionPeriod,
		}

		// Add encryption if KMS key provided
		if props.EncryptionMasterKey != nil {
			dlqProps.EncryptionMasterKey = props.EncryptionMasterKey
			dlqProps.DataKeyReuse = props.DataKeyReuse
		}

		// Handle FIFO DLQ
		if props.FifoQueue != nil && *props.FifoQueue {
			dlqProps.Fifo = jsii.Bool(true)
			if dlqName != nil && len(*dlqName) >= 5 && (*dlqName)[len(*dlqName)-5:] != ".fifo" {
				dlqProps.QueueName = jsii.String(*dlqName + ".fifo")
			}
		}

		this.DeadLetterQueue = awssqs.NewQueue(this, jsii.String("DeadLetterQueue"), dlqProps)
	}

	// Create main queue
	queueProps := &awssqs.QueueProps{
		QueueName:              props.QueueName,
		VisibilityTimeout:      props.VisibilityTimeout,
		RetentionPeriod:        props.MessageRetentionPeriod,
		ReceiveMessageWaitTime: props.ReceiveMessageWaitTime,
	}

	// Add encryption if KMS key provided
	if props.EncryptionMasterKey != nil {
		queueProps.EncryptionMasterKey = props.EncryptionMasterKey
		queueProps.DataKeyReuse = props.DataKeyReuse
	}

	// Configure dead letter queue
	if *props.EnableDeadLetterQueue && this.DeadLetterQueue != nil {
		queueProps.DeadLetterQueue = &awssqs.DeadLetterQueue{
			MaxReceiveCount: props.MaxReceiveCount,
			Queue:           this.DeadLetterQueue,
		}
	}

	// Handle FIFO configuration
	if props.FifoQueue != nil && *props.FifoQueue {
		queueProps.Fifo = jsii.Bool(true)
		if props.EnableContentBasedDeduplication != nil {
			queueProps.ContentBasedDeduplication = props.EnableContentBasedDeduplication
		}
		// Ensure FIFO queue name ends with .fifo
		if props.QueueName != nil && len(*props.QueueName) >= 5 && (*props.QueueName)[len(*props.QueueName)-5:] != ".fifo" {
			queueProps.QueueName = jsii.String(*props.QueueName + ".fifo")
		}
	}

	this.Queue = awssqs.NewQueue(this, jsii.String("Queue"), queueProps)

	// Grant permissions
	if *props.GrantSendMessages {
		this.Queue.GrantSendMessages(props.Function)
	}
	if *props.GrantConsumeMessages {
		this.Queue.GrantConsumeMessages(props.Function)
	}

	// Add environment variables to Lambda function
	if props.QueueUrlEnvVar != nil {
		props.Function.AddEnvironment(jsii.String(*props.QueueUrlEnvVar), this.Queue.QueueUrl(), nil)
	} else {
		// Default environment variable name
		props.Function.AddEnvironment(jsii.String("SQS_QUEUE_URL"), this.Queue.QueueUrl(), nil)
	}

	if props.DLQUrlEnvVar != nil && this.DeadLetterQueue != nil {
		props.Function.AddEnvironment(jsii.String(*props.DLQUrlEnvVar), this.DeadLetterQueue.QueueUrl(), nil)
	}

	// Create event source if enabled
	if *props.EnableEventSource {
		eventSourceProps := &awslambdaeventsources.SqsEventSourceProps{
			BatchSize:               props.BatchSize,
			ReportBatchItemFailures: props.ReportBatchItemFailures,
			MaxConcurrency:          props.MaxConcurrency,
		}

		// Only set batching window for non-FIFO queues
		if props.FifoQueue == nil || !*props.FifoQueue {
			eventSourceProps.MaxBatchingWindow = props.MaxBatchingWindow
		}

		this.EventSource = awslambdaeventsources.NewSqsEventSource(this.Queue, eventSourceProps)
		this.EventSource.Bind(props.Function)
	}

	// Store queue URL in SSM Parameter Store if enabled
	if *props.EnableSSMParameter && props.SSMParameterName != nil {
		ssmDescription := props.SSMDescription
		if ssmDescription == nil {
			ssmDescription = jsii.String(fmt.Sprintf("Queue URL for %s", *props.QueueName))
		}

		this.SSMParameter = awsssm.NewStringParameter(this, jsii.String("SSMParameter"), &awsssm.StringParameterProps{
			ParameterName: props.SSMParameterName,
			StringValue:   this.Queue.QueueUrl(),
			Description:   ssmDescription,
		})
	}

	return this
}

// GetQueueUrl returns the queue URL
func (q *LiftSQSQueue) GetQueueUrl() *string {
	return q.Queue.QueueUrl()
}

// GetQueueArn returns the queue ARN
func (q *LiftSQSQueue) GetQueueArn() *string {
	return q.Queue.QueueArn()
}

// GetQueueName returns the queue name
func (q *LiftSQSQueue) GetQueueName() *string {
	return q.Queue.QueueName()
}

// GetDeadLetterQueueUrl returns the DLQ URL (if enabled)
func (q *LiftSQSQueue) GetDeadLetterQueueUrl() *string {
	if q.DeadLetterQueue != nil {
		return q.DeadLetterQueue.QueueUrl()
	}
	return nil
}

// GrantSendMessages grants additional permission to send messages to the queue
func (q *LiftSQSQueue) GrantSendMessages(grantee awslambda.Function) {
	q.Queue.GrantSendMessages(grantee)
}

// GrantConsumeMessages grants additional permission to consume messages from the queue
func (q *LiftSQSQueue) GrantConsumeMessages(grantee awslambda.Function) {
	q.Queue.GrantConsumeMessages(grantee)
}
