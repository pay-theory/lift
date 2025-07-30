package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// EventBridgeHandlerProps defines properties for an EventBridge handler
type EventBridgeHandlerProps struct {
	// Lambda function properties
	FunctionProps awslambda.FunctionProps

	// Event rule properties (optional - creates new rule if not provided)
	RuleProps *awsevents.RuleProps

	// Existing rule to use (optional - creates new if not provided)
	ExistingRule awsevents.Rule

	// Existing event bus to use (optional - uses default if not provided)
	ExistingEventBus awsevents.IEventBus

	// Event bus properties for creating a custom event bus
	EventBusProps *awsevents.EventBusProps

	// Event pattern for filtering events
	EventPattern *awsevents.EventPattern

	// Schedule expression for scheduled events (conflicts with EventPattern)
	ScheduleExpression *string

	// Lambda target properties
	TargetProps *awseventstargets.LambdaFunctionProps

	// Dead letter queue properties (optional)
	DeadLetterQueueProps *awssqs.QueueProps

	// Enable dead letter queue (default: true)
	EnableDeadLetterQueue *bool

	// Maximum event age in seconds (default: 3600)
	MaxEventAge awscdk.Duration

	// Retry attempts for failed invocations (default: 3)
	RetryAttempts *float64

	// Enable input transformation
	InputTransformation *awsevents.RuleTargetInput

	// Lift-specific settings
	EnableTracing     *bool
	EnableMultiTenant *bool
	EnableMonitoring  *bool

	// Cross-account event bus support
	CrossAccountEventBusArn *string
}

// EventBridgeHandler represents an EventBridge rule with Lambda handler
type EventBridgeHandler struct {
	constructs.Construct

	// The Lambda function handling events
	Function *LiftFunction

	// The EventBridge rule
	Rule awsevents.Rule

	// The event bus (default or custom)
	EventBus awsevents.IEventBus

	// Dead letter queue (if enabled)
	DeadLetterQueue awssqs.IQueue

	// Lambda target
	Target awseventstargets.LambdaFunction
}

