package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awswafv2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// SecurityRule defines a network security rule
type SecurityRule struct {
	Port        float64
	Protocol    awsec2.Protocol
	Source      awsec2.IPeer
	Description string
	RuleAction  string // "allow" or "deny"
}

// SecretConfig defines configuration for secrets
type SecretConfig struct {
	Name             string
	Description      string
	Template         string
	GenerateKey      string
	ExcludeChars     string
	Length           float64
	EnableRotation   bool
	RotationLambda   awslambda.IFunction
	RotationSchedule *awssecretsmanager.RotationScheduleOptions
}

// WAFRuleConfig defines WAF rule configuration
type WAFRuleConfig struct {
	EnableRateLimit      *bool
	RateLimit            *float64
	EnableSQLiProtection *bool
	EnableXSSProtection  *bool
	EnableKnownBadInputs *bool
	CustomRules          *[]WAFCustomRule
	IPWhitelist          *[]*string
	IPBlacklist          *[]*string
	GeoBlocking          *[]string
}

// WAFCustomRule defines a custom WAF rule
type WAFCustomRule struct {
	Name        string
	Priority    float64
	Statement   string
	Action      string
	Description string
}

// EnhancedSecurityProps defines properties for enhanced security
type EnhancedSecurityProps struct {
	// VPC configuration
	Vpc awsec2.IVpc
	// Allowed ingress rules
	IngressRules []SecurityRule
	// Allowed egress rules
	EgressRules []SecurityRule
	// WAF configuration
	EnableWAF *bool
	WAFConfig *WAFRuleConfig
	// Secrets to create
	Secrets []SecretConfig
	// Enable VPC Flow Logs
	EnableVPCFlowLogs *bool
	// Enable GuardDuty integration
	EnableGuardDuty *bool
	// Enable Security Hub integration
	EnableSecurityHub *bool
	// Enable Config rules
	EnableConfigRules *bool
	// Environment tag
	Environment *string
	// Application name
	ApplicationName *string
}

// EnhancedSecurity provides comprehensive security features
type EnhancedSecurity struct {
	constructs.Construct
	SecurityGroup    awsec2.SecurityGroup
	WAF              awswafv2.CfnWebACL
	Secrets          map[string]awssecretsmanager.Secret
	VPCFlowLogsGroup awslogs.LogGroup
	SecurityMetrics  map[string]awscloudwatch.IMetric
	VPCEndpoints     map[string]awsec2.InterfaceVpcEndpoint
}

// NewEnhancedSecurity creates a comprehensive security construct
func NewEnhancedSecurity(scope constructs.Construct, id *string, props *EnhancedSecurityProps) *EnhancedSecurity {
	this := constructs.NewConstruct(scope, id)

	security := &EnhancedSecurity{
		Construct:       this,
		Secrets:         make(map[string]awssecretsmanager.Secret),
		SecurityMetrics: make(map[string]awscloudwatch.IMetric),
		VPCEndpoints:    make(map[string]awsec2.InterfaceVpcEndpoint),
	}

	// Set defaults
	security.setDefaults(props)

	// Create security group with least privilege
	security.createSecurityGroup(props)

	// Configure WAF if enabled
	if props.EnableWAF != nil && *props.EnableWAF {
		security.configureWAF(props)
	}

	// Create secrets
	security.createSecrets(props)

	// Set up VPC endpoints for AWS services
	security.createVPCEndpoints(props)

	// Enable VPC Flow Logs if requested
	if props.EnableVPCFlowLogs != nil && *props.EnableVPCFlowLogs {
		security.enableVPCFlowLogs(props)
	}

	// Set up security monitoring
	security.configureSecurityMonitoring(props)

	return security
}

