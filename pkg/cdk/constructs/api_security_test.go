package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestLiftAPI_EnableApiKeyAuth_CreatesAuthorizerAndValidator(t *testing.T) {
	stack := test.NewTestStack()

	api := NewLiftAPI(stack.Stack(), jsii.String("API"), &LiftAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-api"),
		},
	})

	authorizer := api.EnableApiKeyAuth()
	if authorizer == nil {
		t.Fatal("expected api key authorizer")
	}
	if api.GetResourceName() == nil {
		t.Fatal("expected api resource name")
	}

	fn := awslambda.NewFunction(stack.Stack(), jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => ({ statusCode: 200, body: \"ok\" });")),
	})

	api.AddLambdaRouteWithOptions(jsii.String("/protected"), awsapigatewayv2.HttpMethod_GET, fn, &RouteOptions{
		Authorizer: authorizer,
	})

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Authorizer"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(2)) // route handler + validator
}

func TestAPIKeyAuthorizer_QueryIdentitySource(t *testing.T) {
	stack := test.NewTestStack()

	props := &APIKeyAuthorizerProps{
		APIKeySource:    jsii.String("query"),
		APIKeyParameter: jsii.String("apiKey"),
	}

	auth := NewAPIKeyAuthorizer(stack.Stack(), jsii.String("Auth"), props)
	if auth == nil {
		t.Fatal("expected authorizer")
	}
	if got := auth.getIdentitySource(props); got != "$request.querystring.apiKey" {
		t.Fatalf("unexpected identity source: %s", got)
	}
}

func TestLiftAPI_VPCAuthorizerAndAuthorizedRoute(t *testing.T) {
	stack := test.NewTestStack()

	api := NewLiftAPI(stack.Stack(), jsii.String("API"), &LiftAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-api"),
		},
	})

	fn := awslambda.NewFunction(stack.Stack(), jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => ({ statusCode: 200, body: \"ok\" });")),
	})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when vpc authorizer not enabled")
		}
	}()
	api.AddVPCAuthorizedRoute(jsii.String("GET /secure"), fn)
}

func TestLiftAPI_VPCAuthorizer_EnablesAndCreatesRouteAndGrant(t *testing.T) {
	stack := test.NewTestStack()

	api := NewLiftAPI(stack.Stack(), jsii.String("API"), &LiftAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-api"),
		},
	})

	fn := awslambda.NewFunction(stack.Stack(), jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => ({ statusCode: 200, body: \"ok\" });")),
	})

	api.EnableVPCAuthorizer(
		"arn:aws:lambda:us-east-1:123456789012:function:vpc-authorizer",
		"vpc-authorizer",
		"arn:aws:iam::123456789012:role/vpc-authorizer-role",
	)

	api.AddVPCAuthorizedRoute(jsii.String("GET /secure"), fn)

	role := awsiam.NewRole(stack.Stack(), jsii.String("CallerRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewAccountPrincipal(jsii.String("123456789012")),
	})
	api.GrantInvoke(role)

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Authorizer"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Route"), jsii.Number(1))
}
