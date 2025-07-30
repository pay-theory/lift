package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudtrail"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsconfig"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsguardduty"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecurityhub"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsservicecatalog"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// ComplianceFramework defines the compliance framework to implement
type ComplianceFramework string

const (
	// SOC2 Service Organization Control 2
	SOC2 ComplianceFramework = "SOC2"
	// HIPAA Health Insurance Portability and Accountability Act
	HIPAA ComplianceFramework = "HIPAA"
	// PCI_DSS Payment Card Industry Data Security Standard
	PCI_DSS ComplianceFramework = "PCI_DSS"
	// ISO27001 Information Security Management System
	ISO27001 ComplianceFramework = "ISO27001"
	// FedRAMP Federal Risk and Authorization Management Program
	FedRAMP ComplianceFramework = "FedRAMP"
	// GDPR General Data Protection Regulation
	GDPR ComplianceFramework = "GDPR"
)

// ComplianceStackProps defines properties for ComplianceStack
type ComplianceStackProps struct {
	// Application name for resource naming
	AppName *string

	// Compliance frameworks to implement
	ComplianceFrameworks *[]ComplianceFramework

	// Enable CloudTrail logging
	EnableCloudTrail *bool

	// Enable AWS Config rules
	EnableConfig *bool

	// Enable GuardDuty threat detection
	EnableGuardDuty *bool

	// Enable Security Hub
	EnableSecurityHub *bool

	// Enable data encryption at rest
	EnableEncryption *bool

	// Data retention period in days
	DataRetentionDays *float64

	// Enable compliance reports
	EnableComplianceReports *bool

	// S3 bucket for compliance data
	ComplianceBucket awss3.IBucket

	// KMS key for encryption
	EncryptionKey awskms.IKey

	// CloudWatch log group for compliance logs
	ComplianceLogGroup awslogs.ILogGroup

	// Enable detailed access logging
	EnableDetailedLogging *bool

	// Enable audit trail
	EnableAuditTrail *bool

	// Environment for compliance (dev, staging, prod)
	Environment *string

	// Organization ID for multi-account setup
	OrganizationId *string

	// Enable compliance automation
	EnableAutomation *bool

	// Notification topic ARN for compliance alerts
	NotificationTopicArn *string
}

// ComplianceStack creates a comprehensive compliance stack
type ComplianceStack struct {
	constructs.Construct
	CloudTrail         awscloudtrail.Trail
	ConfigRecorder     awsconfig.CfnConfigurationRecorder
	GuardDutyDetector  awsguardduty.CfnDetector
	SecurityHub        awssecurityhub.CfnHub
	ComplianceBucket   awss3.Bucket
	EncryptionKey      awskms.Key
	ComplianceLogGroup awslogs.LogGroup
	ComplianceFunction awslambda.Function
}

