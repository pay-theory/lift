package patterns

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

// TenantManagementStackProps defines properties for the tenant management stack
type TenantManagementStackProps struct {
	awscdk.StackProps
	// AppName is the name of the application
	AppName *string
	// MultiTenantAPI is the multi-tenant API to integrate with
	MultiTenantAPI *liftconstructs.MultiTenantAPI
	// EnableBilling enables billing integration
	EnableBilling *bool
	// EnableQuotas enables tenant quota management
	EnableQuotas *bool
	// EnableAuditLog enables comprehensive audit logging
	EnableAuditLog *bool
	// DefaultTenantPlan is the default plan for new tenants
	DefaultTenantPlan *string
	// CustomDomain for tenant management APIs
	CustomDomain *string
	// CertificateArn for custom domain
	CertificateArn *string
}

// TenantManagementStack provides comprehensive tenant lifecycle management
type TenantManagementStack interface {
	awscdk.Stack
	// OnboardingStateMachine returns the tenant onboarding state machine
	OnboardingStateMachine() awsstepfunctions.StateMachine
	// OffboardingStateMachine returns the tenant offboarding state machine
	OffboardingStateMachine() awsstepfunctions.StateMachine
	// TenantTable returns the tenant management table
	TenantTable() awsdynamodb.Table
	// QuotaTable returns the tenant quota table
	QuotaTable() awsdynamodb.Table
	// BillingTable returns the billing tracking table
	BillingTable() awsdynamodb.Table
	// AuditBucket returns the audit log bucket
	AuditBucket() awss3.Bucket
}

type tenantManagementStack struct {
	awscdk.Stack
	onboardingStateMachine  awsstepfunctions.StateMachine
	offboardingStateMachine awsstepfunctions.StateMachine
	tenantTable            awsdynamodb.Table
	quotaTable             awsdynamodb.Table
	billingTable           awsdynamodb.Table
	auditBucket            awss3.Bucket
}

