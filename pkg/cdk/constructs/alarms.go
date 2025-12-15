package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// BaseAlarmsConfig contains common alarm configuration fields
type BaseAlarmsConfig struct {
	// EvaluationPeriods is the number of periods to evaluate
	EvaluationPeriods *float64

	// Period is the evaluation period in seconds
	// Default: 300 (5 minutes)
	Period *float64
}

// applyBaseAlarmsDefaults applies default values to the base config fields
func applyBaseAlarmsDefaults(evaluationPeriods, period *float64, defaultEvalPeriods float64) (*float64, *float64) {
	if evaluationPeriods == nil {
		evaluationPeriods = jsii.Number(defaultEvalPeriods)
	}
	if period == nil {
		period = jsii.Number(300) // 5 minutes default
	}
	return evaluationPeriods, period
}

// SQSAlarmsConfig defines configuration for SQS alarms
type SQSAlarmsConfig struct {
	BaseAlarmsConfig

	// VisibleMessagesThreshold is the threshold for visible messages alarm
	// Default: 2
	VisibleMessagesThreshold *float64

	// NotVisibleMessagesThreshold is the threshold for not visible messages alarm
	// Default: 2
	NotVisibleMessagesThreshold *float64

	// OldestMessageAgeThreshold is the threshold in seconds for oldest message alarm
	// Default: 900 (15 minutes)
	OldestMessageAgeThreshold *float64

	// DLQOldestMessageAgeThreshold is the threshold in seconds for DLQ oldest message alarm
	// Default: 1 (alert immediately when any message hits DLQ)
	DLQOldestMessageAgeThreshold *float64
}

// SQSAlarmsProps defines properties for creating SQS alarms
type SQSAlarmsProps struct {
	// Queue is the main SQS queue to monitor (required)
	Queue awssqs.IQueue

	// DeadLetterQueue is the DLQ to monitor (optional)
	DeadLetterQueue awssqs.IQueue

	// AlarmTopic is the SNS topic for alarm notifications (required)
	AlarmTopic awssns.ITopic

	// AlarmNamePrefix is the prefix for alarm names (required)
	// Example: "merchant-application-partner-stage"
	AlarmNamePrefix *string

	// Config contains threshold configuration (optional - uses defaults if nil)
	Config *SQSAlarmsConfig
}

// LiftSQSAlarms contains CloudWatch alarms for SQS queues
type LiftSQSAlarms struct {
	Construct constructs.Construct

	// Main queue alarms
	VisibleMessagesAlarm    awscloudwatch.Alarm
	NotVisibleMessagesAlarm awscloudwatch.Alarm
	OldestMessageAlarm      awscloudwatch.Alarm

	// DLQ alarms
	DLQVisibleMessagesAlarm    awscloudwatch.Alarm
	DLQNotVisibleMessagesAlarm awscloudwatch.Alarm
	DLQOldestMessageAlarm      awscloudwatch.Alarm
}

