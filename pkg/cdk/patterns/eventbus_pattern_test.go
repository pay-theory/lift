package patterns

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/require"
)

func TestNewEventBusPattern_PanicsWhenAppNameMissing(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	require.Panics(t, func() {
		NewEventBusPattern(stack, jsii.String("EventBus"), &EventBusPatternProps{})
	})
}

func TestNewEventBusPattern_CreatesTableAndProcessorWithNaming(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	extraEnv := map[string]*string{
		"EXTRA": jsii.String("1"),
	}
	tags := map[string]*string{
		"Environment": jsii.String("dev"),
	}

	pattern := NewEventBusPattern(stack, jsii.String("EventBus"), &EventBusPatternProps{
		AppName:                   "my-app",
		Stage:                     "dev",
		Partner:                   "tenant-1",
		ProcessorCodePath:         jsii.String("."),
		ProcessorEnvironment:      &extraEnv,
		EnableEventIDIndex:        jsii.Bool(true),
		EnableStream:              nil,
		EnablePointInTimeRecovery: jsii.Bool(true),
		Tags:                      &tags,
	})

	require.NotNil(t, pattern)
	require.NotNil(t, pattern.Table)
	require.NotNil(t, pattern.Processor)
	require.NotNil(t, pattern.GetStreamArn())
	require.NotNil(t, pattern.GetTableName())
	require.NotNil(t, pattern.GetTableArn())
	require.NotNil(t, pattern.GetEnvironmentVariables())

	grantFn := awslambda.NewFunction(stack, jsii.String("GrantFn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		Handler: jsii.String("index.handler"),
	})
	pattern.GrantPublish(grantFn)
	pattern.GrantQuery(grantFn)
	pattern.GrantFullAccess(grantFn)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::Table"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Lambda::EventSourceMapping"), jsii.Number(1))

	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"TableName": "my-app-tenant-1-events-lab",
	})

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"FunctionName": "my-app-tenant-1-eventbus-processor-lab",
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"APP_NAME": "my-app",
				"STAGE":    "lab",
				"PARTNER":  "tenant-1",
				"EXTRA":    "1",
			},
		},
	})
}

func TestNewEventBusPattern_CanDisableProcessorWhenStreamDisabled(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	pattern := NewEventBusPattern(stack, jsii.String("EventBus"), &EventBusPatternProps{
		AppName:           "my-app",
		ProcessorCodePath: jsii.String("."),
		EnableStream:      jsii.Bool(false),
	})

	require.NotNil(t, pattern)
	require.NotNil(t, pattern.Table)
	require.Nil(t, pattern.Processor)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(0))
}
