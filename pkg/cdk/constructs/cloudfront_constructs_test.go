package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/jsii-runtime-go"
)

func TestStaticSite_CreatesPrivateBucketWithOAC(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	zone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("Zone"), &awsroute53.HostedZoneAttributes{
		HostedZoneId: jsii.String("Z1234567890"),
		ZoneName:     jsii.String("example.com"),
	})
	cert := awscertificatemanager.Certificate_FromCertificateArn(stack, jsii.String("Cert"), jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"))

	NewStaticSite(stack, jsii.String("Site"), &StaticSiteProps{
		DomainName:        jsii.String("example.com"),
		HostedZone:        zone,
		Certificate:       cert,
		AppName:           jsii.String("repo"),
		Stage:             jsii.String("live"),
		EnableWWWRedirect: jsii.Bool(false),
	})

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
		"BucketName": "repo-static-site-live",
		"PublicAccessBlockConfiguration": map[string]interface{}{
			"BlockPublicAcls":       true,
			"BlockPublicPolicy":     true,
			"IgnorePublicAcls":      true,
			"RestrictPublicBuckets": true,
		},
	})
	template.ResourceCountIs(jsii.String("AWS::CloudFront::Distribution"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudFront::OriginAccessControl"), jsii.Number(1))
}

func TestHostRedirect_UsesCloudFrontFunction(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	zone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("Zone"), &awsroute53.HostedZoneAttributes{
		HostedZoneId: jsii.String("Z1234567890"),
		ZoneName:     jsii.String("example.com"),
	})
	cert := awscertificatemanager.Certificate_FromCertificateArn(stack, jsii.String("Cert"), jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"))

	NewHostRedirect(stack, jsii.String("Redirect"), &HostRedirectProps{
		FromDomainName: jsii.String("www.example.com"),
		ToDomainName:   jsii.String("example.com"),
		HostedZone:     zone,
		Certificate:    cert,
		EnableIpv6:     jsii.Bool(false),
	})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::CloudFront::Function"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::CloudFront::Function"), map[string]interface{}{
		"FunctionConfig": map[string]interface{}{
			"Runtime": "cloudfront-js-2.0",
		},
	})
}

func TestMediaCDN_PrivateMediaCreatesKeyGroup(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	zone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("Zone"), &awsroute53.HostedZoneAttributes{
		HostedZoneId: jsii.String("Z1234567890"),
		ZoneName:     jsii.String("example.com"),
	})
	cert := awscertificatemanager.Certificate_FromCertificateArn(stack, jsii.String("Cert"), jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"))

	NewMediaCDN(stack, jsii.String("Media"), &MediaCDNProps{
		DomainName:          jsii.String("media.example.com"),
		HostedZone:          zone,
		Certificate:         cert,
		AppName:             jsii.String("repo"),
		Stage:               jsii.String("lab"),
		EnablePrivateMedia:  jsii.Bool(true),
		PublicKeyEncoded:    jsii.String("-----BEGIN PUBLIC KEY-----\\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn\\n-----END PUBLIC KEY-----"),
		PrivatePathPatterns: &[]*string{jsii.String("private/*")},
	})

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::CloudFront::PublicKey"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudFront::KeyGroup"), jsii.Number(1))
}

func TestFrontendDistribution_AddsApiBehaviors(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	zone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("Zone"), &awsroute53.HostedZoneAttributes{
		HostedZoneId: jsii.String("Z1234567890"),
		ZoneName:     jsii.String("example.com"),
	})
	cert := awscertificatemanager.Certificate_FromCertificateArn(stack, jsii.String("Cert"), jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"))

	NewFrontendDistribution(stack, jsii.String("Frontend"), &FrontendDistributionProps{
		DomainName:          jsii.String("example.com"),
		HostedZone:          zone,
		Certificate:         cert,
		ApiOriginDomainName: jsii.String("api.example.com"),
		AppName:             jsii.String("repo"),
		Stage:               jsii.String("study"),
		EnableWWWRedirect:   jsii.Bool(false),
	})

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::CloudFront::Distribution"), map[string]interface{}{
		"DistributionConfig": map[string]interface{}{
			"CacheBehaviors": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ObjectLike(&map[string]interface{}{
					"PathPattern": "api/*",
				}),
			}),
		},
	})
}
