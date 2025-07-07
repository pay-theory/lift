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

// DynamORMStreamProcessorProps defines properties for a DynamORM stream processor
type DynamORMStreamProcessorProps struct {
	// Lambda function properties
	FunctionProps awslambda.FunctionProps

	// DynamORMTable to attach stream processor to (required)
	DynamORMTable *DynamORMTable

	// Dead letter queue properties (optional)
	DeadLetterQueueProps *awssqs.QueueProps

	// Enable dead letter queue (default: true)
	EnableDeadLetterQueue *bool

	// DynamoDB Streams event source configuration
	EventSourceProps *awslambdaeventsources.DynamoEventSourceProps

	// Additional DynamoDB stream processor settings
	BatchSize             *float64            // Default: 10
	MaxBatchingWindow     awscdk.Duration     // Default: 5 seconds
	StartingPosition      awslambda.StartingPosition // Default: LATEST
	MaxRecordAge          awscdk.Duration     // Default: 24 hours
	BisectBatchOnError    *bool               // Default: false
	RetryAttempts         *float64            // Default: 10000
	ReportBatchItemFailures *bool             // Default: true
	TumblingWindow        awscdk.Duration     // For tumbling window processing
	ParallelizationFactor *float64            // Default: 1

	// DynamORM-specific settings
	EnableTracing        *bool
	EnableMultiTenant    *bool
	EnableMonitoring     *bool
	TenantAttribute      *string              // Default: "TenantID"
	
	// Event filtering and routing
	EventFilters         []StreamEventFilter  // Custom event filters
	ProcessingMode       StreamProcessingMode // Default: SEQUENTIAL
	
	// Performance optimization
	EnableMetricsCollection *bool               // Default: true
	CustomMetrics          []string            // Custom metrics to track
}

// StreamEventFilter defines event filtering criteria
type StreamEventFilter struct {
	EventName    *string            // INSERT, MODIFY, REMOVE
	AttributeFilters map[string]string // Attribute filters
	TenantFilter *string            // Filter by tenant ID
}

// StreamProcessingMode defines how events are processed
type StreamProcessingMode string

const (
	StreamProcessingMode_SEQUENTIAL StreamProcessingMode = "SEQUENTIAL"
	StreamProcessingMode_PARALLEL   StreamProcessingMode = "PARALLEL"
	StreamProcessingMode_BATCHED    StreamProcessingMode = "BATCHED"
)

// DynamORMStreamProcessor represents a DynamORM table with stream processor
type DynamORMStreamProcessor struct {
	constructs.Construct

	// The Lambda function processing DynamoDB stream records
	Function *LiftFunction

	// The DynamORM table
	Table *DynamORMTable

	// Dead letter queue (if enabled)
	DeadLetterQueue awssqs.IQueue

	// Event source mapping
	EventSource awslambdaeventsources.DynamoEventSource
	
	// SNS topic for alerts
	AlertTopic awssns.ITopic
	
	// Track if monitoring is enabled to prevent duplicate setup
	monitoringEnabled bool
}

// validateStreamProcessorProps validates the required properties for DynamORMStreamProcessor
func validateStreamProcessorProps(props *DynamORMStreamProcessorProps) error {
	if props == nil {
		return fmt.Errorf("DynamORMStreamProcessorProps cannot be nil")
	}
	if props.DynamORMTable == nil {
		return fmt.Errorf("DynamORMTable is required for DynamORMStreamProcessor")
	}
	// FunctionProps validation would go here if needed
	// Note: FunctionProps is a struct, not a pointer, so it can't be nil
	return nil
}