// NewComplianceStack creates a new compliance stack construct
func NewComplianceStack(scope constructs.Construct, id string, props *ComplianceStackProps) *ComplianceStack {
	this := constructs.NewConstruct(scope, &id)

	// Set defaults
	if props.EnableCloudTrail == nil {
		props.EnableCloudTrail = jsii.Bool(true)
	}
	if props.EnableConfig == nil {
		props.EnableConfig = jsii.Bool(true)
	}
	if props.EnableGuardDuty == nil {
		props.EnableGuardDuty = jsii.Bool(true)
	}
	if props.EnableSecurityHub == nil {
		props.EnableSecurityHub = jsii.Bool(true)
	}
	if props.EnableEncryption == nil {
		props.EnableEncryption = jsii.Bool(true)
	}
	if props.DataRetentionDays == nil {
		props.DataRetentionDays = jsii.Number(2555) // 7 years
	}
	if props.EnableComplianceReports == nil {
		props.EnableComplianceReports = jsii.Bool(true)
	}
	if props.Environment == nil {
		props.Environment = jsii.String("prod")
	}
	if props.EnableAutomation == nil {
		props.EnableAutomation = jsii.Bool(true)
	}

	// Create KMS key for encryption
	var encryptionKey awskms.Key
	if props.EnableEncryption != nil && *props.EnableEncryption {
		if props.EncryptionKey != nil {
			if key, ok := props.EncryptionKey.(awskms.Key); ok {
				encryptionKey = key
			}
		} else {
			encryptionKey = awskms.NewKey(this, jsii.String("ComplianceKey"), &awskms.KeyProps{
				Description:       jsii.String(fmt.Sprintf("Compliance encryption key for %s", *props.AppName)),
				EnableKeyRotation: jsii.Bool(true),
				Policy: awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
					Statements: &[]awsiam.PolicyStatement{
						awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
							Sid:    jsii.String("Enable IAM User Permissions"),
							Effect: awsiam.Effect_ALLOW,
							Principals: &[]awsiam.IPrincipal{
								awsiam.NewAccountRootPrincipal(),
							},
							Actions:   &[]*string{jsii.String("kms:*")},
							Resources: &[]*string{jsii.String("*")},
						}),
						awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
							Sid:    jsii.String("Allow CloudTrail to encrypt logs"),
							Effect: awsiam.Effect_ALLOW,
							Principals: &[]awsiam.IPrincipal{
								awsiam.NewServicePrincipal(jsii.String("cloudtrail.amazonaws.com"), nil),
							},
							Actions: &[]*string{
								jsii.String("kms:GenerateDataKey*"),
								jsii.String("kms:DescribeKey"),
							},
							Resources: &[]*string{jsii.String("*")},
						}),
					},
				}),
			})
			// Add alias for easier identification
			encryptionKey.AddAlias(jsii.String(fmt.Sprintf("alias/%s-compliance", *props.AppName)))
		}
	}

	// Create S3 bucket for compliance data
	var complianceBucket awss3.Bucket
	if props.ComplianceBucket != nil {
		if bucket, ok := props.ComplianceBucket.(awss3.Bucket); ok {
			complianceBucket = bucket
		}
	} else {
		complianceBucket = awss3.NewBucket(this, jsii.String("ComplianceBucket"), &awss3.BucketProps{
			BucketName: jsii.String(fmt.Sprintf("%s-compliance-%s", *props.AppName, *awscdk.Stack_Of(this).Region())),
			Encryption: func() awss3.BucketEncryption {
				if props.EnableEncryption != nil && *props.EnableEncryption {
					return awss3.BucketEncryption_KMS
				}
				return awss3.BucketEncryption_S3_MANAGED
			}(),
			EncryptionKey: func() awskms.IKey {
				if props.EnableEncryption != nil && *props.EnableEncryption {
					return encryptionKey
				}
				return nil
			}(),
			BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
			Versioned:         jsii.Bool(true),
			LifecycleRules: &[]*awss3.LifecycleRule{
				{
					Id: jsii.String("ComplianceDataLifecycle"),
					Transitions: &[]*awss3.Transition{
						{
							StorageClass:    awss3.StorageClass_INFREQUENT_ACCESS(),
							TransitionAfter: awscdk.Duration_Days(jsii.Number(30)),
						},
						{
							StorageClass:    awss3.StorageClass_GLACIER(),
							TransitionAfter: awscdk.Duration_Days(jsii.Number(90)),
						},
						{
							StorageClass:    awss3.StorageClass_DEEP_ARCHIVE(),
							TransitionAfter: awscdk.Duration_Days(jsii.Number(365)),
						},
					},
					Expiration: awscdk.Duration_Days(props.DataRetentionDays),
				},
			},
			ServerAccessLogsPrefix: jsii.String("access-logs/"),
		})
	}

	// Create CloudWatch log group for compliance logs
	var complianceLogGroup awslogs.LogGroup
	if props.ComplianceLogGroup != nil {
		if lg, ok := props.ComplianceLogGroup.(awslogs.LogGroup); ok {
			complianceLogGroup = lg
		}
	} else {
		complianceLogGroup = awslogs.NewLogGroup(this, jsii.String("ComplianceLogGroup"), &awslogs.LogGroupProps{
			LogGroupName:  jsii.String(fmt.Sprintf("/aws/compliance/%s", *props.AppName)),
			Retention:     awslogs.RetentionDays_ONE_YEAR,
			RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
			EncryptionKey: func() awskms.IKey {
				if props.EnableEncryption != nil && *props.EnableEncryption {
					return encryptionKey
				}
				return nil
			}(),
		})
	}

	// Create CloudTrail
	var cloudTrail awscloudtrail.Trail
	if props.EnableCloudTrail != nil && *props.EnableCloudTrail {
		cloudTrail = awscloudtrail.NewTrail(this, jsii.String("CloudTrail"), &awscloudtrail.TrailProps{
			TrailName:                  jsii.String(fmt.Sprintf("%s-compliance-trail", *props.AppName)),
			Bucket:                     complianceBucket,
			S3KeyPrefix:                jsii.String("cloudtrail/"),
			IncludeGlobalServiceEvents: jsii.Bool(true),
			IsMultiRegionTrail:         jsii.Bool(true),
			EnableFileValidation:       jsii.Bool(true),
			SendToCloudWatchLogs:       jsii.Bool(true),
			CloudWatchLogGroup:         complianceLogGroup,
		})
	}

	// Create Config configuration recorder
	var configRecorder awsconfig.CfnConfigurationRecorder
	if props.EnableConfig != nil && *props.EnableConfig {
		// Create Config service role
		configRole := awsiam.NewRole(this, jsii.String("ConfigRole"), &awsiam.RoleProps{
			AssumedBy: awsiam.NewServicePrincipal(jsii.String("config.amazonaws.com"), nil),
			ManagedPolicies: &[]awsiam.IManagedPolicy{
				awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/ConfigRole")),
			},
		})

		// Create Config delivery channel
		awsconfig.NewCfnDeliveryChannel(this, jsii.String("ConfigDeliveryChannel"), &awsconfig.CfnDeliveryChannelProps{
			S3BucketName: complianceBucket.BucketName(),
			S3KeyPrefix:  jsii.String("config/"),
			ConfigSnapshotDeliveryProperties: &awsconfig.CfnDeliveryChannel_ConfigSnapshotDeliveryPropertiesProperty{
				DeliveryFrequency: jsii.String("TwentyFour_Hours"),
			},
		})

		// Create Config recorder
		configRecorder = awsconfig.NewCfnConfigurationRecorder(this, jsii.String("ConfigRecorder"), &awsconfig.CfnConfigurationRecorderProps{
			RoleArn: configRole.RoleArn(),
			RecordingGroup: &awsconfig.CfnConfigurationRecorder_RecordingGroupProperty{
				AllSupported:               jsii.Bool(true),
				IncludeGlobalResourceTypes: jsii.Bool(true),
				RecordingStrategy: &awsconfig.CfnConfigurationRecorder_RecordingStrategyProperty{
					UseOnly: jsii.String("ALL_SUPPORTED_RESOURCE_TYPES"),
				},
			},
		})

		// Create Config rules for compliance frameworks
		if props.ComplianceFrameworks != nil {
			for _, framework := range *props.ComplianceFrameworks {
				createConfigRulesForFramework(this, framework)
			}
		}
	}

	// Create GuardDuty detector
	var guardDutyDetector awsguardduty.CfnDetector
	if props.EnableGuardDuty != nil && *props.EnableGuardDuty {
		guardDutyDetector = awsguardduty.NewCfnDetector(this, jsii.String("GuardDutyDetector"), &awsguardduty.CfnDetectorProps{
			Enable:                     jsii.Bool(true),
			FindingPublishingFrequency: jsii.String("FIFTEEN_MINUTES"),
			Features: &[]interface{}{
				&awsguardduty.CfnDetector_CFNFeatureConfigurationProperty{
					Name:   jsii.String("S3_DATA_EVENTS"),
					Status: jsii.String("ENABLED"),
				},
				&awsguardduty.CfnDetector_CFNFeatureConfigurationProperty{
					Name:   jsii.String("EKS_AUDIT_LOGS"),
					Status: jsii.String("ENABLED"),
				},
				&awsguardduty.CfnDetector_CFNFeatureConfigurationProperty{
					Name:   jsii.String("RDS_LOGIN_EVENTS"),
					Status: jsii.String("ENABLED"),
				},
				&awsguardduty.CfnDetector_CFNFeatureConfigurationProperty{
					Name:   jsii.String("LAMBDA_NETWORK_LOGS"),
					Status: jsii.String("ENABLED"),
				},
			},
		})
	}

	// Create Security Hub
	var securityHub awssecurityhub.CfnHub
	if props.EnableSecurityHub != nil && *props.EnableSecurityHub {
		securityHub = awssecurityhub.NewCfnHub(this, jsii.String("SecurityHub"), &awssecurityhub.CfnHubProps{
			AutoEnableControls:     jsii.Bool(true),
			EnableDefaultStandards: jsii.Bool(true),
			Tags: map[string]*string{
				"Application": props.AppName,
				"Environment": props.Environment,
			},
		})

		// Enable compliance standards
		if props.ComplianceFrameworks != nil {
			for i, framework := range *props.ComplianceFrameworks {
				enableComplianceStandard(this, framework, i)
			}
		}
	}

	// Create compliance automation function
	var complianceFunction awslambda.Function
	if props.EnableAutomation != nil && *props.EnableAutomation {
		complianceFunction = createComplianceFunction(this, props, complianceBucket, encryptionKey)
	}

	// Create compliance reports
	if props.EnableComplianceReports != nil && *props.EnableComplianceReports {
		createComplianceReports(this, props, complianceBucket, complianceFunction)
	}

	// Store compliance configuration in SSM Parameter Store
	storeComplianceConfiguration(this, props)

	return &ComplianceStack{
		Construct:          this,
		CloudTrail:         cloudTrail,
		ConfigRecorder:     configRecorder,
		GuardDutyDetector:  guardDutyDetector,
		SecurityHub:        securityHub,
		ComplianceBucket:   complianceBucket,
		EncryptionKey:      encryptionKey,
		ComplianceLogGroup: complianceLogGroup,
		ComplianceFunction: complianceFunction,
	}
}