func (s *EnhancedSecurity) setDefaults(props *EnhancedSecurityProps) {
	if props.EnableWAF == nil {
		props.EnableWAF = jsii.Bool(true)
	}
	if props.EnableVPCFlowLogs == nil {
		props.EnableVPCFlowLogs = jsii.Bool(true)
	}
	if props.EnableGuardDuty == nil {
		props.EnableGuardDuty = jsii.Bool(true)
	}
	if props.Environment == nil {
		props.Environment = jsii.String("production")
	}
	if props.ApplicationName == nil {
		props.ApplicationName = jsii.String("lift-app")
	}
	if props.WAFConfig == nil {
		props.WAFConfig = &WAFRuleConfig{
			EnableRateLimit:      jsii.Bool(true),
			RateLimit:            jsii.Number(2000),
			EnableSQLiProtection: jsii.Bool(true),
			EnableXSSProtection:  jsii.Bool(true),
			EnableKnownBadInputs: jsii.Bool(true),
		}
	}
}

func (s *EnhancedSecurity) createSecurityGroup(props *EnhancedSecurityProps) {
	s.SecurityGroup = awsec2.NewSecurityGroup(s.Construct, jsii.String("SecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:                props.Vpc,
		Description:        jsii.String(fmt.Sprintf("Security group for %s", *props.ApplicationName)),
		AllowAllOutbound:   jsii.Bool(false), // Explicit egress rules only
		DisableInlineRules: jsii.Bool(true),  // Force explicit rule creation
	})

	// Add ingress rules with least privilege
	for i, rule := range props.IngressRules {
		ruleId := fmt.Sprintf("IngressRule%d", i)
		s.SecurityGroup.AddIngressRule(
			rule.Source,
			s.getPort(rule.Port, rule.Protocol),
			jsii.String(rule.Description),
			jsii.Bool(false),
		)

		// Create security metric for this rule
		s.createSecurityRuleMetric(ruleId, "ingress", rule)
	}

	// Add egress rules with least privilege
	for i, rule := range props.EgressRules {
		ruleId := fmt.Sprintf("EgressRule%d", i)
		s.SecurityGroup.AddEgressRule(
			rule.Source,
			s.getPort(rule.Port, rule.Protocol),
			jsii.String(rule.Description),
			jsii.Bool(false),
		)

		// Create security metric for this rule
		s.createSecurityRuleMetric(ruleId, "egress", rule)
	}

	// Always allow HTTPS to AWS services (required for Lambda)
	s.SecurityGroup.AddEgressRule(
		awsec2.Peer_AnyIpv4(),
		awsec2.Port_Tcp(jsii.Number(443)),
		jsii.String("Allow HTTPS to AWS services"),
		jsii.Bool(false),
	)

	// Allow DNS resolution
	s.SecurityGroup.AddEgressRule(
		awsec2.Peer_AnyIpv4(),
		awsec2.Port_Udp(jsii.Number(53)),
		jsii.String("Allow DNS resolution"),
		jsii.Bool(false),
	)

	// Add tags for compliance
	awscdk.Tags_Of(s.SecurityGroup).Add(jsii.String("Environment"), props.Environment, &awscdk.TagProps{})
	awscdk.Tags_Of(s.SecurityGroup).Add(jsii.String("Application"), props.ApplicationName, &awscdk.TagProps{})
	awscdk.Tags_Of(s.SecurityGroup).Add(jsii.String("SecurityLevel"), jsii.String("Enhanced"), &awscdk.TagProps{})
}

func (s *EnhancedSecurity) getPort(port float64, protocol awsec2.Protocol) awsec2.Port {
	switch protocol {
	case awsec2.Protocol_TCP:
		return awsec2.Port_Tcp(jsii.Number(port))
	case awsec2.Protocol_UDP:
		return awsec2.Port_Udp(jsii.Number(port))
	case awsec2.Protocol_ALL:
		return awsec2.Port_AllTraffic()
	default:
		return awsec2.Port_Tcp(jsii.Number(port))
	}
}

