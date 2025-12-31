package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestEnhancedMonitoring_LambdaResourceCreatesDashboardAndAlarms(t *testing.T) {
	stack := test.NewTestStack()

	fn := NewLiftFunction(stack.Stack(), jsii.String("Fn"), &LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			Runtime: awslambda.Runtime_NODEJS_18_X(),
			Handler: jsii.String("index.handler"),
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		},
	})

	topic := awssns.NewTopic(stack.Stack(), jsii.String("Alerts"), nil)

	monitoring := NewEnhancedMonitoring(stack.Stack(), jsii.String("Monitoring"), &EnhancedMonitoringProps{
		Resource:                fn,
		AlertTopic:              topic,
		EnableRealTimeStreaming: jsii.Bool(true),
	})

	if monitoring.GetMetric("Requests") == nil {
		t.Fatal("expected Requests metric")
	}
	if monitoring.GetAlarm("HighErrorRate") == nil {
		t.Fatal("expected HighErrorRate alarm")
	}

	monitoring.AddCustomMetric("Custom", awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("Test"),
		MetricName: jsii.String("Custom"),
	}))
	if monitoring.GetMetric("Custom") == nil {
		t.Fatal("expected custom metric")
	}

	monitoring.AddCustomAlarm("CustomAlarm", awscloudwatch.NewAlarm(stack.Stack(), jsii.String("CustomAlarm"), &awscloudwatch.AlarmProps{
		Metric:            monitoring.GetMetric("Custom"),
		Threshold:         jsii.Number(1),
		EvaluationPeriods: jsii.Number(1),
	}))
	if monitoring.GetAlarm("CustomAlarm") == nil {
		t.Fatal("expected custom alarm")
	}

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Dashboard"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(5))
	template.ResourceCountIs(jsii.String("AWS::Logs::MetricFilter"), jsii.Number(2))
}

func TestEnhancedMonitoring_APICreatesDashboard(t *testing.T) {
	stack := test.NewTestStack()

	api := NewLiftAPI(stack.Stack(), jsii.String("API"), &LiftAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-api"),
		},
	})

	NewEnhancedMonitoring(stack.Stack(), jsii.String("Monitoring"), &EnhancedMonitoringProps{
		Resource: api,
	})

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Dashboard"), jsii.Number(1))
}

func TestEnhancedMonitoring_DynamoDBCreatesDashboard(t *testing.T) {
	stack := test.NewTestStack()

	table := NewLiftTable(stack.Stack(), jsii.String("Table"), &LiftTableProps{
		TableName:        jsii.String("test-table"),
		PartitionKeyName: jsii.String("PK"),
		SortKeyName:      jsii.String("SK"),
	})

	NewEnhancedMonitoring(stack.Stack(), jsii.String("Monitoring"), &EnhancedMonitoringProps{
		Resource: table,
	})

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Dashboard"), jsii.Number(1))
}
