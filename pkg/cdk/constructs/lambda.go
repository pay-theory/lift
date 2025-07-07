package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// LiftFunctionProps extends standard Lambda function properties with Lift-specific configuration
type LiftFunctionProps struct {
	awslambda.FunctionProps
	// EnableTracing enables X-Ray tracing for the function
	EnableTracing *bool
	// EnableMetrics enables CloudWatch metrics
	EnableMetrics *bool
	// EnableMultiTenant enables multi-tenant support
	EnableMultiTenant *bool
	// EnableDeadLetterQueue creates a DLQ for failed invocations
	EnableDeadLetterQueue *bool
	// DeadLetterQueue to use (optional - will create if not provided)
	DeadLetterQueue awssqs.IQueue
	// DeadLetterQueueMaxReceiveCount before sending to DLQ (default: 3)
	DeadLetterQueueMaxReceiveCount *float64
	// LogRetentionDays for CloudWatch Logs (default: 30)
	LogRetentionDays *float64
	// ReservedConcurrentExecutions to limit concurrent executions
	ReservedConcurrentExecutions *float64
	// EnableDynamORM configures DynamORM environment variables
	EnableDynamORM *bool
	// DynamORM table name (optional - for when using DynamORM)
	DynamORMTableName *string
	// DynamORM debug mode
	DynamORMDebug *bool
}

// LiftFunction is a Lambda function construct optimized for Lift applications
type LiftFunction struct {
	constructs.Construct
	Function awslambda.Function
	LogGroup awslogs.LogGroup
	DeadLetterQueue awssqs.IQueue
}

// GetResourceName returns the function name
func (l *LiftFunction) GetResourceName() *string {
	return l.Function.FunctionName()
}

