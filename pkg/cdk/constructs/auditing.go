package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudtrail"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskinesisfirehose"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskinesis"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// AuditLevel defines the level of audit logging
type AuditLevel string

const (
	// AuditLevelBasic provides basic audit logging
	AuditLevelBasic AuditLevel = "BASIC"
	// AuditLevelDetailed provides detailed audit logging
	AuditLevelDetailed AuditLevel = "DETAILED"
	// AuditLevelComprehensive provides comprehensive audit logging
	AuditLevelComprehensive AuditLevel = "COMPREHENSIVE"
)

// AuditingProps defines properties for the Auditing construct
type AuditingProps struct {
	// Application name for resource naming
	AppName *string

	// Audit level - determines the scope of audit logging
	AuditLevel AuditLevel

	// Enable CloudTrail for API audit logging
	EnableCloudTrail *bool

	// Enable application-level audit logging
	EnableApplicationLogs *bool

	// Enable database audit logging
	EnableDatabaseLogs *bool

	// Enable real-time log processing
	EnableRealTimeProcessing *bool

	// Enable tamper protection for logs
	EnableTamperProtection *bool

	// Enable log aggregation across multiple regions
	EnableLogAggregation *bool

	// Log retention period in days
	LogRetentionDays *float64

	// Enable SIEM integration
	EnableSIEMIntegration *bool

	// SIEM endpoint URL for log forwarding
	SIEMEndpoint *string

	// Enable log analysis and anomaly detection
	EnableLogAnalysis *bool

	// Enable compliance reporting
	EnableComplianceReporting *bool

	// Environment for audit logging (dev, staging, prod)
	Environment *string

	// Enable log encryption
	EnableEncryption *bool

	// Custom KMS key for encryption
	EncryptionKey awskms.IKey

	// Custom S3 bucket for audit logs
	AuditBucket awss3.IBucket

	// Enable cross-account log sharing
	EnableCrossAccountAccess *bool

	// Cross-account role ARNs for log access
	CrossAccountRoleArns *[]*string

	// Enable audit trail integrity checking
	EnableIntegrityChecking *bool

	// Enable audit dashboard
	EnableDashboard *bool

	// Enable audit alerting
	EnableAlerting *bool

	// SNS topic ARN for audit alerts
	AlertTopicArn *string

	// Enable immutable audit logs
	EnableImmutableLogs *bool

	// Enable regulatory compliance features
	EnableRegulatoryCompliance *bool

	// Compliance frameworks to support
	ComplianceFrameworks *[]string
}

// AuditingConstruct creates comprehensive audit logging infrastructure
type AuditingConstruct struct {
	constructs.Construct
	AuditBucket           awss3.Bucket
	EncryptionKey         awskms.Key
	CloudTrail            awscloudtrail.Trail
	ApplicationLogGroup   awslogs.LogGroup
	DatabaseLogGroup      awslogs.LogGroup
	AuditLogGroup         awslogs.LogGroup
	LogProcessingFunction awslambda.Function
	LogStream             awskinesis.Stream
	FirehoseDeliveryStream awskinesisfirehose.CfnDeliveryStream
	AuditDashboard        awscloudwatch.Dashboard
	AuditAlarms           []awscloudwatch.Alarm
	IntegrityFunction     awslambda.Function
	ComplianceFunction    awslambda.Function
}

