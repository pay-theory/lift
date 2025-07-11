package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3notifications"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// S3ProcessorProps defines properties for an S3 processor
type S3ProcessorProps struct {
	// Lambda function properties
	FunctionProps awslambda.FunctionProps

	// S3 bucket properties (optional - creates new bucket if not provided)
	BucketProps *awss3.BucketProps

	// Existing bucket to use (optional - creates new if not provided)
	ExistingBucket awss3.IBucket

	// S3 event types to process (default: ObjectCreated)
	EventTypes *[]awss3.EventType

	// Key prefix filter for S3 events (optional)
	KeyPrefix *string

	// Key suffix filter for S3 events (optional)
	KeySuffix *string

	// Dead letter queue properties (optional)
	DeadLetterQueueProps *awssqs.QueueProps

	// Enable dead letter queue (default: true)
	EnableDeadLetterQueue *bool

	// S3 event source configuration
	EventSourceProps *awslambdaeventsources.S3EventSourceProps

	// Additional S3 processor settings
	BatchSize         *float64        // Default: 10
	MaxBatchingWindow awscdk.Duration // Default: 5 seconds

	// Multi-region support
	CrossRegionReplication *bool
	ReplicationBucket      awss3.IBucket

	// Lifecycle rules
	EnableLifecycleRules *bool
	LifecycleRules       *[]*awss3.LifecycleRule

	// External bucket support
	ExternalBucket awss3.IBucket

	// Event filtering
	EventFilter *S3EventFilter

	// Access logging
	EnableAccessLogging *bool
	AccessLogsBucket    awss3.IBucket
	AccessLogsPrefix    *string

	// Versioning and backup
	EnableVersioning *bool
	EnableBackup     *bool

	// Lift-specific settings
	EnableTracing     *bool
	EnableMultiTenant *bool
	EnableMonitoring  *bool
}

// S3EventFilter defines event filtering options
type S3EventFilter struct {
	Prefix *string
	Suffix *string
}

// S3Processor represents an S3 bucket with Lambda processor
type S3Processor struct {
	constructs.Construct

	// The Lambda function processing S3 events
	Function *LiftFunction

	// The S3 bucket
	Bucket awss3.IBucket

	// Dead letter queue (if enabled)
	DeadLetterQueue awssqs.IQueue

	// Event source mapping
	EventSource awslambdaeventsources.S3EventSource

	// Replication bucket (if cross-region replication is enabled)
	ReplicationBucket awss3.IBucket
}