func (s *EnhancedSecurity) configureWAF(props *EnhancedSecurityProps) {
	// Create WAF rules
	var rules []awswafv2.CfnWebACL_RuleProperty
	priority := float64(1)

	// Rate limiting rule
	if props.WAFConfig.EnableRateLimit != nil && *props.WAFConfig.EnableRateLimit {
		rateLimit := props.WAFConfig.RateLimit
		if rateLimit == nil {
			rateLimit = jsii.Number(2000)
		}

		rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
			Name:     jsii.String("RateLimitRule"),
			Priority: jsii.Number(priority),
			Statement: &awswafv2.CfnWebACL_StatementProperty{
				RateBasedStatement: &awswafv2.CfnWebACL_RateBasedStatementProperty{
					Limit:            rateLimit,
					AggregateKeyType: jsii.String("IP"),
				},
			},
			Action: &awswafv2.CfnWebACL_RuleActionProperty{
				Block: &awswafv2.CfnWebACL_BlockActionProperty{
					CustomResponse: &awswafv2.CfnWebACL_CustomResponseProperty{
						ResponseCode:          jsii.Number(429),
						CustomResponseBodyKey: jsii.String("RateLimitExceeded"),
					},
				},
			},
			VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
				SampledRequestsEnabled:   jsii.Bool(true),
				CloudWatchMetricsEnabled: jsii.Bool(true),
				MetricName:               jsii.String("RateLimitRule"),
			},
		})
		priority++
	}

	// SQL injection protection
	if props.WAFConfig.EnableSQLiProtection != nil && *props.WAFConfig.EnableSQLiProtection {
		rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
			Name:     jsii.String("SQLiProtection"),
			Priority: jsii.Number(priority),
			Statement: &awswafv2.CfnWebACL_StatementProperty{
				ManagedRuleGroupStatement: &awswafv2.CfnWebACL_ManagedRuleGroupStatementProperty{
					VendorName: jsii.String("AWS"),
					Name:       jsii.String("AWSManagedRulesSQLiRuleSet"),
				},
			},
			OverrideAction: &awswafv2.CfnWebACL_OverrideActionProperty{
				None: &map[string]interface{}{},
			},
			VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
				SampledRequestsEnabled:   jsii.Bool(true),
				CloudWatchMetricsEnabled: jsii.Bool(true),
				MetricName:               jsii.String("SQLiProtection"),
			},
		})
		priority++
	}

	// XSS protection
	if props.WAFConfig.EnableXSSProtection != nil && *props.WAFConfig.EnableXSSProtection {
		rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
			Name:     jsii.String("XSSProtection"),
			Priority: jsii.Number(priority),
			Statement: &awswafv2.CfnWebACL_StatementProperty{
				ManagedRuleGroupStatement: &awswafv2.CfnWebACL_ManagedRuleGroupStatementProperty{
					VendorName: jsii.String("AWS"),
					Name:       jsii.String("AWSManagedRulesCommonRuleSet"),
				},
			},
			OverrideAction: &awswafv2.CfnWebACL_OverrideActionProperty{
				None: &map[string]interface{}{},
			},
			VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
				SampledRequestsEnabled:   jsii.Bool(true),
				CloudWatchMetricsEnabled: jsii.Bool(true),
				MetricName:               jsii.String("XSSProtection"),
			},
		})
		priority++
	}

	// Known bad inputs
	if props.WAFConfig.EnableKnownBadInputs != nil && *props.WAFConfig.EnableKnownBadInputs {
		rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
			Name:     jsii.String("KnownBadInputs"),
			Priority: jsii.Number(priority),
			Statement: &awswafv2.CfnWebACL_StatementProperty{
				ManagedRuleGroupStatement: &awswafv2.CfnWebACL_ManagedRuleGroupStatementProperty{
					VendorName: jsii.String("AWS"),
					Name:       jsii.String("AWSManagedRulesKnownBadInputsRuleSet"),
				},
			},
			OverrideAction: &awswafv2.CfnWebACL_OverrideActionProperty{
				None: &map[string]interface{}{},
			},
			VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
				SampledRequestsEnabled:   jsii.Bool(true),
				CloudWatchMetricsEnabled: jsii.Bool(true),
				MetricName:               jsii.String("KnownBadInputs"),
			},
		})
		priority++
	}

	// IP whitelist rule
	if props.WAFConfig.IPWhitelist != nil && len(*props.WAFConfig.IPWhitelist) > 0 {
		rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
			Name:     jsii.String("IPWhitelist"),
			Priority: jsii.Number(priority),
			Statement: &awswafv2.CfnWebACL_StatementProperty{
				IpSetReferenceStatement: &awswafv2.CfnWebACL_IPSetReferenceStatementProperty{
					Arn: s.createIPSet("Whitelist", props.WAFConfig.IPWhitelist),
				},
			},
			Action: &awswafv2.CfnWebACL_RuleActionProperty{
				Allow: &map[string]interface{}{},
			},
			VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
				SampledRequestsEnabled:   jsii.Bool(true),
				CloudWatchMetricsEnabled: jsii.Bool(true),
				MetricName:               jsii.String("IPWhitelist"),
			},
		})
		priority++
	}

	// IP blacklist rule
	if props.WAFConfig.IPBlacklist != nil && len(*props.WAFConfig.IPBlacklist) > 0 {
		rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
			Name:     jsii.String("IPBlacklist"),
			Priority: jsii.Number(priority),
			Statement: &awswafv2.CfnWebACL_StatementProperty{
				IpSetReferenceStatement: &awswafv2.CfnWebACL_IPSetReferenceStatementProperty{
					Arn: s.createIPSet("Blacklist", props.WAFConfig.IPBlacklist),
				},
			},
			Action: &awswafv2.CfnWebACL_RuleActionProperty{
				Block: &awswafv2.CfnWebACL_BlockActionProperty{},
			},
			VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
				SampledRequestsEnabled:   jsii.Bool(true),
				CloudWatchMetricsEnabled: jsii.Bool(true),
				MetricName:               jsii.String("IPBlacklist"),
			},
		})
		priority++
	}

	// Geo blocking rule
	if props.WAFConfig.GeoBlocking != nil && len(*props.WAFConfig.GeoBlocking) > 0 {
		countryCodes := make([]*string, len(*props.WAFConfig.GeoBlocking))
		for i, country := range *props.WAFConfig.GeoBlocking {
			countryCodes[i] = jsii.String(country)
		}

		rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
			Name:     jsii.String("GeoBlocking"),
			Priority: jsii.Number(priority),
			Statement: &awswafv2.CfnWebACL_StatementProperty{
				GeoMatchStatement: &awswafv2.CfnWebACL_GeoMatchStatementProperty{
					CountryCodes: &countryCodes,
				},
			},
			Action: &awswafv2.CfnWebACL_RuleActionProperty{
				Block: &awswafv2.CfnWebACL_BlockActionProperty{},
			},
			VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
				SampledRequestsEnabled:   jsii.Bool(true),
				CloudWatchMetricsEnabled: jsii.Bool(true),
				MetricName:               jsii.String("GeoBlocking"),
			},
		})
	}

	// Custom response bodies
	customResponseBodies := map[string]awswafv2.CfnWebACL_CustomResponseBodyProperty{
		"RateLimitExceeded": {
			ContentType: jsii.String("APPLICATION_JSON"),
			Content:     jsii.String(`{"error": "rate_limit_exceeded", "message": "Too many requests", "retry_after": 60}`),
		},
		"AccessDenied": {
			ContentType: jsii.String("APPLICATION_JSON"),
			Content:     jsii.String(`{"error": "access_denied", "message": "Access denied by security policy"}`),
		},
	}

	// Create WAF Web ACL
	s.WAF = awswafv2.NewCfnWebACL(s.Construct, jsii.String("WebACL"), &awswafv2.CfnWebACLProps{
		Scope:                jsii.String("REGIONAL"),
		DefaultAction:        &awswafv2.CfnWebACL_DefaultActionProperty{Allow: &map[string]interface{}{}},
		Rules:                &rules,
		CustomResponseBodies: customResponseBodies,
		VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
			SampledRequestsEnabled:   jsii.Bool(true),
			CloudWatchMetricsEnabled: jsii.Bool(true),
			MetricName:               jsii.String(fmt.Sprintf("%sWAF", *props.ApplicationName)),
		},
		Tags: &[]*awscdk.CfnTag{
			{
				Key:   jsii.String("Environment"),
				Value: props.Environment,
			},
			{
				Key:   jsii.String("Application"),
				Value: props.ApplicationName,
			},
		},
	})

	// Create WAF logging configuration
	s.createWAFLogging(props)
}

