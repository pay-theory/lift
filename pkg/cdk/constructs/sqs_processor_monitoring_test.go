package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func TestSQSProcessor_MonitoringAndHelperMethods(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	processor := NewSQSProcessor(stack, jsii.String("Processor"), &SQSProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("monitored-sqs-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		EnableMonitoring: jsii.Bool(true),
	})

	if processor.GetQueueName() == nil {
		t.Fatal("expected queue name")
	}
	if processor.GetQueueUrl() == nil {
		t.Fatal("expected queue url")
	}
	if processor.GetQueueArn() == nil {
		t.Fatal("expected queue arn")
	}

	processor.AddEnvironmentVariable("EXTRA", "1")

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::CloudWatch::Alarm")
	assertResourceExists(t, template, "AWS::CloudWatch::Dashboard")
}