// NewLiftSQSAlarms creates CloudWatch alarms for SQS queues
func NewLiftSQSAlarms(scope constructs.Construct, id *string, props *SQSAlarmsProps) *LiftSQSAlarms {
	if props.Queue == nil {
		panic("Queue is required")
	}
	if props.AlarmTopic == nil {
		panic("AlarmTopic is required")
	}
	if props.AlarmNamePrefix == nil {
		panic("AlarmNamePrefix is required")
	}

	construct := constructs.NewConstruct(scope, id)
	this := &LiftSQSAlarms{Construct: construct}

	// Apply defaults
	config := applySQSAlarmsDefaults(props.Config)

	// Create SNS action for alarms
	snsAction := awscloudwatchactions.NewSnsAction(props.AlarmTopic)

	// Create main queue alarms
	this.VisibleMessagesAlarm = createSQSAlarm(this.Construct, &sqsAlarmInput{
		alarmName:   fmt.Sprintf("sqs-visible-messages-%s", *props.AlarmNamePrefix),
		description: "Alarm if there are too many visible messages in the SQS Queue",
		metricName:  "ApproximateNumberOfMessagesVisible",
		queueName:   props.Queue.QueueName(),
		threshold:   config.VisibleMessagesThreshold,
		periods:     config.EvaluationPeriods,
		period:      config.Period,
		snsAction:   snsAction,
	})

	this.NotVisibleMessagesAlarm = createSQSAlarm(this.Construct, &sqsAlarmInput{
		alarmName:   fmt.Sprintf("sqs-not-visible-messages-%s", *props.AlarmNamePrefix),
		description: "Alarm if there are too many not visible messages in the SQS Queue",
		metricName:  "ApproximateNumberOfMessagesNotVisible",
		queueName:   props.Queue.QueueName(),
		threshold:   config.NotVisibleMessagesThreshold,
		periods:     config.EvaluationPeriods,
		period:      config.Period,
		snsAction:   snsAction,
	})

	this.OldestMessageAlarm = createSQSAlarm(this.Construct, &sqsAlarmInput{
		alarmName:   fmt.Sprintf("sqs-oldest-messages-%s", *props.AlarmNamePrefix),
		description: "Alarm if messages are waiting too long in the SQS Queue",
		metricName:  "ApproximateAgeOfOldestMessage",
		queueName:   props.Queue.QueueName(),
		threshold:   config.OldestMessageAgeThreshold,
		periods:     jsii.Number(3), // 3 periods for oldest message
		period:      config.Period,
		statistic:   jsii.String("Maximum"),
		snsAction:   snsAction,
	})

	// Create DLQ alarms if DLQ is provided
	if props.DeadLetterQueue != nil {
		dlqPrefix := fmt.Sprintf("%s-dlq", *props.AlarmNamePrefix)

		this.DLQVisibleMessagesAlarm = createSQSAlarm(this.Construct, &sqsAlarmInput{
			alarmName:   fmt.Sprintf("sqs-visible-messages-%s", dlqPrefix),
			description: "Alarm if there are too many visible messages in the DLQ",
			metricName:  "ApproximateNumberOfMessagesVisible",
			queueName:   props.DeadLetterQueue.QueueName(),
			threshold:   config.VisibleMessagesThreshold,
			periods:     config.EvaluationPeriods,
			period:      config.Period,
			snsAction:   snsAction,
		})

		this.DLQNotVisibleMessagesAlarm = createSQSAlarm(this.Construct, &sqsAlarmInput{
			alarmName:   fmt.Sprintf("sqs-not-visible-messages-%s", dlqPrefix),
			description: "Alarm if there are too many not visible messages in the DLQ",
			metricName:  "ApproximateNumberOfMessagesNotVisible",
			queueName:   props.DeadLetterQueue.QueueName(),
			threshold:   config.NotVisibleMessagesThreshold,
			periods:     config.EvaluationPeriods,
			period:      config.Period,
			snsAction:   snsAction,
		})

		this.DLQOldestMessageAlarm = createSQSAlarm(this.Construct, &sqsAlarmInput{
			alarmName:   fmt.Sprintf("sqs-oldest-messages-%s", dlqPrefix),
			description: "Alarm for messages in DLQ exceeding age threshold",
			metricName:  "ApproximateAgeOfOldestMessage",
			queueName:   props.DeadLetterQueue.QueueName(),
			threshold:   config.DLQOldestMessageAgeThreshold,
			periods:     jsii.Number(1), // Alert immediately for DLQ
			period:      jsii.Number(60),
			statistic:   jsii.String("Maximum"),
			snsAction:   snsAction,
		})
	}

	return this
}

type sqsAlarmInput struct {
	snsAction   awscloudwatch.IAlarmAction
	queueName   *string
	threshold   *float64
	periods     *float64
	period      *float64
	statistic   *string
	alarmName   string
	description string
	metricName  string
}

