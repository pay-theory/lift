package constructs

import (
	"fmt"
	
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// SQSProcessorProps defines properties for an SQS processor
type SQSProcessorProps struct {
	// Lambda function properties
	FunctionProps awslambda.FunctionProps

	// SQS queue properties (optional - creates new queue if not provided)
	QueueProps *awssqs.QueueProps

	// Existing queue to use (optional - creates new if not provided)
	ExistingQueue awssqs.IQueue

	// Dead letter queue properties (optional)
	DeadLetterQueueProps *awssqs.QueueProps

	// Enable dead letter queue (default: true)
	EnableDeadLetterQueue *bool

	// SQS event source configuration
	EventSourceProps *awslambdaeventsources.SqsEventSourceProps

	// Additional SQS processor settings
	BatchSize           *float64 // Default: 10
	MaxBatchingWindow   awscdk.Duration // Default: 5 seconds
	VisibilityTimeout   awscdk.Duration // Default: 6 times function timeout
	MessageRetentionPeriod awscdk.Duration // Default: 14 days
	MaxReceiveCount     *float64 // Default: 3
	EnableContentBasedDeduplication *bool // For FIFO queues
	FifoQueue          *bool     // Default: false
	ReceiveMessageWaitTimeSeconds *float64 // For long polling (0-20)

	// Lift-specific settings
	EnableTracing     *bool
	EnableMultiTenant *bool
	EnableMonitoring  *bool
}

// SQSProcessor represents an SQS queue with Lambda processor
type SQSProcessor struct {
	constructs.Construct

	// The Lambda function processing SQS messages
	Function *LiftFunction

	// The SQS queue
	Queue awssqs.IQueue

	// Dead letter queue (if enabled)
	DeadLetterQueue awssqs.IQueue

	// Event source mapping
	EventSource awslambdaeventsources.SqsEventSource
}

// NewSQSProcessor creates a new SQS processor construct
func NewSQSProcessor(scope constructs.Construct, id *string, props *SQSProcessorProps) *SQSProcessor {
	this := &SQSProcessor{}
	constructs.NewConstruct_Override(this, scope, id)

	// Set defaults
	if props == nil {
		props = &SQSProcessorProps{}
	}

	// Default values
	batchSize := float64(10)
	if props.BatchSize != nil {
		batchSize = *props.BatchSize
	}

	maxBatchingWindow := awscdk.Duration_Seconds(jsii.Number(5))
	if props.MaxBatchingWindow != nil {
		maxBatchingWindow = props.MaxBatchingWindow
	}

	visibilityTimeout := awscdk.Duration_Minutes(jsii.Number(5))
	if props.VisibilityTimeout != nil {
		visibilityTimeout = props.VisibilityTimeout
	}

	messageRetentionPeriod := awscdk.Duration_Days(jsii.Number(14))
	if props.MessageRetentionPeriod != nil {
		messageRetentionPeriod = props.MessageRetentionPeriod
	}

	maxReceiveCount := float64(3)
	if props.MaxReceiveCount != nil {
		maxReceiveCount = *props.MaxReceiveCount
	}

	enableDLQ := true
	if props.EnableDeadLetterQueue != nil {
		enableDLQ = *props.EnableDeadLetterQueue
	}

	fifoQueue := false
	if props.FifoQueue != nil {
		fifoQueue = *props.FifoQueue
	}

	longPollingWaitTime := float64(0)
	if props.ReceiveMessageWaitTimeSeconds != nil {
		longPollingWaitTime = *props.ReceiveMessageWaitTimeSeconds
	}

	// Create or use existing queue
	if props.ExistingQueue != nil {
		this.Queue = props.ExistingQueue
	} else {
		// Create dead letter queue if enabled
		var dlqConfig *awssqs.DeadLetterQueue
		if enableDLQ {
			dlqProps := &awssqs.QueueProps{}
			if props.DeadLetterQueueProps != nil {
				dlqProps = props.DeadLetterQueueProps
			}

			// Set DLQ defaults
			if dlqProps.RetentionPeriod == nil {
				dlqProps.RetentionPeriod = awscdk.Duration_Days(jsii.Number(14))
			}
			if dlqProps.QueueName == nil && props.FunctionProps.FunctionName != nil {
				dlqProps.QueueName = jsii.String(*props.FunctionProps.FunctionName + "-dlq")
			}

			// Handle FIFO DLQ suffix
			if fifoQueue && dlqProps.QueueName != nil {
				queueName := *dlqProps.QueueName
				if len(queueName) < 5 || queueName[len(queueName)-5:] != ".fifo" {
					dlqProps.QueueName = jsii.String(queueName + ".fifo")
				}
				dlqProps.Fifo = jsii.Bool(true)
				if props.EnableContentBasedDeduplication != nil {
					dlqProps.ContentBasedDeduplication = props.EnableContentBasedDeduplication
				}
			}

			this.DeadLetterQueue = awssqs.NewQueue(this, jsii.String("DeadLetterQueue"), dlqProps)

			dlqConfig = &awssqs.DeadLetterQueue{
				MaxReceiveCount: jsii.Number(maxReceiveCount),
				Queue:          this.DeadLetterQueue,
			}
		}

		// Create main queue
		queueProps := &awssqs.QueueProps{
			VisibilityTimeout:       visibilityTimeout,
			RetentionPeriod:        messageRetentionPeriod,
			DeadLetterQueue:        dlqConfig,
			ReceiveMessageWaitTime: awscdk.Duration_Seconds(jsii.Number(longPollingWaitTime)),
		}

		// Override with user-provided props
		if props.QueueProps != nil {
			if props.QueueProps.QueueName != nil {
				queueProps.QueueName = props.QueueProps.QueueName
			}
			if props.QueueProps.VisibilityTimeout != nil {
				queueProps.VisibilityTimeout = props.QueueProps.VisibilityTimeout
			}
			if props.QueueProps.RetentionPeriod != nil {
				queueProps.RetentionPeriod = props.QueueProps.RetentionPeriod
			}
			if props.QueueProps.ReceiveMessageWaitTime != nil {
				queueProps.ReceiveMessageWaitTime = props.QueueProps.ReceiveMessageWaitTime
			}
		}

		// FIFO queue configuration
		if fifoQueue {
			queueProps.Fifo = jsii.Bool(true)
			if props.EnableContentBasedDeduplication != nil {
				queueProps.ContentBasedDeduplication = props.EnableContentBasedDeduplication
			}

			// Ensure FIFO queue name ends with .fifo
			if queueProps.QueueName != nil {
				queueName := *queueProps.QueueName
				if len(queueName) < 5 || queueName[len(queueName)-5:] != ".fifo" {
					queueProps.QueueName = jsii.String(queueName + ".fifo")
				}
			}
		}

		// Set default queue name if not provided
		if queueProps.QueueName == nil && props.FunctionProps.FunctionName != nil {
			suffix := ""
			if fifoQueue {
				suffix = ".fifo"
			}
			queueProps.QueueName = jsii.String(*props.FunctionProps.FunctionName + "-queue" + suffix)
		}

		this.Queue = awssqs.NewQueue(this, jsii.String("Queue"), queueProps)
	}

	// Create Lambda function with SQS environment variables
	functionEnv := make(map[string]*string)
	if props.FunctionProps.Environment != nil {
		for k, v := range *props.FunctionProps.Environment {
			functionEnv[k] = v
		}
	}

	// Add SQS-specific environment variables
	functionEnv["SQS_QUEUE_URL"] = this.Queue.QueueUrl()
	if this.DeadLetterQueue != nil {
		functionEnv["SQS_DLQ_URL"] = this.DeadLetterQueue.QueueUrl()
	}

	// Create LiftFunction with enhanced properties
	liftProps := &LiftFunctionProps{
		FunctionProps: props.FunctionProps,
	}

	// Override environment
	liftProps.FunctionProps.Environment = &functionEnv

	// Set Lift-specific properties
	if props.EnableTracing != nil {
		liftProps.EnableTracing = props.EnableTracing
	}
	if props.EnableMultiTenant != nil {
		liftProps.EnableMultiTenant = props.EnableMultiTenant
	}
	
	// Disable Lambda DLQ when SQS DLQ is disabled to avoid confusion
	if !enableDLQ {
		liftProps.EnableDeadLetterQueue = jsii.Bool(false)
	}

	this.Function = NewLiftFunction(this, jsii.String("Function"), liftProps)

	// Configure SQS event source
	eventSourceProps := &awslambdaeventsources.SqsEventSourceProps{
		BatchSize:               jsii.Number(batchSize),
		ReportBatchItemFailures: jsii.Bool(true),
	}

	// Don't set batching window for FIFO queues (not supported)
	if !fifoQueue {
		eventSourceProps.MaxBatchingWindow = maxBatchingWindow
	}

	// Override with user-provided event source props
	if props.EventSourceProps != nil {
		if props.EventSourceProps.BatchSize != nil {
			eventSourceProps.BatchSize = props.EventSourceProps.BatchSize
		}
		// Only set batching window for non-FIFO queues
		if props.EventSourceProps.MaxBatchingWindow != nil && !fifoQueue {
			eventSourceProps.MaxBatchingWindow = props.EventSourceProps.MaxBatchingWindow
		}
		if props.EventSourceProps.ReportBatchItemFailures != nil {
			eventSourceProps.ReportBatchItemFailures = props.EventSourceProps.ReportBatchItemFailures
		}
		if props.EventSourceProps.MaxConcurrency != nil {
			eventSourceProps.MaxConcurrency = props.EventSourceProps.MaxConcurrency
		}
		if props.EventSourceProps.Enabled != nil {
			eventSourceProps.Enabled = props.EventSourceProps.Enabled
		}
		if props.EventSourceProps.Filters != nil {
			eventSourceProps.Filters = props.EventSourceProps.Filters
		}
	}

	// Create and add event source
	this.EventSource = awslambdaeventsources.NewSqsEventSource(this.Queue, eventSourceProps)
	this.Function.Function.AddEventSource(this.EventSource)

	// Grant permissions
	this.Queue.GrantConsumeMessages(this.Function.Function)
	if this.DeadLetterQueue != nil {
		this.DeadLetterQueue.GrantSendMessages(this.Function.Function)
	}

	// Add monitoring if enabled
	if props.EnableMonitoring != nil && *props.EnableMonitoring {
		this.enableMonitoring()
	}

	return this
}

// enableMonitoring adds CloudWatch alarms and metrics for the SQS processor
func (s *SQSProcessor) enableMonitoring() {
	// Create SNS topic for alerts
	_ = awssns.NewTopic(s, jsii.String("AlarmTopic"), &awssns.TopicProps{
		TopicName: jsii.String(fmt.Sprintf("%s-alarms", *s.Queue.QueueName())),
		DisplayName: jsii.String(fmt.Sprintf("Alarms for %s", *s.Queue.QueueName())),
	})
	
	// Queue depth alarm - warns when messages accumulate
	awscloudwatch.NewAlarm(s, jsii.String("QueueDepthAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-queue-depth", *s.Queue.QueueName())),
		AlarmDescription:  jsii.String("Queue depth is too high"),
		Metric: s.Queue.MetricApproximateNumberOfMessagesVisible(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(1000),
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	
	// Message age alarm - warns when messages are not processed quickly
	awscloudwatch.NewAlarm(s, jsii.String("MessageAgeAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-message-age", *s.Queue.QueueName())),
		AlarmDescription:  jsii.String("Messages are aging in queue"),
		Metric: s.Queue.MetricApproximateAgeOfOldestMessage(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(300), // 5 minutes
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	
	// DLQ message count alarm
	if s.DeadLetterQueue != nil {
		awscloudwatch.NewAlarm(s, jsii.String("DLQMessagesAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-dlq-messages", *s.Queue.QueueName())),
			AlarmDescription:  jsii.String("Messages in dead letter queue"),
			Metric: s.DeadLetterQueue.MetricApproximateNumberOfMessagesVisible(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:         jsii.Number(1),
			EvaluationPeriods: jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}
	
	// Lambda function monitoring
	if s.Function != nil && s.Function.Function != nil {
		function := s.Function.Function
		
		// Function error rate alarm
		awscloudwatch.NewAlarm(s, jsii.String("FunctionErrorAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-processor-errors", *s.Queue.QueueName())),
			AlarmDescription:  jsii.String("SQS processor function errors"),
			Metric: function.MetricErrors(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:         jsii.Number(5),
			EvaluationPeriods: jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
		
		// Function throttles alarm
		awscloudwatch.NewAlarm(s, jsii.String("FunctionThrottleAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-processor-throttles", *s.Queue.QueueName())),
			AlarmDescription:  jsii.String("SQS processor function throttled"),
			Metric: function.MetricThrottles(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:         jsii.Number(1),
			EvaluationPeriods: jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
		
		// Function duration alarm
		awscloudwatch.NewAlarm(s, jsii.String("FunctionDurationAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-processor-duration", *s.Queue.QueueName())),
			AlarmDescription:  jsii.String("SQS processor taking too long"),
			Metric: function.MetricDuration(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
				Statistic: awscloudwatch.Stats_AVERAGE(),
			}),
			Threshold:         jsii.Number(30000), // 30 seconds
			EvaluationPeriods: jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}
	
	// Queue in-flight messages alarm
	awscloudwatch.NewAlarm(s, jsii.String("InFlightMessagesAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-in-flight-messages", *s.Queue.QueueName())),
		AlarmDescription:  jsii.String("Too many messages in flight"),
		Metric: s.Queue.MetricApproximateNumberOfMessagesNotVisible(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(5000),
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	
	// Create CloudWatch dashboard
	dashboard := awscloudwatch.NewDashboard(s, jsii.String("ProcessorDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(fmt.Sprintf("%s-processor-dashboard", *s.Queue.QueueName())),
	})
	
	// Add widgets to dashboard
	dashboard.AddWidgets(
		awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
			Title: jsii.String("Queue Metrics"),
			Left: &[]awscloudwatch.IMetric{
				s.Queue.MetricApproximateNumberOfMessagesVisible(nil),
				s.Queue.MetricApproximateNumberOfMessagesNotVisible(nil),
			},
			Right: &[]awscloudwatch.IMetric{
				s.Queue.MetricApproximateAgeOfOldestMessage(nil),
			},
		}),
	)
	
	if s.DeadLetterQueue != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
				Title: jsii.String("Dead Letter Queue"),
				Metrics: &[]awscloudwatch.IMetric{
					s.DeadLetterQueue.MetricApproximateNumberOfMessagesVisible(nil),
				},
			}),
		)
	}
	
	if s.Function != nil && s.Function.Function != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("Lambda Function Metrics"),
				Left: &[]awscloudwatch.IMetric{
					s.Function.Function.MetricInvocations(nil),
					s.Function.Function.MetricErrors(nil),
					s.Function.Function.MetricThrottles(nil),
				},
				Right: &[]awscloudwatch.IMetric{
					s.Function.Function.MetricDuration(nil),
				},
			}),
		)
	}
}

// GrantSendMessages grants permission to send messages to the queue
func (s *SQSProcessor) GrantSendMessages(grantee awslambda.IFunction) {
	s.Queue.GrantSendMessages(grantee)
}

// GrantConsumeMessages grants permission to consume messages from the queue
func (s *SQSProcessor) GrantConsumeMessages(grantee awslambda.IFunction) {
	s.Queue.GrantConsumeMessages(grantee)
}

// AddEnvironmentVariable adds an environment variable to the Lambda function
func (s *SQSProcessor) AddEnvironmentVariable(key string, value string) {
	s.Function.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
}

// GetQueueName returns the queue name
func (s *SQSProcessor) GetQueueName() *string {
	return s.Queue.QueueName()
}

// GetQueueUrl returns the queue URL
func (s *SQSProcessor) GetQueueUrl() *string {
	return s.Queue.QueueUrl()
}

// GetQueueArn returns the queue ARN
func (s *SQSProcessor) GetQueueArn() *string {
	return s.Queue.QueueArn()
}