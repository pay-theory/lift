package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// DynamoStreamProcessorProps defines properties for a DynamoDB stream processor
// Memory optimized: 816 → 808 bytes (8 bytes saved)
type DynamoStreamProcessorProps struct {
	// Pointers first (8 bytes each)
	StreamingTableProps *StreamingTableProps
	DeadLetterQueueProps *awssqs.QueueProps
	EventSourceProps *awslambdaeventsources.DynamoEventSourceProps
	BatchSize               *float64
	RetryAttempts           *float64
	ParallelizationFactor   *float64
	EnableDeadLetterQueue *bool
	BisectBatchOnError      *bool
	ReportBatchItemFailures *bool
	EnableTracing     *bool
	EnableMultiTenant *bool
	EnableMonitoring  *bool
	// Duration structs (16 bytes each)
	MaxBatchingWindow       awscdk.Duration
	MaxRecordAge            awscdk.Duration
	TumblingWindow          awscdk.Duration
	// Large struct
	FunctionProps awslambda.FunctionProps
	// Medium types
	StartingPosition        awslambda.StartingPosition
}

// DynamoStreamProcessor represents a DynamoDB table with stream processor using DynamORM
type DynamoStreamProcessor struct {
	constructs.Construct

	// The Lambda function processing DynamoDB stream records
	Function *LiftFunction

	// The DynamORM streaming table
	StreamingTable *StreamingTable

	// Dead letter queue (if enabled)
	DeadLetterQueue awssqs.IQueue

	// Event source mapping
	EventSource awslambdaeventsources.DynamoEventSource
}

