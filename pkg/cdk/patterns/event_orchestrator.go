package patterns

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

// EventOrchestratorProps defines properties for an event orchestrator pattern
type EventOrchestratorProps struct {
	DefaultFunctionProps awslambda.FunctionProps
	EnableEventArchive   *bool
	EventBusName         *string
	AppName                *string
	EventRoutingTableProps *liftconstructs.EventRoutingTableProps
	DefaultMemorySize      *float64
	DefaultTimeout         *float64
	DefaultEnvironment     *map[string]*string
	EventRetentionDays     *float64
	EnableEventRouting     *bool
	EnableSagaPattern      *bool
	ArchiveRetentionDays   *float64
	EnableEventCorrelation *bool
	MaxRetryAttempts       *float64
	RetryBackoffRate       *float64
	EnableTracing          *bool
	EnableMultiTenant      *bool
	EnableMonitoring       *bool
	EventSources           []EventSourceConfig
}

// EventSourceConfig defines configuration for an event source
type EventSourceConfig struct {
	SourceName     *string
	HandlerProps   *awslambda.FunctionProps
	ProcessingMode *string
	EventFilters   map[string]interface{}
	EventTypes     []*string
}

// EventOrchestrator represents a multi-source event orchestration pattern
type EventOrchestrator struct {
	constructs.Construct

	// Event routing table (DynamORM-based)
	EventRoutingTable *liftconstructs.EventRoutingTable

	// Event source handlers
	EventHandlers map[string]*liftconstructs.EventBridgeHandler

	// Orchestration function
	OrchestratorFunction *liftconstructs.LiftFunction

	// Correlation function (if enabled)
	CorrelationFunction *liftconstructs.LiftFunction

	// Dead letter handler
	DLQHandler *liftconstructs.LiftFunction
}

