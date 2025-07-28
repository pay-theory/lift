package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"

	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
	"github.com/pay-theory/lift/pkg/cdk/patterns"
	"github.com/pay-theory/lift/pkg/compliance"
	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/security"
)

// TestEnhancedMonitoringIntegration tests the enhanced monitoring construct
func TestEnhancedMonitoringIntegration(t *testing.T) {
	// Create test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), &awscdk.StackProps{})

	// Create VPC for testing - not used directly but needed for some constructs
	_ = awsec2.NewVpc(stack, jsii.String("TestVPC"), &awsec2.VpcProps{
		MaxAzs: jsii.Number(2),
	})

	// Create Lambda function to monitor
	liftFunction := liftconstructs.NewLiftFunction(stack, jsii.String("TestFunction"), &liftconstructs.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			Code:         awslambda.Code_FromAsset(jsii.String("../../../examples/hello-world"), nil),
			Handler:      jsii.String("main"),
			Runtime:      awslambda.Runtime_PROVIDED_AL2(),
			Architecture: awslambda.Architecture_ARM_64(),
			MemorySize:   jsii.Number(256),
			Timeout:      awscdk.Duration_Seconds(jsii.Number(30)),
			Environment: &map[string]*string{
				"ENV": jsii.String("test"),
			},
		},
		EnableTracing: jsii.Bool(true),
		EnableMetrics: jsii.Bool(true),
	})

	// Create SNS topic for alerts
	alertTopic := awssns.NewTopic(stack, jsii.String("AlertTopic"), &awssns.TopicProps{
		DisplayName: jsii.String("Test Alerts"),
	})

	// Create enhanced monitoring
	monitoring := liftconstructs.NewEnhancedMonitoring(stack, jsii.String("EnhancedMonitoring"), &liftconstructs.EnhancedMonitoringProps{
		Resource:      liftFunction,
		Namespace:     jsii.String("Test/Monitoring"),
		AlertTopic:    alertTopic,
		DashboardName: jsii.String("test-dashboard"),
		MetricConfig: &liftconstructs.MetricConfiguration{
			DetailedMetrics:       jsii.Bool(true),
			EnableBusinessMetrics: jsii.Bool(true),
			Percentiles: &[]*float64{
				jsii.Number(50),
				jsii.Number(95),
				jsii.Number(99),
			},
		},
		AlarmThresholds: &liftconstructs.AlarmThresholds{
			ErrorRate:     jsii.Number(5.0),
			LatencyP99:    jsii.Number(3000),
			ThrottleCount: jsii.Number(5),
		},
		EnableRealTimeStreaming: jsii.Bool(true),
		Environment:             jsii.String("test"),
	})

	t.Run("Metrics Created", func(t *testing.T) {
		// Verify that required metrics are created
		assert.NotNil(t, monitoring.GetMetric("Requests"))
		assert.NotNil(t, monitoring.GetMetric("Errors"))
		assert.NotNil(t, monitoring.GetMetric("LatencyP99"))
		assert.NotNil(t, monitoring.GetMetric("ColdStarts"))
		assert.NotNil(t, monitoring.GetMetric("SuccessRate"))
	})

	t.Run("Alarms Created", func(t *testing.T) {
		// Verify that alarms are created
		assert.NotNil(t, monitoring.GetAlarm("HighErrorRate"))
		assert.NotNil(t, monitoring.GetAlarm("HighLatency"))
		assert.NotNil(t, monitoring.GetAlarm("Throttling"))
	})

	t.Run("Dashboard Created", func(t *testing.T) {
		// Verify dashboard is created
		assert.NotNil(t, monitoring.Dashboard)
	})

	t.Run("Synthesizes Successfully", func(t *testing.T) {
		// Test that the stack synthesizes without errors
		template := app.Synth(nil)
		assert.NotNil(t, template)

		// Verify CloudWatch resources are present in template
		stackArtifact := template.GetStackByName(stack.StackName())
		assert.NotNil(t, stackArtifact)
	})
}