// NewEventBridgeHandler creates a new EventBridge handler construct
func NewEventBridgeHandler(scope constructs.Construct, id *string, props *EventBridgeHandlerProps) (*EventBridgeHandler, error) {
	this := &EventBridgeHandler{}
	constructs.NewConstruct_Override(this, scope, id)

	// Set defaults
	if props == nil {
		props = &EventBridgeHandlerProps{}
	}

	// Default values
	maxEventAge := awscdk.Duration_Hours(jsii.Number(1))
	if props.MaxEventAge != nil {
		maxEventAge = props.MaxEventAge
	}

	retryAttempts := float64(3)
	if props.RetryAttempts != nil {
		retryAttempts = *props.RetryAttempts
	}

	enableDLQ := true
	if props.EnableDeadLetterQueue != nil {
		enableDLQ = *props.EnableDeadLetterQueue
	}

	// Create or use existing event bus
	if props.ExistingEventBus != nil {
		this.EventBus = props.ExistingEventBus
	} else if props.EventBusProps != nil {
		this.EventBus = awsevents.NewEventBus(this, jsii.String("EventBus"), props.EventBusProps)
	} else if props.CrossAccountEventBusArn != nil {
		// Reference cross-account event bus
		this.EventBus = awsevents.EventBus_FromEventBusArn(this, jsii.String("CrossAccountEventBus"), props.CrossAccountEventBusArn)
	} else {
		// Use default event bus
		this.EventBus = awsevents.EventBus_FromEventBusName(this, jsii.String("DefaultEventBus"), jsii.String("default"))
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
			dlqProps.QueueName = jsii.String(*props.FunctionProps.FunctionName + "-eventbridge-dlq")
		}

		this.DeadLetterQueue = awssqs.NewQueue(this, jsii.String("DeadLetterQueue"), dlqProps)
	}

	// Create Lambda function with EventBridge environment variables
	functionEnv := make(map[string]*string)
	if props.FunctionProps.Environment != nil {
		for k, v := range *props.FunctionProps.Environment {
			functionEnv[k] = v
		}
	}

	// Add EventBridge-specific environment variables
	functionEnv["EVENT_BUS_NAME"] = this.EventBus.EventBusName()
	functionEnv["EVENT_BUS_ARN"] = this.EventBus.EventBusArn()
	if this.DeadLetterQueue != nil {
		functionEnv["EVENTBRIDGE_DLQ_URL"] = this.DeadLetterQueue.QueueUrl()
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

	// EventBridge handles its own DLQ through SQS, no need for Lambda DLQ

	this.Function = NewLiftFunction(this, jsii.String("Function"), liftProps)

	// Create or use existing rule
	if props.ExistingRule != nil {
		this.Rule = props.ExistingRule
	} else {
		// Create new rule
		ruleProps := &awsevents.RuleProps{}

		// Override with user-provided props
		if props.RuleProps != nil {
			if props.RuleProps.RuleName != nil {
				ruleProps.RuleName = props.RuleProps.RuleName
			}
			if props.RuleProps.Description != nil {
				ruleProps.Description = props.RuleProps.Description
			}
			if props.RuleProps.Enabled != nil {
				ruleProps.Enabled = props.RuleProps.Enabled
			}
		}

		// Set event pattern or schedule
		if props.EventPattern != nil && props.ScheduleExpression != nil {
			return nil, fmt.Errorf("EventPattern and ScheduleExpression cannot both be specified")
		}

		if props.EventPattern != nil {
			ruleProps.EventPattern = props.EventPattern
			// Only set event bus for event pattern rules
			ruleProps.EventBus = this.EventBus
		} else if props.ScheduleExpression != nil {
			ruleProps.Schedule = awsevents.Schedule_Expression(props.ScheduleExpression)
			// Scheduled rules don't use event buses
		} else {
			// Default to match all events if neither pattern nor schedule is provided
			ruleProps.EventPattern = &awsevents.EventPattern{
				Source: &[]*string{jsii.String("*")},
			}
			ruleProps.EventBus = this.EventBus
		}

		// Set default rule name if not provided
		if ruleProps.RuleName == nil && props.FunctionProps.FunctionName != nil {
			ruleProps.RuleName = jsii.String(*props.FunctionProps.FunctionName + "-rule")
		}

		this.Rule = awsevents.NewRule(this, jsii.String("Rule"), ruleProps)
	}

	// Configure Lambda target
	targetProps := &awseventstargets.LambdaFunctionProps{
		MaxEventAge:   maxEventAge,
		RetryAttempts: jsii.Number(retryAttempts),
	}

	// Add dead letter queue to target if enabled
	if this.DeadLetterQueue != nil {
		targetProps.DeadLetterQueue = this.DeadLetterQueue
	}

	// Override with user-provided target props
	if props.TargetProps != nil {
		if props.TargetProps.Event != nil {
			targetProps.Event = props.TargetProps.Event
		}
		if props.TargetProps.MaxEventAge != nil {
			targetProps.MaxEventAge = props.TargetProps.MaxEventAge
		}
		if props.TargetProps.RetryAttempts != nil {
			targetProps.RetryAttempts = props.TargetProps.RetryAttempts
		}
		if props.TargetProps.DeadLetterQueue != nil {
			targetProps.DeadLetterQueue = props.TargetProps.DeadLetterQueue
		}
	}

	// Apply input transformation if provided
	if props.InputTransformation != nil {
		targetProps.Event = *props.InputTransformation
	}

	// Create and add target
	this.Target = awseventstargets.NewLambdaFunction(this.Function.Function, targetProps)

	// Add target to rule
	this.Rule.AddTarget(this.Target)

	// Grant permissions
	this.EventBus.GrantPutEventsTo(this.Function.Function, jsii.String("eventbridge"))
	if this.DeadLetterQueue != nil {
		this.DeadLetterQueue.GrantSendMessages(this.Function.Function)
	}

	// Add monitoring if enabled
	if props.EnableMonitoring != nil && *props.EnableMonitoring {
		this.enableMonitoring()
	}

	return this, nil
}

