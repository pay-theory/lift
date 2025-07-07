package patterns

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
	"github.com/stretchr/testify/assert"
)

func TestTenantManagementStack_BasicConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	// Create a mock MultiTenantAPI (this would be replaced with actual construct in real usage)
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestTenantManagementStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("test-app"),
		MultiTenantAPI:    mockAPI,
		EnableBilling:     jsii.Bool(true),
		EnableQuotas:      jsii.Bool(true),
		EnableAuditLog:    jsii.Bool(true),
		DefaultTenantPlan: jsii.String("basic"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test that tenant table is created
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
		"AttributeDefinitions": []map[string]interface{}{
			{
				"AttributeName": "tenantId",
				"AttributeType": "S",
			},
		},
		"KeySchema": []map[string]interface{}{
			{
				"AttributeName": "tenantId",
				"KeyType":       "HASH",
			},
		},
		"BillingMode":         "PAY_PER_REQUEST",
		"PointInTimeRecovery": map[string]interface{}{
			"PointInTimeRecoveryEnabled": true,
		},
		"StreamSpecification": map[string]interface{}{
			"StreamViewType": "NEW_AND_OLD_IMAGES",
		},
	})

	// Test that GSI for status is created
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
		"GlobalSecondaryIndexes": []map[string]interface{}{
			{
				"IndexName": "StatusIndex",
				"KeySchema": []map[string]interface{}{
					{
						"AttributeName": "status",
						"KeyType":       "HASH",
					},
					{
						"AttributeName": "createdAt",
						"KeyType":       "RANGE",
					},
				},
				"Projection": map[string]interface{}{
					"ProjectionType": "ALL",
				},
			},
		},
	})

	// Test that quota table is created when enabled
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
		"AttributeDefinitions": []map[string]interface{}{
			{
				"AttributeName": "tenantId",
				"AttributeType": "S",
			},
			{
				"AttributeName": "quotaType",
				"AttributeType": "S",
			},
		},
		"KeySchema": []map[string]interface{}{
			{
				"AttributeName": "tenantId",
				"KeyType":       "HASH",
			},
			{
				"AttributeName": "quotaType",
				"KeyType":       "RANGE",
			},
		},
		"TimeToLiveSpecification": map[string]interface{}{
			"AttributeName": "ttl",
			"Enabled":       true,
		},
	})

	// Test that billing table is created when enabled
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
		"AttributeDefinitions": []map[string]interface{}{
			{
				"AttributeName": "tenantId",
				"AttributeType": "S",
			},
			{
				"AttributeName": "timestamp",
				"AttributeType": "S",
			},
		},
		"KeySchema": []map[string]interface{}{
			{
				"AttributeName": "tenantId",
				"KeyType":       "HASH",
			},
			{
				"AttributeName": "timestamp",
				"KeyType":       "RANGE",
			},
		},
		"StreamSpecification": map[string]interface{}{
			"StreamViewType": "NEW_AND_OLD_IMAGES",
		},
	})

	// Test that audit S3 bucket is created when enabled
	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
		"BucketName": "test-app-tenant-audit-logs",
		"BucketEncryption": map[string]interface{}{
			"ServerSideEncryptionConfiguration": []map[string]interface{}{
				{
					"ServerSideEncryptionByDefault": map[string]interface{}{
						"SSEAlgorithm": "AES256",
					},
				},
			},
		},
		"VersioningConfiguration": map[string]interface{}{
			"Status": "Enabled",
		},
		"LifecycleConfiguration": map[string]interface{}{
			"Rules": []map[string]interface{}{
				{
					"Id":     "ArchiveOldLogs",
					"Status": "Enabled",
					"Transitions": []map[string]interface{}{
						{
							"StorageClass":     "STANDARD_IA",
							"TransitionInDays": 30,
						},
						{
							"StorageClass":     "GLACIER",
							"TransitionInDays": 90,
						},
					},
				},
			},
		},
	})

	// Test Lambda functions are created
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(7)) // 7 Lambda functions expected

	// Test Step Functions state machines are created
	template.ResourceCountIs(jsii.String("AWS::StepFunctions::StateMachine"), jsii.Number(2)) // onboarding and offboarding

	// Test EventBridge rules are created
	template.ResourceCountIs(jsii.String("AWS::Events::Rule"), jsii.Number(4)) // 2 tenant lifecycle + 2 scheduled
}

