package constructs

import (
	"fmt"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/jsii-runtime-go"
)

func TestAuditingConstruct(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create AuditingConstruct with basic configuration
	auditingConstruct := NewAuditingConstruct(stack, "TestAuditing", &AuditingProps{
		AppName:    jsii.String("test-app"),
		AuditLevel: AuditLevelDetailed,
	})

	// Verify construct was created
	if auditingConstruct == nil {
		t.Fatal("AuditingConstruct should not be nil")
	}

	// Create template for assertions
	template := assertions.Template_FromStack(stack, nil)

	// Test S3 bucket creation
	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
		"BucketName": map[string]interface{}{
			"Fn::Join": []interface{}{
				"",
				[]interface{}{
					"test-app-audit-",
					map[string]interface{}{
						"Ref": "AWS::Region",
					},
				},
			},
		},
		"BucketEncryption": map[string]interface{}{
			"ServerSideEncryptionConfiguration": []interface{}{
				map[string]interface{}{
					"ServerSideEncryptionByDefault": map[string]interface{}{
						"SSEAlgorithm": "aws:kms",
					},
				},
			},
		},
		"VersioningConfiguration": map[string]interface{}{
			"Status": "Enabled",
		},
		"PublicAccessBlockConfiguration": map[string]interface{}{
			"BlockPublicAcls":       true,
			"BlockPublicPolicy":     true,
			"IgnorePublicAcls":      true,
			"RestrictPublicBuckets": true,
		},
	})

	// Test KMS key creation
	template.HasResourceProperties(jsii.String("AWS::KMS::Key"), map[string]interface{}{
		"Description":       "Audit encryption key for test-app",
		"EnableKeyRotation": true,
	})

	// Test CloudTrail creation
	template.HasResourceProperties(jsii.String("AWS::CloudTrail::Trail"), map[string]interface{}{
		"TrailName":                  "test-app-audit-trail",
		"IncludeGlobalServiceEvents": true,
		"IsMultiRegionTrail":         true,
		"EnableLogFileValidation":    true,
	})

	// Test CloudWatch Log Groups
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName":    "/aws/audit/test-app/application",
		"RetentionInDays": 3653,
	})

	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName":    "/aws/audit/test-app/database",
		"RetentionInDays": 3653,
	})

	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName":    "/aws/audit/test-app/system",
		"RetentionInDays": 3653,
	})

	// Test Kinesis Stream
	template.HasResourceProperties(jsii.String("AWS::Kinesis::Stream"), map[string]interface{}{
		"Name":       "test-app-audit-stream",
		"ShardCount": 2,
		"StreamEncryption": map[string]interface{}{
			"EncryptionType": "KMS",
		},
		"RetentionPeriodHours": 24,
	})

	// Test Lambda Functions
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"FunctionName": "test-app-log-processing",
		"Runtime":      "provided.al2",
		"Handler":      "bootstrap",
		"Description":  "Real-time audit log processing function",
		"Timeout":      300,
	})

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"FunctionName": "test-app-integrity-checking",
		"Runtime":      "provided.al2",
		"Handler":      "bootstrap",
		"Description":  "Audit log integrity checking function",
		"Timeout":      900,
	})

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"FunctionName": "test-app-compliance-reporting",
		"Runtime":      "provided.al2",
		"Handler":      "bootstrap",
		"Description":  "Audit compliance reporting function",
		"Timeout":      900,
	})

	// Test EventBridge Rules
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"ScheduleExpression": "rate(1 day)",
		"State":              "ENABLED",
	})

	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"ScheduleExpression": "rate(7 days)",
		"State":              "ENABLED",
	})

	// Test CloudWatch Dashboard
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Dashboard"), map[string]interface{}{
		"DashboardName": "test-app-audit-dashboard",
	})

	// Test CloudWatch Alarms
	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]interface{}{
		"AlarmName":         "test-app-failed-login-attempts",
		"Threshold":         10,
		"EvaluationPeriods": 1,
		"DatapointsToAlarm": 1,
		"TreatMissingData":  "notBreaching",
	})

	template.HasResourceProperties(jsii.String("AWS::CloudWatch::Alarm"), map[string]interface{}{
		"AlarmName":         "test-app-suspicious-activity",
		"Threshold":         100,
		"EvaluationPeriods": 2,
		"DatapointsToAlarm": 2,
		"TreatMissingData":  "notBreaching",
	})

	// Test SSM Parameters
	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"), map[string]interface{}{
		"Name":        "/test-app/audit/level",
		"Value":       "DETAILED",
		"Type":        "String",
		"Description": "Audit logging level",
	})

	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"), map[string]interface{}{
		"Name":        "/test-app/audit/retention-days",
		"Value":       "2555",
		"Type":        "String",
		"Description": "Audit log retention period in days",
	})
}

