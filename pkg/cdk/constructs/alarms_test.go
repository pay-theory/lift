package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/jsii-runtime-go"
)

func TestNewLiftSQSAlarms(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create test queue
	queue := awssqs.NewQueue(stack, jsii.String("TestQueue"), &awssqs.QueueProps{
		QueueName: jsii.String("test-queue"),
	})

	// Create test DLQ
	dlq := awssqs.NewQueue(stack, jsii.String("TestDLQ"), &awssqs.QueueProps{
		QueueName: jsii.String("test-dlq"),
	})

	// Create test SNS topic
	topic := awssns.NewTopic(stack, jsii.String("TestTopic"), &awssns.TopicProps{
		TopicName: jsii.String("test-topic"),
	})

	// Create SQS alarms
	alarms := NewLiftSQSAlarms(stack, jsii.String("TestSQSAlarms"), &SQSAlarmsProps{
		Queue:           queue,
		DeadLetterQueue: dlq,
		AlarmTopic:      topic,
		AlarmNamePrefix: jsii.String("test-service-partner-stage"),
	})

	// Verify alarms were created
	if alarms.VisibleMessagesAlarm == nil {
		t.Error("VisibleMessagesAlarm should not be nil")
	}
	if alarms.NotVisibleMessagesAlarm == nil {
		t.Error("NotVisibleMessagesAlarm should not be nil")
	}
	if alarms.OldestMessageAlarm == nil {
		t.Error("OldestMessageAlarm should not be nil")
	}
	if alarms.DLQVisibleMessagesAlarm == nil {
		t.Error("DLQVisibleMessagesAlarm should not be nil")
	}
	if alarms.DLQNotVisibleMessagesAlarm == nil {
		t.Error("DLQNotVisibleMessagesAlarm should not be nil")
	}
	if alarms.DLQOldestMessageAlarm == nil {
		t.Error("DLQOldestMessageAlarm should not be nil")
	}

	// Verify CloudFormation template
	template := assertions.Template_FromStack(stack, nil)

	// Check that 6 alarms were created (3 for main queue, 3 for DLQ)
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(6))
}

func TestNewLiftSQSAlarmsWithoutDLQ(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create test queue
	queue := awssqs.NewQueue(stack, jsii.String("TestQueue"), &awssqs.QueueProps{
		QueueName: jsii.String("test-queue"),
	})

	// Create test SNS topic
	topic := awssns.NewTopic(stack, jsii.String("TestTopic"), &awssns.TopicProps{
		TopicName: jsii.String("test-topic"),
	})

	// Create SQS alarms without DLQ
	alarms := NewLiftSQSAlarms(stack, jsii.String("TestSQSAlarms"), &SQSAlarmsProps{
		Queue:           queue,
		AlarmTopic:      topic,
		AlarmNamePrefix: jsii.String("test-service-partner-stage"),
	})

	// Verify main queue alarms were created
	if alarms.VisibleMessagesAlarm == nil {
		t.Error("VisibleMessagesAlarm should not be nil")
	}

	// Verify DLQ alarms are nil
	if alarms.DLQVisibleMessagesAlarm != nil {
		t.Error("DLQVisibleMessagesAlarm should be nil when no DLQ provided")
	}

	// Verify CloudFormation template
	template := assertions.Template_FromStack(stack, nil)

	// Check that only 3 alarms were created (main queue only)
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(3))
}

