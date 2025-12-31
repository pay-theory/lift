package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/jsii-runtime-go"
)

func TestNormalizePathRoutedFrontendDistributionProps_RequiresFields(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing required fields")
		}
	}()

	_ = normalizePathRoutedFrontendDistributionProps(nil)
}

func TestPathRoutedFrontendDistribution_CreatesBucketsDistributionAndRewriteFunction(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	zone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("Zone"), &awsroute53.HostedZoneAttributes{
		HostedZoneId: jsii.String("Z1234567890"),
		ZoneName:     jsii.String("example.com"),
	})

	NewPathRoutedFrontendDistribution(stack, jsii.String("Dist"), &PathRoutedFrontendDistributionProps{
		HostedZone:          zone,
		DomainName:          jsii.String("dev.example.com"),
		ApiOriginDomainName: jsii.String("api.dev.example.com"),
		ClientBucketName:    jsii.String("dev-example-client-bucket"),
		AuthBucketName:      jsii.String("dev-example-auth-bucket"),
		ClientPathPrefix:    jsii.String("/l/"),
		AuthPathPrefix:      jsii.String("auth/"),
		AuthSinglePageApp:   jsii.Bool(false),
		RemovalPolicy:       awscdk.RemovalPolicy_DESTROY,
	})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(2))
	template.ResourceCountIs(jsii.String("AWS::CloudFront::Distribution"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudFront::Function"), jsii.Number(1))
}

func TestPathRoutedFrontendDistributionPrefixHelpers(t *testing.T) {
	if got := normalizePathPrefix(jsii.String(" /foo/ "), "bar"); got != "foo" {
		t.Fatalf("expected normalized prefix foo, got %q", got)
	}
	if got := normalizePathPrefix(jsii.String("  "), "bar"); got != "bar" {
		t.Fatalf("expected default prefix bar, got %q", got)
	}
	if got := normalizePathPattern("/auth/wallet/*/"); got != "auth/wallet/*" {
		t.Fatalf("expected normalized pattern auth/wallet/*, got %q", got)
	}
}