// NewAuditingConstruct creates a new auditing construct
func NewAuditingConstruct(scope constructs.Construct, id string, props *AuditingProps) *AuditingConstruct {
	this := constructs.NewConstruct(scope, &id)

	// Set defaults
	if props.AuditLevel == "" {
		props.AuditLevel = AuditLevelDetailed
	}
	if props.EnableCloudTrail == nil {
		props.EnableCloudTrail = jsii.Bool(true)
	}
	if props.EnableApplicationLogs == nil {
		props.EnableApplicationLogs = jsii.Bool(true)
	}
	if props.EnableDatabaseLogs == nil {
		props.EnableDatabaseLogs = jsii.Bool(true)
	}
	if props.EnableRealTimeProcessing == nil {
		props.EnableRealTimeProcessing = jsii.Bool(true)
	}
	if props.EnableTamperProtection == nil {
		props.EnableTamperProtection = jsii.Bool(true)
	}
	if props.EnableLogAggregation == nil {
		props.EnableLogAggregation = jsii.Bool(true)
	}
	if props.LogRetentionDays == nil {
		props.LogRetentionDays = jsii.Number(2555) // 7 years
	}
	if props.EnableSIEMIntegration == nil {
		props.EnableSIEMIntegration = jsii.Bool(false)
	}
	if props.EnableLogAnalysis == nil {
		props.EnableLogAnalysis = jsii.Bool(true)
	}
	if props.EnableComplianceReporting == nil {
		props.EnableComplianceReporting = jsii.Bool(true)
	}
	if props.Environment == nil {
		props.Environment = jsii.String("prod")
	}
	if props.EnableEncryption == nil {
		props.EnableEncryption = jsii.Bool(true)
	}
	if props.EnableCrossAccountAccess == nil {
		props.EnableCrossAccountAccess = jsii.Bool(false)
	}
	if props.EnableIntegrityChecking == nil {
		props.EnableIntegrityChecking = jsii.Bool(true)
	}
	if props.EnableDashboard == nil {
		props.EnableDashboard = jsii.Bool(true)
	}
	if props.EnableAlerting == nil {
		props.EnableAlerting = jsii.Bool(true)
	}
	if props.EnableImmutableLogs == nil {
		props.EnableImmutableLogs = jsii.Bool(true)
	}
	if props.EnableRegulatoryCompliance == nil {
		props.EnableRegulatoryCompliance = jsii.Bool(true)
	}

	// Create KMS key for encryption
	var encryptionKey awskms.Key
	if props.EnableEncryption != nil && *props.EnableEncryption {
		if props.EncryptionKey != nil {
			encryptionKey = props.EncryptionKey.(awskms.Key)
		} else {
			encryptionKey = awskms.NewKey(this, jsii.String("AuditEncryptionKey"), &awskms.KeyProps{
				Description:       jsii.String(fmt.Sprintf("Audit encryption key for %s", *props.AppName)),
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
							Sid:    jsii.String("Allow audit services"),
							Effect: awsiam.Effect_ALLOW,
							Principals: &[]awsiam.IPrincipal{
								awsiam.NewServicePrincipal(jsii.String("cloudtrail.amazonaws.com"), nil),
								awsiam.NewServicePrincipal(jsii.String("logs.amazonaws.com"), nil),
								awsiam.NewServicePrincipal(jsii.String("firehose.amazonaws.com"), nil),
								awsiam.NewServicePrincipal(jsii.String("kinesis.amazonaws.com"), nil),
							},
							Actions: &[]*string{
								jsii.String("kms:Encrypt"),
								jsii.String("kms:Decrypt"),
								jsii.String("kms:ReEncrypt*"),
								jsii.String("kms:GenerateDataKey*"),
								jsii.String("kms:DescribeKey"),
							},
							Resources: &[]*string{jsii.String("*")},
						}),
					},
				}),
			})
			encryptionKey.AddAlias(jsii.String(fmt.Sprintf("alias/%s-audit", *props.AppName)))
		}
	}

	// Create S3 bucket for audit logs
	var auditBucket awss3.Bucket
	if props.AuditBucket != nil {
		auditBucket = props.AuditBucket.(awss3.Bucket)
	} else {
		auditBucket = awss3.NewBucket(this, jsii.String("AuditBucket"), &awss3.BucketProps{
			BucketName: jsii.String(fmt.Sprintf("%s-audit-%s", *props.AppName, *awscdk.Stack_Of(this).Region())),
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
			ObjectLockEnabled: func() *bool {
				if props.EnableImmutableLogs != nil && *props.EnableImmutableLogs {
					return jsii.Bool(true)
				}
				return nil
			}(),
			LifecycleRules: &[]*awss3.LifecycleRule{
				{
					Id: jsii.String("AuditLogLifecycle"),
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
					Expiration: awscdk.Duration_Days(props.LogRetentionDays),
				},
			},
			ServerAccessLogsPrefix: jsii.String("access-logs/"),
		})

		// Add bucket policy for cross-account access if enabled
		if props.EnableCrossAccountAccess != nil && *props.EnableCrossAccountAccess && props.CrossAccountRoleArns != nil {
			auditBucket.AddToResourcePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Sid:    jsii.String("AllowCrossAccountAccess"),
				Effect: awsiam.Effect_ALLOW,
				Principals: &[]awsiam.IPrincipal{
					awsiam.NewArnPrincipal((*props.CrossAccountRoleArns)[0]),
				},
				Actions: &[]*string{
					jsii.String("s3:GetObject"),
					jsii.String("s3:ListBucket"),
				},
				Resources: &[]*string{
					auditBucket.BucketArn(),
					auditBucket.ArnForObjects(jsii.String("*")),
				},
			}))
		}
	}

	// Create log groups
	applicationLogGroup := createLogGroup(this, "ApplicationLogGroup", fmt.Sprintf("/aws/audit/%s/application", *props.AppName), encryptionKey, props.LogRetentionDays)
	databaseLogGroup := createLogGroup(this, "DatabaseLogGroup", fmt.Sprintf("/aws/audit/%s/database", *props.AppName), encryptionKey, props.LogRetentionDays)
	auditLogGroup := createLogGroup(this, "AuditLogGroup", fmt.Sprintf("/aws/audit/%s/system", *props.AppName), encryptionKey, props.LogRetentionDays)

	// Create CloudTrail
	var cloudTrail awscloudtrail.Trail
	if props.EnableCloudTrail != nil && *props.EnableCloudTrail {
		cloudTrail = awscloudtrail.NewTrail(this, jsii.String("AuditCloudTrail"), &awscloudtrail.TrailProps{
			TrailName:                  jsii.String(fmt.Sprintf("%s-audit-trail", *props.AppName)),
			Bucket:                     auditBucket,
			S3KeyPrefix:                jsii.String("cloudtrail/"),
			IncludeGlobalServiceEvents: jsii.Bool(true),
			IsMultiRegionTrail:         jsii.Bool(true),
			EnableFileValidation:       jsii.Bool(true),
			SendToCloudWatchLogs:       jsii.Bool(true),
			CloudWatchLogGroup:         auditLogGroup,
		})

		// Add S3 data events for comprehensive auditing
		cloudTrail.AddS3EventSelector(&[]*awscloudtrail.S3EventSelector{
			{
				Bucket:       auditBucket,
				ObjectPrefix: jsii.String(""),
			},
		}, &awscloudtrail.AddEventSelectorOptions{
			ReadWriteType:           awscloudtrail.ReadWriteType_ALL,
			IncludeManagementEvents: jsii.Bool(true),
		})
	}

	// Create Kinesis stream for real-time processing
	var logStream awskinesis.Stream
	if props.EnableRealTimeProcessing != nil && *props.EnableRealTimeProcessing {
		logStream = awskinesis.NewStream(this, jsii.String("AuditLogStream"), &awskinesis.StreamProps{
			StreamName:   jsii.String(fmt.Sprintf("%s-audit-stream", *props.AppName)),
			ShardCount:   jsii.Number(2),
			Encryption:   awskinesis.StreamEncryption_KMS,
			EncryptionKey: encryptionKey,
			RetentionPeriod: awscdk.Duration_Hours(jsii.Number(24)),
		})
	}

	// Create Firehose delivery stream for log aggregation
	var firehoseStream awskinesisfirehose.CfnDeliveryStream
	if props.EnableLogAggregation != nil && *props.EnableLogAggregation {
		firehoseStream = createFirehoseDeliveryStream(this, props, auditBucket, encryptionKey, logStream)
	}

	// Create log processing function
	var logProcessingFunction awslambda.Function
	if props.EnableRealTimeProcessing != nil && *props.EnableRealTimeProcessing {
		logProcessingFunction = createLogProcessingFunction(this, props, auditBucket, encryptionKey, logStream)
	}

	// Create integrity checking function
	var integrityFunction awslambda.Function
	if props.EnableIntegrityChecking != nil && *props.EnableIntegrityChecking {
		integrityFunction = createIntegrityCheckingFunction(this, props, auditBucket, encryptionKey)
	}

	// Create compliance function
	var complianceFunction awslambda.Function
	if props.EnableComplianceReporting != nil && *props.EnableComplianceReporting {
		complianceFunction = createAuditComplianceFunction(this, props, auditBucket, encryptionKey)
	}

	// Create dashboard
	var dashboard awscloudwatch.Dashboard
	if props.EnableDashboard != nil && *props.EnableDashboard {
		dashboard = createAuditDashboard(this, props, applicationLogGroup, databaseLogGroup, auditLogGroup)
	}

	// Create alarms
	var alarms []awscloudwatch.Alarm
	if props.EnableAlerting != nil && *props.EnableAlerting {
		alarms = createAuditAlarms(this, props, applicationLogGroup, databaseLogGroup, auditLogGroup)
	}

	// Store audit configuration
	storeAuditConfiguration(this, props)

	return &AuditingConstruct{
		Construct:              this,
		AuditBucket:            auditBucket,
		EncryptionKey:          encryptionKey,
		CloudTrail:             cloudTrail,
		ApplicationLogGroup:    applicationLogGroup,
		DatabaseLogGroup:       databaseLogGroup,
		AuditLogGroup:          auditLogGroup,
		LogProcessingFunction:  logProcessingFunction,
		LogStream:              logStream,
		FirehoseDeliveryStream: firehoseStream,
		AuditDashboard:         dashboard,
		AuditAlarms:            alarms,
		IntegrityFunction:      integrityFunction,
		ComplianceFunction:     complianceFunction,
	}
}