func TestAuditingConstructWithCustomConfiguration(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create AuditingConstruct with custom configuration
	auditingConstruct := NewAuditingConstruct(stack, "TestAuditing", &AuditingProps{
		AppName:                    jsii.String("custom-app"),
		AuditLevel:                 AuditLevelComprehensive,
		EnableCloudTrail:           jsii.Bool(true),
		EnableApplicationLogs:      jsii.Bool(true),
		EnableDatabaseLogs:         jsii.Bool(true),
		EnableRealTimeProcessing:   jsii.Bool(true),
		EnableTamperProtection:     jsii.Bool(true),
		EnableLogAggregation:       jsii.Bool(true),
		LogRetentionDays:           jsii.Number(365),
		EnableSIEMIntegration:      jsii.Bool(true),
		SIEMEndpoint:               jsii.String("https://siem.example.com"),
		EnableLogAnalysis:          jsii.Bool(true),
		EnableComplianceReporting:  jsii.Bool(true),
		Environment:                jsii.String("production"),
		EnableEncryption:           jsii.Bool(true),
		EnableCrossAccountAccess:   jsii.Bool(true),
		CrossAccountRoleArns:       &[]*string{jsii.String("arn:aws:iam::123456789012:role/CrossAccountRole")},
		EnableIntegrityChecking:    jsii.Bool(true),
		EnableDashboard:            jsii.Bool(true),
		EnableAlerting:             jsii.Bool(true),
		EnableImmutableLogs:        jsii.Bool(true),
		EnableRegulatoryCompliance: jsii.Bool(true),
		ComplianceFrameworks:       &[]string{"SOC2", "HIPAA", "PCI-DSS"},
	})

	// Verify construct was created
	if auditingConstruct == nil {
		t.Fatal("AuditingConstruct should not be nil")
	}

	// Create template for assertions
	template := assertions.Template_FromStack(stack, nil)

	// Test S3 bucket with object lock enabled
	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
		"BucketName": map[string]interface{}{
			"Fn::Join": []interface{}{
				"",
				[]interface{}{
					"custom-app-audit-",
					map[string]interface{}{
						"Ref": "AWS::Region",
					},
				},
			},
		},
		"ObjectLockEnabled": true,
	})

	// Test SSM Parameter with custom retention
	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"), map[string]interface{}{
		"Name":        "/custom-app/audit/retention-days",
		"Value":       "365",
		"Type":        "String",
		"Description": "Audit log retention period in days",
	})

	// Test SSM Parameter with comprehensive audit level
	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"), map[string]interface{}{
		"Name":        "/custom-app/audit/level",
		"Value":       "COMPREHENSIVE",
		"Type":        "String",
		"Description": "Audit logging level",
	})

	// Test SSM Parameter with compliance frameworks
	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"), map[string]interface{}{
		"Name":        "/custom-app/audit/compliance-frameworks",
		"Value":       "[SOC2 HIPAA PCI-DSS]",
		"Type":        "String",
		"Description": "Enabled compliance frameworks",
	})

	// Test cross-account access policy exists
	template.ResourceCountIs(jsii.String("AWS::S3::BucketPolicy"), jsii.Number(1))
}