// createConfigRulesForFramework creates AWS Config rules based on the compliance framework
func createConfigRulesForFramework(scope constructs.Construct, framework ComplianceFramework) {
	switch framework {
	case SOC2:
		// Create SOC2-specific Config rules
		awsconfig.NewCfnConfigRule(scope, jsii.String("SOC2RootAccountMFAEnabled"), &awsconfig.CfnConfigRuleProps{
			ConfigRuleName: jsii.String("soc2-root-account-mfa-enabled"),
			Description:    jsii.String("Checks whether MFA is enabled for the root user"),
			Source: &awsconfig.CfnConfigRule_SourceProperty{
				Owner:            jsii.String("AWS"),
				SourceIdentifier: jsii.String("ROOT_ACCOUNT_MFA_ENABLED"),
			},
		})
	case HIPAA:
		// Create HIPAA-specific Config rules
		awsconfig.NewCfnConfigRule(scope, jsii.String("HIPAAEncryptedVolumes"), &awsconfig.CfnConfigRuleProps{
			ConfigRuleName: jsii.String("hipaa-encrypted-volumes"),
			Description:    jsii.String("Checks whether EBS volumes are encrypted"),
			Source: &awsconfig.CfnConfigRule_SourceProperty{
				Owner:            jsii.String("AWS"),
				SourceIdentifier: jsii.String("ENCRYPTED_VOLUMES"),
			},
		})
	case PCI_DSS:
		// Create PCI DSS-specific Config rules
		awsconfig.NewCfnConfigRule(scope, jsii.String("PCIDSSAccessLogsEnabled"), &awsconfig.CfnConfigRuleProps{
			ConfigRuleName: jsii.String("pci-dss-access-logs-enabled"),
			Description:    jsii.String("Checks whether access logs are enabled"),
			Source: &awsconfig.CfnConfigRule_SourceProperty{
				Owner:            jsii.String("AWS"),
				SourceIdentifier: jsii.String("S3_BUCKET_LOGGING_ENABLED"),
			},
		})
	}
}