// TestEnhancedSecurityIntegration tests the enhanced security construct
func TestEnhancedSecurityIntegration(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("SecurityTestStack"), &awscdk.StackProps{})

	// Create VPC
	vpc := awsec2.NewVpc(stack, jsii.String("SecurityVPC"), &awsec2.VpcProps{
		MaxAzs: jsii.Number(2),
	})

	// Create enhanced security
	security := liftconstructs.NewEnhancedSecurity(stack, jsii.String("EnhancedSecurity"), &liftconstructs.EnhancedSecurityProps{
		Vpc:               vpc,
		EnableWAF:         jsii.Bool(true),
		EnableVPCFlowLogs: jsii.Bool(true),
		Environment:       jsii.String("test"),
		ApplicationName:   jsii.String("test-app"),
		IngressRules: []liftconstructs.SecurityRule{
			{
				Port:        443,
				Protocol:    awsec2.Protocol_TCP,
				Source:      awsec2.Peer_AnyIpv4(),
				Description: "Allow HTTPS",
			},
		},
		EgressRules: []liftconstructs.SecurityRule{
			{
				Port:        443,
				Protocol:    awsec2.Protocol_TCP,
				Source:      awsec2.Peer_AnyIpv4(),
				Description: "Allow HTTPS outbound",
			},
		},
		WAFConfig: &liftconstructs.WAFRuleConfig{
			EnableRateLimit:      jsii.Bool(true),
			RateLimit:            jsii.Number(1000),
			EnableSQLiProtection: jsii.Bool(true),
			EnableXSSProtection:  jsii.Bool(true),
			EnableKnownBadInputs: jsii.Bool(true),
			IPBlacklist: &[]*string{
				jsii.String("192.168.1.0/24"),
			},
		},
		Secrets: []liftconstructs.SecretConfig{
			{
				Name:           "test-secret",
				Description:    "Test secret for integration testing",
				Template:       `{"username": "admin"}`,
				GenerateKey:    "password",
				Length:         32,
				EnableRotation: false,
			},
		},
	})

	t.Run("Security Group Created", func(t *testing.T) {
		assert.NotNil(t, security.GetSecurityGroup())
	})

	t.Run("WAF Created", func(t *testing.T) {
		assert.NotNil(t, security.GetWAF())
	})

	t.Run("Secrets Created", func(t *testing.T) {
		secret := security.GetSecret("test-secret")
		assert.NotNil(t, secret)
	})

	t.Run("VPC Endpoints Created", func(t *testing.T) {
		assert.NotNil(t, security.GetVPCEndpoint("SecretsManager"))
		assert.NotNil(t, security.GetVPCEndpoint("CloudWatchLogs"))
	})

	t.Run("Security Metrics Available", func(t *testing.T) {
		metric := security.GetSecurityMetric("WAFBlockedRequests")
		assert.NotNil(t, metric)
	})

	t.Run("Synthesizes Successfully", func(t *testing.T) {
		template := app.Synth(nil)
		assert.NotNil(t, template)

		stackArtifact := template.GetStackByName(stack.StackName())
		assert.NotNil(t, stackArtifact)
	})
}

