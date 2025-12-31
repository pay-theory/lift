package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestEnhancedSecurity_CreatesWAFSecretsEndpointsAndFlowLogs(t *testing.T) {
	stack := test.NewTestStack()

	vpc := awsec2.NewVpc(stack.Stack(), jsii.String("VPC"), &awsec2.VpcProps{
		NatGateways: jsii.Number(0),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{Name: jsii.String("public"), SubnetType: awsec2.SubnetType_PUBLIC},
			{Name: jsii.String("private"), SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS},
		},
	})

	rotationFn := awslambda.NewFunction(stack.Stack(), jsii.String("RotationFn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {};")),
	})

	security := NewEnhancedSecurity(stack.Stack(), jsii.String("Security"), &EnhancedSecurityProps{
		Vpc: vpc,
		IngressRules: []SecurityRule{
			{
				Source:      awsec2.Peer_AnyIpv4(),
				Protocol:    awsec2.Protocol_TCP,
				Port:        443,
				Description: "Allow HTTPS",
			},
		},
		EgressRules: []SecurityRule{
			{
				Source:      awsec2.Peer_AnyIpv4(),
				Protocol:    awsec2.Protocol_UDP,
				Port:        53,
				Description: "Allow DNS",
			},
		},
		Secrets: []SecretConfig{
			{
				Name:           "DbPassword",
				Description:    "Database password",
				EnableRotation: true,
				RotationLambda: rotationFn,
				Length:         32,
			},
		},
		WAFConfig: &WAFRuleConfig{
			EnableRateLimit:      jsii.Bool(true),
			RateLimit:            jsii.Number(1000),
			EnableSQLiProtection: jsii.Bool(true),
			EnableXSSProtection:  jsii.Bool(true),
			EnableKnownBadInputs: jsii.Bool(true),
			IPWhitelist:          &[]*string{jsii.String("203.0.113.0/24")},
			IPBlacklist:          &[]*string{jsii.String("198.51.100.0/24")},
			GeoBlocking:          &[]string{"US"},
		},
		VPCEndpointConfig: &VPCEndpointConfig{
			EnableSecretsManager:       jsii.Bool(true),
			EnableCloudWatchLogs:       jsii.Bool(true),
			EnableXRay:                 jsii.Bool(true),
			EnableKMS:                  jsii.Bool(true),
			EnableCloudWatchMonitoring: jsii.Bool(true),
			PrivateDNSEnabled:          jsii.Bool(true),
		},
	})

	security.AddCustomSecurityRule(SecurityRule{
		Source:      awsec2.Peer_AnyIpv4(),
		Protocol:    awsec2.Protocol_ALL,
		Port:        0,
		Description: "All traffic",
	}, "egress")

	if security.GetSecurityGroup() == nil {
		t.Fatal("expected security group")
	}
	if security.GetWAF() == nil {
		t.Fatal("expected waf")
	}
	if security.GetSecret("DbPassword") == nil {
		t.Fatal("expected secret")
	}

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::EC2::SecurityGroup"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::WAFv2::WebACL"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::SecretsManager::Secret"), jsii.Number(1))

	endpoints := findResourcesByType(template, "AWS::EC2::VPCEndpoint")
	if len(endpoints) < 3 {
		t.Fatalf("expected at least 3 vpc endpoints, got %d", len(endpoints))
	}
}