// NewEventOrchestrator creates a new event orchestrator pattern using DynamORM
func NewEventOrchestrator(scope constructs.Construct, id *string, props *EventOrchestratorProps) *EventOrchestrator {
	this := &EventOrchestrator{
		EventHandlers: make(map[string]*liftconstructs.EventBridgeHandler),
	}
	constructs.NewConstruct_Override(this, scope, id)

	// Set defaults
	if props == nil {
		props = &EventOrchestratorProps{}
	}

	appName := "event-orchestrator"
	if props.AppName != nil {
		appName = *props.AppName
	}

	eventBusName := "default"
	if props.EventBusName != nil {
		eventBusName = *props.EventBusName
	}

	enableEventRouting := true
	if props.EnableEventRouting != nil {
		enableEventRouting = *props.EnableEventRouting
	}

	enableSagaPattern := false
	if props.EnableSagaPattern != nil {
		enableSagaPattern = *props.EnableSagaPattern
	}

	enableEventCorrelation := true
	if props.EnableEventCorrelation != nil {
		enableEventCorrelation = *props.EnableEventCorrelation
	}

	// Create event routing table using DynamORM if enabled
	if enableEventRouting {
		eventRoutingProps := &liftconstructs.EventRoutingTableProps{
			TableName: jsii.String(appName + "-routing"),
			// GSIs for source, status, and date indexes are now defined in DynamORM models
			// Example model:
			// type EventRoute struct {
			//     PK         string `dynamorm:"pk"`                    // event#{event_id}
			//     SK         string `dynamorm:"sk"`                    // route#{route_id}
			//     Source     string `dynamorm:"index:source-index,pk"` // For source queries
			//     Status     string `dynamorm:"index:status-index,pk"` // For status queries
			//     Date       string `dynamorm:"index:date-index,pk"`   // For date queries
			// }
		}

		// Override with user-provided props
		if props.EventRoutingTableProps != nil {
			eventRoutingProps = props.EventRoutingTableProps
		}

		this.EventRoutingTable = liftconstructs.NewEventRoutingTable(this, jsii.String("EventRouting"), eventRoutingProps)
	}

	// Create orchestrator function
	orchestratorEnv := make(map[string]*string)
	if props.DefaultEnvironment != nil {
		for k, v := range *props.DefaultEnvironment {
			orchestratorEnv[k] = v
		}
	}

	// Add environment variables for orchestration
	orchestratorEnv["EVENT_BUS_NAME"] = jsii.String(eventBusName)
	orchestratorEnv["SAGA_ENABLED"] = jsii.String(fmt.Sprintf("%t", enableSagaPattern))
	orchestratorEnv["CORRELATION_ENABLED"] = jsii.String(fmt.Sprintf("%t", enableEventCorrelation))
	if this.EventRoutingTable != nil {
		orchestratorEnv["EVENT_ROUTING_TABLE"] = this.EventRoutingTable.GetTableName()
		orchestratorEnv["EVENT_ROUTING_TABLE_ARN"] = this.EventRoutingTable.GetTableArn()

		// GSI names are now determined by DynamORM model struct tags
		// The index names in the model would be like "source-index", "status-index", "date-index", etc.
	}

	// Create orchestrator function
	orchestratorProps := props.DefaultFunctionProps
	orchestratorProps.FunctionName = jsii.String(appName + "-orchestrator")
	orchestratorProps.Environment = &orchestratorEnv
	if props.DefaultMemorySize != nil {
		orchestratorProps.MemorySize = props.DefaultMemorySize
	}
	if props.DefaultTimeout != nil {
		orchestratorProps.Timeout = awscdk.Duration_Seconds(props.DefaultTimeout)
	}

	this.OrchestratorFunction = liftconstructs.NewLiftFunction(this, jsii.String("Orchestrator"), &liftconstructs.LiftFunctionProps{
		FunctionProps:     orchestratorProps,
		EnableTracing:     props.EnableTracing,
		EnableMultiTenant: props.EnableMultiTenant,
	})

	// Grant permissions to orchestrator
	if this.EventRoutingTable != nil {
		this.EventRoutingTable.GrantEventManagement(this.OrchestratorFunction.Function)
	}

	// Create correlation function if enabled
	if enableEventCorrelation {
		correlationEnv := make(map[string]*string)
		for k, v := range orchestratorEnv {
			correlationEnv[k] = v
		}

		correlationProps := props.DefaultFunctionProps
		correlationProps.FunctionName = jsii.String(appName + "-correlator")
		correlationProps.Environment = &correlationEnv

		this.CorrelationFunction = liftconstructs.NewLiftFunction(this, jsii.String("Correlator"), &liftconstructs.LiftFunctionProps{
			FunctionProps:     correlationProps,
			EnableTracing:     props.EnableTracing,
			EnableMultiTenant: props.EnableMultiTenant,
		})

		// Grant permissions to correlator
		if this.EventRoutingTable != nil {
			this.EventRoutingTable.GrantEventManagement(this.CorrelationFunction.Function)
		}
	}

	// Create event source handlers
	for _, sourceConfig := range props.EventSources {
		if sourceConfig.SourceName == nil {
			continue
		}

		sourceName := *sourceConfig.SourceName

		// Create handler environment
		handlerEnv := make(map[string]*string)
		if props.DefaultEnvironment != nil {
			for k, v := range *props.DefaultEnvironment {
				handlerEnv[k] = v
			}
		}

		// Add source-specific environment
		handlerEnv["EVENT_SOURCE"] = jsii.String(sourceName)
		handlerEnv["PROCESSING_MODE"] = sourceConfig.ProcessingMode
		if this.EventRoutingTable != nil {
			handlerEnv["EVENT_ROUTING_TABLE"] = this.EventRoutingTable.GetTableName()
		}

		// Create handler function props
		handlerProps := props.DefaultFunctionProps
		if sourceConfig.HandlerProps != nil {
			handlerProps = *sourceConfig.HandlerProps
		}
		handlerProps.FunctionName = jsii.String(fmt.Sprintf("%s-%s-handler", appName, sourceName))
		handlerProps.Environment = &handlerEnv

		// Create event pattern
		eventPattern := &awsevents.EventPattern{
			Source: &[]*string{jsii.String(sourceName)},
		}
		if len(sourceConfig.EventTypes) > 0 {
			eventPattern.DetailType = &sourceConfig.EventTypes
		}
		if len(sourceConfig.EventFilters) > 0 {
			eventPattern.Detail = &sourceConfig.EventFilters
		}

		// Create EventBridge handler
		handler, err := liftconstructs.NewEventBridgeHandler(this, jsii.String(sourceName+"Handler"), &liftconstructs.EventBridgeHandlerProps{
			FunctionProps:     handlerProps,
			EnableTracing:     props.EnableTracing,
			EnableMultiTenant: props.EnableMultiTenant,
			RuleProps: &awsevents.RuleProps{
				RuleName:     jsii.String(fmt.Sprintf("%s-%s-rule", appName, sourceName)),
				Description:  jsii.String(fmt.Sprintf("Process %s events", sourceName)),
				EventPattern: eventPattern,
			},
		})
		if err != nil {
			// Log error and skip this handler
			fmt.Printf("Warning: Failed to create EventBridge handler for %s: %v\n", sourceName, err)
			continue
		}

		// Grant permissions
		if this.EventRoutingTable != nil {
			this.EventRoutingTable.GrantEventManagement(handler.Function.Function)
		}

		this.EventHandlers[sourceName] = handler
	}

	// Create DLQ handler for event-specific dead letter queues
	// Note: Individual event sources (like SQS) handle their own DLQs
	if false { // Removed automatic DLQ handler creation
		dlqEnv := make(map[string]*string)
		if props.DefaultEnvironment != nil {
			for k, v := range *props.DefaultEnvironment {
				dlqEnv[k] = v
			}
		}

		dlqEnv["EVENT_BUS_NAME"] = jsii.String(eventBusName)
		if this.EventRoutingTable != nil {
			dlqEnv["EVENT_ROUTING_TABLE"] = this.EventRoutingTable.GetTableName()
		}

		dlqProps := props.DefaultFunctionProps
		dlqProps.FunctionName = jsii.String(appName + "-dlq-handler")
		dlqProps.Environment = &dlqEnv

		this.DLQHandler = liftconstructs.NewLiftFunction(this, jsii.String("DLQHandler"), &liftconstructs.LiftFunctionProps{
			FunctionProps:     dlqProps,
			EnableTracing:     props.EnableTracing,
			EnableMultiTenant: props.EnableMultiTenant,
		})

		// Grant permissions
		if this.EventRoutingTable != nil {
			this.EventRoutingTable.GrantEventManagement(this.DLQHandler.Function)
		}
	}

	// Enable monitoring if requested
	if props.EnableMonitoring != nil && *props.EnableMonitoring {
		this.enableMonitoring(props)
	}

	return this
}

