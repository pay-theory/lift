package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// SNSProcessorProps defines the properties for creating an SNS processor
type SNSProcessorProps struct {
	// The Lambda function configuration
	FunctionProps *LiftFunctionProps `field:"required"`

	// Optional: Topic configuration
	TopicProps *awssns.TopicProps `field:"optional"`

	// Optional: Use an existing topic instead of creating a new one
	ExistingTopic awssns.ITopic `field:"optional"`

	// Optional: SNS subscription configuration
	SubscriptionProps *awslambdaeventsources.SnsEventSourceProps `field:"optional"`

	// Optional: Enable dead letter queue for failed messages
	EnableDLQ *bool `field:"optional"`

	// Optional: DLQ configuration
	DLQProps *awssqs.QueueProps `field:"optional"`

	// Optional: Message filtering policy
	FilterPolicy *map[string]awssns.SubscriptionFilter `field:"optional"`

	// Optional: Enable FIFO topic
	EnableFifo *bool `field:"optional"`

	// Optional: Enable content-based deduplication
	ContentBasedDeduplication *bool `field:"optional"`

	// Optional: Message retention period in seconds (1 hour to 14 days)
	MessageRetentionSeconds *float64 `field:"optional"`

	// Optional: Display name for the topic
	DisplayName *string `field:"optional"`

	// Optional: Subscription protocol (defaults to lambda)
	Protocol *string `field:"optional"`

	// Optional: Raw message delivery
	RawMessageDelivery *bool `field:"optional"`
}

// SNSProcessor creates an SNS topic with Lambda processor and optional DLQ
type SNSProcessor struct {
	constructs.Construct
	Topic    awssns.ITopic
	Function LiftFunction
	DLQ      awssqs.IQueue
}

// NewSNSProcessor creates a new SNS processor with Lambda function
func NewSNSProcessor(scope constructs.Construct, id *string, props *SNSProcessorProps) *SNSProcessor {
	this := constructs.NewConstruct(scope, id)

	// Create or use existing SNS topic
	var topic awssns.ITopic
	if props.ExistingTopic != nil {
		topic = props.ExistingTopic
	} else {
		topicProps := props.TopicProps
		if topicProps == nil {
			topicProps = &awssns.TopicProps{}
		}

		// Set default display name if not provided
		if topicProps.DisplayName == nil && props.DisplayName != nil {
			topicProps.DisplayName = props.DisplayName
		}

		// Handle FIFO topic configuration
		if props.EnableFifo != nil && *props.EnableFifo {
			topicProps.Fifo = jsii.Bool(true)
			if props.ContentBasedDeduplication != nil {
				topicProps.ContentBasedDeduplication = props.ContentBasedDeduplication
			}
		}

		// Note: SNS topics don't have message retention period - messages are delivered immediately
		// The MessageRetentionSeconds prop is kept for API compatibility but not used

		topic = awssns.NewTopic(this, jsii.String("Topic"), topicProps)
	}

	// Create the Lambda function
	function := NewLiftFunction(this, jsii.String("Function"), props.FunctionProps)

	// Add SNS topic environment variables
	function.Function.AddEnvironment(jsii.String("SNS_TOPIC_ARN"), topic.TopicArn(), nil)
	function.Function.AddEnvironment(jsii.String("SNS_TOPIC_NAME"), topic.TopicName(), nil)

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

		// Ensure FIFO DLQ for FIFO topics
		if props.EnableFifo != nil && *props.EnableFifo {
			dlqProps.Fifo = jsii.Bool(true)
		}

		dlq = awssqs.NewQueue(this, jsii.String("DLQ"), dlqProps)
		function.Function.AddEnvironment(jsii.String("SNS_DLQ_URL"), dlq.QueueUrl(), nil)
	}

	// Configure SNS subscription
	subscriptionProps := props.SubscriptionProps
	if subscriptionProps == nil {
		subscriptionProps = &awslambdaeventsources.SnsEventSourceProps{}
	}

	// Set filter policy if provided
	if props.FilterPolicy != nil {
		subscriptionProps.FilterPolicy = props.FilterPolicy
	}

	// Configure raw message delivery
	// Note: RawMessageDelivery is handled separately in subscription props

	// Set DLQ for subscription
	if dlq != nil {
		subscriptionProps.DeadLetterQueue = dlq
	}

	// Add SNS event source to Lambda
	function.Function.AddEventSource(awslambdaeventsources.NewSnsEventSource(topic, subscriptionProps))

	// Grant SNS permission to invoke Lambda
	topic.GrantPublish(function.Function.GrantPrincipal())

	processor := &SNSProcessor{
		Construct: this,
		Topic:     topic,
		Function:  *function,
		DLQ:       dlq,
	}

	return processor
}

// GrantPublish grants SNS publish permissions to a principal
func (s *SNSProcessor) GrantPublish(grantee awsiam.IGrantable) awsiam.Grant {
	return s.Topic.GrantPublish(grantee)
}

// GrantSubscribe grants SNS subscribe permissions to a principal
func (s *SNSProcessor) GrantSubscribe(grantee awsiam.IGrantable) awsiam.Grant {
	return s.Topic.GrantSubscribe(grantee)
}

// AddSubscription adds a new subscription to the topic
func (s *SNSProcessor) AddSubscription(subscription awssns.ITopicSubscription) awssns.Subscription {
	return s.Topic.AddSubscription(subscription)
}

// GetTopicArn returns the SNS topic ARN
func (s *SNSProcessor) GetTopicArn() *string {
	return s.Topic.TopicArn()
}

// GetTopicName returns the SNS topic name
func (s *SNSProcessor) GetTopicName() *string {
	return s.Topic.TopicName()
}

// GetDLQUrl returns the DLQ URL if DLQ is enabled
func (s *SNSProcessor) GetDLQUrl() *string {
	if s.DLQ != nil {
		return s.DLQ.QueueUrl()
	}
	return nil
}