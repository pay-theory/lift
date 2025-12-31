package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/jsii-runtime-go"
)

func TestLiftCertificate_CreatesDNSValidatedCertificate(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	zone := awsroute53.NewPublicHostedZone(stack, jsii.String("Zone"), &awsroute53.PublicHostedZoneProps{
		ZoneName: jsii.String("example.com"),
	})

	cert := NewLiftCertificate(stack, jsii.String("Cert"), &LiftCertificateProps{
		DomainName:                 jsii.String("api.example.com"),
		SubjectAlternativeNames:    &[]*string{jsii.String("www.example.com")},
		HostedZone:                 zone,
		ValidationZone:             zone,
		TransparencyLoggingEnabled: jsii.Bool(true),
		CertificateName:            jsii.String("api-example-cert"),
	})

	if cert == nil || cert.Certificate == nil {
		t.Fatal("expected certificate to be created")
	}
	if cert.GetCertificate() == nil {
		t.Fatal("expected certificate object")
	}
	if cert.GetCertificateArn() == nil {
		t.Fatal("expected certificate arn")
	}

	cert.AddDependency(zone)

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::CertificateManager::Certificate"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::CertificateManager::Certificate"), map[string]interface{}{
		"DomainName": "api.example.com",
		"SubjectAlternativeNames": []interface{}{
			"www.example.com",
		},
	})
}