// NewS3Processor creates a new S3 processor construct
func NewS3Processor(scope constructs.Construct, id *string, props *S3ProcessorProps) *S3Processor {
	this := &S3Processor{}
	constructs.NewConstruct_Override(this, scope, id)

	// Set defaults
	if props == nil {
		props = &S3ProcessorProps{}
	}

	// Default values for future use
	// batchSize and maxBatchingWindow are not used in S3 events but kept for potential future use

	enableDLQ := true
	if props.EnableDeadLetterQueue != nil {
		enableDLQ = *props.EnableDeadLetterQueue
	}

	enableVersioning := false
	if props.EnableVersioning != nil {
		enableVersioning = *props.EnableVersioning
	}

	enableLifecycleRules := false
	if props.EnableLifecycleRules != nil {
		enableLifecycleRules = *props.EnableLifecycleRules
	}

	enableAccessLogging := false
	if props.EnableAccessLogging != nil {
		enableAccessLogging = *props.EnableAccessLogging
	}

	// Default event types
	eventTypes := []awss3.EventType{awss3.EventType_OBJECT_CREATED}
	if props.EventTypes != nil {
		eventTypes = *props.EventTypes
	}

	// Create or use existing bucket
	if props.ExistingBucket != nil {
		this.Bucket = props.ExistingBucket
	} else {
		// Create bucket
		bucketProps := &awss3.BucketProps{
			Versioned:          jsii.Bool(enableVersioning),
			BlockPublicAccess:  awss3.BlockPublicAccess_BLOCK_ALL(),
			Encryption:         awss3.BucketEncryption_S3_MANAGED,
			EnforceSSL:         jsii.Bool(true),
			EventBridgeEnabled: jsii.Bool(true),
		}

		// Override with user-provided props
		if props.BucketProps != nil {
			if props.BucketProps.BucketName != nil {
				bucketProps.BucketName = props.BucketProps.BucketName
			}
			if props.BucketProps.Versioned != nil {
				bucketProps.Versioned = props.BucketProps.Versioned
			}
			if props.BucketProps.BlockPublicAccess != nil {
				bucketProps.BlockPublicAccess = props.BucketProps.BlockPublicAccess
			}
			if props.BucketProps.EncryptionKey != nil {
				bucketProps.EncryptionKey = props.BucketProps.EncryptionKey
			}
		}

		// Set default bucket name if not provided
		if bucketProps.BucketName == nil && props.FunctionProps.FunctionName != nil {
			bucketProps.BucketName = jsii.String(*props.FunctionProps.FunctionName + "-bucket")
		}

		// Configure access logging
		if enableAccessLogging && props.AccessLogsBucket != nil {
			bucketProps.ServerAccessLogsBucket = props.AccessLogsBucket
			if props.AccessLogsPrefix != nil {
				bucketProps.ServerAccessLogsPrefix = props.AccessLogsPrefix
			}
		}

		// Configure lifecycle rules
		if enableLifecycleRules && props.LifecycleRules != nil {
			bucketProps.LifecycleRules = props.LifecycleRules
		} else if enableLifecycleRules {
			// Default lifecycle rules
			defaultLifecycleRules := []*awss3.LifecycleRule{
				{
					Id:                                  jsii.String("DeleteIncompleteMultipartUploads"),
					Enabled:                             jsii.Bool(true),
					AbortIncompleteMultipartUploadAfter: awscdk.Duration_Days(jsii.Number(1)),
				},
				{
					Id:      jsii.String("TransitionToIA"),
					Enabled: jsii.Bool(true),
					Transitions: &[]*awss3.Transition{
						{
							StorageClass:    awss3.StorageClass_INFREQUENT_ACCESS(),
							TransitionAfter: awscdk.Duration_Days(jsii.Number(30)),
						},
					},
				},
			}
			bucketProps.LifecycleRules = &defaultLifecycleRules
		}

		this.Bucket = awss3.NewBucket(this, jsii.String("Bucket"), bucketProps)

		// Configure cross-region replication if enabled
		if props.CrossRegionReplication != nil && *props.CrossRegionReplication {
			if props.ReplicationBucket != nil {
				this.ReplicationBucket = props.ReplicationBucket
				this.enableCrossRegionReplication()
			}
		}
	}

	// Create dead letter queue if enabled
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
			dlqProps.QueueName = jsii.String(*props.FunctionProps.FunctionName + "-s3-dlq")
		}

		this.DeadLetterQueue = awssqs.NewQueue(this, jsii.String("DeadLetterQueue"), dlqProps)
	}

	// Create Lambda function with S3 environment variables
	functionEnv := make(map[string]*string)
	if props.FunctionProps.Environment != nil {
		for k, v := range *props.FunctionProps.Environment {
			functionEnv[k] = v
		}
	}

	// Add S3-specific environment variables
	functionEnv["S3_BUCKET_NAME"] = this.Bucket.BucketName()
	functionEnv["S3_BUCKET_ARN"] = this.Bucket.BucketArn()
	if this.DeadLetterQueue != nil {
		functionEnv["S3_DLQ_URL"] = this.DeadLetterQueue.QueueUrl()
	}
	if this.ReplicationBucket != nil {
		functionEnv["S3_REPLICATION_BUCKET_NAME"] = this.ReplicationBucket.BucketName()
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

	// Disable Lambda DLQ when S3 DLQ is disabled to avoid confusion
	if !enableDLQ {
		liftProps.EnableDeadLetterQueue = jsii.Bool(false)
	}

	this.Function = NewLiftFunction(this, jsii.String("Function"), liftProps)

	// Configure S3 event source
	eventSourceProps := &awslambdaeventsources.S3EventSourceProps{
		Events: &eventTypes,
	}

	// Add key filters if provided
	filters := []*awss3.NotificationKeyFilter{}
	if props.KeyPrefix != nil {
		filters = append(filters, &awss3.NotificationKeyFilter{
			Prefix: props.KeyPrefix,
		})
	}
	if props.KeySuffix != nil {
		filters = append(filters, &awss3.NotificationKeyFilter{
			Suffix: props.KeySuffix,
		})
	}
	if len(filters) > 0 {
		eventSourceProps.Filters = &filters
	}

	// Override with user-provided event source props
	if props.EventSourceProps != nil {
		if props.EventSourceProps.Events != nil {
			eventSourceProps.Events = props.EventSourceProps.Events
		}
		if props.EventSourceProps.Filters != nil {
			eventSourceProps.Filters = props.EventSourceProps.Filters
		}
	}

	// Create and add event source
	if bucket, ok := this.Bucket.(awss3.Bucket); ok {
		this.EventSource = awslambdaeventsources.NewS3EventSource(bucket, eventSourceProps)
		this.Function.Function.AddEventSource(this.EventSource)
	} else {
		// For external buckets, we need to handle event source differently
		// External buckets require bucket notification configuration
		if props.ExternalBucket != nil {
			// Add bucket notification for external bucket
			bucket.AddEventNotification(
				awss3.EventType_OBJECT_CREATED,
				awss3notifications.NewLambdaDestination(this.Function.Function),
				&awss3.NotificationKeyFilter{
					Prefix: props.EventFilter.Prefix,
					Suffix: props.EventFilter.Suffix,
				},
			)
		}
	}

	// Grant permissions
	this.Bucket.GrantRead(this.Function.Function, jsii.String("*"))
	this.Bucket.GrantWrite(this.Function.Function, jsii.String("*"), nil)
	if this.DeadLetterQueue != nil {
		this.DeadLetterQueue.GrantSendMessages(this.Function.Function)
	}
	if this.ReplicationBucket != nil {
		this.ReplicationBucket.GrantWrite(this.Function.Function, jsii.String("*"), nil)
	}

	// Add monitoring if enabled
	if props.EnableMonitoring != nil && *props.EnableMonitoring {
		this.enableMonitoring()
	}

	return this
}