func TestTenantManagementStack_MinimalConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestMinimalStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("minimal-app"),
		MultiTenantAPI:    mockAPI,
		EnableBilling:     jsii.Bool(false),
		EnableQuotas:      jsii.Bool(false),
		EnableAuditLog:    jsii.Bool(false),
		DefaultTenantPlan: jsii.String("free"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test that only core resources are created
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::Table"), jsii.Number(1)) // Only tenant table
	template.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(0))      // No audit bucket
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(4)) // Core functions only
	template.ResourceCountIs(jsii.String("AWS::Events::Rule"), jsii.Number(2))     // Only lifecycle rules

	// Test that tenant table still has required GSIs
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
		"GlobalSecondaryIndexes": []map[string]interface{}{
			{
				"IndexName": "StatusIndex",
			},
			{
				"IndexName": "PlanIndex",
			},
		},
	})
}

func TestTenantManagementStack_LambdaFunctionConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestLambdaStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("lambda-test"),
		MultiTenantAPI:    mockAPI,
		DefaultTenantPlan: jsii.String("standard"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test Lambda runtime configuration
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Runtime":      "provided.al2023",
		"Handler":      "bootstrap",
		"Architecture": []string{"arm64"},
		"TracingConfig": map[string]interface{}{
			"Mode": "Active",
		},
	})

	// Test that Lambda functions have appropriate timeouts
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Timeout": 300, // 5 minutes for provisioning
	})

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Timeout": 600, // 10 minutes for resource creation
	})

	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Timeout": 900, // 15 minutes for cleanup
	})
}

func TestTenantManagementStack_StateMachineConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestStateMachineStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("statemachine-test"),
		MultiTenantAPI:    mockAPI,
		DefaultTenantPlan: jsii.String("premium"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test state machine tracing is enabled
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"TracingConfiguration": map[string]interface{}{
			"Enabled": true,
		},
	})

	// Test state machine logging is configured
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"LoggingConfiguration": map[string]interface{}{
			"Level":                "ALL",
			"IncludeExecutionData": true,
		},
	})
}

func TestTenantManagementStack_EventBridgeConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestEventBridgeStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("eventbridge-test"),
		MultiTenantAPI:    mockAPI,
		DefaultTenantPlan: jsii.String("enterprise"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test EventBridge event bus is created
	template.HasResourceProperties(jsii.String("AWS::Events::EventBus"), map[string]interface{}{
		"Name": "eventbridge-test-tenant-events",
	})

	// Test EventBridge rules are configured correctly
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"Name":        "new-tenant-registration",
		"Description": "Trigger onboarding for new tenant registrations",
		"EventPattern": map[string]interface{}{
			"source":      []string{"tenant.management"},
			"detail-type": []string{"Tenant Registration"},
		},
	})

	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"Name":        "tenant-deletion-request",
		"Description": "Trigger offboarding for tenant deletion requests",
		"EventPattern": map[string]interface{}{
			"source":      []string{"tenant.management"},
			"detail-type": []string{"Tenant Deletion Request"},
		},
	})
}

func TestTenantManagementStack_IAMPermissions(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestIAMStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("iam-test"),
		MultiTenantAPI:    mockAPI,
		EnableBilling:     jsii.Bool(true),
		EnableQuotas:      jsii.Bool(true),
		EnableAuditLog:    jsii.Bool(true),
		DefaultTenantPlan: jsii.String("business"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test IAM role is created for Lambda functions
	template.HasResourceProperties(jsii.String("AWS::IAM::Role"), map[string]interface{}{
		"AssumeRolePolicyDocument": map[string]interface{}{
			"Statement": []map[string]interface{}{
				{
					"Effect": "Allow",
					"Principal": map[string]interface{}{
						"Service": "lambda.amazonaws.com",
					},
					"Action": "sts:AssumeRole",
				},
			},
		},
		"ManagedPolicyArns": []string{
			"arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
		},
	})

	// Test IAM policy statements for resource creation
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": []map[string]interface{}{
				{
					"Effect": "Allow",
					"Action": []string{
						"dynamodb:CreateTable",
						"dynamodb:DescribeTable",
						"dynamodb:TagResource",
						"s3:CreateBucket",
						"s3:PutBucketPolicy",
						"s3:PutBucketTagging",
						"iam:CreateRole",
						"iam:AttachRolePolicy",
						"iam:TagRole",
					},
					"Resource": "*",
				},
			},
		},
	})

	// Test IAM policy statements for cleanup
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": []map[string]interface{}{
				{
					"Effect": "Allow",
					"Action": []string{
						"dynamodb:DeleteTable",
						"dynamodb:ListTables",
						"s3:DeleteBucket",
						"s3:DeleteObject",
						"s3:ListBucket",
						"iam:DeleteRole",
						"iam:DetachRolePolicy",
						"iam:ListAttachedRolePolicies",
					},
					"Resource": "*",
				},
			},
		},
	})
}