func createSQSAlarm(scope constructs.Construct, input *sqsAlarmInput) awscloudwatch.Alarm {
	statistic := input.statistic
	if statistic == nil {
		statistic = jsii.String("Sum")
	}

	metric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/SQS"),
		MetricName: jsii.String(input.metricName),
		DimensionsMap: &map[string]*string{
			"QueueName": input.queueName,
		},
		Statistic: statistic,
		Period:    awscdk.Duration_Seconds(input.period),
	})

	alarm := awscloudwatch.NewAlarm(scope, jsii.String(input.alarmName), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(input.alarmName),
		AlarmDescription:   jsii.String(input.description),
		Metric:             metric,
		Threshold:          input.threshold,
		EvaluationPeriods:  input.periods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})

	alarm.AddAlarmAction(input.snsAction)

	return alarm
}

func applySQSAlarmsDefaults(config *SQSAlarmsConfig) *SQSAlarmsConfig {
	if config == nil {
		config = &SQSAlarmsConfig{}
	}

	if config.VisibleMessagesThreshold == nil {
		config.VisibleMessagesThreshold = jsii.Number(2)
	}
	if config.NotVisibleMessagesThreshold == nil {
		config.NotVisibleMessagesThreshold = jsii.Number(2)
	}
	if config.OldestMessageAgeThreshold == nil {
		config.OldestMessageAgeThreshold = jsii.Number(900) // 15 minutes
	}
	if config.DLQOldestMessageAgeThreshold == nil {
		config.DLQOldestMessageAgeThreshold = jsii.Number(1) // Alert immediately
	}
	config.EvaluationPeriods, config.Period = applyBaseAlarmsDefaults(config.EvaluationPeriods, config.Period, 1)

	return config
}

// APIGatewayAlarmsConfig defines configuration for API Gateway alarms
type APIGatewayAlarmsConfig struct {
	BaseAlarmsConfig

	// ClientErrorThreshold is the threshold for 4xx errors
	// Default: 10
	ClientErrorThreshold *float64

	// ServerErrorThreshold is the threshold for 5xx errors
	// Default: 5
	ServerErrorThreshold *float64
}

// APIGatewayAlarmsProps defines properties for creating API Gateway alarms
type APIGatewayAlarmsProps struct {
	// ApiId is the API Gateway ID (required)
	ApiId *string

	// StageName is the API Gateway stage name
	// Default: "latest"
	StageName *string

	// AlarmTopic is the SNS topic for alarm notifications (required)
	AlarmTopic awssns.ITopic

	// AlarmNamePrefix is the prefix for alarm names (required)
	// Example: "merchant-application-partner-stage"
	AlarmNamePrefix *string

	// Config contains threshold configuration (optional - uses defaults if nil)
	Config *APIGatewayAlarmsConfig
}

// LiftAPIGatewayAlarms contains CloudWatch alarms for API Gateway
type LiftAPIGatewayAlarms struct {
	Construct constructs.Construct

	ClientErrorsAlarm awscloudwatch.Alarm
	ServerErrorsAlarm awscloudwatch.Alarm
}