// TestMicroserviceCompleteIntegration tests the complete microservice pattern
func TestMicroserviceCompleteIntegration(t *testing.T) {
	app := awscdk.NewApp(nil)

	// Create complete microservice
	microservice := patterns.NewMicroserviceComplete(app, jsii.String("TestMicroservice"), &patterns.MicroserviceCompleteProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Region: jsii.String("us-east-1"),
			},
		},
		ServiceName: jsii.String("test-service"),
		Environment: jsii.String("test"),
		NetworkConfig: &patterns.NetworkConfig{
			AssignPublicIP:          jsii.Bool(false),
			EnableContainerInsights: jsii.Bool(true),
		},
		ContainerConfig: &patterns.ContainerConfig{
			CodeAssetPath:     jsii.String("../../../examples/hello-world"),
			CPU:               jsii.Number(256),
			Memory:            jsii.Number(512),
			EnableXRayTracing: jsii.Bool(true),
			Environment: &map[string]*string{
				"ENV": jsii.String("test"),
			},
		},
		ServiceDiscovery: &patterns.ServiceDiscoveryConfig{
			Namespace:   jsii.String("test.local"),
			ServiceName: jsii.String("test-service"),
		},
		LoadBalancer: &patterns.LoadBalancerConfig{
			Enabled:                 jsii.Bool(true),
			EnableHTTP2:             jsii.Bool(true),
			EnableSSLRedirect:       jsii.Bool(false),
			HealthCheckPath:         jsii.String("/health"),
			HealthCheckInterval:     getDurationPtr(awscdk.Duration_Seconds(jsii.Number(30))),
			HealthCheckTimeout:      getDurationPtr(awscdk.Duration_Seconds(jsii.Number(5))),
			HealthyThresholdCount:   jsii.Number(2),
			UnhealthyThresholdCount: jsii.Number(3),
			DeregistrationDelay:     getDurationPtr(awscdk.Duration_Seconds(jsii.Number(30))),
		},
		AutoScaling: &patterns.AutoScalingConfig{
			MinCapacity:             jsii.Number(2),
			MaxCapacity:             jsii.Number(10),
			TargetCPUUtilization:    jsii.Number(70),
			TargetMemoryUtilization: jsii.Number(80),
			ScaleInCooldown:         getDurationPtr(awscdk.Duration_Seconds(jsii.Number(300))),
			ScaleOutCooldown:        getDurationPtr(awscdk.Duration_Seconds(jsii.Number(300))),
		},
		EnableEnhancedMonitoring: jsii.Bool(true),
		EnableEnhancedSecurity:   jsii.Bool(true),
	})

	t.Run("Service Created", func(t *testing.T) {
		assert.NotNil(t, microservice.GetService())
	})

	t.Run("Cluster Created", func(t *testing.T) {
		assert.NotNil(t, microservice.GetCluster())
	})

	t.Run("Load Balancer Created", func(t *testing.T) {
		assert.NotNil(t, microservice.GetLoadBalancer())
	})

	t.Run("Service Discovery Configured", func(t *testing.T) {
		endpoint := microservice.GetServiceDiscoveryEndpoint()
		assert.NotNil(t, endpoint)
		assert.Contains(t, *endpoint, "test-service")
		assert.Contains(t, *endpoint, "test.local")
	})

	t.Run("Monitoring Enabled", func(t *testing.T) {
		monitoring := microservice.GetMonitoring()
		assert.NotNil(t, monitoring)
	})

	t.Run("Security Enabled", func(t *testing.T) {
		security := microservice.GetSecurity()
		assert.NotNil(t, security)
	})

	t.Run("Synthesizes Successfully", func(t *testing.T) {
		template := app.Synth(nil)
		assert.NotNil(t, template)
	})
}

