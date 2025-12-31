package patterns

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/require"
)

func TestNewMicroserviceComplete_WithLoadBalancerAndScaling(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	idle := awscdk.Duration_Seconds(jsii.Number(60))
	interval := awscdk.Duration_Seconds(jsii.Number(30))
	timeout := awscdk.Duration_Seconds(jsii.Number(5))
	deregDelay := awscdk.Duration_Seconds(jsii.Number(10))

	actions := []ScheduledScalingAction{
		{
			Name:        "ScaleUp",
			Schedule:    "cron(0 8 * * ? *)",
			MinCapacity: jsii.Number(4),
			MaxCapacity: jsii.Number(8),
		},
	}

	tags := map[string]*string{
		"Owner": jsii.String("team"),
	}

	microservice := NewMicroserviceComplete(stack, jsii.String("Microservice"), &MicroserviceCompleteProps{
		ServiceName: jsii.String("orders"),
		Environment: jsii.String("lab"),
		ContainerConfig: &ContainerConfig{
			ImageURI: jsii.String("nginx:alpine"),
			Command: &[]*string{
				jsii.String("nginx"),
				jsii.String("-g"),
				jsii.String("daemon off;"),
			},
			EntryPoint: &[]*string{
				jsii.String("sh"),
				jsii.String("-c"),
			},
			WorkingDirectory: jsii.String("/"),
			User:             jsii.String("root"),
		},
		ServiceDiscovery: &ServiceDiscoveryConfig{
			HealthCheckPath: jsii.String("/health"),
		},
		LoadBalancer: &LoadBalancerConfig{
			Enabled:                 jsii.Bool(true),
			Certificate:             awselasticloadbalancingv2.ListenerCertificate_FromArn(jsii.String("arn:aws:acm:us-east-1:123456789012:certificate/abc")),
			EnableSSLRedirect:       jsii.Bool(true),
			EnableHTTP2:             jsii.Bool(true),
			IdleTimeout:             &idle,
			HealthCheckPath:         jsii.String("/health"),
			HealthCheckInterval:     &interval,
			HealthCheckTimeout:      &timeout,
			HealthyThresholdCount:   jsii.Number(2),
			UnhealthyThresholdCount: jsii.Number(3),
			DeregistrationDelay:     &deregDelay,
			TargetGroupProtocol:     awselasticloadbalancingv2.ApplicationProtocol_HTTP,
		},
		AutoScaling: &AutoScalingConfig{
			RequestsPerTarget:       jsii.Number(1000),
			ScheduledScalingActions: &actions,
		},
		Tags: &tags,
	})

	require.NotNil(t, microservice)
	require.NotNil(t, microservice.GetCluster())
	require.NotNil(t, microservice.GetService())
	require.NotNil(t, microservice.GetLoadBalancer())
	require.NotNil(t, microservice.GetMonitoring())
	require.NotNil(t, microservice.GetSecurity())
	require.NotNil(t, microservice.GetServiceEndpoint())
	require.NotNil(t, microservice.GetServiceDiscoveryEndpoint())

	template := assertions.Template_FromStack(stack, nil)
	template.ResourceCountIs(jsii.String("AWS::ECS::Cluster"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ECS::Service"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ElasticLoadBalancingV2::LoadBalancer"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ServiceDiscovery::PrivateDnsNamespace"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ApplicationAutoScaling::ScalableTarget"), jsii.Number(1))
}
