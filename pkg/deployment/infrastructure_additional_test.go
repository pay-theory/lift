package deployment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateTemplate_CoversMonitoringSecurityNetworkingAndHTTPAPI(t *testing.T) {
	config := InfrastructureConfig{
		ApplicationName: "test-app",
		Environment:     "dev",
		Region:          "us-east-1",
		Lambda: LambdaConfig{
			Runtime:    "go1.x",
			Handler:    "main",
			Timeout:    10,
			MemorySize: 128,
			VPCConfig: &VPCConfig{
				CIDR:               "10.0.0.0/16",
				AvailabilityZones:  []string{"us-east-1a", "us-east-1b"},
				EnableDNSSupport:   true,
				EnableDNSHostnames: true,
			},
		},
		APIGateway: APIGatewayConfig{
			Type:      "HTTP",
			StageName: "dev",
			CORS: CORSConfig{
				AllowOrigins:     []string{"https://example.com"},
				AllowMethods:     []string{"GET"},
				AllowHeaders:     []string{"content-type"},
				ExposeHeaders:    []string{"x-request-id"},
				MaxAge:           123,
				AllowCredentials: true,
			},
		},
		Database: DatabaseConfig{
			Type: "DynamoDB",
			Tables: []TableConfig{
				{
					Name:        "users",
					BillingMode: "PAY_PER_REQUEST",
					HashKey:     "pk",
					Attributes: []AttributeConfig{
						{Name: "pk", Type: "S"},
						{Name: "email", Type: "S"},
					},
					GlobalIndexes: []GlobalIndexConfig{
						{
							Name:    "email-index",
							HashKey: "email",
							Projection: ProjectionConfig{
								Type:       "INCLUDE",
								Attributes: []string{"pk"},
							},
						},
					},
				},
			},
		},
		Monitoring: MonitoringConfig{
			CloudWatch: CloudWatchConfig{
				LogGroups: []LogGroupConfig{
					{
						Name:          "app-logs",
						RetentionDays: 7,
						KMSKeyId:      "alias/test-key",
					},
				},
			},
			Alarms: []AlarmConfig{
				{
					Name:               "errors",
					MetricName:         "Errors",
					Namespace:          "AWS/Lambda",
					Statistic:          "Sum",
					ComparisonOperator: "GreaterThanThreshold",
					Threshold:          1,
					EvaluationPeriods:  1,
					Period:             60,
					Actions:            []string{"arn:aws:sns:us-east-1:123456789012:alerts"},
				},
			},
		},
		Security: SecurityConfig{
			IAMRoles: []IAMRoleConfig{
				{
					Name: "custom-role",
					AssumeRolePolicy: map[string]any{
						"Version": "2012-10-17",
					},
					Policies: []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
					InlinePolicies: []InlinePolicyConfig{
						{
							Name: "inline",
							Policy: map[string]any{
								"Statement": []any{},
							},
						},
					},
				},
			},
			KMSKeys: []KMSKeyConfig{
				{
					Alias:       "data-key",
					Description: "data key",
					Policy: map[string]any{
						"Version": "2012-10-17",
					},
				},
			},
		},
		Networking: NetworkingConfig{
			VPC: &VPCConfig{
				CIDR:               "10.0.0.0/16",
				AvailabilityZones:  []string{"us-east-1a", "us-east-1b"},
				EnableDNSSupport:   true,
				EnableDNSHostnames: true,
			},
			Subnets: []SubnetConfig{
				{Name: "public-1", CIDR: "10.0.0.0/24", AvailabilityZone: "us-east-1a", Type: "public"},
				{Name: "private-1", CIDR: "10.0.1.0/24", AvailabilityZone: "us-east-1a", Type: "private"},
			},
			SecurityGroups: []SecurityGroupConfig{
				{
					Name:        "app-sg",
					Description: "app",
					IngressRules: []SecurityGroupRule{
						{Protocol: "tcp", FromPort: 80, ToPort: 80, CIDRBlocks: []string{"0.0.0.0/0"}},
						{Protocol: "tcp", FromPort: 443, ToPort: 443, SourceSG: "sg-123"},
					},
				},
			},
		},
		Tags: map[string]string{
			"Environment": "dev",
		},
	}

	generator := NewInfrastructureGenerator(ProviderCDK, config)
	template, err := generator.GenerateTemplate()
	require.NoError(t, err)

	api := template.Resources["APIGateway"]
	require.Equal(t, "AWS::ApiGatewayV2::Api", api.Type)
	require.Contains(t, api.Properties, "CorsConfiguration")

	logGroup := template.Resources["LogGroupApplogs"]
	require.Equal(t, "AWS::Logs::LogGroup", logGroup.Type)
	require.Equal(t, "alias/test-key", logGroup.Properties["KmsKeyId"])

	alarm := template.Resources["AlarmErrors"]
	require.Equal(t, "AWS::CloudWatch::Alarm", alarm.Type)
	require.Contains(t, alarm.Properties, "AlarmActions")

	role := template.Resources["IAMRoleCustomrole"]
	require.Equal(t, "AWS::IAM::Role", role.Type)
	require.Contains(t, role.Properties, "Policies")

	key := template.Resources["KMSKeyDatakey"]
	require.Equal(t, "AWS::KMS::Key", key.Type)
	require.Equal(t, map[string]any{"Version": "2012-10-17"}, key.Properties["KeyPolicy"])
	require.Contains(t, template.Resources, "KMSKeyDatakeyAlias")

	subnet := template.Resources["PublicSubnet1"]
	require.Equal(t, true, subnet.Properties["MapPublicIpOnLaunch"])

	sg := template.Resources["SecurityGroupAppsg"]
	ingress := sg.Properties["SecurityGroupIngress"].([]map[string]any)
	require.Len(t, ingress, 2)
	require.Contains(t, ingress[0], "CidrIp")
	require.Contains(t, ingress[1], "SourceSecurityGroupId")

	table := template.Resources["DynamoTableUsers"]
	gsis := table.Properties["GlobalSecondaryIndexes"].([]map[string]any)
	projection := gsis[0]["Projection"].(map[string]any)
	require.Equal(t, []string{"pk"}, projection["NonKeyAttributes"])

	yamlBytes, err := generator.ExportTemplate(template, "yaml")
	require.NoError(t, err)
	require.NotEmpty(t, yamlBytes)
}