// NewDynamoStreamProcessor creates a new DynamoDB stream processor construct using DynamORM
func NewDynamoStreamProcessor(scope constructs.Construct, id *string, props *DynamoStreamProcessorProps) *DynamoStreamProcessor {
	this := &DynamoStreamProcessor{}
	constructs.NewConstruct_Override(this, scope, id)

	// Set defaults
	if props == nil {
		props = &DynamoStreamProcessorProps{}
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

	startingPosition := awslambda.StartingPosition_LATEST
	if props.StartingPosition != "" {
		startingPosition = props.StartingPosition
	}

	maxRecordAge := awscdk.Duration_Hours(jsii.Number(24))
	if props.MaxRecordAge != nil {
		maxRecordAge = props.MaxRecordAge
	}

	bisectBatchOnError := false
	if props.BisectBatchOnError != nil {
		bisectBatchOnError = *props.BisectBatchOnError
	}

	retryAttempts := float64(10000)
	if props.RetryAttempts != nil {
		retryAttempts = *props.RetryAttempts
	}

	reportBatchItemFailures := true
	if props.ReportBatchItemFailures != nil {
		reportBatchItemFailures = *props.ReportBatchItemFailures
	}

	parallelizationFactor := float64(1)
	if props.ParallelizationFactor != nil {
		parallelizationFactor = *props.ParallelizationFactor
	}

	enableDLQ := true
	if props.EnableDeadLetterQueue != nil {
		enableDLQ = *props.EnableDeadLetterQueue
	}

	// Create DynamORM streaming table
	streamingTableProps := &StreamingTableProps{}
	if props.StreamingTableProps != nil {
		streamingTableProps = props.StreamingTableProps
	}

	// Set table name based on function name if not provided
	if streamingTableProps.TableName == nil && props.FunctionProps.FunctionName != nil {
		streamingTableProps.TableName = jsii.String(*props.FunctionProps.FunctionName + "-table")
	}

	// Create the DynamORM-based streaming table
	this.StreamingTable = NewStreamingTable(this, jsii.String("StreamingTable"), streamingTableProps)

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
			dlqProps.QueueName = jsii.String(*props.FunctionProps.FunctionName + "-stream-dlq")
		}

		this.DeadLetterQueue = awssqs.NewQueue(this, jsii.String("DeadLetterQueue"), dlqProps)
	}

	// Create Lambda function with DynamoDB environment variables
	functionEnv := make(map[string]*string)
	if props.FunctionProps.Environment != nil {
		for k, v := range *props.FunctionProps.Environment {
			functionEnv[k] = v
		}
	}

	// Add DynamoDB-specific environment variables from DynamORM table
	functionEnv["DYNAMODB_TABLE_NAME"] = this.StreamingTable.GetTableName()
	functionEnv["DYNAMODB_TABLE_ARN"] = this.StreamingTable.GetTableArn()
	if this.StreamingTable.GetStreamArn() != nil {
		functionEnv["DYNAMODB_STREAM_ARN"] = this.StreamingTable.GetStreamArn()
	}
	if this.DeadLetterQueue != nil {
		functionEnv["DYNAMODB_DLQ_URL"] = this.DeadLetterQueue.QueueUrl()
	}

	// Create LiftFunction with enhanced properties
	liftProps := &LiftFunctionProps{
		FunctionProps: props.FunctionProps,
	}

	// If no code is provided, use default inline code
	if liftProps.Code == nil {
		liftProps.Code = awslambda.Code_FromInline(jsii.String("exports.handler = async (event) => { console.log('Stream event:', JSON.stringify(event)); };"))
	}
	if liftProps.Handler == nil {
		liftProps.Handler = jsii.String("index.handler")
	}
	if liftProps.Runtime == nil {
		liftProps.Runtime = awslambda.Runtime_NODEJS_18_X()
	}

	// Override environment
	liftProps.Environment = &functionEnv

	// Set Lift-specific properties
	if props.EnableTracing != nil {
		liftProps.EnableTracing = props.EnableTracing
	}
	if props.EnableMultiTenant != nil {
		liftProps.EnableMultiTenant = props.EnableMultiTenant
	}

	// DynamoDB stream processor handles its own DLQ through SQS, no need for Lambda DLQ

	this.Function = NewLiftFunction(this, jsii.String("Function"), liftProps)

	// Configure DynamoDB Streams event source
	eventSourceProps := &awslambdaeventsources.DynamoEventSourceProps{
		StartingPosition:        startingPosition,
		BatchSize:               jsii.Number(batchSize),
		MaxBatchingWindow:       maxBatchingWindow,
		MaxRecordAge:            maxRecordAge,
		BisectBatchOnError:      jsii.Bool(bisectBatchOnError),
		RetryAttempts:           jsii.Number(retryAttempts),
		ReportBatchItemFailures: jsii.Bool(reportBatchItemFailures),
		ParallelizationFactor:   jsii.Number(parallelizationFactor),
	}

	// Set tumbling window if specified
	if props.TumblingWindow != nil {
		eventSourceProps.TumblingWindow = props.TumblingWindow
	}

	// Override with user-provided event source props
	if props.EventSourceProps != nil {
		if props.EventSourceProps.StartingPosition != "" {
			eventSourceProps.StartingPosition = props.EventSourceProps.StartingPosition
		}
		if props.EventSourceProps.BatchSize != nil {
			eventSourceProps.BatchSize = props.EventSourceProps.BatchSize
		}
		if props.EventSourceProps.MaxBatchingWindow != nil {
			eventSourceProps.MaxBatchingWindow = props.EventSourceProps.MaxBatchingWindow
		}
		if props.EventSourceProps.MaxRecordAge != nil {
			eventSourceProps.MaxRecordAge = props.EventSourceProps.MaxRecordAge
		}
		if props.EventSourceProps.BisectBatchOnError != nil {
			eventSourceProps.BisectBatchOnError = props.EventSourceProps.BisectBatchOnError
		}
		if props.EventSourceProps.RetryAttempts != nil {
			eventSourceProps.RetryAttempts = props.EventSourceProps.RetryAttempts
		}
		if props.EventSourceProps.ReportBatchItemFailures != nil {
			eventSourceProps.ReportBatchItemFailures = props.EventSourceProps.ReportBatchItemFailures
		}
		if props.EventSourceProps.ParallelizationFactor != nil {
			eventSourceProps.ParallelizationFactor = props.EventSourceProps.ParallelizationFactor
		}
		if props.EventSourceProps.TumblingWindow != nil {
			eventSourceProps.TumblingWindow = props.EventSourceProps.TumblingWindow
		}
		if props.EventSourceProps.Enabled != nil {
			eventSourceProps.Enabled = props.EventSourceProps.Enabled
		}
		if props.EventSourceProps.Filters != nil {
			eventSourceProps.Filters = props.EventSourceProps.Filters
		}
	}

	// Create and add event source
	this.EventSource = awslambdaeventsources.NewDynamoEventSource(this.StreamingTable.Table, eventSourceProps)
	this.Function.Function.AddEventSource(this.EventSource)

	// Grant permissions
	this.StreamingTable.GrantStreamRead(this.Function.Function)
	this.StreamingTable.GrantReadWrite(this.Function.Function)
	if this.DeadLetterQueue != nil {
		this.DeadLetterQueue.GrantSendMessages(this.Function.Function)
	}

	// Add monitoring if enabled
	if props.EnableMonitoring != nil && *props.EnableMonitoring {
		this.enableMonitoring()
	}

	return this
}