// enableMonitoring adds CloudWatch alarms and metrics
func (e *EventOrchestrator) enableMonitoring(props *EventOrchestratorProps) {
	appName := "event-orchestrator"
	if props != nil && props.AppName != nil {
		appName = *props.AppName
	}

	// Create SNS topic for alerts
	alertTopic := awssns.NewTopic(e, jsii.String("AlertTopic"), &awssns.TopicProps{
		TopicName:   jsii.String(fmt.Sprintf("%s-orchestrator-alerts", appName)),
		DisplayName: jsii.String(fmt.Sprintf("%s Event Orchestrator Alerts", appName)),
	})

	// 1. Event processing latency by source
	for name, handler := range e.EventHandlers {
		if handler != nil && handler.Function != nil {
			function := handler.Function.Function

			// Create function monitoring alarms using helper
			e.createFunctionAlarms(appName, name, function, alertTopic)
		}
	}

	// 2. Event routing table metrics (if enabled)
	if e.EventRoutingTable != nil {
		routingTableName := e.EventRoutingTable.GetTableName()

		// Create DynamoDB throttling alarms using helper
		e.createDynamoDBThrottleAlarms(appName, routingTableName, alertTopic)
	}

	// 3. Custom metrics for correlation success rates
	correlationMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String(fmt.Sprintf("Lift/EventOrchestrator/%s", appName)),
		MetricName: jsii.String("CorrelationSuccess"),
		Statistic:  jsii.String("Average"),
		Period:     awscdk.Duration_Minutes(jsii.Number(5)),
	})

	correlationAlarm := awscloudwatch.NewAlarm(e, jsii.String("CorrelationFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(fmt.Sprintf("%s-correlation-failure", appName)),
		AlarmDescription:   jsii.String("Low event correlation success rate"),
		Metric:             correlationMetric,
		Threshold:          jsii.Number(0.95), // 95% success rate
		ComparisonOperator: awscloudwatch.ComparisonOperator_LESS_THAN_THRESHOLD,
		EvaluationPeriods:  jsii.Number(3),
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	correlationAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alertTopic))

	// 4. Saga completion metrics
	sagaMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String(fmt.Sprintf("Lift/EventOrchestrator/%s", appName)),
		MetricName: jsii.String("SagaCompletion"),
		Statistic:  jsii.String("Average"),
		Period:     awscdk.Duration_Minutes(jsii.Number(15)),
	})

	sagaAlarm := awscloudwatch.NewAlarm(e, jsii.String("SagaFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(fmt.Sprintf("%s-saga-failure", appName)),
		AlarmDescription:   jsii.String("Low saga completion rate"),
		Metric:             sagaMetric,
		Threshold:          jsii.Number(0.90), // 90% completion rate
		ComparisonOperator: awscloudwatch.ComparisonOperator_LESS_THAN_THRESHOLD,
		EvaluationPeriods:  jsii.Number(3),
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	sagaAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alertTopic))

	// 5. DLQ metrics (if DLQs exist)
	if e.DLQHandler != nil {
		dlqMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/SQS"),
			MetricName: jsii.String("ApproximateNumberOfMessages"),
			DimensionsMap: &map[string]*string{
				"QueueName": jsii.String(fmt.Sprintf("%s-dlq", appName)),
			},
			Statistic: jsii.String("Maximum"),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		})

		dlqAlarm := awscloudwatch.NewAlarm(e, jsii.String("DLQAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:          jsii.String(fmt.Sprintf("%s-dlq-messages", appName)),
			AlarmDescription:   jsii.String("Messages in dead letter queue"),
			Metric:             dlqMetric,
			Threshold:          jsii.Number(10),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			EvaluationPeriods:  jsii.Number(1),
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
		dlqAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alertTopic))
	}

	// Create CloudWatch Dashboard
	e.createMonitoringDashboard(appName, alertTopic)
}