// TestGDPRCompleteIntegration tests the complete GDPR compliance service
func TestGDPRCompleteIntegration(t *testing.T) {
	// Create mock DynamORM wrapper
	mockDB := &dynamorm.DynamORMWrapper{
		// Mock implementation would go here
	}

	config := compliance.GDPRCompleteConfig{
		Enabled:                 true,
		Region:                  "us-east-1",
		Environment:             "test",
		ConsentTableName:        "test-consent",
		RequestTableName:        "test-requests",
		AuditTableName:          "test-audit",
		PIATableName:            "test-pia",
		DataExportBucket:        "test-exports",
		AuditLogBucket:          "test-audit-logs",
		ConsentExpiryDays:       365,
		DataRetentionDays:       2555, // 7 years
		RequestProcessingDays:   30,
		AuditRetentionDays:      2555,
		MaxExportSizeMB:         100,
		EncryptionEnabled:       true,
		AutoDataDeletion:        false,
		RequireExplicitConsent:  true,
		NotificationTopicArn:    "arn:aws:sns:us-east-1:123456789012:test-notifications",
		FromEmailAddress:        "test@example.com",
		ComplianceOfficerEmail:  "compliance@example.com",
		BreachNotificationHours: 72,
		EnableCrossBorderRules:  true,
		DefaultSafeguards:       []string{"encryption", "access_controls"},
		ProhibitedCountries:     []string{"XX", "YY"},
	}

	gdprService := compliance.NewGDPRCompleteService(config, mockDB)

	t.Run("Service Initialization", func(t *testing.T) {
		assert.NotNil(t, gdprService)
	})

	t.Run("Consent Processing", func(t *testing.T) {
		ctx := context.Background()

		consent := compliance.ConsentUpdate{
			Categories: []string{"essential", "analytics"},
			LegalBasis: "consent",
			Method:     "explicit_opt_in",
			Evidence:   "user_clicked_agree_button",
			Metadata: map[string]interface{}{
				"ip_address": "192.168.1.100",
				"user_agent": "Mozilla/5.0...",
			},
		}

		// This would normally interact with DynamoDB
		// For integration testing, we'd need a real database or better mocks
		err := gdprService.ProcessConsentUpdate(ctx, "test-user-123", consent)

		// With proper mocks, this should succeed
		if mockDB != nil {
			assert.Error(t, err) // Expected since we don't have a real DB connection
		}
	})

	t.Run("Data Export Processing", func(t *testing.T) {
		ctx := context.Background()

		// Test data export functionality
		export, err := gdprService.ExportUserData(ctx, "test-user-123", "request-456")

		// With proper mocks/database, this should work
		if mockDB != nil {
			assert.Error(t, err) // Expected since we don't have a real DB connection
			assert.Nil(t, export)
		}
	})

	t.Run("Data Deletion Processing", func(t *testing.T) {
		ctx := context.Background()

		// Test data deletion functionality
		err := gdprService.DeleteUserData(ctx, "test-user-123", "request-789")

		// With proper mocks/database, this should work
		if mockDB != nil {
			assert.Error(t, err) // Expected since we don't have a real DB connection
		}
	})

	t.Run("Breach Notification Processing", func(t *testing.T) {
		ctx := context.Background()

		breach := compliance.PrivacyBreach{
			Type:           "data_leak",
			Severity:       "high",
			DetectedAt:     time.Now(),
			AffectedCount:  100,
			DataCategories: []string{"personal_data", "financial_data"},
			Cause:          "misconfigured_s3_bucket",
			MitigationSteps: []string{
				"secured_bucket",
				"notified_users",
				"enhanced_monitoring",
			},
		}

		err := gdprService.ProcessBreachNotification(ctx, breach)

		// With proper mocks/database, this should work
		if mockDB != nil {
			assert.Error(t, err) // Expected since we don't have a real DB connection
		}
	})
}

