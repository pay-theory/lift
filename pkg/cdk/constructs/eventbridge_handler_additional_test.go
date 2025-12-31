package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func TestEventBridgeHandler_MonitoringAndHelperMethods(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	handler, err := NewEventBridgeHandler(stack, jsii.String("Handler"), &EventBridgeHandlerProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("monitored-eventbridge-handler"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableMonitoring:      jsii.Bool(true),
		MaxEventAge:           awscdk.Duration_Minutes(jsii.Number(10)),
		RetryAttempts:         jsii.Number(7),
		EnableDeadLetterQueue: jsii.Bool(true),
		RuleProps: &awsevents.RuleProps{
			RuleName:     jsii.String("custom-rule"),
			Description:  jsii.String("custom description"),
			Enabled:      jsii.Bool(false),
			EventPattern: &awsevents.EventPattern{Source: &[]*string{jsii.String("custom.source")}},
		},
		EventPattern: &awsevents.EventPattern{
			Source: &[]*string{jsii.String("custom.source")},
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	otherFn := awslambda.NewFunction(stack, jsii.String("OtherFn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
	})

	handler.GrantPutEvents(otherFn)
	handler.AddEnvironmentVariable("EXTRA", "value")

	if handler.GetEventBusName() == nil {
		t.Fatal("expected event bus name")
	}
	if handler.GetRuleName() == nil {
		t.Fatal("expected rule name")
	}
	if handler.GetRuleArn() == nil {
		t.Fatal("expected rule arn")
	}

	if err := handler.AddEventPattern(nil); err == nil {
		t.Fatal("expected error from AddEventPattern")
	}
	if err := handler.EnableRule(); err == nil {
		t.Fatal("expected error from EnableRule")
	}
	if err := handler.DisableRule(); err == nil {
		t.Fatal("expected error from DisableRule")
	}

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::CloudWatch::Alarm")
}