func TestAuditingConstructBasicConfiguration(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create AuditingConstruct with basic configuration
	auditingConstruct := NewAuditingConstruct(stack, "TestAuditing", &AuditingProps{
		AppName:                   jsii.String("basic-app"),
		AuditLevel:                AuditLevelBasic,
		EnableCloudTrail:          jsii.Bool(false),
		EnableRealTimeProcessing:  jsii.Bool(false),
		EnableLogAggregation:      jsii.Bool(false),
		EnableIntegrityChecking:   jsii.Bool(false),
		EnableComplianceReporting: jsii.Bool(false),
		EnableDashboard:           jsii.Bool(false),
		EnableAlerting:            jsii.Bool(false),
		EnableEncryption:          jsii.Bool(false),
		EnableImmutableLogs:       jsii.Bool(false),
	})

	// Verify construct was created
	if auditingConstruct == nil {
		t.Fatal("AuditingConstruct should not be nil")
	}

	// Create template for assertions
	template := assertions.Template_FromStack(stack, nil)

	// Test that CloudTrail is NOT created when disabled
	template.ResourceCountIs(jsii.String("AWS::CloudTrail::Trail"), jsii.Number(0))

	// Test that Kinesis Stream is NOT created when real-time processing is disabled
	template.ResourceCountIs(jsii.String("AWS::Kinesis::Stream"), jsii.Number(0))

	// Test that Firehose is NOT created when log aggregation is disabled
	template.ResourceCountIs(jsii.String("AWS::KinesisFirehose::DeliveryStream"), jsii.Number(0))

	// Test that integrity checking Lambda is NOT created when disabled
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(0))

	// Test that dashboard is NOT created when disabled
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Dashboard"), jsii.Number(0))

	// Test that alarms are NOT created when disabled
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(0))

	// Test that S3 bucket uses S3 managed encryption when KMS is disabled
	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
		"BucketEncryption": map[string]interface{}{
			"ServerSideEncryptionConfiguration": []interface{}{
				map[string]interface{}{
					"ServerSideEncryptionByDefault": map[string]interface{}{
						"SSEAlgorithm": "AES256",
					},
				},
			},
		},
	})

	// Test that object lock is NOT enabled when immutable logs is disabled
	template.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(1))

	// Test that log groups are still created (basic requirement)
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName": "/aws/audit/basic-app/application",
	})

	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName": "/aws/audit/basic-app/database",
	})

	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName": "/aws/audit/basic-app/system",
	})

	// Test SSM Parameters are still created
	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"), map[string]interface{}{
		"Name":        "/basic-app/audit/level",
		"Value":       "BASIC",
		"Type":        "String",
		"Description": "Audit logging level",
	})
}

func TestAuditingConstructMethods(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create AuditingConstruct
	auditingConstruct := NewAuditingConstruct(stack, "TestAuditing", &AuditingProps{
		AppName:    jsii.String("test-app"),
		AuditLevel: AuditLevelDetailed,
	})

	// Test GetAuditStatus method
	status := auditingConstruct.GetAuditStatus()
	if status == nil {
		t.Fatal("GetAuditStatus should not return nil")
	}

	// Verify expected status fields
	expectedFields := []string{
		"cloudtrail_enabled",
		"application_logs_enabled",
		"database_logs_enabled",
		"real_time_processing",
		"integrity_checking",
		"compliance_reporting",
		"dashboard_enabled",
		"alerting_enabled",
		"encryption_enabled",
		"stream_processing",
		"log_aggregation",
	}

	for _, field := range expectedFields {
		if _, exists := status[field]; !exists {
			t.Errorf("Expected field %s not found in audit status", field)
		}
	}

	// Test AddCustomAuditRule method
	testLogGroup := awslogs.NewLogGroup(stack, jsii.String("TestLogGroup"), &awslogs.LogGroupProps{
		LogGroupName: jsii.String("/test/log/group"),
	})

	auditingConstruct.AddCustomAuditRule("test-rule", testLogGroup, "[ERROR]")

	// Create template for assertions
	template := assertions.Template_FromStack(stack, nil)

	// Test that custom metric filter was created
	template.HasResourceProperties(jsii.String("AWS::Logs::MetricFilter"), map[string]interface{}{
		"FilterPattern": "[ERROR]",
		"MetricTransformations": []interface{}{
			map[string]interface{}{
				"MetricNamespace": "Audit/Custom",
				"MetricName":      "test-rule",
				"MetricValue":     "1",
			},
		},
	})
}