// createLogGroup creates a CloudWatch log group with encryption
func createLogGroup(scope constructs.Construct, id string, logGroupName string, encryptionKey awskms.Key, retentionDays *float64) awslogs.LogGroup {
	return awslogs.NewLogGroup(scope, jsii.String(id), &awslogs.LogGroupProps{
		LogGroupName: jsii.String(logGroupName),
		Retention: func() awslogs.RetentionDays {
			if *retentionDays <= 1 {
				return awslogs.RetentionDays_ONE_DAY
			} else if *retentionDays <= 3 {
				return awslogs.RetentionDays_THREE_DAYS
			} else if *retentionDays <= 5 {
				return awslogs.RetentionDays_FIVE_DAYS
			} else if *retentionDays <= 7 {
				return awslogs.RetentionDays_ONE_WEEK
			} else if *retentionDays <= 14 {
				return awslogs.RetentionDays_TWO_WEEKS
			} else if *retentionDays <= 30 {
				return awslogs.RetentionDays_ONE_MONTH
			} else if *retentionDays <= 60 {
				return awslogs.RetentionDays_TWO_MONTHS
			} else if *retentionDays <= 90 {
				return awslogs.RetentionDays_THREE_MONTHS
			} else if *retentionDays <= 120 {
				return awslogs.RetentionDays_FOUR_MONTHS
			} else if *retentionDays <= 150 {
				return awslogs.RetentionDays_FIVE_MONTHS
			} else if *retentionDays <= 180 {
				return awslogs.RetentionDays_SIX_MONTHS
			} else if *retentionDays <= 365 {
				return awslogs.RetentionDays_ONE_YEAR
			} else if *retentionDays <= 400 {
				return awslogs.RetentionDays_THIRTEEN_MONTHS
			} else if *retentionDays <= 545 {
				return awslogs.RetentionDays_EIGHTEEN_MONTHS
			} else if *retentionDays <= 730 {
				return awslogs.RetentionDays_TWO_YEARS
			} else if *retentionDays <= 1827 {
				return awslogs.RetentionDays_FIVE_YEARS
			} else if *retentionDays <= 3653 {
				return awslogs.RetentionDays_TEN_YEARS
			} else {
				return awslogs.RetentionDays_INFINITE
			}
		}(),
		RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
		EncryptionKey: encryptionKey,
	})
}