// NewDynamORMStreamProcessor creates a new DynamORM stream processor construct
func NewDynamORMStreamProcessor(scope constructs.Construct, id *string, props *DynamORMStreamProcessorProps) *DynamORMStreamProcessor {
	this := &DynamORMStreamProcessor{}
	constructs.NewConstruct_Override(this, scope, id)

	// Validate required properties
	if err := validateStreamProcessorProps(props); err != nil {
		// For CDK constructs, panic is acceptable during construction with clear error messages
		panic(fmt.Sprintf("DynamORMStreamProcessor validation failed: %v", err))
	}

	// Set the table reference
	this.Table = props.DynamORMTable

	// Set defaults
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

	enableMetrics := true
	if props.EnableMetricsCollection != nil {
		enableMetrics = *props.EnableMetricsCollection
	}

	tenantAttribute := "TenantID"
	if props.TenantAttribute != nil {
		tenantAttribute = *props.TenantAttribute
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
			dlqProps.QueueName = jsii.String(*props.FunctionProps.FunctionName + "-dynamorm-stream-dlq")
		}

		this.DeadLetterQueue = awssqs.NewQueue(this, jsii.String("DeadLetterQueue"), dlqProps)
	}

	// Create Lambda function with DynamORM environment variables
	functionEnv := make(map[string]*string)
	if props.FunctionProps.Environment != nil {
		for k, v := range *props.FunctionProps.Environment {
			functionEnv[k] = v
		}
	}

	// Add DynamORM-specific environment variables
	dynamormEnv := this.Table.GetEnvironmentVariables()
	for k, v := range *dynamormEnv {
		functionEnv[k] = v
	}

	// Add stream-specific environment variables
	if this.Table.GetStreamArn() != nil {
		functionEnv["DYNAMODB_STREAM_ARN"] = this.Table.GetStreamArn()
	}
	if this.DeadLetterQueue != nil {
		functionEnv["DYNAMODB_DLQ_URL"] = this.DeadLetterQueue.QueueUrl()
	}

	// Add DynamORM-specific configuration
	functionEnv["DYNAMORM_STREAM_PROCESSING"] = jsii.String("true")
	functionEnv["DYNAMORM_TENANT_ATTRIBUTE"] = jsii.String(tenantAttribute)
	functionEnv["DYNAMORM_METRICS_ENABLED"] = jsii.String(fmt.Sprintf("%v", enableMetrics))
	
	// Add processing mode
	if props.ProcessingMode != "" {
		functionEnv["DYNAMORM_PROCESSING_MODE"] = jsii.String(string(props.ProcessingMode))
	} else {
		functionEnv["DYNAMORM_PROCESSING_MODE"] = jsii.String(string(StreamProcessingMode_SEQUENTIAL))
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

	// Disable Lambda DLQ when stream DLQ is disabled to avoid confusion
	if !enableDLQ {
		liftProps.EnableDeadLetterQueue = jsii.Bool(false)
	}

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

	// Add event filters if specified
	if len(props.EventFilters) > 0 {
		filters := make([]map[string]interface{}, len(props.EventFilters))
		for i, filter := range props.EventFilters {
			filterMap := make(map[string]interface{})
			
			if filter.EventName != nil {
				filterMap["eventName"] = []string{*filter.EventName}
			}
			
			if filter.AttributeFilters != nil {
				dynamodbFilters := make(map[string]interface{})
				for key, value := range filter.AttributeFilters {
					dynamodbFilters[key] = map[string]interface{}{
						"S": []string{value},
					}
				}
				filterMap["dynamodb"] = dynamodbFilters
			}
			
			if filter.TenantFilter != nil {
				if filterMap["dynamodb"] == nil {
					filterMap["dynamodb"] = make(map[string]interface{})
				}
				dynamodbMap := filterMap["dynamodb"].(map[string]interface{})
				dynamodbMap[tenantAttribute] = map[string]interface{}{
					"S": []string{*filter.TenantFilter},
				}
			}
			
			filters[i] = filterMap
		}
		// Convert to correct type
		filterPtrs := make([]*map[string]interface{}, len(filters))
		for i := range filters {
			filterPtrs[i] = &filters[i]
		}
		eventSourceProps.Filters = &filterPtrs
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
	this.EventSource = awslambdaeventsources.NewDynamoEventSource(this.Table.Table, eventSourceProps)
	this.Function.Function.AddEventSource(this.EventSource)

	// Grant permissions using DynamORM methods
	this.Table.GrantStream(this.Function.Function)
	this.Table.AddDynamORMPermissions(this.Function.Function)
	
	// Grant X-Ray permissions if tracing is enabled
	if props.EnableTracing != nil && *props.EnableTracing {
		this.Table.AddXRayPermissions(this.Function.Function)
	}
	
	if this.DeadLetterQueue != nil {
		this.DeadLetterQueue.GrantSendMessages(this.Function.Function)
	}

	// Add monitoring if enabled
	if props.EnableMonitoring != nil && *props.EnableMonitoring {
		this.enableComprehensiveMonitoring(props.CustomMetrics)
	}

	return this
}

// enableComprehensiveMonitoring adds comprehensive CloudWatch monitoring for DynamORM stream processing
func (d *DynamORMStreamProcessor) enableComprehensiveMonitoring(customMetrics []string) {
	// Check if monitoring is already set up
	if d.monitoringEnabled {
		// Just update custom metrics if provided
		if len(customMetrics) > 0 {
			d.addCustomMetrics(customMetrics)
		}
		return
	}
	
	// Mark monitoring as enabled
	d.monitoringEnabled = true
	
	// Create SNS topic for alerts if not already created
	if d.AlertTopic == nil {
		d.AlertTopic = awssns.NewTopic(d, jsii.String("AlertTopic"), &awssns.TopicProps{
			TopicName: jsii.String(fmt.Sprintf("%s-dynamorm-stream-alerts", *d.Table.GetTableName())),
			DisplayName: jsii.String(fmt.Sprintf("DynamORM Stream alerts for %s", *d.Table.GetTableName())),
		})
	}
	
	// Add comprehensive DynamORM table monitoring
	d.Table.AddCloudWatchMetrics()
	alarms := d.Table.AddCloudWatchAlarms(d.AlertTopic.TopicArn())
	
	// Add Lambda function monitoring
	d.addLambdaMonitoring()
	
	// Add stream-specific monitoring
	d.addStreamMonitoring()
	
	// Add custom metrics if specified
	if len(customMetrics) > 0 {
		d.addCustomMetrics(customMetrics)
	}
	
	// Create comprehensive dashboard
	d.createComprehensiveDashboard(alarms)
}

// addLambdaMonitoring adds Lambda function monitoring
func (d *DynamORMStreamProcessor) addLambdaMonitoring() {
	if d.Function == nil || d.Function.Function == nil {
		return
	}
	
	function := d.Function.Function
	
	// Function error rate alarm
	awscloudwatch.NewAlarm(d, jsii.String("FunctionErrorAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-dynamorm-stream-errors", *d.Table.GetTableName())),
		AlarmDescription:  jsii.String("DynamORM stream processor function errors"),
		Metric: function.MetricErrors(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(3),
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	
	// Function throttles alarm
	awscloudwatch.NewAlarm(d, jsii.String("FunctionThrottleAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-dynamorm-stream-throttles", *d.Table.GetTableName())),
		AlarmDescription:  jsii.String("DynamORM stream processor function throttled"),
		Metric: function.MetricThrottles(&awscloudwatch.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(1),
		EvaluationPeriods: jsii.Number(1),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	
	// Function duration alarm
	awscloudwatch.NewAlarm(d, jsii.String("FunctionDurationAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-dynamorm-stream-duration", *d.Table.GetTableName())),
		AlarmDescription:  jsii.String("DynamORM stream processor taking too long"),
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

// addStreamMonitoring adds stream-specific monitoring
func (d *DynamORMStreamProcessor) addStreamMonitoring() {
	if d.Function == nil || d.Function.Function == nil {
		return
	}
	
	// Iterator age alarm - critical for stream processing
	iteratorAgeMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:   jsii.String("AWS/Lambda"),
		MetricName:  jsii.String("IteratorAge"),
		DimensionsMap: &map[string]*string{
			"FunctionName": d.Function.Function.FunctionName(),
		},
		Period: awscdk.Duration_Minutes(jsii.Number(5)),
		Statistic: awscloudwatch.Stats_MAXIMUM(),
	})
	
	awscloudwatch.NewAlarm(d, jsii.String("IteratorAgeAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-dynamorm-iterator-age", *d.Table.GetTableName())),
		AlarmDescription:  jsii.String("DynamORM stream iterator age is too high"),
		Metric:           iteratorAgeMetric,
		Threshold:         jsii.Number(60000), // 1 minute in milliseconds
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	
	// Stream processing failures
	streamFailuresMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:   jsii.String("AWS/Lambda"),
		MetricName:  jsii.String("DestinationDeliveryFailures"),
		DimensionsMap: &map[string]*string{
			"FunctionName": d.Function.Function.FunctionName(),
		},
		Period: awscdk.Duration_Minutes(jsii.Number(5)),
		Statistic: awscloudwatch.Stats_SUM(),
	})
	
	awscloudwatch.NewAlarm(d, jsii.String("StreamFailuresAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:         jsii.String(fmt.Sprintf("%s-dynamorm-stream-failures", *d.Table.GetTableName())),
		AlarmDescription:  jsii.String("DynamORM stream processing failures"),
		Metric:           streamFailuresMetric,
		Threshold:         jsii.Number(3),
		EvaluationPeriods: jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})

	// DLQ monitoring if enabled
	if d.DeadLetterQueue != nil {
		awscloudwatch.NewAlarm(d, jsii.String("DLQMessagesAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-dynamorm-stream-dlq", *d.Table.GetTableName())),
			AlarmDescription:  jsii.String("Messages in DynamORM stream processor dead letter queue"),
			Metric: d.DeadLetterQueue.MetricApproximateNumberOfMessagesVisible(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:         jsii.Number(1),
			EvaluationPeriods: jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}
}

// addCustomMetrics adds custom metrics for DynamORM stream processing
func (d *DynamORMStreamProcessor) addCustomMetrics(customMetrics []string) {
	for _, metricName := range customMetrics {
		// Create custom metric
		customMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/StreamProcessor"),
			MetricName: jsii.String(metricName),
			DimensionsMap: &map[string]*string{
				"TableName": d.Table.GetTableName(),
				"FunctionName": d.Function.Function.FunctionName(),
			},
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
			Statistic: awscloudwatch.Stats_SUM(),
		})
		
		// Create alarm for custom metric
		awscloudwatch.NewAlarm(d, jsii.String(fmt.Sprintf("CustomMetric%sAlarm", metricName)), &awscloudwatch.AlarmProps{
			AlarmName:         jsii.String(fmt.Sprintf("%s-dynamorm-%s", *d.Table.GetTableName(), metricName)),
			AlarmDescription:  jsii.String(fmt.Sprintf("Custom metric %s for DynamORM stream processor", metricName)),
			Metric:           customMetric,
			Threshold:         jsii.Number(10),
			EvaluationPeriods: jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}
}

// createComprehensiveDashboard creates a comprehensive dashboard for DynamORM stream processing
func (d *DynamORMStreamProcessor) createComprehensiveDashboard(_ map[string]awscloudwatch.Alarm) {
	// Create dashboard
	dashboard := awscloudwatch.NewDashboard(d, jsii.String("DynamORMStreamDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(fmt.Sprintf("%s-dynamorm-stream-dashboard", *d.Table.GetTableName())),
	})
	
	// Add Lambda function metrics
	if d.Function != nil && d.Function.Function != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("DynamORM Stream Processor - Lambda Metrics"),
				Width: jsii.Number(12),
				Height: jsii.Number(6),
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
		iteratorAgeMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:   jsii.String("AWS/Lambda"),
			MetricName:  jsii.String("IteratorAge"),
			DimensionsMap: &map[string]*string{
				"FunctionName": d.Function.Function.FunctionName(),
			},
		})
		
		dashboard.AddWidgets(
			awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
				Title: jsii.String("Stream Iterator Age (ms)"),
				Width: jsii.Number(6),
				Height: jsii.Number(6),
				Metrics: &[]awscloudwatch.IMetric{
					iteratorAgeMetric,
				},
			}),
		)
	}
	
	// Add DynamORM table metrics
	if d.Table != nil {
		tableMetrics := d.Table.GetTableMetrics()
		
		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("DynamORM Table - Capacity Utilization"),
				Width: jsii.Number(12),
				Height: jsii.Number(6),
				Left: &[]awscloudwatch.IMetric{
					tableMetrics["ConsumedReadCapacity"],
					tableMetrics["ConsumedWriteCapacity"],
				},
			}),
		)
		
		dashboard.AddWidgets(
			awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
				Title: jsii.String("DynamORM Table - Errors and Throttling"),
				Width: jsii.Number(12),
				Height: jsii.Number(6),
				Left: &[]awscloudwatch.IMetric{
					tableMetrics["ThrottledRequests"],
					tableMetrics["SystemErrors"],
				},
			}),
		)
		
		// Add tenant metrics if multi-tenant is enabled
		tenantMetrics := d.Table.GetTenantMetrics()
		if tenantMetrics != nil {
			dashboard.AddWidgets(
				awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
					Title: jsii.String("DynamORM Multi-Tenant Metrics"),
					Width: jsii.Number(12),
					Height: jsii.Number(6),
					Left: &[]awscloudwatch.IMetric{
						tenantMetrics["TenantOperations"],
						tenantMetrics["TenantAccessViolations"],
					},
				}),
			)
		}
	}
	
	// Add DLQ metrics if enabled
	if d.DeadLetterQueue != nil {
		dashboard.AddWidgets(
			awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
				Title: jsii.String("Dead Letter Queue Messages"),
				Width: jsii.Number(6),
				Height: jsii.Number(6),
				Metrics: &[]awscloudwatch.IMetric{
					d.DeadLetterQueue.MetricApproximateNumberOfMessagesVisible(nil),
				},
			}),
		)
	}
}