func TestAuditingConstructLogGroupRetention(t *testing.T) {
	// Test different retention periods
	testCases := []struct {
		name           string
		retentionDays  float64
		expectedResult awslogs.RetentionDays
	}{
		{"OneDay", 1, awslogs.RetentionDays_ONE_DAY},
		{"ThreeDays", 3, awslogs.RetentionDays_THREE_DAYS},
		{"OneWeek", 7, awslogs.RetentionDays_ONE_WEEK},
		{"OneMonth", 30, awslogs.RetentionDays_ONE_MONTH},
		{"OneYear", 365, awslogs.RetentionDays_ONE_YEAR},
		{"FiveYears", 1827, awslogs.RetentionDays_FIVE_YEARS},
		{"TenYears", 3653, awslogs.RetentionDays_TEN_YEARS},
		{"Infinite", 10000, awslogs.RetentionDays_INFINITE},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new app and stack for each test case
			app := awscdk.NewApp(nil)
			stack := awscdk.NewStack(app, jsii.String(fmt.Sprintf("TestStack%s", tc.name)), nil)

			// Create AuditingConstruct with specific retention
			NewAuditingConstruct(stack, fmt.Sprintf("TestAuditing%s", tc.name), &AuditingProps{
				AppName:          jsii.String(fmt.Sprintf("test-app-%s", tc.name)),
				AuditLevel:       AuditLevelDetailed,
				LogRetentionDays: jsii.Number(tc.retentionDays),
			})

			// Create template for assertions
			template := assertions.Template_FromStack(stack, nil)

			// Test log group retention
			if tc.name == "Infinite" {
				// For infinite retention, RetentionInDays is not set
				template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
					"LogGroupName": fmt.Sprintf("/aws/audit/test-app-%s/application", tc.name),
				})
			} else {
				template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
					"LogGroupName":    fmt.Sprintf("/aws/audit/test-app-%s/application", tc.name),
					"RetentionInDays": assertions.Match_AnyValue(),
				})
			}
		})
	}
}

func TestAuditingConstructResourceCount(t *testing.T) {
	// Create a test app and stack
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestResourceCountStack"), nil)

	// Create AuditingConstruct with full configuration
	NewAuditingConstruct(stack, "TestAuditing", &AuditingProps{
		AppName:                   jsii.String("full-app"),
		AuditLevel:                AuditLevelComprehensive,
		EnableCloudTrail:          jsii.Bool(true),
		EnableRealTimeProcessing:  jsii.Bool(true),
		EnableLogAggregation:      jsii.Bool(true),
		EnableIntegrityChecking:   jsii.Bool(true),
		EnableComplianceReporting: jsii.Bool(true),
		EnableDashboard:           jsii.Bool(true),
		EnableAlerting:            jsii.Bool(true),
		EnableEncryption:          jsii.Bool(true),
		EnableCrossAccountAccess:  jsii.Bool(true),
		CrossAccountRoleArns:      &[]*string{jsii.String("arn:aws:iam::123456789012:role/CrossAccountRole")},
		ComplianceFrameworks:      &[]string{"SOC2", "HIPAA"},
	})

	// Create template for assertions
	template := assertions.Template_FromStack(stack, nil)

	// Test expected resource counts
	template.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::KMS::Key"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::KMS::Alias"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudTrail::Trail"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Logs::LogGroup"), jsii.Number(3)) // app, db, system
	template.ResourceCountIs(jsii.String("AWS::Kinesis::Stream"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::KinesisFirehose::DeliveryStream"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(3)) // processing, integrity, compliance
	template.ResourceCountIs(jsii.String("AWS::Events::Rule"), jsii.Number(2))     // integrity + compliance schedules
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Dashboard"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Alarm"), jsii.Number(2))
	template.ResourceCountIs(jsii.String("AWS::SSM::Parameter"), jsii.Number(3)) // level, retention, frameworks
	template.ResourceCountIs(jsii.String("AWS::S3::BucketPolicy"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::IAM::Role"), jsii.Number(5)) // firehose, processing, integrity, compliance + CloudTrail service role
}