func TestNewLiftSQSAlarmsWithCustomConfig(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create test queue
	queue := awssqs.NewQueue(stack, jsii.String("TestQueue"), &awssqs.QueueProps{
		QueueName: jsii.String("test-queue"),
	})

	// Create test SNS topic
	topic := awssns.NewTopic(stack, jsii.String("TestTopic"), &awssns.TopicProps{
		TopicName: jsii.String("test-topic"),
	})

	// Create SQS alarms with custom config
	NewLiftSQSAlarms(stack, jsii.String("TestSQSAlarms"), &SQSAlarmsProps{
		Queue:           queue,
		AlarmTopic:      topic,
		AlarmNamePrefix: jsii.String("test-service"),
		Config: &SQSAlarmsConfig{
			VisibleMessagesThreshold: jsii.Number(5),
			OldestMessageAgeThreshold: jsii.Number(600),
		},
	})

	// Verify CloudFormation template has custom threshold
	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]any{
		"AlarmName": "sqs-visible-messages-test-service",
		"Threshold": 5,
	})
}

func TestNewLiftAPIGatewayAlarms(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create test SNS topic
	topic := awssns.NewTopic(stack, jsii.String("TestTopic"), &awssns.TopicProps{
		TopicName: jsii.String("test-topic"),
	})

	// Create API Gateway alarms
	alarms := NewLiftAPIGatewayAlarms(stack, jsii.String("TestAPIAlarms"), &APIGatewayAlarmsProps{
		ApiId:           jsii.String("test-api-id"),
		AlarmTopic:      topic,
		AlarmNamePrefix: jsii.String("test-service-partner-stage"),
	})

	// Verify alarms were created
	if alarms.ClientErrorsAlarm == nil {
		t.Error("ClientErrorsAlarm should not be nil")
	}
	if alarms.ServerErrorsAlarm == nil {
		t.Error("ServerErrorsAlarm should not be nil")
	}

	// Verify CloudFormation template
	template := assertions.Template_FromStack(stack, nil)

	// Check that 2 alarms were created
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(2))

	// Check 4xx alarm properties
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]any{
		"AlarmName": "api-gateway-client-errors-test-service-partner-stage",
		"Threshold": 10,
	})

	// Check 5xx alarm properties
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]any{
		"AlarmName": "api-gateway-server-errors-test-service-partner-stage",
		"Threshold": 5,
	})
}

func TestNewLiftDynamoDBAlarms(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create test SNS topic
	topic := awssns.NewTopic(stack, jsii.String("TestTopic"), &awssns.TopicProps{
		TopicName: jsii.String("test-topic"),
	})

	// Create DynamoDB alarms
	alarms := NewLiftDynamoDBAlarms(stack, jsii.String("TestDDBAlarms"), &DynamoDBAlarmsProps{
		TableName:       jsii.String("test-table"),
		AlarmTopic:      topic,
		AlarmNamePrefix: jsii.String("test-service-partner-stage"),
	})

	// Verify alarms were created
	if alarms.LatencyAlarm == nil {
		t.Error("LatencyAlarm should not be nil")
	}
	if alarms.ReadCapacityAlarm == nil {
		t.Error("ReadCapacityAlarm should not be nil")
	}
	if alarms.WriteCapacityAlarm == nil {
		t.Error("WriteCapacityAlarm should not be nil")
	}

	// Verify CloudFormation template
	template := assertions.Template_FromStack(stack, nil)

	// Check that 3 alarms were created
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(3))

	// Check latency alarm properties
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]any{
		"AlarmName": "dynamodb-latency-test-service-partner-stage",
		"Threshold": 250,
	})

	// Check read capacity alarm properties
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]any{
		"AlarmName": "dynamodb-read-capacity-test-service-partner-stage",
		"Threshold": 900,
	})

	// Check write capacity alarm properties
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]any{
		"AlarmName": "dynamodb-write-capacity-test-service-partner-stage",
		"Threshold": 900,
	})
}

func TestNewLiftDynamoDBAlarmsPanicsWithoutTableName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when TableName is nil")
		}
	}()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	topic := awssns.NewTopic(stack, jsii.String("TestTopic"), nil)

	NewLiftDynamoDBAlarms(stack, jsii.String("TestDDBAlarms"), &DynamoDBAlarmsProps{
		AlarmTopic:      topic,
		AlarmNamePrefix: jsii.String("test"),
	})
}