// enableComplianceStandard enables specific compliance standards in Security Hub
func enableComplianceStandard(scope constructs.Construct, framework ComplianceFramework, index int) {
	// Enable specific standards based on the framework using CfnStandard
	switch framework {
	case SOC2:
		// Enable CIS AWS Foundations Benchmark for SOC2
		awssecurityhub.NewCfnStandard(scope, jsii.String(fmt.Sprintf("SOC2Standard%d", index)), &awssecurityhub.CfnStandardProps{
			StandardsArn: jsii.String("arn:aws:securityhub:::standard/cis-aws-foundations-benchmark/v/1.2.0"),
		})
	case HIPAA:
		// Enable AWS Foundational Security Standard for HIPAA
		awssecurityhub.NewCfnStandard(scope, jsii.String(fmt.Sprintf("HIPAAStandard%d", index)), &awssecurityhub.CfnStandardProps{
			StandardsArn: jsii.String("arn:aws:securityhub:::standard/aws-foundational-security-best-practices/v/1.0.0"),
		})
	case PCI_DSS:
		// Enable PCI DSS standard
		awssecurityhub.NewCfnStandard(scope, jsii.String(fmt.Sprintf("PCIDSSStandard%d", index)), &awssecurityhub.CfnStandardProps{
			StandardsArn: jsii.String("arn:aws:securityhub:::standard/pci-dss/v/3.2.1"),
		})
	case FedRAMP:
		// Enable AWS Foundational Security Standard for FedRAMP
		awssecurityhub.NewCfnStandard(scope, jsii.String(fmt.Sprintf("FedRAMPStandard%d", index)), &awssecurityhub.CfnStandardProps{
			StandardsArn: jsii.String("arn:aws:securityhub:::standard/aws-foundational-security-best-practices/v/1.0.0"),
		})
	}
}