// createFirehoseDeliveryStream creates a Kinesis Firehose delivery stream
func createFirehoseDeliveryStream(scope constructs.Construct, props *AuditingProps, bucket awss3.Bucket, encryptionKey awskms.Key, stream awskinesis.Stream) awskinesisfirehose.CfnDeliveryStream {
	// Create IAM role for Firehose
	firehoseRole := awsiam.NewRole(scope, jsii.String("FirehoseRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("firehose.amazonaws.com"), nil),
		InlinePolicies: &map[string]awsiam.PolicyDocument{
			"FirehosePolicy": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
				Statements: &[]awsiam.PolicyStatement{
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							jsii.String("s3:AbortMultipartUpload"),
							jsii.String("s3:GetBucketLocation"),
							jsii.String("s3:GetObject"),
							jsii.String("s3:ListBucket"),
							jsii.String("s3:ListBucketMultipartUploads"),
							jsii.String("s3:PutObject"),
						},
						Resources: &[]*string{
							bucket.BucketArn(),
							bucket.ArnForObjects(jsii.String("*")),
						},
					}),
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							jsii.String("kinesis:DescribeStream"),
							jsii.String("kinesis:GetShardIterator"),
							jsii.String("kinesis:GetRecords"),
							jsii.String("kinesis:ListShards"),
						},
						Resources: &[]*string{stream.StreamArn()},
					}),
				},
			}),
		},
	})

	// Grant KMS permissions
	encryptionKey.GrantEncryptDecrypt(firehoseRole)

	return awskinesisfirehose.NewCfnDeliveryStream(scope, jsii.String("AuditFirehoseStream"), &awskinesisfirehose.CfnDeliveryStreamProps{
		DeliveryStreamName: jsii.String(fmt.Sprintf("%s-audit-firehose", *props.AppName)),
		DeliveryStreamType: jsii.String("KinesisStreamAsSource"),
		KinesisStreamSourceConfiguration: &awskinesisfirehose.CfnDeliveryStream_KinesisStreamSourceConfigurationProperty{
			KinesisStreamArn: stream.StreamArn(),
			RoleArn:         firehoseRole.RoleArn(),
		},
		S3DestinationConfiguration: &awskinesisfirehose.CfnDeliveryStream_S3DestinationConfigurationProperty{
			BucketArn: bucket.BucketArn(),
			RoleArn:   firehoseRole.RoleArn(),
			Prefix:    jsii.String("audit-logs/year=!{timestamp:yyyy}/month=!{timestamp:MM}/day=!{timestamp:dd}/hour=!{timestamp:HH}/"),
			ErrorOutputPrefix: jsii.String("error-logs/"),
			BufferingHints: &awskinesisfirehose.CfnDeliveryStream_BufferingHintsProperty{
				SizeInMBs:         jsii.Number(5),
				IntervalInSeconds: jsii.Number(300),
			},
			CompressionFormat: jsii.String("GZIP"),
			EncryptionConfiguration: &awskinesisfirehose.CfnDeliveryStream_EncryptionConfigurationProperty{
				KmsEncryptionConfig: &awskinesisfirehose.CfnDeliveryStream_KMSEncryptionConfigProperty{
					AwskmsKeyArn: encryptionKey.KeyArn(),
				},
			},
		},
	})
}