// GrantDynamORMAccess grants comprehensive DynamORM access to the stream processor
func (d *DynamORMStreamProcessor) GrantDynamORMAccess(grantee awslambda.IFunction) {
	d.Table.AddDynamORMPermissions(grantee)
}

// GrantStreamRead grants permission to read from the DynamoDB stream
func (d *DynamORMStreamProcessor) GrantStreamRead(grantee awslambda.IFunction) {
	d.Table.GrantStream(grantee)
}

// GrantTenantIsolatedAccess grants tenant-isolated access to the stream processor
func (d *DynamORMStreamProcessor) GrantTenantIsolatedAccess(grantee awslambda.IFunction, tenantAttribute string) {
	d.Table.GrantTenantIsolatedAccess(grantee, tenantAttribute)
}

// AddEnvironmentVariable adds an environment variable to the Lambda function
func (d *DynamORMStreamProcessor) AddEnvironmentVariable(key string, value string) {
	d.Function.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
}

// AddEventFilter adds an event filter to the stream processor
func (d *DynamORMStreamProcessor) AddEventFilter(filter StreamEventFilter) {
	// Convert filter to environment variables for runtime filtering
	filterConfig := make(map[string]string)
	
	if filter.EventName != nil {
		filterConfig["FILTER_EVENT_NAME"] = *filter.EventName
	}
	
	if filter.TenantFilter != nil {
		filterConfig["FILTER_TENANT_ID"] = *filter.TenantFilter
	}
	
	// Add attribute filters
	for key, value := range filter.AttributeFilters {
		filterConfig[fmt.Sprintf("FILTER_ATTR_%s", key)] = value
	}
	
	// Add filter configuration to Lambda environment
	for key, value := range filterConfig {
		d.Function.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
	}
	
	// Track filter count for multiple filters
	filterCount := 0
	
	// Add filter index for multiple filters
	d.Function.Function.AddEnvironment(
		jsii.String(fmt.Sprintf("FILTER_%d_ENABLED", filterCount)),
		jsii.String("true"),
		nil,
	)
}

