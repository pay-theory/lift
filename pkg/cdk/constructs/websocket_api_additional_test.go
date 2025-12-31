package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func TestWebSocketAPI_CustomRoutesThrottlingAndGrants(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	connectFunction, disconnectFunction, defaultFunction := createWSTestFunctions(stack, "Custom")
	customFunction := awslambda.NewFunction(stack, jsii.String("CustomRouteFn"), &awslambda.FunctionProps{
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
		Handler: jsii.String("index.handler"),
		Runtime: awslambda.Runtime_NODEJS_18_X(),
	})

	wsApi := NewWebSocketAPI(stack, jsii.String("API"), &WebSocketAPIProps{
		ApiName:                 jsii.String("test-api"),
		ConnectRouteFunction:    connectFunction,
		DisconnectRouteFunction: disconnectFunction,
		DefaultRouteFunction:    defaultFunction,
		Routes: []*WebSocketRouteConfig{
			{
				RouteKey: jsii.String("custom"),
				Function: customFunction,
			},
		},
		ThrottleRateLimit:   jsii.Number(50),
		ThrottleBurstLimit:  jsii.Number(100),
		EnableAccessLogging: jsii.Bool(false),
	})

	grantee := awsiam.NewRole(stack, jsii.String("Grantee"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
	})
	wsApi.GrantConnectionManagement(grantee)
	wsApi.GrantApiInvoke(grantee)
	wsApi.grantApiGatewayInvokePermissions()

	template := synthesizeTemplate(t, stack)

	routes := findResourcesByType(template, "AWS::ApiGatewayV2::Route")
	if len(routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(routes))
	}

	stages := findResourcesByType(template, "AWS::ApiGatewayV2::Stage")
	if len(stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(stages))
	}
	for _, stage := range stages {
		props, ok := stage["Properties"].(map[string]interface{})
		if !ok {
			continue
		}
		settings, ok := props["DefaultRouteSettings"].(map[string]interface{})
		if !ok {
			t.Fatal("expected DefaultRouteSettings to be set when throttling is configured")
		}
		if _, ok := settings["ThrottlingRateLimit"]; !ok {
			t.Fatal("expected ThrottlingRateLimit to be set")
		}
		if _, ok := settings["ThrottlingBurstLimit"]; !ok {
			t.Fatal("expected ThrottlingBurstLimit to be set")
		}
	}

	assertResourceExists(t, template, "AWS::IAM::Policy")
}