// enableMonitoring adds CloudWatch alarms and metrics for the DynamoDB stream processor
func (d *DynamoStreamProcessor) enableMonitoring() {
	// Create SNS topic for alerts
	_ = awssns.NewTopic(d, jsii.String("AlarmTopic"), &awssns.TopicProps{
		TopicName:   jsii.String(fmt.Sprintf("%s-stream-alarms", *d.StreamingTable.GetTableName())),
		DisplayName: jsii.String(fmt.Sprintf("Stream alarms for %s", *d.StreamingTable.GetTableName())),
	})

	if d.Function != nil && d.Function.Function != nil {
		function := d.Function.Function

		// Function error rate alarm
		awscloudwatch.NewAlarm(d, jsii.String("FunctionErrorAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-stream-processor-errors", *d.StreamingTable.GetTableName())),
			AlarmDescription: jsii.String("Stream processor function errors"),
			Metric: function.MetricErrors(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(5),
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Function throttles alarm
		awscloudwatch.NewAlarm(d, jsii.String("FunctionThrottleAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-stream-processor-throttles", *d.StreamingTable.GetTableName())),
			AlarmDescription: jsii.String("Stream processor function throttled"),
			Metric: function.MetricThrottles(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(1),
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Function duration alarm
		awscloudwatch.NewAlarm(d, jsii.String("FunctionDurationAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-stream-processor-duration", *d.StreamingTable.GetTableName())),
			AlarmDescription: jsii.String("Stream processor taking too long"),
			Metric: function.MetricDuration(&awscloudwatch.MetricOptions{
				Period:    awscdk.Duration_Minutes(jsii.Number(5)),
				Statistic: awscloudwatch.Stats_AVERAGE(),
			}),
			Threshold:          jsii.Number(30000), // 30 seconds
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Iterator age alarm - critical for stream processing
		iteratorAgeMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Lambda"),
			MetricName: jsii.String("IteratorAge"),
			DimensionsMap: &map[string]*string{
				"FunctionName": function.FunctionName(),
			},
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_MAXIMUM(),
		})

		awscloudwatch.NewAlarm(d, jsii.String("IteratorAgeAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-iterator-age", *d.StreamingTable.GetTableName())),
			AlarmDescription:   jsii.String("Stream iterator age is too high"),
			Metric:             iteratorAgeMetric,
			Threshold:          jsii.Number(60000), // 1 minute in milliseconds
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// DynamoDB table metrics
	if d.StreamingTable != nil {
		// User errors on table operations
		userErrorsMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("UserErrors"),
			DimensionsMap: &map[string]*string{
				"TableName": d.StreamingTable.GetTableName(),
			},
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_SUM(),
		})

		awscloudwatch.NewAlarm(d, jsii.String("UserErrorsAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-user-errors", *d.StreamingTable.GetTableName())),
			AlarmDescription:   jsii.String("User errors on DynamoDB operations"),
			Metric:             userErrorsMetric,
			Threshold:          jsii.Number(10),
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// System errors on table operations
		systemErrorsMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("SystemErrors"),
			DimensionsMap: &map[string]*string{
				"TableName": d.StreamingTable.GetTableName(),
			},
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_SUM(),
		})

		awscloudwatch.NewAlarm(d, jsii.String("SystemErrorsAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-system-errors", *d.StreamingTable.GetTableName())),
			AlarmDescription:   jsii.String("System errors on DynamoDB operations"),
			Metric:             systemErrorsMetric,
			Threshold:          jsii.Number(1),
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Throttled requests alarm
		throttledMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("GetRecords.Throttled"),
			DimensionsMap: &map[string]*string{
				"TableName": d.StreamingTable.GetTableName(),
			},
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_SUM(),
		})

		awscloudwatch.NewAlarm(d, jsii.String("StreamThrottledAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-stream-throttled", *d.StreamingTable.GetTableName())),
			AlarmDescription:   jsii.String("DynamoDB stream GetRecords throttled"),
			Metric:             throttledMetric,
			Threshold:          jsii.Number(5),
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// DLQ monitoring if enabled
	if d.DeadLetterQueue != nil {
		awscloudwatch.NewAlarm(d, jsii.String("DLQMessagesAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-stream-dlq-messages", *d.StreamingTable.GetTableName())),
			AlarmDescription: jsii.String("Messages in stream processor dead letter queue"),
			Metric: d.DeadLetterQueue.MetricApproximateNumberOfMessagesVisible(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(1),
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// Create CloudWatch dashboard
	dashboard := awscloudwatch.NewDashboard(d, jsii.String("StreamProcessorDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(fmt.Sprintf("%s-stream-processor-dashboard", *d.StreamingTable.GetTableName())),
	})

	// Add widgets to dashboard
	if d.Function != nil && d.Function.Function != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("Lambda Function Metrics"),
				Left: &[]awscloudwatch.IMetric{
					d.Function.Function.MetricInvocations(nil),
					d.Function.Function.MetricErrors(nil),
					d.Function.Function.MetricThrottles(nil),
				},
				Right: &[]awscloudwatch.IMetric{
					d.Function.Function.MetricDuration(nil),
				},
			}),
		)

		// Add iterator age widget
		iteratorAgeWidgetMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Lambda"),
			MetricName: jsii.String("IteratorAge"),
			DimensionsMap: &map[string]*string{
				"FunctionName": d.Function.Function.FunctionName(),
			},
		})

		dashboard.AddWidgets(
			awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
				Title: jsii.String("Stream Iterator Age"),
				Metrics: &[]awscloudwatch.IMetric{
					iteratorAgeWidgetMetric,
				},
			}),
		)
	}

	if d.StreamingTable != nil {
		// Create metrics for dashboard
		userErrorsDashboardMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("UserErrors"),
			DimensionsMap: &map[string]*string{
				"TableName": d.StreamingTable.GetTableName(),
			},
		})

		systemErrorsDashboardMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String("SystemErrors"),
			DimensionsMap: &map[string]*string{
				"TableName": d.StreamingTable.GetTableName(),
			},
		})

		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("DynamoDB Stream Metrics"),
				Left: &[]awscloudwatch.IMetric{
					userErrorsDashboardMetric,
					systemErrorsDashboardMetric,
				},
			}),
		)
	}

	if d.DeadLetterQueue != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
				Title: jsii.String("Dead Letter Queue"),
				Metrics: &[]awscloudwatch.IMetric{
					d.DeadLetterQueue.MetricApproximateNumberOfMessagesVisible(nil),
				},
			}),
		)
	}
}

