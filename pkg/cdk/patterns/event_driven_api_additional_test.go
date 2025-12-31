package patterns

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/require"
)

func TestEventDrivenAPIConfig_DefaultsAndOverrides(t *testing.T) {
	cfg := buildEventDrivenAPIConfig(nil)
	require.Equal(t, "event-driven-api", cfg.appName)
	require.Equal(t, "event-driven-api-api", cfg.apiName)
	require.Equal(t, "default", cfg.eventBusName)
	require.Equal(t, "APIRequest", cfg.detailType)
	require.Equal(t, true, cfg.enableRequestTracking)

	custom := &EventDrivenAPIProps{
		AppName:               jsii.String("orders"),
		ApiName:               jsii.String("orders-api"),
		EventBusName:          jsii.String("bus"),
		EventSource:           jsii.String("source"),
		DetailType:            jsii.String("detail"),
		EnableRequestTracking: jsii.Bool(false),
	}
	cfg = buildEventDrivenAPIConfig(custom)
	require.Equal(t, "orders", cfg.appName)
	require.Equal(t, "orders-api", cfg.apiName)
	require.Equal(t, "bus", cfg.eventBusName)
	require.Equal(t, "source", cfg.eventSource)
	require.Equal(t, "detail", cfg.detailType)
	require.Equal(t, false, cfg.enableRequestTracking)
}

func TestNewEventDrivenAPI_WithRequestTrackingAndMonitoring(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	env := map[string]*string{
		"CUSTOM": jsii.String("value"),
	}

	fnProps := awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		Handler: jsii.String("index.handler"),
	}

	pattern := NewEventDrivenAPI(stack, jsii.String("EventDrivenAPI"), &EventDrivenAPIProps{
		AppName:               jsii.String("orders"),
		FunctionProps:         fnProps,
		Environment:           &env,
		EventBusName:          jsii.String("bus"),
		EventSource:           jsii.String("source"),
		DetailType:            jsii.String("detail"),
		MemorySize:            jsii.Number(512),
		Timeout:               jsii.Number(15),
		EnableRequestTracking: jsii.Bool(true),
		EnableMonitoring:      jsii.Bool(true),
		EnableTracing:         jsii.Bool(true),
		EnableMultiTenant:     jsii.Bool(true),
	})

	require.NotNil(t, pattern)
	require.NotNil(t, pattern.API)
	require.NotNil(t, pattern.APIFunction)
	require.NotNil(t, pattern.EventHandler)
	require.NotNil(t, pattern.RequestTrackingTable)

	extraRouteFn := awslambda.NewFunction(stack, jsii.String("ExtraRouteFn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		Handler: jsii.String("index.handler"),
	})
	pattern.AddAPIRoute(jsii.String("/extra"), "GET", extraRouteFn)
	pattern.GrantRequestTrackingAccess(extraRouteFn)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::Table"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(3))
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Api"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Route"), jsii.Number(3))
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Events::Rule"), jsii.Number(1))

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"MemorySize": 512,
		"Timeout":    15,
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"EVENT_BUS_NAME":             "bus",
				"EVENT_SOURCE":               "source",
				"EVENT_DETAIL_TYPE":          "detail",
				"REQUEST_TRACKING_TABLE":     assertions.Match_AnyValue(),
				"REQUEST_TRACKING_TABLE_ARN": assertions.Match_AnyValue(),
				"CUSTOM":                     "value",
			},
		},
	})
}

func TestNewEventDrivenAPI_WithoutRequestTracking(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	fnProps := awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		Handler: jsii.String("index.handler"),
	}

	pattern := NewEventDrivenAPI(stack, jsii.String("EventDrivenAPI"), &EventDrivenAPIProps{
		AppName:               jsii.String("orders"),
		FunctionProps:         fnProps,
		EnableRequestTracking: jsii.Bool(false),
	})

	require.NotNil(t, pattern)
	require.Nil(t, pattern.GetRequestTrackingTableName())

	grantFn := awslambda.NewFunction(stack, jsii.String("GrantFn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
		Handler: jsii.String("index.handler"),
	})
	pattern.GrantRequestTrackingAccess(grantFn)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::Table"), jsii.Number(0))
}