// enableMonitoring adds CloudWatch alarms and metrics for the S3 processor
func (s *S3Processor) enableMonitoring() {
	// Create SNS topic for alerts
	_ = awssns.NewTopic(s, jsii.String("AlarmTopic"), &awssns.TopicProps{
		TopicName:   jsii.String(fmt.Sprintf("%s-alarms", *s.Bucket.BucketName())),
		DisplayName: jsii.String(fmt.Sprintf("Alarms for %s processor", *s.Bucket.BucketName())),
	})

	// Lambda function monitoring
	if s.Function != nil {
		function := s.Function.GetFunction()

		// Function error alarm
		awscloudwatch.NewAlarm(s, jsii.String("FunctionErrorAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-processor-errors", *s.Bucket.BucketName())),
			AlarmDescription: jsii.String("S3 processor function errors"),
			Metric: function.MetricErrors(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(5),
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Function throttles alarm
		awscloudwatch.NewAlarm(s, jsii.String("FunctionThrottleAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-processor-throttles", *s.Bucket.BucketName())),
			AlarmDescription: jsii.String("S3 processor function throttled"),
			Metric: function.MetricThrottles(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(1),
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Function duration alarm
		awscloudwatch.NewAlarm(s, jsii.String("FunctionDurationAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-processor-duration", *s.Bucket.BucketName())),
			AlarmDescription: jsii.String("S3 processor taking too long"),
			Metric: function.MetricDuration(&awscloudwatch.MetricOptions{
				Period:    awscdk.Duration_Minutes(jsii.Number(5)),
				Statistic: awscloudwatch.Stats_AVERAGE(),
			}),
			Threshold:          jsii.Number(30000), // 30 seconds
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Function concurrent executions alarm
		concurrentExecutionsMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Lambda"),
			MetricName: jsii.String("ConcurrentExecutions"),
			DimensionsMap: &map[string]*string{
				"FunctionName": function.FunctionName(),
			},
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_MAXIMUM(),
		})

		awscloudwatch.NewAlarm(s, jsii.String("FunctionConcurrencyAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-processor-concurrency", *s.Bucket.BucketName())),
			AlarmDescription:   jsii.String("S3 processor high concurrent executions"),
			Metric:             concurrentExecutionsMetric,
			Threshold:          jsii.Number(900), // Near default Lambda limit
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// S3 bucket metrics
	var client4xxErrorsMetric awscloudwatch.IMetric
	var server5xxErrorsMetric awscloudwatch.IMetric
	var objectSizeMetric awscloudwatch.IMetric
	var objectCountMetric awscloudwatch.IMetric

	if s.Bucket != nil {
		// 4xx errors alarm
		client4xxErrorsMetric = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/S3"),
			MetricName: jsii.String("4xxErrors"),
			DimensionsMap: &map[string]*string{
				"BucketName": s.Bucket.BucketName(),
			},
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_SUM(),
		})

		awscloudwatch.NewAlarm(s, jsii.String("Bucket4xxErrorsAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-4xx-errors", *s.Bucket.BucketName())),
			AlarmDescription:   jsii.String("S3 bucket 4xx errors"),
			Metric:             client4xxErrorsMetric,
			Threshold:          jsii.Number(10),
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// 5xx errors alarm
		server5xxErrorsMetric = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/S3"),
			MetricName: jsii.String("5xxErrors"),
			DimensionsMap: &map[string]*string{
				"BucketName": s.Bucket.BucketName(),
			},
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_SUM(),
		})

		awscloudwatch.NewAlarm(s, jsii.String("Bucket5xxErrorsAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-5xx-errors", *s.Bucket.BucketName())),
			AlarmDescription:   jsii.String("S3 bucket 5xx errors"),
			Metric:             server5xxErrorsMetric,
			Threshold:          jsii.Number(1),
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Object size monitoring (for large file detection)
		objectSizeMetric = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/S3"),
			MetricName: jsii.String("BucketSizeBytes"),
			DimensionsMap: &map[string]*string{
				"BucketName":  s.Bucket.BucketName(),
				"StorageType": jsii.String("StandardStorage"),
			},
			Period:    awscdk.Duration_Days(jsii.Number(1)),
			Statistic: awscloudwatch.Stats_AVERAGE(),
		})

		awscloudwatch.NewAlarm(s, jsii.String("BucketSizeAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-bucket-size", *s.Bucket.BucketName())),
			AlarmDescription:   jsii.String("S3 bucket size is large"),
			Metric:             objectSizeMetric,
			Threshold:          jsii.Number(100 * 1024 * 1024 * 1024), // 100 GB
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Number of objects monitoring
		objectCountMetric = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/S3"),
			MetricName: jsii.String("NumberOfObjects"),
			DimensionsMap: &map[string]*string{
				"BucketName":  s.Bucket.BucketName(),
				"StorageType": jsii.String("AllStorageTypes"),
			},
			Period:    awscdk.Duration_Days(jsii.Number(1)),
			Statistic: awscloudwatch.Stats_AVERAGE(),
		})

		awscloudwatch.NewAlarm(s, jsii.String("ObjectCountAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-object-count", *s.Bucket.BucketName())),
			AlarmDescription:   jsii.String("S3 bucket has many objects"),
			Metric:             objectCountMetric,
			Threshold:          jsii.Number(1000000), // 1 million objects
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// DLQ monitoring if enabled
	if s.DeadLetterQueue != nil {
		awscloudwatch.NewAlarm(s, jsii.String("DLQMessagesAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-processor-dlq-messages", *s.Bucket.BucketName())),
			AlarmDescription: jsii.String("Messages in S3 processor dead letter queue"),
			Metric: s.DeadLetterQueue.MetricApproximateNumberOfMessagesVisible(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(1),
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// Create CloudWatch dashboard
	dashboard := awscloudwatch.NewDashboard(s, jsii.String("ProcessorDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(fmt.Sprintf("%s-processor-dashboard", *s.Bucket.BucketName())),
	})

	// Add widgets to dashboard
	if s.Function != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("Lambda Function Metrics"),
				Left: &[]awscloudwatch.IMetric{
					s.Function.GetFunction().MetricInvocations(nil),
					s.Function.GetFunction().MetricErrors(nil),
					s.Function.GetFunction().MetricThrottles(nil),
				},
				Right: &[]awscloudwatch.IMetric{
					s.Function.GetFunction().MetricDuration(nil),
				},
			}),
		)
	}

	if s.Bucket != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("S3 Bucket Metrics"),
				Left: &[]awscloudwatch.IMetric{
					client4xxErrorsMetric,
					server5xxErrorsMetric,
				},
			}),
			awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
				Title: jsii.String("Bucket Storage"),
				Metrics: &[]awscloudwatch.IMetric{
					objectSizeMetric,
					objectCountMetric,
				},
			}),
		)
	}

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
}