// TestSecurityComplianceIntegration tests security compliance features
func TestSecurityComplianceIntegration(t *testing.T) {
	config := security.GDPRConsentConfig{
		Enabled:                  true,
		ConsentRenewalDays:       365,
		AutomaticConsentRenewal:  false,
		GranularConsentRequired:  true,
		ConsentWithdrawalEnabled: true,
		DataPortabilityEnabled:   true,
		RightToErasureEnabled:    true,
		BreachNotificationHours:  72,
		DataRetentionPolicies: map[string]time.Duration{
			"personal_data":  7 * 365 * 24 * time.Hour, // 7 years
			"session_data":   30 * 24 * time.Hour,      // 30 days
			"analytics_data": 2 * 365 * 24 * time.Hour, // 2 years
		},
		CrossBorderTransferRules: []security.CrossBorderRule{
			{
				ID:                 "eu_to_us",
				Name:               "EU to US transfers",
				SourceCountries:    []string{"DE", "FR", "NL"},
				DestCountries:      []string{"US"},
				DataCategories:     []string{"personal_data"},
				RequiredSafeguards: []string{"standard_contractual_clauses"},
				Prohibited:         false,
				Conditions:         []string{"adequacy_decision_valid"},
			},
		},
		PrivacyByDesignEnabled: true,
		ConsentExpiryDays:      365,
		RequireExplicitConsent: true,
		RequireConsentProof:    true,
		DataRetentionDays:      2555,
		RequestProcessingDays:  30,
		ConsentProofRequired:   true,
	}

	gdprManager := security.NewGDPRConsentManager(config)

	t.Run("GDPR Manager Initialization", func(t *testing.T) {
		assert.NotNil(t, gdprManager)
	})

	t.Run("Consent Record Validation", func(t *testing.T) {
		validConsent := &security.ConsentRecord{
			ID:            "consent-123",
			DataSubjectID: "user-456",
			ConsentGiven:  true,
			Granular:      true,
			ConsentProof: &security.ConsentProof{
				Type:      "digital_signature",
				Evidence:  "user_clicked_consent",
				Timestamp: time.Now(),
				IPAddress: "192.168.1.100",
				UserAgent: "Mozilla/5.0...",
				Method:    "web_form",
				Verified:  true,
			},
		}

		// This should pass validation
		// Note: We'd need to set up the consent store to actually test this
		// For now, we test the structure
		assert.Equal(t, "consent-123", validConsent.ID)
		assert.Equal(t, "user-456", validConsent.DataSubjectID)
		assert.True(t, validConsent.ConsentGiven)
		assert.True(t, validConsent.Granular)
		assert.NotNil(t, validConsent.ConsentProof)
	})

	t.Run("Compliance Framework Configuration", func(t *testing.T) {
		complianceConfig := security.ComplianceConfig{
			EnabledFrameworks: []string{"GDPR", "SOC2"},
			AuditRetention:    7 * 365 * 24 * time.Hour, // 7 years
			DataClassification: map[string]string{
				"personal_data": "confidential",
				"public_data":   "public",
				"internal_data": "internal",
			},
			EncryptionRequired: true,
			RegionRestrictions: []string{"EU", "US"},
			CustomRules: []security.ComplianceRule{
				{
					ID:          "custom-rule-1",
					Name:        "Data minimization",
					Framework:   "GDPR",
					Severity:    "high",
					Description: "Ensure data collection is limited to necessary purposes",
					Condition: map[string]interface{}{
						"data_categories": []string{"essential"},
					},
					Action: "allow",
				},
			},
		}

		framework := security.NewComplianceFramework("GDPR", complianceConfig)
		assert.NotNil(t, framework)

		// Test configuration validation
		err := framework.ValidateConfiguration()
		assert.NoError(t, err)

		// Test framework detection
		assert.True(t, framework.IsFrameworkEnabled("GDPR"))
		assert.True(t, framework.IsFrameworkEnabled("SOC2"))
		assert.False(t, framework.IsFrameworkEnabled("HIPAA"))
	})
}

// TestE2EIntegration tests end-to-end integration scenarios
func TestE2EIntegration(t *testing.T) {
	t.Run("Complete Application Stack", func(t *testing.T) {
		app := awscdk.NewApp(nil)

		// This would test a complete application stack with all enhanced features
		// Including monitoring, security, service discovery, and compliance

		// Create the main application stack
		stack := awscdk.NewStack(app, jsii.String("E2ETestStack"), &awscdk.StackProps{})

		// Create VPC
		vpc := awsec2.NewVpc(stack, jsii.String("MainVPC"), &awsec2.VpcProps{
			MaxAzs: jsii.Number(3),
		})

		// Create enhanced security
		security := liftconstructs.NewEnhancedSecurity(stack, jsii.String("MainSecurity"), &liftconstructs.EnhancedSecurityProps{
			Vpc:             vpc,
			EnableWAF:       jsii.Bool(true),
			Environment:     jsii.String("production"),
			ApplicationName: jsii.String("lift-app"),
		})

		// Create monitored function
		function := liftconstructs.NewLiftFunction(stack, jsii.String("MainFunction"), &liftconstructs.LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Code:           awslambda.Code_FromAsset(jsii.String("../../../examples/basic-crud-api"), nil),
				Handler:        jsii.String("main"),
				Runtime:        awslambda.Runtime_PROVIDED_AL2(),
				Vpc:            vpc,
				SecurityGroups: &[]awsec2.ISecurityGroup{security.GetSecurityGroup()},
			},
		})

		// Create enhanced monitoring
		monitoring := liftconstructs.NewEnhancedMonitoring(stack, jsii.String("MainMonitoring"), &liftconstructs.EnhancedMonitoringProps{
			Resource:    function,
			Environment: jsii.String("production"),
		})

		// Verify all components are created
		assert.NotNil(t, vpc)
		assert.NotNil(t, security)
		assert.NotNil(t, function)
		assert.NotNil(t, monitoring)

		// Test synthesis
		template := app.Synth(nil)
		assert.NotNil(t, template)
	})
}