// enableMonitoring adds CloudWatch alarms and metrics for the EventBridge handler
func (e *EventBridgeHandler) enableMonitoring() {
	if e.Function != nil {
		function := e.Function.GetFunction()

		// Function error rate alarm
		awscloudwatch.NewAlarm(e, jsii.String("FunctionErrorAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-errors", *e.Rule.RuleName())),
			AlarmDescription: jsii.String("EventBridge handler function errors"),
			Metric: function.MetricErrors(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(3),
			EvaluationPeriods:  jsii.Number(2),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})

		// Function duration alarm
		awscloudwatch.NewAlarm(e, jsii.String("FunctionDurationAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-duration", *e.Rule.RuleName())),
			AlarmDescription: jsii.String("EventBridge handler function duration"),
			Metric: function.MetricDuration(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(30000), // 30 seconds
			EvaluationPeriods:  jsii.Number(3),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}

	// EventBridge rule invocation metrics (not used but kept for reference)
	_ = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/Events"),
		MetricName: jsii.String("InvocationsCount"),
		DimensionsMap: &map[string]*string{
			"RuleName": e.Rule.RuleName(),
		},
		Period: awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Rule invocation failure alarm
	awscloudwatch.NewAlarm(e, jsii.String("RuleFailureAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-rule-failures", *e.Rule.RuleName())),
		AlarmDescription: jsii.String("EventBridge rule invocation failures"),
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Events"),
			MetricName: jsii.String("FailedInvocations"),
			DimensionsMap: &map[string]*string{
				"RuleName": e.Rule.RuleName(),
			},
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:          jsii.Number(5),
		EvaluationPeriods:  jsii.Number(2),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})

	// DLQ monitoring if DLQ exists
	if e.DeadLetterQueue != nil {
		awscloudwatch.NewAlarm(e, jsii.String("DLQAlarm"), &awscloudwatch.AlarmProps{
			AlarmName:        jsii.String(fmt.Sprintf("%s-dlq-messages", *e.Rule.RuleName())),
			AlarmDescription: jsii.String("Messages in EventBridge handler DLQ"),
			Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
				Namespace:  jsii.String("AWS/SQS"),
				MetricName: jsii.String("ApproximateNumberOfMessages"),
				DimensionsMap: &map[string]*string{
					"QueueName": e.DeadLetterQueue.QueueName(),
				},
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold:          jsii.Number(10),
			EvaluationPeriods:  jsii.Number(1),
			ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
			TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
		})
	}
}

// GrantPutEvents grants permission to put events to the event bus
func (e *EventBridgeHandler) GrantPutEvents(grantee awslambda.IFunction) {
	e.EventBus.GrantPutEventsTo(grantee, jsii.String("eventbridge"))
}

// AddEnvironmentVariable adds an environment variable to the Lambda function
func (e *EventBridgeHandler) AddEnvironmentVariable(key string, value string) {
	e.Function.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
}

// GetEventBusName returns the event bus name
func (e *EventBridgeHandler) GetEventBusName() *string {
	return e.EventBus.EventBusName()
}

// GetEventBusArn returns the event bus ARN
func (e *EventBridgeHandler) GetEventBusArn() *string {
	return e.EventBus.EventBusArn()
}

// GetRuleName returns the rule name
func (e *EventBridgeHandler) GetRuleName() *string {
	return e.Rule.RuleName()
}

// GetRuleArn returns the rule ARN
func (e *EventBridgeHandler) GetRuleArn() *string {
	return e.Rule.RuleArn()
}

// AddEventPattern adds an event pattern to the rule
// Note: This method is deprecated as EventBridge patterns cannot be modified after rule creation.
// Create a new EventBridgeHandler with the desired pattern instead.
func (e *EventBridgeHandler) AddEventPattern(_ *awsevents.EventPattern) error {
	// For existing rules, we need to recreate or update the pattern
	// This is a limitation of EventBridge - patterns cannot be modified after creation
	// Users should create a new rule with the desired pattern
	return fmt.Errorf("event patterns cannot be modified after rule creation - create a new EventBridgeHandler with the desired pattern")
}

// EnableRule enables the EventBridge rule
// Note: This method configures the rule to be enabled during deployment.
// To change rule state after deployment, use AWS CLI or AWS Console.
func (e *EventBridgeHandler) EnableRule() error {
	// CDK constructs are immutable at runtime, but we can configure the deployment state
	// The rule state is set during CDK construction time
	// For runtime changes, users should use AWS CLI: aws events enable-rule --name <rule-name>
	return fmt.Errorf("rule state cannot be changed after CDK deployment - use AWS CLI: aws events enable-rule --name %s", *e.Rule.RuleName())
}

// DisableRule disables the EventBridge rule
// Note: This method provides guidance for disabling the rule after deployment.
// To change rule state after deployment, use AWS CLI or AWS Console.
func (e *EventBridgeHandler) DisableRule() error {
	// CDK constructs are immutable at runtime, but we can configure the deployment state
	// The rule state is set during CDK construction time
	// For runtime changes, users should use AWS CLI: aws events disable-rule --name <rule-name>
	return fmt.Errorf("rule state cannot be changed after CDK deployment - use AWS CLI: aws events disable-rule --name %s", *e.Rule.RuleName())
}
