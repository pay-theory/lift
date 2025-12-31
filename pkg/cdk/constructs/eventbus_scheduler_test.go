package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/require"
)

func TestEventBusScheduler_CreatesScheduleAndTable(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	scheduler := NewEventBusScheduler(stack, jsii.String("Scheduler"), &EventBusSchedulerProps{
		AppName:            jsii.String("myapp"),
		Stage:              jsii.String("live"),
		ScheduleExpression: jsii.String("rate(1 minute)"),
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	require.NotNil(t, scheduler)
	require.NotNil(t, scheduler.Table)
	require.NotNil(t, scheduler.Handler)

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::DynamoDB::Table")
	assertResourceExists(t, template, "AWS::Lambda::Function")
	assertResourceExists(t, template, "AWS::Events::Rule")

	// Assert deterministic table name.
	tables := findResourcesByType(template, "AWS::DynamoDB::Table")
	foundTableName := false
	for _, table := range tables {
		props, ok := table["Properties"].(map[string]interface{})
		if !ok {
			continue
		}
		if props["TableName"] == "myapp-events-live" {
			foundTableName = true
			break
		}
	}
	require.True(t, foundTableName, "expected table name myapp-events-live")

	// Assert function name and injected naming env vars.
	functions := findResourcesByType(template, "AWS::Lambda::Function")
	foundFunction := false
	for _, fn := range functions {
		props, ok := fn["Properties"].(map[string]interface{})
		if !ok {
			continue
		}
		if props["FunctionName"] != "myapp-eventbus-scheduler-live" {
			continue
		}

		env, ok := props["Environment"].(map[string]interface{})
		require.True(t, ok, "expected Environment on scheduler function")
		vars, ok := env["Variables"].(map[string]interface{})
		require.True(t, ok, "expected Environment.Variables on scheduler function")
		require.Equal(t, "myapp", vars["APP_NAME"])
		require.Equal(t, "live", vars["STAGE"])

		foundFunction = true
		break
	}
	require.True(t, foundFunction, "expected scheduler function myapp-eventbus-scheduler-live")
}