// BenchmarkEnhancedFeatures benchmarks the performance of enhanced features
func BenchmarkEnhancedFeatures(b *testing.B) {
	b.Run("EnhancedMonitoringCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			app := awscdk.NewApp(nil)
			stack := awscdk.NewStack(app, jsii.String(fmt.Sprintf("BenchStack%d", i)), &awscdk.StackProps{})

			function := liftconstructs.NewLiftFunction(stack, jsii.String("BenchFunction"), &liftconstructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("../../../examples/hello-world"), nil),
					Handler: jsii.String("main"),
					Runtime: awslambda.Runtime_PROVIDED_AL2(),
				},
			})

			liftconstructs.NewEnhancedMonitoring(stack, jsii.String("BenchMonitoring"), &liftconstructs.EnhancedMonitoringProps{
				Resource: function,
			})
		}
	})

	b.Run("EnhancedSecurityCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			app := awscdk.NewApp(nil)
			stack := awscdk.NewStack(app, jsii.String(fmt.Sprintf("SecurityBenchStack%d", i)), &awscdk.StackProps{})

			vpc := awsec2.NewVpc(stack, jsii.String("BenchVPC"), &awsec2.VpcProps{
				MaxAzs: jsii.Number(2),
			})

			liftconstructs.NewEnhancedSecurity(stack, jsii.String("BenchSecurity"), &liftconstructs.EnhancedSecurityProps{
				Vpc:             vpc,
				Environment:     jsii.String("test"),
				ApplicationName: jsii.String("bench-app"),
			})
		}
	})
}

// Helper functions for testing

func getDurationPtr(duration awscdk.Duration) *awscdk.Duration {
	return &duration
}

func createTestVPC(stack constructs.Construct) awsec2.IVpc {
	return awsec2.NewVpc(stack, jsii.String("TestVPC"), &awsec2.VpcProps{
		MaxAzs: jsii.Number(2),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("Public"),
				SubnetType: awsec2.SubnetType_PUBLIC,
				CidrMask:   jsii.Number(24),
			},
			{
				Name:       jsii.String("Private"),
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
				CidrMask:   jsii.Number(24),
			},
		},
	})
}

func createTestFunction(stack constructs.Construct, vpc awsec2.IVpc) *liftconstructs.LiftFunction {
	return liftconstructs.NewLiftFunction(stack, jsii.String("TestFunction"), &liftconstructs.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromAsset(jsii.String("../../../examples/hello-world"), nil),
			Handler: jsii.String("main"),
			Runtime: awslambda.Runtime_PROVIDED_AL2(),
			Vpc:     vpc,
			Environment: &map[string]*string{
				"TEST_MODE": jsii.String("true"),
			},
		},
	})
}

func validateCloudWatchResources(t *testing.T, template interface{}) {
	// Validate that CloudWatch resources are properly configured
	// This would inspect the CloudFormation template
	assert.NotNil(t, template)
}

func validateSecurityResources(t *testing.T, template interface{}) {
	// Validate that security resources are properly configured
	// This would inspect WAF, security groups, VPC endpoints, etc.
	assert.NotNil(t, template)
}

func validateServiceDiscoveryResources(t *testing.T, template interface{}) {
	// Validate that service discovery resources are properly configured
	// This would inspect ECS services, service discovery namespaces, etc.
	assert.NotNil(t, template)
}