// createMonitoringDashboard creates a basic dashboard for the event orchestrator
func (e *EventOrchestrator) createMonitoringDashboard(appName string, _ awssns.ITopic) {
	// Create a basic dashboard
	awscloudwatch.NewDashboard(e, jsii.String("Dashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(fmt.Sprintf("%s-event-orchestrator", appName)),
	})
}

// GetEventRoutingTableName returns the event routing table name
func (e *EventOrchestrator) GetEventRoutingTableName() *string {
	if e.EventRoutingTable != nil {
		return e.EventRoutingTable.GetTableName()
	}
	return nil
}

// GrantEventRoutingAccess grants read/write access to the event routing table
func (e *EventOrchestrator) GrantEventRoutingAccess(grantee awslambda.IFunction) {
	if e.EventRoutingTable != nil {
		e.EventRoutingTable.GrantEventManagement(awsiam.IGrantable(grantee))
	}
}

// AddEventSource adds a new event source to the orchestrator
func (e *EventOrchestrator) AddEventSource(_ EventSourceConfig) {
	// TODO: Implement dynamic event source addition
	// This would create a new EventBridgeHandler and add it to the orchestrator
}

// GetEventHandler returns the handler for a specific event source
func (e *EventOrchestrator) GetEventHandler(sourceName string) *liftconstructs.EventBridgeHandler {
	return e.EventHandlers[sourceName]
}

// lambdaAlarmConfig defines configuration for Lambda function alarms
type lambdaAlarmConfig struct {
	alarmType          string
	alarmSuffix        string
	descriptionSuffix  string
	metricFunc         func(awslambda.IFunction, *awscloudwatch.MetricOptions) awscloudwatch.IMetric
	statistic          string
	threshold          float64
	evaluationPeriods  float64
}