// GrantRead grants permission to read from the bucket
func (s *S3Processor) GrantRead(grantee awslambda.IFunction) {
	s.Bucket.GrantRead(grantee, jsii.String("*"))
}

// GrantWrite grants permission to write to the bucket
func (s *S3Processor) GrantWrite(grantee awslambda.IFunction) {
	s.Bucket.GrantWrite(grantee, jsii.String("*"), nil)
}

// GrantReadWrite grants permission to read and write to the bucket
func (s *S3Processor) GrantReadWrite(grantee awslambda.IFunction) {
	s.Bucket.GrantReadWrite(grantee, jsii.String("*"))
}

// GrantDelete grants permission to delete objects from the bucket
func (s *S3Processor) GrantDelete(grantee awslambda.IFunction) {
	s.Bucket.GrantDelete(grantee, jsii.String("*"))
}

// AddEnvironmentVariable adds an environment variable to the Lambda function
func (s *S3Processor) AddEnvironmentVariable(key string, value string) {
	s.Function.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
}

// GetBucketName returns the bucket name
func (s *S3Processor) GetBucketName() *string {
	return s.Bucket.BucketName()
}

// GetBucketArn returns the bucket ARN
func (s *S3Processor) GetBucketArn() *string {
	return s.Bucket.BucketArn()
}