// GetTableName returns the table name
func (d *DynamORMStreamProcessor) GetTableName() *string {
	return d.Table.GetTableName()
}

// GetTableArn returns the table ARN
func (d *DynamORMStreamProcessor) GetTableArn() *string {
	return d.Table.GetTableArn()
}

// GetStreamArn returns the DynamoDB stream ARN
func (d *DynamORMStreamProcessor) GetStreamArn() *string {
	return d.Table.GetStreamArn()
}

// GetDeadLetterQueueUrl returns the DLQ URL if enabled
func (d *DynamORMStreamProcessor) GetDeadLetterQueueUrl() *string {
	if d.DeadLetterQueue != nil {
		return d.DeadLetterQueue.QueueUrl()
	}
	return nil
}

// GetFunction returns the Lambda function
func (d *DynamORMStreamProcessor) GetFunction() *LiftFunction {
	return d.Function
}

// GetDynamORMTable returns the DynamORM table
func (d *DynamORMStreamProcessor) GetDynamORMTable() *DynamORMTable {
	return d.Table
}

// GetAlertTopic returns the SNS alert topic
func (d *DynamORMStreamProcessor) GetAlertTopic() awssns.ITopic {
	return d.AlertTopic
}

// EnableXRayTracing enables X-Ray tracing for the stream processor
func (d *DynamORMStreamProcessor) EnableXRayTracing(serviceName string) {
	d.Table.ConfigureComprehensiveXRayTracing(serviceName, false)
}