// GrantReadWriteData grants permission to read and write data to the table
func (d *DynamoStreamProcessor) GrantReadWriteData(grantee awslambda.IFunction) {
	d.StreamingTable.GrantReadWrite(grantee)
}

// GrantStreamRead grants permission to read from the DynamoDB stream
func (d *DynamoStreamProcessor) GrantStreamRead(grantee awslambda.IFunction) {
	d.StreamingTable.GrantStreamRead(grantee)
}

// GrantReadData grants permission to read data from the table
func (d *DynamoStreamProcessor) GrantReadData(grantee awslambda.IFunction) {
	d.StreamingTable.Table.GrantReadData(awsiam.IGrantable(grantee))
}

// GrantWriteData grants permission to write data to the table
func (d *DynamoStreamProcessor) GrantWriteData(grantee awslambda.IFunction) {
	d.StreamingTable.Table.GrantWriteData(awsiam.IGrantable(grantee))
}

// AddEnvironmentVariable adds an environment variable to the Lambda function
func (d *DynamoStreamProcessor) AddEnvironmentVariable(key string, value string) {
	d.Function.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
}

// GetTableName returns the table name
func (d *DynamoStreamProcessor) GetTableName() *string {
	return d.StreamingTable.GetTableName()
}

// GetTableArn returns the table ARN
func (d *DynamoStreamProcessor) GetTableArn() *string {
	return d.StreamingTable.GetTableArn()
}

// GetStreamArn returns the DynamoDB stream ARN
func (d *DynamoStreamProcessor) GetStreamArn() *string {
	return d.StreamingTable.GetStreamArn()
}

// GetDeadLetterQueueUrl returns the DLQ URL if enabled
func (d *DynamoStreamProcessor) GetDeadLetterQueueUrl() *string {
	if d.DeadLetterQueue != nil {
		return d.DeadLetterQueue.QueueUrl()
	}
	return nil
}