func (s *EnhancedSecurity) createIPSet(name string, ips *[]*string) *string {
	ipSet := awswafv2.NewCfnIPSet(s.Construct, jsii.String(fmt.Sprintf("IPSet%s", name)), &awswafv2.CfnIPSetProps{
		Scope:            jsii.String("REGIONAL"),
		IpAddressVersion: jsii.String("IPV4"),
		Addresses:        ips,
		Tags: &[]*awscdk.CfnTag{
			{
				Key:   jsii.String("Name"),
				Value: jsii.String(name),
			},
		},
	})
	return ipSet.AttrArn()
}

func (s *EnhancedSecurity) createWAFLogging(props *EnhancedSecurityProps) {
	// Create log group for WAF logs
	wafLogGroup := awslogs.NewLogGroup(s.Construct, jsii.String("WAFLogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String(fmt.Sprintf("/aws/wafv2/%s", *props.ApplicationName)),
		Retention:     awslogs.RetentionDays_ONE_MONTH,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	// Create WAF logging configuration
	awswafv2.NewCfnLoggingConfiguration(s.Construct, jsii.String("WAFLogging"), &awswafv2.CfnLoggingConfigurationProps{
		ResourceArn:           s.WAF.AttrArn(),
		LogDestinationConfigs: &[]*string{wafLogGroup.LogGroupArn()},
		RedactedFields: &[]awswafv2.CfnLoggingConfiguration_FieldToMatchProperty{
			{
				SingleHeader: &awswafv2.CfnLoggingConfiguration_SingleHeaderProperty{
					Name: jsii.String("authorization"),
				},
			},
			{
				SingleHeader: &awswafv2.CfnLoggingConfiguration_SingleHeaderProperty{
					Name: jsii.String("cookie"),
				},
			},
		},
	})
}

func (s *EnhancedSecurity) createSecrets(props *EnhancedSecurityProps) {
	for _, secretConfig := range props.Secrets {
		secretProps := &awssecretsmanager.SecretProps{
			Description:   jsii.String(secretConfig.Description),
			RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
		}

		// Configure secret generation if template provided
		if secretConfig.Template != "" {
			secretProps.GenerateSecretString = &awssecretsmanager.SecretStringGenerator{
				SecretStringTemplate:    jsii.String(secretConfig.Template),
				GenerateStringKey:       jsii.String(secretConfig.GenerateKey),
				ExcludeCharacters:       jsii.String(secretConfig.ExcludeChars),
				PasswordLength:          jsii.Number(secretConfig.Length),
				ExcludePunctuation:      jsii.Bool(true),
				ExcludeNumbers:          jsii.Bool(false),
				ExcludeLowercase:        jsii.Bool(false),
				ExcludeUppercase:        jsii.Bool(false),
				RequireEachIncludedType: jsii.Bool(true),
			}
		}

		secret := awssecretsmanager.NewSecret(s.Construct, jsii.String(secretConfig.Name), secretProps)

		// Enable rotation if configured
		if secretConfig.EnableRotation {
			rotationOptions := secretConfig.RotationSchedule
			if rotationOptions == nil {
				rotationOptions = &awssecretsmanager.RotationScheduleOptions{
					AutomaticallyAfter: awscdk.Duration_Days(jsii.Number(30)),
				}
			}

			if secretConfig.RotationLambda != nil {
				rotationOptions.RotationLambda = secretConfig.RotationLambda
			}

			secret.AddRotationSchedule(jsii.String(fmt.Sprintf("%sRotation", secretConfig.Name)), rotationOptions)
		}

		// Add tags for compliance
		awscdk.Tags_Of(secret).Add(jsii.String("Environment"), props.Environment, &awscdk.TagProps{})
		awscdk.Tags_Of(secret).Add(jsii.String("Application"), props.ApplicationName, &awscdk.TagProps{})
		awscdk.Tags_Of(secret).Add(jsii.String("DataClassification"), jsii.String("Confidential"), &awscdk.TagProps{})

		s.Secrets[secretConfig.Name] = secret
	}
}

func (s *EnhancedSecurity) createVPCEndpoints(props *EnhancedSecurityProps) {
	// Secrets Manager VPC Endpoint
	s.VPCEndpoints["SecretsManager"] = awsec2.NewInterfaceVpcEndpoint(s.Construct, jsii.String("SecretsManagerEndpoint"), &awsec2.InterfaceVpcEndpointProps{
		Vpc:               props.Vpc,
		Service:           awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER(),
		SecurityGroups:    &[]awsec2.ISecurityGroup{s.SecurityGroup},
		PrivateDnsEnabled: jsii.Bool(true),
		Subnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
	})

	// CloudWatch Logs VPC Endpoint
	s.VPCEndpoints["CloudWatchLogs"] = awsec2.NewInterfaceVpcEndpoint(s.Construct, jsii.String("CloudWatchLogsEndpoint"), &awsec2.InterfaceVpcEndpointProps{
		Vpc:               props.Vpc,
		Service:           awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_LOGS(),
		SecurityGroups:    &[]awsec2.ISecurityGroup{s.SecurityGroup},
		PrivateDnsEnabled: jsii.Bool(true),
		Subnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
	})

	// X-Ray VPC Endpoint
	s.VPCEndpoints["XRay"] = awsec2.NewInterfaceVpcEndpoint(s.Construct, jsii.String("XRayEndpoint"), &awsec2.InterfaceVpcEndpointProps{
		Vpc:               props.Vpc,
		Service:           awsec2.InterfaceVpcEndpointAwsService_XRAY(),
		SecurityGroups:    &[]awsec2.ISecurityGroup{s.SecurityGroup},
		PrivateDnsEnabled: jsii.Bool(true),
		Subnets: &awsec2.SubnetSelection{
			SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
		},
	})
}

func (s *EnhancedSecurity) enableVPCFlowLogs(props *EnhancedSecurityProps) {
	// Create log group for VPC Flow Logs
	s.VPCFlowLogsGroup = awslogs.NewLogGroup(s.Construct, jsii.String("VPCFlowLogsGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String(fmt.Sprintf("/aws/vpc/flowlogs/%s", *props.ApplicationName)),
		Retention:     awslogs.RetentionDays_ONE_WEEK,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	// Create IAM role for VPC Flow Logs
	flowLogsRole := awsiam.NewRole(s.Construct, jsii.String("VPCFlowLogsRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("vpc-flow-logs.amazonaws.com"), &awsiam.ServicePrincipalOpts{}),
		InlinePolicies: &map[string]awsiam.PolicyDocument{
			"FlowLogsDeliveryRolePolicy": awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
				Statements: &[]awsiam.PolicyStatement{
					awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
						Actions: &[]*string{
							jsii.String("logs:CreateLogStream"),
							jsii.String("logs:PutLogEvents"),
							jsii.String("logs:DescribeLogGroups"),
							jsii.String("logs:DescribeLogStreams"),
						},
						Resources: &[]*string{s.VPCFlowLogsGroup.LogGroupArn()},
					}),
				},
			}),
		},
	})

	// Enable VPC Flow Logs
	awsec2.NewFlowLog(s.Construct, jsii.String("VPCFlowLogs"), &awsec2.FlowLogProps{
		ResourceType:           awsec2.FlowLogResourceType_FromVpc(props.Vpc),
		Destination:            awsec2.FlowLogDestination_ToCloudWatchLogs(s.VPCFlowLogsGroup, flowLogsRole),
		TrafficType:            awsec2.FlowLogTrafficType_ALL,
		MaxAggregationInterval: awsec2.FlowLogMaxAggregationInterval_ONE_MINUTE,
	})
}