// createLambdaAlarm creates a standardized CloudWatch alarm for Lambda functions
func (e *EventOrchestrator) createLambdaAlarm(appName, handlerName string, function awslambda.IFunction, alertTopic awssns.ITopic, config lambdaAlarmConfig) {
	alarm := awscloudwatch.NewAlarm(e, jsii.String(fmt.Sprintf("%s%sAlarm", config.alarmType, handlerName)), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-orchestrator-%s-%s", appName, config.alarmSuffix, handlerName)),
		AlarmDescription: jsii.String(fmt.Sprintf("%s for event handler %s", config.descriptionSuffix, handlerName)),
		Metric: config.metricFunc(function, &awscloudwatch.MetricOptions{
			Statistic: jsii.String(config.statistic),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:          jsii.Number(config.threshold),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		EvaluationPeriods:  jsii.Number(config.evaluationPeriods),
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alertTopic))
}

// createFunctionAlarms creates standard monitoring alarms for a Lambda function
func (e *EventOrchestrator) createFunctionAlarms(appName, handlerName string, function awslambda.IFunction, alertTopic awssns.ITopic) {
	// Function duration alarm
	e.createLambdaAlarm(appName, handlerName, function, alertTopic, lambdaAlarmConfig{
		alarmType:          "Duration",
		alarmSuffix:        "duration",
		descriptionSuffix:  "High duration",
		metricFunc:         func(f awslambda.IFunction, opts *awscloudwatch.MetricOptions) awscloudwatch.IMetric { return f.MetricDuration(opts) },
		statistic:          "Average",
		threshold:          30000, // 30 seconds
		evaluationPeriods:  2,
	})

	// Function error rate alarm
	e.createLambdaAlarm(appName, handlerName, function, alertTopic, lambdaAlarmConfig{
		alarmType:          "Error",
		alarmSuffix:        "errors",
		descriptionSuffix:  "High error rate",
		metricFunc:         func(f awslambda.IFunction, opts *awscloudwatch.MetricOptions) awscloudwatch.IMetric { return f.MetricErrors(opts) },
		statistic:          "Sum",
		threshold:          5,
		evaluationPeriods:  1,
	})
}

// dynamoThrottleAlarmConfig defines configuration for DynamoDB throttling alarms
type dynamoThrottleAlarmConfig struct {
	alarmIDSuffix     string
	alarmNameSuffix   string
	description       string
	metricName        string
}

// createDynamoThrottleAlarm creates a standardized DynamoDB throttling alarm
func (e *EventOrchestrator) createDynamoThrottleAlarm(appName string, tableName *string, alertTopic awssns.ITopic, config dynamoThrottleAlarmConfig) {
	alarm := awscloudwatch.NewAlarm(e, jsii.String(fmt.Sprintf("Routing%sThrottleAlarm", config.alarmIDSuffix)), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-routing-%s-throttle", appName, config.alarmNameSuffix)),
		AlarmDescription: jsii.String(config.description),
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/DynamoDB"),
			MetricName: jsii.String(config.metricName),
			DimensionsMap: &map[string]*string{
				"TableName": tableName,
			},
			Statistic: jsii.String("Sum"),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:          jsii.Number(0),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		EvaluationPeriods:  jsii.Number(1),
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alertTopic))
}

// createDynamoDBThrottleAlarms creates throttling alarms for a DynamoDB table
func (e *EventOrchestrator) createDynamoDBThrottleAlarms(appName string, tableName *string, alertTopic awssns.ITopic) {
	// Read throttle alarm
	e.createDynamoThrottleAlarm(appName, tableName, alertTopic, dynamoThrottleAlarmConfig{
		alarmIDSuffix:   "Read",
		alarmNameSuffix: "read",
		description:     "Event routing table read throttling",
		metricName:      "ReadThrottleEvents",
	})

	// Write throttle alarm
	e.createDynamoThrottleAlarm(appName, tableName, alertTopic, dynamoThrottleAlarmConfig{
		alarmIDSuffix:   "Write",
		alarmNameSuffix: "write",
		description:     "Event routing table write throttling",
		metricName:      "WriteThrottleEvents",
	})
}