// ConfigureMultiTenantStreaming configures multi-tenant streaming patterns
func (d *DynamORMStreamProcessor) ConfigureMultiTenantStreaming(tenantAttribute string) {
	if tenantAttribute == "" {
		tenantAttribute = "TenantID"
	}
	
	// Configure tenant metrics
	d.Table.ConfigureTenantMetrics(tenantAttribute)
	
	// Add tenant-specific environment variables
	d.AddEnvironmentVariable("DYNAMORM_TENANT_STREAMING", "true")
	d.AddEnvironmentVariable("DYNAMORM_TENANT_ATTRIBUTE", tenantAttribute)
}

// SetupComprehensiveStreamProcessing sets up all stream processing features
func (d *DynamORMStreamProcessor) SetupComprehensiveStreamProcessing(serviceName string, enableXRay bool, customMetrics []string) {
	// Enable monitoring
	d.enableComprehensiveMonitoring(customMetrics)
	
	// Enable X-Ray tracing if requested
	if enableXRay {
		d.EnableXRayTracing(serviceName)
	}
	
	// Configure multi-tenant streaming if enabled
	if d.Table.props.EnableMultiTenant != nil && *d.Table.props.EnableMultiTenant {
		d.ConfigureMultiTenantStreaming(*d.Table.props.TenantAttribute)
	}
}