// GetBucketDomainName returns the bucket domain name
func (s *S3Processor) GetBucketDomainName() *string {
	return s.Bucket.BucketDomainName()
}

// AddCorsRule adds a CORS rule to the bucket
func (s *S3Processor) AddCorsRule(rule *awss3.CorsRule) {
	// Note: CORS rules can only be added to concrete Bucket instances
	if bucket, ok := s.Bucket.(awss3.Bucket); ok {
		bucket.AddCorsRule(rule)
	}
}

// EnableCORS enables CORS on the bucket
func (s *S3Processor) EnableCORS(rules []*awss3.CorsRule) {
	// Note: CORS rules need to be set during bucket creation
	// This is a helper method for documentation purposes
	// In practice, CORS should be configured in BucketProps
	for _, rule := range rules {
		s.AddCorsRule(rule)
	}
}

// SetBucketPolicy sets a bucket policy
func (s *S3Processor) SetBucketPolicy(policy map[string]interface{}) {
	// Parse statements from the policy map
	if statements, ok := policy["Statement"].([]interface{}); ok {
		for _, stmt := range statements {
			if stmtMap, ok := stmt.(map[string]interface{}); ok {
				policyStatement := s.parsePolicyStatement(stmtMap)

				// Apply each policy statement to the bucket
				if bucket, ok := s.Bucket.(awss3.Bucket); ok {
					bucket.AddToResourcePolicy(policyStatement)
				}
			}
		}
	}
}