func TestTenantManagementStack_OutputsCreated(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestOutputsStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("outputs-test"),
		MultiTenantAPI:    mockAPI,
		DefaultTenantPlan: jsii.String("starter"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test that outputs are created
	template.HasOutput(jsii.String("TenantTableName"), map[string]interface{}{
		"Description": "DynamoDB table for tenant management",
	})

	template.HasOutput(jsii.String("OnboardingStateMachineArn"), map[string]interface{}{
		"Description": "ARN of the tenant onboarding state machine",
	})

	template.HasOutput(jsii.String("OffboardingStateMachineArn"), map[string]interface{}{
		"Description": "ARN of the tenant offboarding state machine",
	})

	template.HasOutput(jsii.String("TenantEventBusName"), map[string]interface{}{
		"Description": "Name of the tenant lifecycle event bus",
	})
}

func TestTenantManagementStack_InterfaceMethods(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestInterfaceStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("interface-test"),
		MultiTenantAPI:    mockAPI,
		EnableBilling:     jsii.Bool(true),
		EnableQuotas:      jsii.Bool(true),
		EnableAuditLog:    jsii.Bool(true),
		DefaultTenantPlan: jsii.String("professional"),
	})

	// Test that interface methods return non-nil values
	assert.NotNil(t, stack.OnboardingStateMachine(), "OnboardingStateMachine should not be nil")
	assert.NotNil(t, stack.OffboardingStateMachine(), "OffboardingStateMachine should not be nil")
	assert.NotNil(t, stack.TenantTable(), "TenantTable should not be nil")
	assert.NotNil(t, stack.QuotaTable(), "QuotaTable should not be nil")
	assert.NotNil(t, stack.BillingTable(), "BillingTable should not be nil")
	assert.NotNil(t, stack.AuditBucket(), "AuditBucket should not be nil")
}

func TestTenantManagementStack_ScheduledRules(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestScheduledStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("scheduled-test"),
		MultiTenantAPI:    mockAPI,
		EnableBilling:     jsii.Bool(true),
		EnableQuotas:      jsii.Bool(true),
		DefaultTenantPlan: jsii.String("premium"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test quota monitoring scheduled rule
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"ScheduleExpression": "rate(5 minutes)",
	})

	// Test billing aggregation scheduled rule
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"ScheduleExpression": "cron(0 0 * * ? *)",
	})
}

func TestTenantManagementStack_LogGroups(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestLogGroupsStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("logs-test"),
		MultiTenantAPI:    mockAPI,
		DefaultTenantPlan: jsii.String("basic"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test log groups are created for state machines
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName": "/aws/stepfunctions/logs-test-tenant-onboarding",
		"RetentionInDays": 30,
	})

	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
		"LogGroupName": "/aws/stepfunctions/logs-test-tenant-offboarding",
		"RetentionInDays": 30,
	})
}

// Helper function to validate state machine definition structure
func validateStateMachineDefinition(t *testing.T, definition interface{}) {
	// Convert to JSON to validate structure
	jsonData, err := json.Marshal(definition)
	assert.NoError(t, err, "Should be able to marshal state machine definition")

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	assert.NoError(t, err, "Should be able to unmarshal state machine definition")

	// Validate basic structure
	assert.Contains(t, parsed, "StartAt", "State machine should have StartAt")
	assert.Contains(t, parsed, "States", "State machine should have States")
	
	states, ok := parsed["States"].(map[string]interface{})
	assert.True(t, ok, "States should be a map")
	assert.Greater(t, len(states), 0, "Should have at least one state")
}

func TestTenantManagementStack_StateDefinitionStructure(t *testing.T) {
	app := awscdk.NewApp(nil)
	
	var mockAPI *liftconstructs.MultiTenantAPI
	
	stack := NewTenantManagementStack(app, jsii.String("TestStateDefStack"), &TenantManagementStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("us-east-1"),
			},
		},
		AppName:           jsii.String("state-def-test"),
		MultiTenantAPI:    mockAPI,
		DefaultTenantPlan: jsii.String("enterprise"),
	})

	template := assertions.Template_FromStack(stack.(awscdk.Stack), nil)

	// Test that state machines have proper definition structure
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"StateMachineName": "state-def-test-tenant-onboarding",
	})

	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"StateMachineName": "state-def-test-tenant-offboarding",
	})
}