func (s *EnhancedSecurity) configureSecurityMonitoring(props *EnhancedSecurityProps) {
	// Create custom metrics for security events
	s.SecurityMetrics["WAFBlockedRequests"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/WAFV2"),
		MetricName: jsii.String("BlockedRequests"),
		DimensionsMap: &map[string]*string{
			"WebACL": jsii.String(*props.ApplicationName + "WAF"),
			"Region": awscdk.Stack_Of(s.Construct).Region(),
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	s.SecurityMetrics["SecurityGroupChanges"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/Events"),
		MetricName: jsii.String("SecurityGroupChanges"),
		DimensionsMap: &map[string]*string{
			"Application": props.ApplicationName,
			"Environment": props.Environment,
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Create metric filters for VPC Flow Logs if enabled
	if s.VPCFlowLogsGroup != nil {
		s.createVPCFlowLogMetrics()
	}
}

func (s *EnhancedSecurity) createVPCFlowLogMetrics() {
	// Rejected connections metric
	awslogs.NewMetricFilter(s.Construct, jsii.String("RejectedConnectionsFilter"), &awslogs.MetricFilterProps{
		LogGroup:        s.VPCFlowLogsGroup,
		MetricNamespace: jsii.String("Security/VPC"),
		MetricName:      jsii.String("RejectedConnections"),
		FilterPattern:   awslogs.FilterPattern_SpaceDelimited(jsii.String("version"), jsii.String("account"), jsii.String("eni"), jsii.String("source"), jsii.String("destination"), jsii.String("srcport"), jsii.String("destport"), jsii.String("protocol"), jsii.String("packets"), jsii.String("bytes"), jsii.String("windowstart"), jsii.String("windowend"), jsii.String("action"), jsii.String("flowlogstatus")).WhereString(jsii.String("action"), jsii.String("="), jsii.String("REJECT")),
		MetricValue:     jsii.String("1"),
		DefaultValue:    jsii.Number(0),
	})

	// Suspicious port activity metric
	awslogs.NewMetricFilter(s.Construct, jsii.String("SuspiciousPortsFilter"), &awslogs.MetricFilterProps{
		LogGroup:        s.VPCFlowLogsGroup,
		MetricNamespace: jsii.String("Security/VPC"),
		MetricName:      jsii.String("SuspiciousPortActivity"),
		FilterPattern:   awslogs.FilterPattern_AnyTerm(jsii.String("destport=22"), jsii.String("destport=23"), jsii.String("destport=3389")),
		MetricValue:     jsii.String("1"),
		DefaultValue:    jsii.Number(0),
	})
}

func (s *EnhancedSecurity) createSecurityRuleMetric(ruleId, direction string, rule SecurityRule) {
	metricName := fmt.Sprintf("%s_%s_Traffic", direction, ruleId)
	s.SecurityMetrics[metricName] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("Security/NetworkRules"),
		MetricName: jsii.String(metricName),
		DimensionsMap: &map[string]*string{
			"RuleId":    jsii.String(ruleId),
			"Direction": jsii.String(direction),
			"Port":      jsii.String(fmt.Sprintf("%.0f", rule.Port)),
			"Protocol":  jsii.String(string(rule.Protocol)),
		},
		Statistic: jsii.String("Sum"),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})
}

// GetSecurityGroup returns the security group
func (s *EnhancedSecurity) GetSecurityGroup() awsec2.ISecurityGroup {
	return s.SecurityGroup
}

// GetWAF returns the WAF Web ACL
func (s *EnhancedSecurity) GetWAF() awswafv2.CfnWebACL {
	return s.WAF
}

// GetSecret returns a specific secret by name
func (s *EnhancedSecurity) GetSecret(name string) awssecretsmanager.Secret {
	return s.Secrets[name]
}

// GetVPCEndpoint returns a specific VPC endpoint by name
func (s *EnhancedSecurity) GetVPCEndpoint(name string) awsec2.InterfaceVpcEndpoint {
	return s.VPCEndpoints[name]
}

// GetSecurityMetric returns a specific security metric by name
func (s *EnhancedSecurity) GetSecurityMetric(name string) awscloudwatch.IMetric {
	return s.SecurityMetrics[name]
}

// AddCustomSecurityRule adds a custom security rule to the security group
func (s *EnhancedSecurity) AddCustomSecurityRule(rule SecurityRule, direction string) {
	if direction == "ingress" {
		s.SecurityGroup.AddIngressRule(
			rule.Source,
			s.getPort(rule.Port, rule.Protocol),
			jsii.String(rule.Description),
			jsii.Bool(false),
		)
	} else {
		s.SecurityGroup.AddEgressRule(
			rule.Source,
			s.getPort(rule.Port, rule.Protocol),
			jsii.String(rule.Description),
			jsii.Bool(false),
		)
	}
}