// parsePolicyStatement converts a map to PolicyStatement
func (s *S3Processor) parsePolicyStatement(stmt map[string]interface{}) awsiam.PolicyStatement {
	props := &awsiam.PolicyStatementProps{}

	// Set Effect
	if effect, ok := stmt["Effect"].(string); ok {
		if effect == "Allow" {
			props.Effect = awsiam.Effect_ALLOW
		} else {
			props.Effect = awsiam.Effect_DENY
		}
	}

	// Set Actions
	var actionList []*string
	if actions, ok := stmt["Action"].([]interface{}); ok {
		for _, action := range actions {
			if actionStr, ok := action.(string); ok {
				actionList = append(actionList, jsii.String(actionStr))
			}
		}
	} else if action, ok := stmt["Action"].(string); ok {
		actionList = append(actionList, jsii.String(action))
	}
	if len(actionList) > 0 {
		props.Actions = &actionList
	}

	// Set Resources
	var resourceList []*string
	if resources, ok := stmt["Resource"].([]interface{}); ok {
		for _, resource := range resources {
			if resourceStr, ok := resource.(string); ok {
				resourceList = append(resourceList, jsii.String(resourceStr))
			}
		}
	} else if resource, ok := stmt["Resource"].(string); ok {
		resourceList = append(resourceList, jsii.String(resource))
	}
	if len(resourceList) > 0 {
		props.Resources = &resourceList
	}

	// Set Principals
	var principals []awsiam.IPrincipal
	if principal, ok := stmt["Principal"].(map[string]interface{}); ok {
		if aws, ok := principal["AWS"].(string); ok {
			principals = append(principals, awsiam.NewAccountPrincipal(jsii.String(aws)))
		}
		if service, ok := principal["Service"].(string); ok {
			principals = append(principals, awsiam.NewServicePrincipal(jsii.String(service), nil))
		}
	}
	if len(principals) > 0 {
		props.Principals = &principals
	}

	return awsiam.NewPolicyStatement(props)
}

// enableCrossRegionReplication sets up cross-region replication
func (s *S3Processor) enableCrossRegionReplication() {
	// Create replication role
	replicationRole := awsiam.NewRole(s, jsii.String("ReplicationRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("s3.amazonaws.com"), nil),
		Path:      jsii.String("/"),
	})

	// Grant permissions to read from source bucket
	s.Bucket.GrantRead(replicationRole, jsii.String("*"))

	// Grant permissions to replicate to destination bucket
	if s.ReplicationBucket != nil {
		s.ReplicationBucket.GrantWrite(replicationRole, jsii.String("*"), nil)

		// Add replication configuration
		if sourceBucket, ok := s.Bucket.(awss3.Bucket); ok {
			cfnBucket := sourceBucket.Node().DefaultChild().(awss3.CfnBucket)

			replicationConfig := &awss3.CfnBucket_ReplicationConfigurationProperty{
				Role: replicationRole.RoleArn(),
				Rules: &[]awss3.CfnBucket_ReplicationRuleProperty{
					{
						Id:       jsii.String("ReplicateAll"),
						Status:   jsii.String("Enabled"),
						Priority: jsii.Number(1),
						Filter:   &awss3.CfnBucket_ReplicationRuleFilterProperty{},
						Destination: &awss3.CfnBucket_ReplicationDestinationProperty{
							Bucket:       s.ReplicationBucket.BucketArn(),
							StorageClass: jsii.String("STANDARD_IA"),
						},
					},
				},
			}

			cfnBucket.SetReplicationConfiguration(replicationConfig)
		}
	}
}