// NewLiftFunction creates a new Lift Lambda function with optimized defaults
func NewLiftFunction(scope constructs.Construct, id *string, props *LiftFunctionProps) *LiftFunction {
	this := constructs.NewConstruct(scope, id)

	// Set Lift-optimized defaults
	if props.Runtime == nil {
		props.Runtime = awslambda.Runtime_PROVIDED_AL2023()
	}
	if props.Architecture == nil {
		props.Architecture = awslambda.Architecture_ARM_64()
	}
	if props.MemorySize == nil {
		props.MemorySize = jsii.Number(512)
	}
	if props.Timeout == nil {
		props.Timeout = awscdk.Duration_Seconds(jsii.Number(30))
	}
	if props.EnableTracing != nil && *props.EnableTracing {
		props.Tracing = awslambda.Tracing_ACTIVE
	}
	if props.LogRetentionDays == nil {
		props.LogRetentionDays = jsii.Number(30)
	}
	if props.EnableDeadLetterQueue == nil {
		props.EnableDeadLetterQueue = jsii.Bool(true)
	}
	if props.DeadLetterQueueMaxReceiveCount == nil {
		props.DeadLetterQueueMaxReceiveCount = jsii.Number(3)
	}

	// Configure Dead Letter Queue if enabled
	var dlq awssqs.IQueue
	if *props.EnableDeadLetterQueue {
		if props.DeadLetterQueue != nil {
			dlq = props.DeadLetterQueue
		} else {
			// Create a new DLQ
			dlqName := fmt.Sprintf("%s-dlq", *id)
			dlq = awssqs.NewQueue(this, jsii.String("DeadLetterQueue"), &awssqs.QueueProps{
				QueueName:           jsii.String(dlqName),
				RetentionPeriod:     awscdk.Duration_Days(jsii.Number(14)),
				VisibilityTimeout:   awscdk.Duration_Seconds(jsii.Number(300)),
			})
		}
		
		// Configure DLQ in Lambda props
		props.DeadLetterQueueEnabled = jsii.Bool(true)
		props.DeadLetterQueue = dlq
		props.MaxEventAge = awscdk.Duration_Hours(jsii.Number(6))
		// Lambda retry attempts must be between 0 and 2
		retryAttempts := float64(2)
		if props.DeadLetterQueueMaxReceiveCount != nil && *props.DeadLetterQueueMaxReceiveCount < 2 {
			retryAttempts = *props.DeadLetterQueueMaxReceiveCount
		}
		props.RetryAttempts = jsii.Number(retryAttempts)
	}

	// Set reserved concurrent executions if specified
	if props.ReservedConcurrentExecutions != nil {
		props.FunctionProps.ReservedConcurrentExecutions = props.ReservedConcurrentExecutions
	}

	// Add Lift-specific environment variables
	if props.Environment == nil {
		props.Environment = &map[string]*string{}
	}
	env := *props.Environment
	env["LIFT_VERSION"] = jsii.String("1.0.0")
	if props.EnableMultiTenant != nil && *props.EnableMultiTenant {
		env["LIFT_MULTI_TENANT"] = jsii.String("true")
	}
	if props.EnableMetrics != nil && *props.EnableMetrics {
		env["LIFT_METRICS_ENABLED"] = jsii.String("true")
	}
	
	// Configure DynamORM environment variables if enabled
	if props.EnableDynamORM != nil && *props.EnableDynamORM {
		env["DYNAMORM_REGION"] = awscdk.Stack_Of(this).Region()
		
		if props.DynamORMTableName != nil {
			env["DYNAMODB_TABLE_NAME"] = props.DynamORMTableName
		}
		
		// Set debug mode
		debugMode := "false"
		if props.DynamORMDebug != nil && *props.DynamORMDebug {
			debugMode = "true"
		}
		env["DYNAMORM_DEBUG"] = jsii.String(debugMode)
		
		// Set default retry configuration
		env["DYNAMORM_RETRY_MAX_ATTEMPTS"] = jsii.String("3")
		env["DYNAMORM_RETRY_BASE_DELAY"] = jsii.String("100")
	}
	
	props.Environment = &env

	// Create the Lambda function
	fn := awslambda.NewFunction(this, jsii.String("Function"), &props.FunctionProps)

	// Create CloudWatch Log Group with retention
	logGroupName := fmt.Sprintf("/aws/lambda/%s", *fn.FunctionName())
	logGroup := awslogs.NewLogGroup(this, jsii.String("LogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String(logGroupName),
		Retention:     getRetentionDays(*props.LogRetentionDays),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	return &LiftFunction{
		Construct: this,
		Function:  fn,
		LogGroup:  logGroup,
		DeadLetterQueue: dlq,
	}
}

// GetFunction returns the underlying Lambda function
func (f *LiftFunction) GetFunction() awslambda.Function {
	return f.Function
}

// GetLogGroup returns the CloudWatch log group
func (f *LiftFunction) GetLogGroup() awslogs.LogGroup {
	return f.LogGroup
}

// GetDeadLetterQueue returns the dead letter queue if configured
func (f *LiftFunction) GetDeadLetterQueue() awssqs.IQueue {
	return f.DeadLetterQueue
}

// AddEnvironment adds an environment variable to the function
func (f *LiftFunction) AddEnvironment(key *string, value *string) {
	f.Function.AddEnvironment(key, value, nil)
}

// GrantInvoke grants invoke permissions to the given principal
func (f *LiftFunction) GrantInvoke(grantee awsiam.IGrantable) awsiam.Grant {
	return f.Function.GrantInvoke(grantee)
}

// AddEventSource adds an event source to the function
func (f *LiftFunction) AddEventSource(source awslambda.IEventSource) {
	f.Function.AddEventSource(source)
}

// AddToRolePolicy adds a policy statement to the function's role
func (f *LiftFunction) AddToRolePolicy(statement awsiam.PolicyStatement) {
	f.Function.AddToRolePolicy(statement)
}

// Metric returns a CloudWatch metric for this function
func (f *LiftFunction) Metric(metricName *string, props *awscloudwatch.MetricOptions) awscloudwatch.Metric {
	return f.Function.Metric(metricName, props)
}

// ConfigureDynamORM adds DynamORM environment variables to an existing function
func (f *LiftFunction) ConfigureDynamORM(tableName *string, debug *bool) {
	f.AddEnvironment(jsii.String("DYNAMORM_REGION"), awscdk.Stack_Of(f).Region())
	f.AddEnvironment(jsii.String("DYNAMODB_TABLE_NAME"), tableName)
	
	debugMode := "false"
	if debug != nil && *debug {
		debugMode = "true"
	}
	f.AddEnvironment(jsii.String("DYNAMORM_DEBUG"), jsii.String(debugMode))
	f.AddEnvironment(jsii.String("DYNAMORM_RETRY_MAX_ATTEMPTS"), jsii.String("3"))
	f.AddEnvironment(jsii.String("DYNAMORM_RETRY_BASE_DELAY"), jsii.String("100"))
}

// Helper function to convert retention days
func getRetentionDays(days float64) awslogs.RetentionDays {
	switch days {
	case 1:
		return awslogs.RetentionDays_ONE_DAY
	case 3:
		return awslogs.RetentionDays_THREE_DAYS
	case 5:
		return awslogs.RetentionDays_FIVE_DAYS
	case 7:
		return awslogs.RetentionDays_ONE_WEEK
	case 14:
		return awslogs.RetentionDays_TWO_WEEKS
	case 30:
		return awslogs.RetentionDays_ONE_MONTH
	case 60:
		return awslogs.RetentionDays_TWO_MONTHS
	case 90:
		return awslogs.RetentionDays_THREE_MONTHS
	case 120:
		return awslogs.RetentionDays_FOUR_MONTHS
	case 150:
		return awslogs.RetentionDays_FIVE_MONTHS
	case 180:
		return awslogs.RetentionDays_SIX_MONTHS
	case 365:
		return awslogs.RetentionDays_ONE_YEAR
	case 400:
		return awslogs.RetentionDays_THIRTEEN_MONTHS
	case 545:
		return awslogs.RetentionDays_EIGHTEEN_MONTHS
	case 731:
		return awslogs.RetentionDays_TWO_YEARS
	case 1096:
		return awslogs.RetentionDays_THREE_YEARS
	case 1827:
		return awslogs.RetentionDays_FIVE_YEARS
	case 2192:
		return awslogs.RetentionDays_SIX_YEARS
	case 2557:
		return awslogs.RetentionDays_SEVEN_YEARS
	case 2922:
		return awslogs.RetentionDays_EIGHT_YEARS
	case 3288:
		return awslogs.RetentionDays_NINE_YEARS
	case 3653:
		return awslogs.RetentionDays_TEN_YEARS
	default:
		// Default to 30 days if not a valid value
		return awslogs.RetentionDays_ONE_MONTH
	}
}