// NewTenantManagementStack creates a new tenant management stack
func NewTenantManagementStack(scope constructs.Construct, id *string, props *TenantManagementStackProps) TenantManagementStack {
	stack := awscdk.NewStack(scope, id, &props.StackProps)

	t := &tenantManagementStack{
		Stack: stack,
	}

	// Create tenant management table
	t.tenantTable = awsdynamodb.NewTable(stack, jsii.String("TenantTable"), &awsdynamodb.TableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("tenantId"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		BillingMode:   awsdynamodb.BillingMode_PAY_PER_REQUEST,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		PointInTimeRecovery: jsii.Bool(true),
		Stream: awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Add GSI for tenant status queries
	t.tenantTable.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("StatusIndex"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("status"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("createdAt"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// Add GSI for plan-based queries
	t.tenantTable.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
		IndexName: jsii.String("PlanIndex"),
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("plan"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		SortKey: &awsdynamodb.Attribute{
			Name: jsii.String("tenantId"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		ProjectionType: awsdynamodb.ProjectionType_ALL,
	})

	// Create quota table if enabled
	if props.EnableQuotas != nil && *props.EnableQuotas {
		t.quotaTable = awsdynamodb.NewTable(stack, jsii.String("QuotaTable"), &awsdynamodb.TableProps{
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("tenantId"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("quotaType"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			BillingMode:   awsdynamodb.BillingMode_PAY_PER_REQUEST,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			TimeToLiveAttribute: jsii.String("ttl"),
		})
	}

	// Create billing table if enabled
	if props.EnableBilling != nil && *props.EnableBilling {
		t.billingTable = awsdynamodb.NewTable(stack, jsii.String("BillingTable"), &awsdynamodb.TableProps{
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("tenantId"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("timestamp"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			BillingMode:   awsdynamodb.BillingMode_PAY_PER_REQUEST,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			Stream: awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
		})

		// Add GSI for billing period queries
		t.billingTable.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
			IndexName: jsii.String("PeriodIndex"),
			PartitionKey: &awsdynamodb.Attribute{
				Name: jsii.String("billingPeriod"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			SortKey: &awsdynamodb.Attribute{
				Name: jsii.String("tenantId"),
				Type: awsdynamodb.AttributeType_STRING,
			},
			ProjectionType: awsdynamodb.ProjectionType_ALL,
		})
	}

	// Create audit bucket if enabled
	if props.EnableAuditLog != nil && *props.EnableAuditLog {
		t.auditBucket = awss3.NewBucket(stack, jsii.String("AuditBucket"), &awss3.BucketProps{
			BucketName:    jsii.String(*props.AppName + "-tenant-audit-logs"),
			Versioned:     jsii.Bool(true),
			Encryption:    awss3.BucketEncryption_S3_MANAGED,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			LifecycleRules: &[]*awss3.LifecycleRule{
				{
					Id:      jsii.String("ArchiveOldLogs"),
					Enabled: jsii.Bool(true),
					Transitions: &[]*awss3.Transition{
						{
							StorageClass: awss3.StorageClass_INFREQUENT_ACCESS(),
							TransitionAfter: awscdk.Duration_Days(jsii.Number(30)),
						},
						{
							StorageClass: awss3.StorageClass_GLACIER(),
							TransitionAfter: awscdk.Duration_Days(jsii.Number(90)),
						},
					},
				},
			},
		})
	}

	// Create Lambda functions for tenant operations
	tenantOpsRole := awsiam.NewRole(stack, jsii.String("TenantOpsRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant permissions
	t.tenantTable.GrantReadWriteData(tenantOpsRole)
	if t.quotaTable != nil {
		t.quotaTable.GrantReadWriteData(tenantOpsRole)
	}
	if t.billingTable != nil {
		t.billingTable.GrantReadWriteData(tenantOpsRole)
	}
	if t.auditBucket != nil {
		t.auditBucket.GrantWrite(tenantOpsRole, nil, nil)
	}

	// Create tenant provisioning Lambda
	provisioningLambda := awslambda.NewFunction(stack, jsii.String("ProvisioningFunction"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist/tenant-provisioning"), nil),
		Role:         tenantOpsRole,
		Timeout:      awscdk.Duration_Minutes(jsii.Number(5)),
		MemorySize:   jsii.Number(512),
		Environment: &map[string]*string{
			"TENANT_TABLE":   t.tenantTable.TableName(),
			"DEFAULT_PLAN":   props.DefaultTenantPlan,
			"APP_NAME":       props.AppName,
		},
		Tracing: awslambda.Tracing_ACTIVE,
	})

	// Create tenant validation Lambda
	validationLambda := awslambda.NewFunction(stack, jsii.String("ValidationFunction"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist/tenant-validation"), nil),
		Role:         tenantOpsRole,
		Timeout:      awscdk.Duration_Minutes(jsii.Number(1)),
		MemorySize:   jsii.Number(256),
		Environment: &map[string]*string{
			"TENANT_TABLE": t.tenantTable.TableName(),
		},
		Tracing: awslambda.Tracing_ACTIVE,
	})

	// Create resource creation Lambda
	resourceCreationLambda := awslambda.NewFunction(stack, jsii.String("ResourceCreationFunction"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist/tenant-resources"), nil),
		Role:         tenantOpsRole,
		Timeout:      awscdk.Duration_Minutes(jsii.Number(10)),
		MemorySize:   jsii.Number(1024),
		Environment: &map[string]*string{
			"TENANT_TABLE": t.tenantTable.TableName(),
			"APP_NAME":     props.AppName,
		},
		Tracing: awslambda.Tracing_ACTIVE,
	})

	// Grant additional permissions for resource creation
	resourceCreationLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("dynamodb:CreateTable"),
			jsii.String("dynamodb:DescribeTable"),
			jsii.String("dynamodb:TagResource"),
			jsii.String("s3:CreateBucket"),
			jsii.String("s3:PutBucketPolicy"),
			jsii.String("s3:PutBucketTagging"),
			jsii.String("iam:CreateRole"),
			jsii.String("iam:AttachRolePolicy"),
			jsii.String("iam:TagRole"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))

	// Create notification Lambda
	notificationLambda := awslambda.NewFunction(stack, jsii.String("NotificationFunction"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist/tenant-notification"), nil),
		Role:         tenantOpsRole,
		Timeout:      awscdk.Duration_Minutes(jsii.Number(1)),
		MemorySize:   jsii.Number(256),
		Environment: &map[string]*string{
			"APP_NAME": props.AppName,
		},
		Tracing: awslambda.Tracing_ACTIVE,
	})

	// Grant SNS/SES permissions for notifications
	notificationLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("sns:Publish"),
			jsii.String("ses:SendEmail"),
			jsii.String("ses:SendTemplatedEmail"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))

	// Create onboarding state machine
	t.onboardingStateMachine = t.createOnboardingStateMachine(
		stack,
		validationLambda,
		provisioningLambda,
		resourceCreationLambda,
		notificationLambda,
		props,
	)

	// Create cleanup Lambda for offboarding
	cleanupLambda := awslambda.NewFunction(stack, jsii.String("CleanupFunction"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist/tenant-cleanup"), nil),
		Role:         tenantOpsRole,
		Timeout:      awscdk.Duration_Minutes(jsii.Number(15)),
		MemorySize:   jsii.Number(1024),
		Environment: &map[string]*string{
			"TENANT_TABLE": t.tenantTable.TableName(),
			"APP_NAME":     props.AppName,
		},
		Tracing: awslambda.Tracing_ACTIVE,
	})

	// Grant permissions for cleanup
	cleanupLambda.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("dynamodb:DeleteTable"),
			jsii.String("dynamodb:ListTables"),
			jsii.String("s3:DeleteBucket"),
			jsii.String("s3:DeleteObject"),
			jsii.String("s3:ListBucket"),
			jsii.String("iam:DeleteRole"),
			jsii.String("iam:DetachRolePolicy"),
			jsii.String("iam:ListAttachedRolePolicies"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))

	// Create archive Lambda for offboarding
	archiveLambda := awslambda.NewFunction(stack, jsii.String("ArchiveFunction"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist/tenant-archive"), nil),
		Role:         tenantOpsRole,
		Timeout:      awscdk.Duration_Minutes(jsii.Number(10)),
		MemorySize:   jsii.Number(1024),
		Environment: &map[string]*string{
			"TENANT_TABLE": t.tenantTable.TableName(),
			"AUDIT_BUCKET": func() *string {
				if t.auditBucket != nil {
					return t.auditBucket.BucketName()
				}
				return jsii.String("")
			}(),
		},
		Tracing: awslambda.Tracing_ACTIVE,
	})

	// Create offboarding state machine
	t.offboardingStateMachine = t.createOffboardingStateMachine(
		stack,
		validationLambda,
		archiveLambda,
		cleanupLambda,
		notificationLambda,
		props,
	)

	// Create EventBridge rules for tenant lifecycle events
	tenantEventBus := awsevents.NewEventBus(stack, jsii.String("TenantEventBus"), &awsevents.EventBusProps{
		EventBusName: jsii.String(*props.AppName + "-tenant-events"),
	})

	// Rule for new tenant registrations
	awsevents.NewRule(stack, jsii.String("NewTenantRule"), &awsevents.RuleProps{
		EventBus:    tenantEventBus,
		RuleName:    jsii.String("new-tenant-registration"),
		Description: jsii.String("Trigger onboarding for new tenant registrations"),
		EventPattern: &awsevents.EventPattern{
			Source:     &[]*string{jsii.String("tenant.management")},
			DetailType: &[]*string{jsii.String("Tenant Registration")},
		},
		Targets: &[]awsevents.IRuleTarget{
			awseventstargets.NewSfnStateMachine(t.onboardingStateMachine, &awseventstargets.SfnStateMachineProps{
				Role: awsiam.NewRole(stack, jsii.String("EventBridgeRole"), &awsiam.RoleProps{
					AssumedBy: awsiam.NewServicePrincipal(jsii.String("events.amazonaws.com"), nil),
				}),
			}),
		},
	})

	// Rule for tenant deletion requests
	awsevents.NewRule(stack, jsii.String("TenantDeletionRule"), &awsevents.RuleProps{
		EventBus:    tenantEventBus,
		RuleName:    jsii.String("tenant-deletion-request"),
		Description: jsii.String("Trigger offboarding for tenant deletion requests"),
		EventPattern: &awsevents.EventPattern{
			Source:     &[]*string{jsii.String("tenant.management")},
			DetailType: &[]*string{jsii.String("Tenant Deletion Request")},
		},
		Targets: &[]awsevents.IRuleTarget{
			awseventstargets.NewSfnStateMachine(t.offboardingStateMachine, &awseventstargets.SfnStateMachineProps{
				Role: awsiam.NewRole(stack, jsii.String("EventBridgeDeletionRole"), &awsiam.RoleProps{
					AssumedBy: awsiam.NewServicePrincipal(jsii.String("events.amazonaws.com"), nil),
				}),
			}),
		},
	})

	// Create quota monitoring Lambda if quotas are enabled
	if props.EnableQuotas != nil && *props.EnableQuotas {
		quotaMonitorLambda := awslambda.NewFunction(stack, jsii.String("QuotaMonitorFunction"), &awslambda.FunctionProps{
			Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
			Architecture: awslambda.Architecture_ARM_64(),
			Handler:      jsii.String("bootstrap"),
			Code:         awslambda.Code_FromAsset(jsii.String("./dist/quota-monitor"), nil),
			Role:         tenantOpsRole,
			Timeout:      awscdk.Duration_Minutes(jsii.Number(5)),
			MemorySize:   jsii.Number(512),
			Environment: &map[string]*string{
				"QUOTA_TABLE":  t.quotaTable.TableName(),
				"TENANT_TABLE": t.tenantTable.TableName(),
			},
			Tracing: awslambda.Tracing_ACTIVE,
		})

		// Schedule quota monitoring every 5 minutes
		awsevents.NewRule(stack, jsii.String("QuotaMonitorRule"), &awsevents.RuleProps{
			Schedule: awsevents.Schedule_Rate(awscdk.Duration_Minutes(jsii.Number(5))),
			Targets: &[]awsevents.IRuleTarget{
				awseventstargets.NewLambdaFunction(quotaMonitorLambda, nil),
			},
		})
	}

	// Create billing aggregation Lambda if billing is enabled
	if props.EnableBilling != nil && *props.EnableBilling {
		billingAggregationLambda := awslambda.NewFunction(stack, jsii.String("BillingAggregationFunction"), &awslambda.FunctionProps{
			Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
			Architecture: awslambda.Architecture_ARM_64(),
			Handler:      jsii.String("bootstrap"),
			Code:         awslambda.Code_FromAsset(jsii.String("./dist/billing-aggregation"), nil),
			Role:         tenantOpsRole,
			Timeout:      awscdk.Duration_Minutes(jsii.Number(15)),
			MemorySize:   jsii.Number(1024),
			Environment: &map[string]*string{
				"BILLING_TABLE": t.billingTable.TableName(),
				"TENANT_TABLE":  t.tenantTable.TableName(),
			},
			Tracing: awslambda.Tracing_ACTIVE,
		})

		// Schedule billing aggregation daily
		awsevents.NewRule(stack, jsii.String("BillingAggregationRule"), &awsevents.RuleProps{
			Schedule: awsevents.Schedule_Cron(&awsevents.CronOptions{
				Hour:   jsii.String("0"),
				Minute: jsii.String("0"),
			}),
			Targets: &[]awsevents.IRuleTarget{
				awseventstargets.NewLambdaFunction(billingAggregationLambda, nil),
			},
		})
	}

	// Output important resources
	awscdk.NewCfnOutput(stack, jsii.String("TenantTableName"), &awscdk.CfnOutputProps{
		Value:       t.tenantTable.TableName(),
		Description: jsii.String("DynamoDB table for tenant management"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("OnboardingStateMachineArn"), &awscdk.CfnOutputProps{
		Value:       t.onboardingStateMachine.StateMachineArn(),
		Description: jsii.String("ARN of the tenant onboarding state machine"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("OffboardingStateMachineArn"), &awscdk.CfnOutputProps{
		Value:       t.offboardingStateMachine.StateMachineArn(),
		Description: jsii.String("ARN of the tenant offboarding state machine"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("TenantEventBusName"), &awscdk.CfnOutputProps{
		Value:       tenantEventBus.EventBusName(),
		Description: jsii.String("Name of the tenant lifecycle event bus"),
	})

	return t
}

// createOnboardingStateMachine creates the tenant onboarding workflow
func (t *tenantManagementStack) createOnboardingStateMachine(
	stack awscdk.Stack,
	validationLambda awslambda.Function,
	provisioningLambda awslambda.Function,
	resourceCreationLambda awslambda.Function,
	notificationLambda awslambda.Function,
	props *TenantManagementStackProps,
) awsstepfunctions.StateMachine {
	// Define tasks
	validateTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("ValidateTenant"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: validationLambda,
		OutputPath:     jsii.String("$.Payload"),
	})

	provisionTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("ProvisionTenant"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: provisioningLambda,
		OutputPath:     jsii.String("$.Payload"),
	})

	createResourcesTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("CreateResources"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: resourceCreationLambda,
		OutputPath:     jsii.String("$.Payload"),
	})

	notifySuccessTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("NotifySuccess"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: notificationLambda,
		Payload: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"type":     "onboarding_success",
			"tenantId": awsstepfunctions.JsonPath_StringAt(jsii.String("$.tenantId")),
			"email":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.email")),
		}),
	})

	notifyFailureTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("NotifyFailure"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: notificationLambda,
		Payload: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"type":     "onboarding_failure",
			"tenantId": awsstepfunctions.JsonPath_StringAt(jsii.String("$.tenantId")),
			"email":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.email")),
			"error":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.error")),
		}),
	})

	// Define success and failure states
	success := awsstepfunctions.NewSucceed(stack, jsii.String("OnboardingSuccess"), &awsstepfunctions.SucceedProps{
		Comment: jsii.String("Tenant onboarding completed successfully"),
	})

	failure := awsstepfunctions.NewFail(stack, jsii.String("OnboardingFailure"), &awsstepfunctions.FailProps{
		Comment: jsii.String("Tenant onboarding failed"),
	})

	// Add catch handlers to individual tasks
	validateTask.AddCatch(notifyFailureTask, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})
	provisionTask.AddCatch(notifyFailureTask, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})
	createResourcesTask.AddCatch(notifyFailureTask, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})

	// Chain the workflow
	definition := validateTask.
		Next(provisionTask).
		Next(createResourcesTask).
		Next(notifySuccessTask).
		Next(success)

	notifyFailureTask.Next(failure)

	// Create the state machine
	logGroup := awslogs.NewLogGroup(stack, jsii.String("OnboardingLogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String("/aws/stepfunctions/" + *props.AppName + "-tenant-onboarding"),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		Retention:     awslogs.RetentionDays_ONE_MONTH,
	})

	return awsstepfunctions.NewStateMachine(stack, jsii.String("OnboardingStateMachine"), &awsstepfunctions.StateMachineProps{
		StateMachineName: jsii.String(*props.AppName + "-tenant-onboarding"),
		Definition:       definition,
		TracingEnabled:   jsii.Bool(true),
		Logs: &awsstepfunctions.LogOptions{
			Destination:          logGroup,
			Level:                awsstepfunctions.LogLevel_ALL,
			IncludeExecutionData: jsii.Bool(true),
		},
	})
}

// createOffboardingStateMachine creates the tenant offboarding workflow
func (t *tenantManagementStack) createOffboardingStateMachine(
	stack awscdk.Stack,
	validationLambda awslambda.Function,
	archiveLambda awslambda.Function,
	cleanupLambda awslambda.Function,
	notificationLambda awslambda.Function,
	props *TenantManagementStackProps,
) awsstepfunctions.StateMachine {
	// Define tasks
	validateTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("ValidateOffboarding"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: validationLambda,
		OutputPath:     jsii.String("$.Payload"),
		Payload: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"action":   "validate_offboarding",
			"tenantId": awsstepfunctions.JsonPath_StringAt(jsii.String("$.tenantId")),
		}),
	})

	archiveTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("ArchiveTenant"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: archiveLambda,
		OutputPath:     jsii.String("$.Payload"),
	})

	cleanupTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("CleanupResources"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: cleanupLambda,
		OutputPath:     jsii.String("$.Payload"),
	})

	notifySuccessTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("NotifyOffboardingSuccess"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: notificationLambda,
		Payload: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"type":     "offboarding_success",
			"tenantId": awsstepfunctions.JsonPath_StringAt(jsii.String("$.tenantId")),
			"email":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.email")),
		}),
	})

	notifyFailureTask := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("NotifyOffboardingFailure"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: notificationLambda,
		Payload: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"type":     "offboarding_failure",
			"tenantId": awsstepfunctions.JsonPath_StringAt(jsii.String("$.tenantId")),
			"email":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.email")),
			"error":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.error")),
		}),
	})

	// Define a wait state for grace period
	waitState := awsstepfunctions.NewWait(stack, jsii.String("GracePeriod"), &awsstepfunctions.WaitProps{
		Time: awsstepfunctions.WaitTime_Duration(awscdk.Duration_Days(jsii.Number(7))),
	})

	// Define success and failure states
	success := awsstepfunctions.NewSucceed(stack, jsii.String("OffboardingSuccess"), &awsstepfunctions.SucceedProps{
		Comment: jsii.String("Tenant offboarding completed successfully"),
	})

	failure := awsstepfunctions.NewFail(stack, jsii.String("OffboardingFailure"), &awsstepfunctions.FailProps{
		Comment: jsii.String("Tenant offboarding failed"),
	})

	// Add catch handlers to individual tasks
	validateTask.AddCatch(notifyFailureTask, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})
	archiveTask.AddCatch(notifyFailureTask, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})
	cleanupTask.AddCatch(notifyFailureTask, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})

	// Chain the workflow
	definition := validateTask.
		Next(waitState).
		Next(archiveTask).
		Next(cleanupTask).
		Next(notifySuccessTask).
		Next(success)

	notifyFailureTask.Next(failure)

	// Create the state machine
	logGroup := awslogs.NewLogGroup(stack, jsii.String("OffboardingLogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String("/aws/stepfunctions/" + *props.AppName + "-tenant-offboarding"),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		Retention:     awslogs.RetentionDays_ONE_MONTH,
	})

	return awsstepfunctions.NewStateMachine(stack, jsii.String("OffboardingStateMachine"), &awsstepfunctions.StateMachineProps{
		StateMachineName: jsii.String(*props.AppName + "-tenant-offboarding"),
		Definition:       definition,
		TracingEnabled:   jsii.Bool(true),
		Logs: &awsstepfunctions.LogOptions{
			Destination:          logGroup,
			Level:                awsstepfunctions.LogLevel_ALL,
			IncludeExecutionData: jsii.Bool(true),
		},
	})
}

// Implement interface methods
func (t *tenantManagementStack) OnboardingStateMachine() awsstepfunctions.StateMachine {
	return t.onboardingStateMachine
}

func (t *tenantManagementStack) OffboardingStateMachine() awsstepfunctions.StateMachine {
	return t.offboardingStateMachine
}

func (t *tenantManagementStack) TenantTable() awsdynamodb.Table {
	return t.tenantTable
}

func (t *tenantManagementStack) QuotaTable() awsdynamodb.Table {
	return t.quotaTable
}

func (t *tenantManagementStack) BillingTable() awsdynamodb.Table {
	return t.billingTable
}

func (t *tenantManagementStack) AuditBucket() awss3.Bucket {
	return t.auditBucket
}