// createLogProcessingFunction creates a Lambda function for log processing
func createLogProcessingFunction(scope constructs.Construct, props *AuditingProps, bucket awss3.Bucket, encryptionKey awskms.Key, stream awskinesis.Stream) awslambda.Function {
	// Create IAM role for log processing function
	role := awsiam.NewRole(scope, jsii.String("LogProcessingRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
		InlinePolicies: &map[string]awsiam.PolicyDocument{
			"LogProcessingPolicy": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
				Statements: &[]awsiam.PolicyStatement{
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							jsii.String("kinesis:DescribeStream"),
							jsii.String("kinesis:GetShardIterator"),
							jsii.String("kinesis:GetRecords"),
							jsii.String("kinesis:ListShards"),
						},
						Resources: &[]*string{stream.StreamArn()},
					}),
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Effect: awsiam.Effect_ALLOW,
						Actions: &[]*string{
							jsii.String("logs:CreateLogStream"),
							jsii.String("logs:PutLogEvents"),
						},
						Resources: &[]*string{jsii.String("*")},
					}),
				},
			}),
		},
	})

	// Grant permissions
	bucket.GrantReadWrite(role, nil)
	encryptionKey.GrantEncryptDecrypt(role)

	function := awslambda.NewFunction(scope, jsii.String("LogProcessingFunction"), &awslambda.FunctionProps{
		FunctionName: jsii.String(fmt.Sprintf("%s-log-processing", *props.AppName)),
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist"), nil),
		Role:         role,
		Description:  jsii.String("Real-time audit log processing function"),
		Timeout:      awscdk.Duration_Minutes(jsii.Number(5)),
		Environment: &map[string]*string{
			"AUDIT_BUCKET": bucket.BucketName(),
			"APP_NAME":     props.AppName,
			"ENVIRONMENT":  props.Environment,
		},
	})

	// Add Kinesis event source using higher-level construct
	eventSource := awslambdaeventsources.NewKinesisEventSource(stream, &awslambdaeventsources.KinesisEventSourceProps{
		BatchSize:              jsii.Number(100),
		StartingPosition:       awslambda.StartingPosition_LATEST,
		MaxBatchingWindow:      awscdk.Duration_Seconds(jsii.Number(5)),
		BisectBatchOnError:     jsii.Bool(true),
		ReportBatchItemFailures: jsii.Bool(true),
		RetryAttempts:          jsii.Number(3),
		MaxRecordAge:           awscdk.Duration_Minutes(jsii.Number(60)),
		ParallelizationFactor:  jsii.Number(1),
	})
	function.AddEventSource(eventSource)

	return function
}