// createComplianceFunction creates a Lambda function for compliance automation
func createComplianceFunction(scope constructs.Construct, props *ComplianceStackProps, bucket awss3.Bucket, key awskms.Key) awslambda.Function {

	function := CreateStandardLambdaFunction(scope, "ComplianceFunction", bucket, key, LambdaFunctionConfig{
		FunctionName: fmt.Sprintf("%s-compliance-automation", *props.AppName),
		Description:  "Compliance automation and reporting function",
		Timeout:      awscdk.Duration_Minutes(jsii.Number(15)),
		Permissions:  "readwrite",
		Environment: map[string]*string{
			"COMPLIANCE_BUCKET": bucket.BucketName(),
			"APP_NAME":          props.AppName,
			"ENVIRONMENT":       props.Environment,
		},
	})

	// Add additional compliance-specific permissions
	if roleInterface := function.Role(); roleInterface != nil {
		if functionRole, ok := roleInterface.(awsiam.Role); ok {
			functionRole.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("config:GetComplianceDetailsByConfigRule"),
			jsii.String("config:GetComplianceDetailsByResource"),
			jsii.String("config:DescribeConfigRules"),
			jsii.String("config:DescribeComplianceByConfigRule"),
			jsii.String("securityhub:GetFindings"),
			jsii.String("securityhub:BatchImportFindings"),
			jsii.String("guardduty:GetFindings"),
			jsii.String("cloudtrail:LookupEvents"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))
		}
	}

	return function
}

// createComplianceReports creates compliance reporting automation
func createComplianceReports(scope constructs.Construct, props *ComplianceStackProps, _ awss3.Bucket, _ awslambda.Function) {
	// Create EventBridge rule for daily compliance reports
	// This would trigger the compliance function daily to generate reports

	// Create Service Catalog portfolio for compliance templates
	awsservicecatalog.NewPortfolio(scope, jsii.String("CompliancePortfolio"), &awsservicecatalog.PortfolioProps{
		DisplayName:  jsii.String(fmt.Sprintf("%s Compliance Templates", *props.AppName)),
		Description:  jsii.String("Pre-approved compliance templates for consistent deployment"),
		ProviderName: jsii.String("Compliance Team"),
	})
}

// storeComplianceConfiguration stores compliance configuration in SSM Parameter Store
func storeComplianceConfiguration(scope constructs.Construct, props *ComplianceStackProps) {
	// Store compliance frameworks
	if props.ComplianceFrameworks != nil {
		frameworks := make([]string, len(*props.ComplianceFrameworks))
		for i, framework := range *props.ComplianceFrameworks {
			frameworks[i] = string(framework)
		}

		awsssm.NewStringParameter(scope, jsii.String("ComplianceFrameworks"), &awsssm.StringParameterProps{
			ParameterName: jsii.String(fmt.Sprintf("/%s/compliance/frameworks", *props.AppName)),
			StringValue:   jsii.String(fmt.Sprintf("%v", frameworks)),
			Description:   jsii.String("Enabled compliance frameworks"),
		})
	}

	// Store data retention policy
	awsssm.NewStringParameter(scope, jsii.String("DataRetentionPolicy"), &awsssm.StringParameterProps{
		ParameterName: jsii.String(fmt.Sprintf("/%s/compliance/data-retention-days", *props.AppName)),
		StringValue:   jsii.String(fmt.Sprintf("%.0f", *props.DataRetentionDays)),
		Description:   jsii.String("Data retention period in days"),
	})
}

// GetComplianceStatus returns the current compliance status
func (c *ComplianceStack) GetComplianceStatus() map[string]interface{} {
	return map[string]interface{}{
		"cloudtrail_enabled":  c.CloudTrail != nil,
		"config_enabled":      c.ConfigRecorder != nil,
		"guardduty_enabled":   c.GuardDutyDetector != nil,
		"securityhub_enabled": c.SecurityHub != nil,
		"encryption_enabled":  c.EncryptionKey != nil,
		"function_enabled":    c.ComplianceFunction != nil,
	}
}

// AddComplianceRule adds a new compliance rule to the stack
func (c *ComplianceStack) AddComplianceRule(ruleId string, ruleName string) {
	// Create a Config rule using CfnConfigRule
	awsconfig.NewCfnConfigRule(c.Construct, jsii.String(ruleId), &awsconfig.CfnConfigRuleProps{
		ConfigRuleName: jsii.String(fmt.Sprintf("%s-rule", ruleId)),
		Description:    jsii.String(fmt.Sprintf("Additional compliance rule: %s", ruleName)),
		Source: &awsconfig.CfnConfigRule_SourceProperty{
			Owner:            jsii.String("AWS"),
			SourceIdentifier: jsii.String(ruleName),
		},
	})
}
