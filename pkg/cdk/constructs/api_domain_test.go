package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiftApiDomainUsesDefaultStageWithoutHostedZone(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("DomainStack"), nil)

	api := awsapigatewayv2.NewHttpApi(stack, jsii.String("HttpApi"), nil)
	cert := awscertificatemanager.NewCertificate(stack, jsii.String("Cert"), &awscertificatemanager.CertificateProps{
		DomainName: jsii.String("api.example.com"),
	})

	liftDomain := NewLiftApiDomain(stack, jsii.String("Domain"), &LiftApiDomainProps{
		DomainName:  jsii.String("api.example.com"),
		Certificate: cert,
		HttpAPI:     api,
	})

	require.NotNil(t, liftDomain)
	defaultStage := api.DefaultStage()
	require.NotNil(t, defaultStage)

	mapping := liftDomain.GetApiMapping()
	require.NotNil(t, mapping)

	cfnMapping := mapping.Node().DefaultChild().(awsapigatewayv2.CfnApiMapping)
	assert.Equal(t, *defaultStage.StageName(), *cfnMapping.Stage())
	assert.Nil(t, liftDomain.GetCNAMERecord())
}

func TestLiftApiDomainCreatesCNAMEWhenHostedZoneProvided(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("DomainStack"), nil)

	api := awsapigatewayv2.NewHttpApi(stack, jsii.String("HttpApi"), nil)
	cert := awscertificatemanager.NewCertificate(stack, jsii.String("Cert"), &awscertificatemanager.CertificateProps{
		DomainName: jsii.String("api.example.com"),
	})

	zone := awsroute53.NewHostedZone(stack, jsii.String("Zone"), &awsroute53.HostedZoneProps{
		ZoneName: jsii.String("example.com"),
	})
	mappingKey := jsii.String("v1")
	ttl := jsii.Number(600)

	liftDomain := NewLiftApiDomain(stack, jsii.String("Domain"), &LiftApiDomainProps{
		DomainName:    jsii.String("api.example.com"),
		Certificate:   cert,
		HttpAPI:       api,
		HostedZone:    zone,
		ApiMappingKey: mappingKey,
		RecordTTL:     ttl,
	})

	require.NotNil(t, liftDomain.GetCNAMERecord())
	assert.NotNil(t, liftDomain.GetRegionalDomainName())

	mapping := liftDomain.GetApiMapping()
	require.Equal(t, *mappingKey, *mapping.MappingKey())

	cfnRecord := liftDomain.GetCNAMERecord().Node().DefaultChild().(awsroute53.CfnRecordSet)
	assert.Equal(t, "api.example.com.", *cfnRecord.Name())
	assert.Equal(t, "600", *cfnRecord.Ttl())

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::Route53::RecordSet"), map[string]any{
		"Name": "api.example.com.",
		"Type": "CNAME",
	})
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::DomainName"), map[string]any{
		"DomainName": "api.example.com",
	})
}

func TestLiftApiDomainAdditionalMappingUsesDefaultStage(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("DomainStack"), nil)

	api := awsapigatewayv2.NewHttpApi(stack, jsii.String("HttpApi"), nil)
	cert := awscertificatemanager.NewCertificate(stack, jsii.String("Cert"), &awscertificatemanager.CertificateProps{
		DomainName: jsii.String("api.example.com"),
	})

	liftDomain := NewLiftApiDomain(stack, jsii.String("Domain"), &LiftApiDomainProps{
		DomainName:  jsii.String("api.example.com"),
		Certificate: cert,
		HttpAPI:     api,
	})

	additionalKey := jsii.String("admin")
	additionalMapping := liftDomain.AddAdditionalMapping(api, additionalKey)
	require.NotNil(t, additionalMapping)
	assert.Equal(t, *additionalKey, *additionalMapping.MappingKey())

	defaultStage := api.DefaultStage()
	require.NotNil(t, defaultStage)

	cfnMapping := additionalMapping.Node().DefaultChild().(awsapigatewayv2.CfnApiMapping)
	assert.Equal(t, *defaultStage.StageName(), *cfnMapping.Stage())
}