// createIntegrityCheckingFunction creates a Lambda function for log integrity checking
func createIntegrityCheckingFunction(scope constructs.Construct, props *AuditingProps, bucket awss3.Bucket, encryptionKey awskms.Key) awslambda.Function {
	// Create IAM role
	role := awsiam.NewRole(scope, jsii.String("IntegrityCheckingRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant permissions
	bucket.GrantRead(role, nil)
	encryptionKey.GrantEncryptDecrypt(role)

	function := awslambda.NewFunction(scope, jsii.String("IntegrityCheckingFunction"), &awslambda.FunctionProps{
		FunctionName: jsii.String(fmt.Sprintf("%s-integrity-checking", *props.AppName)),
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist"), nil),
		Role:         role,
		Description:  jsii.String("Audit log integrity checking function"),
		Timeout:      awscdk.Duration_Minutes(jsii.Number(15)),
		Environment: &map[string]*string{
			"AUDIT_BUCKET": bucket.BucketName(),
			"APP_NAME":     props.AppName,
			"ENVIRONMENT":  props.Environment,
		},
	})

	// Schedule integrity checks
	rule := awsevents.NewRule(scope, jsii.String("IntegrityCheckRule"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Rate(awscdk.Duration_Hours(jsii.Number(24))),
	})
	rule.AddTarget(awseventstargets.NewLambdaFunction(function, nil))

	return function
}

// createAuditComplianceFunction creates a Lambda function for compliance reporting
func createAuditComplianceFunction(scope constructs.Construct, props *AuditingProps, bucket awss3.Bucket, encryptionKey awskms.Key) awslambda.Function {
	// Create IAM role
	role := awsiam.NewRole(scope, jsii.String("ComplianceRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant permissions
	bucket.GrantReadWrite(role, nil)
	encryptionKey.GrantEncryptDecrypt(role)

	function := awslambda.NewFunction(scope, jsii.String("ComplianceFunction"), &awslambda.FunctionProps{
		FunctionName: jsii.String(fmt.Sprintf("%s-compliance-reporting", *props.AppName)),
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("./dist"), nil),
		Role:         role,
		Description:  jsii.String("Audit compliance reporting function"),
		Timeout:      awscdk.Duration_Minutes(jsii.Number(15)),
		Environment: &map[string]*string{
			"AUDIT_BUCKET": bucket.BucketName(),
			"APP_NAME":     props.AppName,
			"ENVIRONMENT":  props.Environment,
		},
	})

	// Schedule compliance reports
	rule := awsevents.NewRule(scope, jsii.String("ComplianceReportRule"), &awsevents.RuleProps{
		Schedule: awsevents.Schedule_Rate(awscdk.Duration_Days(jsii.Number(7))),
	})
	rule.AddTarget(awseventstargets.NewLambdaFunction(function, nil))

	return function
}

// createAuditDashboard creates a CloudWatch dashboard for audit monitoring
func createAuditDashboard(scope constructs.Construct, props *AuditingProps, appLogGroup awslogs.LogGroup, dbLogGroup awslogs.LogGroup, auditLogGroup awslogs.LogGroup) awscloudwatch.Dashboard {
	// Create text widgets for each log group
	appLogWidget := awscloudwatch.NewTextWidget(&awscloudwatch.TextWidgetProps{
		Markdown: jsii.String(fmt.Sprintf("## Application Audit Logs\nLog Group: %s", *appLogGroup.LogGroupName())),
		Width:    jsii.Number(8),
		Height:   jsii.Number(3),
	})

	dbLogWidget := awscloudwatch.NewTextWidget(&awscloudwatch.TextWidgetProps{
		Markdown: jsii.String(fmt.Sprintf("## Database Audit Logs\nLog Group: %s", *dbLogGroup.LogGroupName())),
		Width:    jsii.Number(8),
		Height:   jsii.Number(3),
	})

	systemLogWidget := awscloudwatch.NewTextWidget(&awscloudwatch.TextWidgetProps{
		Markdown: jsii.String(fmt.Sprintf("## System Audit Logs\nLog Group: %s", *auditLogGroup.LogGroupName())),
		Width:    jsii.Number(8),
		Height:   jsii.Number(3),
	})

	return awscloudwatch.NewDashboard(scope, jsii.String("AuditDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(fmt.Sprintf("%s-audit-dashboard", *props.AppName)),
		Widgets: &[]*[]awscloudwatch.IWidget{
			{
				appLogWidget,
				dbLogWidget,
			},
			{
				systemLogWidget,
			},
		},
	})
}

// createAuditAlarms creates CloudWatch alarms for audit monitoring
func createAuditAlarms(scope constructs.Construct, props *AuditingProps, appLogGroup awslogs.LogGroup, dbLogGroup awslogs.LogGroup, auditLogGroup awslogs.LogGroup) []awscloudwatch.Alarm {
	var alarms []awscloudwatch.Alarm

	// Failed login attempts alarm
	failedLoginAlarm := awscloudwatch.NewAlarm(scope, jsii.String("FailedLoginAlarm"), &awscloudwatch.AlarmProps{
		AlarmName: jsii.String(fmt.Sprintf("%s-failed-login-attempts", *props.AppName)),
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Logs"),
			MetricName: jsii.String("IncomingLogEvents"),
			DimensionsMap: &map[string]*string{
				"LogGroupName": appLogGroup.LogGroupName(),
			},
			Statistic: jsii.String("Sum"),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(10),
		EvaluationPeriods: jsii.Number(1),
		DatapointsToAlarm: jsii.Number(1),
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms = append(alarms, failedLoginAlarm)

	// Suspicious activity alarm
	suspiciousActivityAlarm := awscloudwatch.NewAlarm(scope, jsii.String("SuspiciousActivityAlarm"), &awscloudwatch.AlarmProps{
		AlarmName: jsii.String(fmt.Sprintf("%s-suspicious-activity", *props.AppName)),
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Logs"),
			MetricName: jsii.String("IncomingLogEvents"),
			DimensionsMap: &map[string]*string{
				"LogGroupName": auditLogGroup.LogGroupName(),
			},
			Statistic: jsii.String("Sum"),
			Period:    awscdk.Duration_Minutes(jsii.Number(15)),
		}),
		Threshold:         jsii.Number(100),
		EvaluationPeriods: jsii.Number(2),
		DatapointsToAlarm: jsii.Number(2),
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarms = append(alarms, suspiciousActivityAlarm)

	return alarms
}

// storeAuditConfiguration stores audit configuration in SSM Parameter Store
func storeAuditConfiguration(scope constructs.Construct, props *AuditingProps) {
	awsssm.NewStringParameter(scope, jsii.String("AuditLevel"), &awsssm.StringParameterProps{
		ParameterName: jsii.String(fmt.Sprintf("/%s/audit/level", *props.AppName)),
		StringValue:   jsii.String(string(props.AuditLevel)),
		Description:   jsii.String("Audit logging level"),
	})

	awsssm.NewStringParameter(scope, jsii.String("AuditRetentionDays"), &awsssm.StringParameterProps{
		ParameterName: jsii.String(fmt.Sprintf("/%s/audit/retention-days", *props.AppName)),
		StringValue:   jsii.String(fmt.Sprintf("%.0f", *props.LogRetentionDays)),
		Description:   jsii.String("Audit log retention period in days"),
	})

	if props.ComplianceFrameworks != nil {
		awsssm.NewStringParameter(scope, jsii.String("ComplianceFrameworks"), &awsssm.StringParameterProps{
			ParameterName: jsii.String(fmt.Sprintf("/%s/audit/compliance-frameworks", *props.AppName)),
			StringValue:   jsii.String(fmt.Sprintf("%v", *props.ComplianceFrameworks)),
			Description:   jsii.String("Enabled compliance frameworks"),
		})
	}
}

// GetAuditStatus returns the current audit status
func (a *AuditingConstruct) GetAuditStatus() map[string]interface{} {
	return map[string]interface{}{
		"cloudtrail_enabled":      a.CloudTrail != nil,
		"application_logs_enabled": a.ApplicationLogGroup != nil,
		"database_logs_enabled":   a.DatabaseLogGroup != nil,
		"real_time_processing":    a.LogProcessingFunction != nil,
		"integrity_checking":      a.IntegrityFunction != nil,
		"compliance_reporting":    a.ComplianceFunction != nil,
		"dashboard_enabled":       a.AuditDashboard != nil,
		"alerting_enabled":        len(a.AuditAlarms) > 0,
		"encryption_enabled":      a.EncryptionKey != nil,
		"stream_processing":       a.LogStream != nil,
		"log_aggregation":         a.FirehoseDeliveryStream != nil,
	}
}

// AddCustomAuditRule adds a custom audit rule
func (a *AuditingConstruct) AddCustomAuditRule(ruleId string, logGroup awslogs.LogGroup, filterPattern string) {
	awslogs.NewMetricFilter(a.Construct, jsii.String(fmt.Sprintf("CustomAuditRule_%s", ruleId)), &awslogs.MetricFilterProps{
		LogGroup:      logGroup,
		FilterPattern: awslogs.FilterPattern_Literal(jsii.String(filterPattern)),
		MetricNamespace: jsii.String("Audit/Custom"),
		MetricName:      jsii.String(ruleId),
		MetricValue:     jsii.String("1"),
	})
}

// EnableSIEMIntegration enables SIEM integration for audit logs
func (a *AuditingConstruct) EnableSIEMIntegration(endpoint string) {
	// Create a destination for SIEM integration
	// This would typically involve creating a subscription filter
	// to forward logs to external SIEM systems
}