// NewLiftAPIGatewayAlarms creates CloudWatch alarms for API Gateway
func NewLiftAPIGatewayAlarms(scope constructs.Construct, id *string, props *APIGatewayAlarmsProps) *LiftAPIGatewayAlarms {
	if props.ApiId == nil {
		panic("ApiId is required")
	}
	if props.AlarmTopic == nil {
		panic("AlarmTopic is required")
	}
	if props.AlarmNamePrefix == nil {
		panic("AlarmNamePrefix is required")
	}

	construct := constructs.NewConstruct(scope, id)
	this := &LiftAPIGatewayAlarms{Construct: construct}

	// Apply defaults
	config := applyAPIGatewayAlarmsDefaults(props.Config)
	stageName := props.StageName
	if stageName == nil {
		stageName = jsii.String("latest")
	}

	// Create SNS action for alarms
	snsAction := awscloudwatchactions.NewSnsAction(props.AlarmTopic)

	// Create 4xx client errors alarm
	clientErrorMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGateway"),
		MetricName: jsii.String("4xx"),
		DimensionsMap: &map[string]*string{
			"ApiId": props.ApiId,
			"Stage": stageName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	clientAlarmName := fmt.Sprintf("api-gateway-client-errors-%s", *props.AlarmNamePrefix)
	this.ClientErrorsAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("ClientErrorsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(clientAlarmName),
		AlarmDescription:   jsii.String("Alarm if API Gateway client errors hit threshold"),
		Metric:             clientErrorMetric,
		Threshold:          config.ClientErrorThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.ClientErrorsAlarm.AddAlarmAction(snsAction)

	// Create 5xx server errors alarm
	serverErrorMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGateway"),
		MetricName: jsii.String("5xx"),
		DimensionsMap: &map[string]*string{
			"ApiId": props.ApiId,
			"Stage": stageName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	serverAlarmName := fmt.Sprintf("api-gateway-server-errors-%s", *props.AlarmNamePrefix)
	this.ServerErrorsAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("ServerErrorsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(serverAlarmName),
		AlarmDescription:   jsii.String("Alarm if API Gateway server errors hit threshold"),
		Metric:             serverErrorMetric,
		Threshold:          config.ServerErrorThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.ServerErrorsAlarm.AddAlarmAction(snsAction)

	return this
}

func applyAPIGatewayAlarmsDefaults(config *APIGatewayAlarmsConfig) *APIGatewayAlarmsConfig {
	if config == nil {
		config = &APIGatewayAlarmsConfig{}
	}

	if config.ClientErrorThreshold == nil {
		config.ClientErrorThreshold = jsii.Number(10)
	}
	if config.ServerErrorThreshold == nil {
		config.ServerErrorThreshold = jsii.Number(5)
	}
	config.EvaluationPeriods, config.Period = applyBaseAlarmsDefaults(config.EvaluationPeriods, config.Period, 3)

	return config
}

// DynamoDBAlarmsConfig defines configuration for DynamoDB alarms
type DynamoDBAlarmsConfig struct {
	BaseAlarmsConfig

	// LatencyThreshold is the threshold in milliseconds for latency alarm
	// Default: 250
	LatencyThreshold *float64

	// ReadCapacityThreshold is the threshold for consumed read capacity units
	// Default: 900
	ReadCapacityThreshold *float64

	// WriteCapacityThreshold is the threshold for consumed write capacity units
	// Default: 900
	WriteCapacityThreshold *float64
}

// DynamoDBAlarmsProps defines properties for creating DynamoDB alarms
type DynamoDBAlarmsProps struct {
	// TableName is the DynamoDB table name (required)
	TableName *string

	// AlarmTopic is the SNS topic for alarm notifications (required)
	AlarmTopic awssns.ITopic

	// AlarmNamePrefix is the prefix for alarm names (required)
	// Example: "merchant-application-partner-stage"
	AlarmNamePrefix *string

	// Config contains threshold configuration (optional - uses defaults if nil)
	Config *DynamoDBAlarmsConfig
}

// LiftDynamoDBAlarms contains CloudWatch alarms for DynamoDB
type LiftDynamoDBAlarms struct {
	Construct constructs.Construct

	LatencyAlarm       awscloudwatch.Alarm
	ReadCapacityAlarm  awscloudwatch.Alarm
	WriteCapacityAlarm awscloudwatch.Alarm
}

// NewLiftDynamoDBAlarms creates CloudWatch alarms for DynamoDB tables
func NewLiftDynamoDBAlarms(scope constructs.Construct, id *string, props *DynamoDBAlarmsProps) *LiftDynamoDBAlarms {
	if props.TableName == nil {
		panic("TableName is required")
	}
	if props.AlarmTopic == nil {
		panic("AlarmTopic is required")
	}
	if props.AlarmNamePrefix == nil {
		panic("AlarmNamePrefix is required")
	}

	construct := constructs.NewConstruct(scope, id)
	this := &LiftDynamoDBAlarms{Construct: construct}

	// Apply defaults
	config := applyDynamoDBAlarmsDefaults(props.Config)

	// Create SNS action for alarms
	snsAction := awscloudwatchactions.NewSnsAction(props.AlarmTopic)

	// Create latency alarm
	latencyMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("SuccessfulRequestLatency"),
		DimensionsMap: &map[string]*string{
			"TableName": props.TableName,
			"Operation": jsii.String("PutItem"),
		},
		Statistic: jsii.String("Average"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	latencyAlarmName := fmt.Sprintf("dynamodb-latency-%s", *props.AlarmNamePrefix)
	this.LatencyAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("LatencyAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(latencyAlarmName),
		AlarmDescription:   jsii.String("Alarm if DynamoDB latency hits threshold"),
		Metric:             latencyMetric,
		Threshold:          config.LatencyThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.LatencyAlarm.AddAlarmAction(snsAction)

	// Create read capacity alarm
	readCapacityMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("ConsumedReadCapacityUnits"),
		DimensionsMap: &map[string]*string{
			"TableName": props.TableName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	readAlarmName := fmt.Sprintf("dynamodb-read-capacity-%s", *props.AlarmNamePrefix)
	this.ReadCapacityAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("ReadCapacityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(readAlarmName),
		AlarmDescription:   jsii.String("Alarm if DynamoDB table read capacity exceeds threshold"),
		Metric:             readCapacityMetric,
		Threshold:          config.ReadCapacityThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.ReadCapacityAlarm.AddAlarmAction(snsAction)

	// Create write capacity alarm
	writeCapacityMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/DynamoDB"),
		MetricName: jsii.String("ConsumedWriteCapacityUnits"),
		DimensionsMap: &map[string]*string{
			"TableName": props.TableName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	writeAlarmName := fmt.Sprintf("dynamodb-write-capacity-%s", *props.AlarmNamePrefix)
	this.WriteCapacityAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("WriteCapacityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(writeAlarmName),
		AlarmDescription:   jsii.String("Alarm if DynamoDB table write capacity exceeds threshold"),
		Metric:             writeCapacityMetric,
		Threshold:          config.WriteCapacityThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.WriteCapacityAlarm.AddAlarmAction(snsAction)

	return this
}

func applyDynamoDBAlarmsDefaults(config *DynamoDBAlarmsConfig) *DynamoDBAlarmsConfig {
	if config == nil {
		config = &DynamoDBAlarmsConfig{}
	}

	if config.LatencyThreshold == nil {
		config.LatencyThreshold = jsii.Number(250) // 250ms
	}
	if config.ReadCapacityThreshold == nil {
		config.ReadCapacityThreshold = jsii.Number(900)
	}
	if config.WriteCapacityThreshold == nil {
		config.WriteCapacityThreshold = jsii.Number(900)
	}
	config.EvaluationPeriods, config.Period = applyBaseAlarmsDefaults(config.EvaluationPeriods, config.Period, 3)

	return config
}

// LambdaAlarmsConfig defines configuration for Lambda alarms
type LambdaAlarmsConfig struct {
	BaseAlarmsConfig

	// ErrorThreshold is the threshold for Lambda errors
	// Default: 1
	ErrorThreshold *float64

	// ThrottleThreshold is the threshold for Lambda throttles
	// Default: 1
	ThrottleThreshold *float64

	// DurationThreshold is the threshold in milliseconds for duration alarm
	// Default: 30000 (30 seconds)
	DurationThreshold *float64
}

// LambdaAlarmsProps defines properties for creating Lambda alarms
type LambdaAlarmsProps struct {
	// FunctionName is the Lambda function name (required)
	FunctionName *string

	// AlarmTopic is the SNS topic for alarm notifications (required)
	AlarmTopic awssns.ITopic

	// AlarmNamePrefix is the prefix for alarm names (required)
	// Example: "merchant-application-partner-stage-my-function"
	AlarmNamePrefix *string

	// Config contains threshold configuration (optional - uses defaults if nil)
	Config *LambdaAlarmsConfig
}

// LiftLambdaAlarms contains CloudWatch alarms for Lambda functions
type LiftLambdaAlarms struct {
	Construct constructs.Construct

	ErrorsAlarm    awscloudwatch.Alarm
	ThrottlesAlarm awscloudwatch.Alarm
	DurationAlarm  awscloudwatch.Alarm
}

// NewLiftLambdaAlarms creates CloudWatch alarms for Lambda functions
func NewLiftLambdaAlarms(scope constructs.Construct, id *string, props *LambdaAlarmsProps) *LiftLambdaAlarms {
	if props.FunctionName == nil {
		panic("FunctionName is required")
	}
	if props.AlarmTopic == nil {
		panic("AlarmTopic is required")
	}
	if props.AlarmNamePrefix == nil {
		panic("AlarmNamePrefix is required")
	}

	construct := constructs.NewConstruct(scope, id)
	this := &LiftLambdaAlarms{Construct: construct}

	// Apply defaults
	config := applyLambdaAlarmsDefaults(props.Config)

	// Create SNS action for alarms
	snsAction := awscloudwatchactions.NewSnsAction(props.AlarmTopic)

	// Create errors alarm
	errorsMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/Lambda"),
		MetricName: jsii.String("Errors"),
		DimensionsMap: &map[string]*string{
			"FunctionName": props.FunctionName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	errorsAlarmName := fmt.Sprintf("lambda-errors-%s", *props.AlarmNamePrefix)
	this.ErrorsAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("ErrorsAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(errorsAlarmName),
		AlarmDescription:   jsii.String("Alarm if Lambda function errors hit threshold"),
		Metric:             errorsMetric,
		Threshold:          config.ErrorThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.ErrorsAlarm.AddAlarmAction(snsAction)

	// Create throttles alarm
	throttlesMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/Lambda"),
		MetricName: jsii.String("Throttles"),
		DimensionsMap: &map[string]*string{
			"FunctionName": props.FunctionName,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	throttlesAlarmName := fmt.Sprintf("lambda-throttles-%s", *props.AlarmNamePrefix)
	this.ThrottlesAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("ThrottlesAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(throttlesAlarmName),
		AlarmDescription:   jsii.String("Alarm if Lambda function throttles hit threshold"),
		Metric:             throttlesMetric,
		Threshold:          config.ThrottleThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.ThrottlesAlarm.AddAlarmAction(snsAction)

	// Create duration alarm
	durationMetric := awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/Lambda"),
		MetricName: jsii.String("Duration"),
		DimensionsMap: &map[string]*string{
			"FunctionName": props.FunctionName,
		},
		Statistic: jsii.String("Average"),
		Period:    awscdk.Duration_Seconds(config.Period),
	})

	durationAlarmName := fmt.Sprintf("lambda-duration-%s", *props.AlarmNamePrefix)
	this.DurationAlarm = awscloudwatch.NewAlarm(this.Construct, jsii.String("DurationAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:          jsii.String(durationAlarmName),
		AlarmDescription:   jsii.String("Alarm if Lambda function duration hits threshold"),
		Metric:             durationMetric,
		Threshold:          config.DurationThreshold,
		EvaluationPeriods:  config.EvaluationPeriods,
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	this.DurationAlarm.AddAlarmAction(snsAction)

	return this
}

func applyLambdaAlarmsDefaults(config *LambdaAlarmsConfig) *LambdaAlarmsConfig {
	if config == nil {
		config = &LambdaAlarmsConfig{}
	}

	if config.ErrorThreshold == nil {
		config.ErrorThreshold = jsii.Number(1)
	}
	if config.ThrottleThreshold == nil {
		config.ThrottleThreshold = jsii.Number(1)
	}
	if config.DurationThreshold == nil {
		config.DurationThreshold = jsii.Number(30000) // 30 seconds
	}
	config.EvaluationPeriods, config.Period = applyBaseAlarmsDefaults(config.EvaluationPeriods, config.Period, 3)

	return config
}
