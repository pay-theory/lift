package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

func TestLiftRestAPI_CORSCustomDomainApiKeysUsagePlansAndHelpers(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	api := NewLiftRestAPI(stack, jsii.String("API"), &LiftRestAPIProps{
		AppName: jsii.String("test-rest-api"),
		APICommonProps: APICommonProps{
			EnableCORS:          jsii.Bool(true),
			AllowOrigins:        &[]*string{jsii.String("https://example.com")},
			EnableAccessLogging: jsii.Bool(true),
			DomainName:          jsii.String("api.example.com"),
			CertificateArn:      jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/abc"),
			ThrottleRateLimit:   jsii.Number(100),
			ThrottleBurstLimit:  jsii.Number(200),
		},
		EnableDetailedMetrics: jsii.Bool(true),
		RequireApiKey:         jsii.Bool(true),
	})

	if api.GetResourceName() == nil {
		t.Fatal("expected api resource name")
	}
	if api.GetUrl() == nil {
		t.Fatal("expected api url")
	}
	if api.GetArn() == nil {
		t.Fatal("expected api arn")
	}
	if api.GetStage() == nil {
		t.Fatal("expected api stage")
	}

	api.CreateAPIKey(jsii.String("test-key"))
	api.CreateUsagePlan(
		jsii.String("test-plan"),
		&awsapigateway.ThrottleSettings{RateLimit: jsii.Number(10), BurstLimit: jsii.Number(20)},
		&awsapigateway.QuotaSettings{Limit: jsii.Number(1000), Period: awsapigateway.Period_MONTH},
	)

	grantee := awsiam.NewRole(stack, jsii.String("Grantee"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
	})
	api.GrantInvoke(grantee)

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::ApiGateway::RestApi")
	assertResourceExists(t, template, "AWS::ApiGateway::DomainName")
	assertResourceExists(t, template, "AWS::ApiGateway::ApiKey")
	assertResourceExists(t, template, "AWS::ApiGateway::UsagePlan")
	assertResourceExists(t, template, "AWS::Logs::LogGroup")
	assertResourceExists(t, template, "AWS::IAM::Policy")
}

func TestLiftRestAPI_CustomDomain_UsesProvidedCertificate(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	cert := awscertificatemanager.Certificate_FromCertificateArn(
		stack,
		jsii.String("Cert"),
		jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/def"),
	)

	api := NewLiftRestAPI(stack, jsii.String("API"), &LiftRestAPIProps{
		APICommonProps: APICommonProps{
			Name:       jsii.String("test-rest-api"),
			DomainName: jsii.String("api.example.com"),
		},
		Certificate: cert,
	})

	if api == nil {
		t.Fatal("expected api")
	}

	fn := awslambda.NewFunction(stack, jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => ({ statusCode: 200, body: \"ok\" });")),
	})
	api.AddLambdaIntegration(jsii.String("/ping"), jsii.String("GET"), fn)

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::ApiGateway::DomainName")
}
