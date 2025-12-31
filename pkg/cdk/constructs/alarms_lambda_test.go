package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/jsii-runtime-go"
)

func TestNewLiftLambdaAlarms_CreatesThreeAlarms(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	topic := awssns.NewTopic(stack, jsii.String("Topic"), nil)

	alarms := NewLiftLambdaAlarms(stack, jsii.String("Alarms"), &LambdaAlarmsProps{
		FunctionName:    jsii.String("test-fn"),
		AlarmTopic:      topic,
		AlarmNamePrefix: jsii.String("service-partner-stage-fn"),
	})

	if alarms.ErrorsAlarm == nil || alarms.ThrottlesAlarm == nil || alarms.DurationAlarm == nil {
		t.Fatal("expected lambda alarms to be created")
	}

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(3))
}

func TestNewLiftLambdaAlarms_PanicsWithoutRequiredProps(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	NewLiftLambdaAlarms(stack, jsii.String("Alarms"), &LambdaAlarmsProps{
		AlarmNamePrefix: jsii.String("missing-topic-and-function"),
	})
}
