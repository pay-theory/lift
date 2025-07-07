# Sprint 3: Infrastructure Completion
**Duration**: 2 weeks  
**Priority**: HIGH  
**Goal**: Complete CDK infrastructure implementations and monitoring

## Overview
This sprint focuses on replacing CDK construct stubs with real AWS resource creation, implementing proper monitoring with CloudWatch metrics, and completing security configurations. All CDK constructs must follow AWS CDK best practices and integrate with DynamORM patterns.

## Task 1: Implement Real CloudWatch Metrics

### Current State:
```go
// Returns placeholder metrics
metrics := map[string]float64{
    "requests": 100,
    "errors": 5,
}
```

### Implementation Requirements:

#### Enhanced Monitoring Construct:
```go
// pkg/cdk/constructs/monitoring_enhanced.go
package constructs

import (
    "github.com/aws/aws-cdk-go/awscdk/v2"
    "github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
    "github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
    "github.com/aws/aws-cdk-go/awscdk/v2/awssns"
)

type EnhancedMonitoringProps struct {
    // Resource to monitor
    Resource      MonitorableResource
    // Custom namespace for metrics
    Namespace     *string
    // Alert configuration
    AlertTopic    awssns.ITopic
    // Dashboard configuration
    DashboardName *string
    // Metric configuration
    MetricConfig  *MetricConfiguration
}

type MetricConfiguration struct {
    // Enable detailed metrics
    DetailedMetrics *bool
    // Custom dimensions
    Dimensions      *map[string]*string
    // Metric resolution (1 or 60 seconds)
    Resolution      *float64
    // Percentiles to track
    Percentiles     *[]*float64
}

type EnhancedMonitoring struct {
    constructs.Construct
    Metrics    map[string]awscloudwatch.IMetric
    Alarms     map[string]awscloudwatch.IAlarm
    Dashboard  awscloudwatch.Dashboard
    LogGroup   awslogs.LogGroup
}

func NewEnhancedMonitoring(scope constructs.Construct, id *string, props *EnhancedMonitoringProps) *EnhancedMonitoring {
    this := constructs.NewConstruct(scope, id)
    
    monitoring := &EnhancedMonitoring{
        Construct: this,
        Metrics:   make(map[string]awscloudwatch.IMetric),
        Alarms:    make(map[string]awscloudwatch.IAlarm),
    }
    
    // Create custom namespace
    namespace := props.Namespace
    if namespace == nil {
        namespace = jsii.String("Lift/Application")
    }
    
    // Create metrics based on resource type
    monitoring.createMetrics(props)
    
    // Create alarms
    monitoring.createAlarms(props)
    
    // Create dashboard
    monitoring.createDashboard(props)
    
    // Set up metric streams for real-time monitoring
    monitoring.createMetricStreams(props)
    
    return monitoring
}

func (m *EnhancedMonitoring) createMetrics(props *EnhancedMonitoringProps) {
    // Lambda function metrics
    if fn, ok := props.Resource.(*LiftFunction); ok {
        // Request metrics
        m.Metrics["Requests"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
            Namespace:  props.Namespace,
            MetricName: jsii.String("Requests"),
            Dimensions: &map[string]*string{
                "FunctionName": fn.Function.FunctionName(),
                "Environment":  jsii.String(getEnvironment()),
            },
            Statistic: jsii.String("Sum"),
            Period:    awscdk.Duration_Minutes(jsii.Number(1)),
        })
        
        // Error metrics with detail
        m.Metrics["Errors"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
            Namespace:  props.Namespace,
            MetricName: jsii.String("Errors"),
            Dimensions: &map[string]*string{
                "FunctionName": fn.Function.FunctionName(),
                "ErrorType":    jsii.String("$ErrorType"), // Will be set by metric filter
            },
        })
        
        // Latency percentiles
        percentiles := props.MetricConfig.Percentiles
        if percentiles == nil {
            percentiles = &[]*float64{
                jsii.Number(50),
                jsii.Number(95),
                jsii.Number(99),
            }
        }
        
        for _, p := range *percentiles {
            m.Metrics[fmt.Sprintf("LatencyP%v", *p)] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
                Namespace:  props.Namespace,
                MetricName: jsii.String("Latency"),
                Dimensions: &map[string]*string{
                    "FunctionName": fn.Function.FunctionName(),
                },
                Statistic: jsii.String(fmt.Sprintf("p%v", *p)),
            })
        }
        
        // Cold start metrics
        m.createColdStartMetrics(fn, props)
        
        // Concurrent executions
        m.Metrics["ConcurrentExecutions"] = fn.Function.MetricConcurrentExecutions()
        
        // Throttles
        m.Metrics["Throttles"] = fn.Function.MetricThrottles()
    }
    
    // DynamoDB table metrics
    if table, ok := props.Resource.(*DynamORMTable); ok {
        m.createDynamoDBMetrics(table, props)
    }
    
    // API Gateway metrics
    if api, ok := props.Resource.(*LiftAPI); ok {
        m.createAPIMetrics(api, props)
    }
}

func (m *EnhancedMonitoring) createColdStartMetrics(fn *LiftFunction, props *EnhancedMonitoringProps) {
    // Create log metric filter for cold starts
    coldStartFilter := awslogs.NewMetricFilter(m.Construct, jsii.String("ColdStartFilter"), &awslogs.MetricFilterProps{
        LogGroup:       fn.LogGroup,
        MetricNamespace: props.Namespace,
        MetricName:     jsii.String("ColdStarts"),
        FilterPattern:  awslogs.FilterPattern_Literal(jsii.String("[REPORT RequestId *] INIT_START")),
        MetricValue:    jsii.String("1"),
        DefaultValue:   jsii.Number(0),
    })
    
    // Cold start duration
    durationFilter := awslogs.NewMetricFilter(m.Construct, jsii.String("ColdStartDurationFilter"), &awslogs.MetricFilterProps{
        LogGroup:       fn.LogGroup,
        MetricNamespace: props.Namespace,
        MetricName:     jsii.String("ColdStartDuration"),
        FilterPattern:  awslogs.FilterPattern_SpaceDelimited(jsii.String("REPORT"), jsii.String("RequestId"), jsii.String("Duration:"), jsii.String("$duration")),
        MetricValue:    jsii.String("$duration"),
    })
    
    m.Metrics["ColdStarts"] = coldStartFilter.Metric()
    m.Metrics["ColdStartDuration"] = durationFilter.Metric()
}

func (m *EnhancedMonitoring) createDynamoDBMetrics(table *DynamORMTable, props *EnhancedMonitoringProps) {
    // Consumed capacity
    m.Metrics["ConsumedReadCapacity"] = table.Table.MetricConsumedReadCapacityUnits()
    m.Metrics["ConsumedWriteCapacity"] = table.Table.MetricConsumedWriteCapacityUnits()
    
    // Throttled requests
    m.Metrics["ReadThrottles"] = table.Table.MetricUserErrors(&awscloudwatch.MetricOptions{
        Dimensions: &map[string]*string{
            "TableName": table.Table.TableName(),
            "ErrorType": jsii.String("UserErrors"),
        },
    })
    
    // System errors
    m.Metrics["SystemErrors"] = table.Table.MetricSystemErrorsForOperations(&awscloudwatch.OperationsMetricOptions{
        Operations: &[]awsdynamodb.Operation{
            awsdynamodb.Operation_GET_ITEM,
            awsdynamodb.Operation_PUT_ITEM,
            awsdynamodb.Operation_QUERY,
            awsdynamodb.Operation_SCAN,
        },
    })
    
    // Latency by operation
    operations := []string{"GetItem", "PutItem", "Query", "Scan"}
    for _, op := range operations {
        m.Metrics[fmt.Sprintf("%sLatency", op)] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
            Namespace:  jsii.String("AWS/DynamoDB"),
            MetricName: jsii.String("SuccessfulRequestLatency"),
            Dimensions: &map[string]*string{
                "TableName": table.Table.TableName(),
                "Operation": jsii.String(op),
            },
            Statistic: jsii.String("Average"),
        })
    }
}

func (m *EnhancedMonitoring) createAlarms(props *EnhancedMonitoringProps) {
    // Error rate alarm
    if errorMetric, ok := m.Metrics["Errors"]; ok {
        m.Alarms["HighErrorRate"] = awscloudwatch.NewAlarm(m.Construct, jsii.String("HighErrorRate"), &awscloudwatch.AlarmProps{
            Metric:            errorMetric,
            Threshold:         jsii.Number(10),
            EvaluationPeriods: jsii.Number(2),
            TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING,
            AlarmDescription:  jsii.String("Error rate is too high"),
        })
        
        if props.AlertTopic != nil {
            m.Alarms["HighErrorRate"].AddAlarmAction(awscloudwatch.NewSnsAction(props.AlertTopic))
        }
    }
    
    // Latency alarm (p99)
    if latencyMetric, ok := m.Metrics["LatencyP99"]; ok {
        m.Alarms["HighLatency"] = awscloudwatch.NewAlarm(m.Construct, jsii.String("HighLatency"), &awscloudwatch.AlarmProps{
            Metric:            latencyMetric,
            Threshold:         jsii.Number(1000), // 1 second
            EvaluationPeriods: jsii.Number(3),
            AlarmDescription:  jsii.String("P99 latency is above 1 second"),
        })
    }
    
    // Throttling alarm
    if throttleMetric, ok := m.Metrics["Throttles"]; ok {
        m.Alarms["Throttling"] = awscloudwatch.NewAlarm(m.Construct, jsii.String("Throttling"), &awscloudwatch.AlarmProps{
            Metric:            throttleMetric,
            Threshold:         jsii.Number(1),
            EvaluationPeriods: jsii.Number(1),
            AlarmDescription:  jsii.String("Function is being throttled"),
        })
    }
}

func (m *EnhancedMonitoring) createDashboard(props *EnhancedMonitoringProps) {
    dashboardName := props.DashboardName
    if dashboardName == nil {
        dashboardName = jsii.String(fmt.Sprintf("%s-Dashboard", *awscdk.Stack_Of(m.Construct).StackName()))
    }
    
    m.Dashboard = awscloudwatch.NewDashboard(m.Construct, jsii.String("Dashboard"), &awscloudwatch.DashboardProps{
        DashboardName: dashboardName,
        Widgets: &[][]awscloudwatch.IWidget{
            // Row 1: Request metrics
            {
                awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
                    Title:  jsii.String("Request Rate"),
                    Left:   &[]awscloudwatch.IMetric{m.Metrics["Requests"]},
                    Width:  jsii.Number(12),
                    Height: jsii.Number(6),
                }),
                awscloudwatch.NewSingleValueWidget(&awscloudwatch.SingleValueWidgetProps{
                    Title:   jsii.String("Error Rate"),
                    Metrics: &[]awscloudwatch.IMetric{m.Metrics["Errors"]},
                    Width:   jsii.Number(6),
                    Height:  jsii.Number(6),
                }),
                awscloudwatch.NewGaugeWidget(&awscloudwatch.GaugeWidgetProps{
                    Title:   jsii.String("Success Rate"),
                    Metrics: &[]awscloudwatch.IMetric{m.createSuccessRateMetric()},
                    LeftYAxis: &awscloudwatch.YAxisProps{
                        Min: jsii.Number(0),
                        Max: jsii.Number(100),
                    },
                    Width:  jsii.Number(6),
                    Height: jsii.Number(6),
                }),
            },
            // Row 2: Latency metrics
            {
                awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
                    Title: jsii.String("Latency Percentiles"),
                    Left: &[]awscloudwatch.IMetric{
                        m.Metrics["LatencyP50"],
                        m.Metrics["LatencyP95"],
                        m.Metrics["LatencyP99"],
                    },
                    Width:  jsii.Number(12),
                    Height: jsii.Number(6),
                }),
                awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
                    Title:  jsii.String("Cold Starts"),
                    Left:   &[]awscloudwatch.IMetric{m.Metrics["ColdStarts"]},
                    Right:  &[]awscloudwatch.IMetric{m.Metrics["ColdStartDuration"]},
                    Width:  jsii.Number(12),
                    Height: jsii.Number(6),
                }),
            },
            // Row 3: Alarms
            {
                awscloudwatch.NewAlarmWidget(&awscloudwatch.AlarmWidgetProps{
                    Title: jsii.String("Active Alarms"),
                    Alarms: &[]awscloudwatch.IAlarm{
                        m.Alarms["HighErrorRate"],
                        m.Alarms["HighLatency"],
                        m.Alarms["Throttling"],
                    },
                    Width:  jsii.Number(24),
                    Height: jsii.Number(4),
                }),
            },
        },
    })
}

// Helper to create success rate metric
func (m *EnhancedMonitoring) createSuccessRateMetric() awscloudwatch.IMetric {
    return awscloudwatch.NewMathExpression(&awscloudwatch.MathExpressionProps{
        Expression: jsii.String("100 * (requests - errors) / requests"),
        UsingMetrics: &map[string]awscloudwatch.IMetric{
            "requests": m.Metrics["Requests"],
            "errors":   m.Metrics["Errors"],
        },
        Label: jsii.String("Success Rate (%)"),
    })
}
```

## Task 2: Complete Security Group Configuration

### Implementation Requirements:

```go
// pkg/cdk/constructs/security_enhanced.go
package constructs

type EnhancedSecurityProps struct {
    // VPC configuration
    Vpc           awsec2.IVpc
    // Allowed ingress rules
    IngressRules  []SecurityRule
    // Allowed egress rules
    EgressRules   []SecurityRule
    // WAF configuration
    EnableWAF     *bool
    // Secrets to create
    Secrets       []SecretConfig
}

type SecurityRule struct {
    Port        float64
    Protocol    awsec2.Protocol
    Source      awsec2.IPeer
    Description string
}

type EnhancedSecurity struct {
    constructs.Construct
    SecurityGroup awsec2.SecurityGroup
    WAF          awswafv2.CfnWebACL
    Secrets      map[string]awssecretsmanager.Secret
}

func NewEnhancedSecurity(scope constructs.Construct, id *string, props *EnhancedSecurityProps) *EnhancedSecurity {
    this := constructs.NewConstruct(scope, id)
    
    security := &EnhancedSecurity{
        Construct: this,
        Secrets:   make(map[string]awssecretsmanager.Secret),
    }
    
    // Create security group with least privilege
    security.createSecurityGroup(props)
    
    // Configure WAF if enabled
    if props.EnableWAF != nil && *props.EnableWAF {
        security.configureWAF(props)
    }
    
    // Create secrets
    security.createSecrets(props)
    
    // Set up security monitoring
    security.configureSecurityMonitoring()
    
    return security
}

func (s *EnhancedSecurity) createSecurityGroup(props *EnhancedSecurityProps) {
    s.SecurityGroup = awsec2.NewSecurityGroup(s.Construct, jsii.String("SecurityGroup"), &awsec2.SecurityGroupProps{
        Vpc:               props.Vpc,
        Description:       jsii.String("Security group for Lift application"),
        AllowAllOutbound: jsii.Bool(false), // Explicit egress rules only
    })
    
    // Add ingress rules
    for _, rule := range props.IngressRules {
        s.SecurityGroup.AddIngressRule(
            rule.Source,
            awsec2.Port_Tcp(jsii.Number(rule.Port)),
            jsii.String(rule.Description),
            jsii.Bool(false),
        )
    }
    
    // Add egress rules (least privilege)
    for _, rule := range props.EgressRules {
        s.SecurityGroup.AddEgressRule(
            rule.Source,
            awsec2.Port_Tcp(jsii.Number(rule.Port)),
            jsii.String(rule.Description),
            jsii.Bool(false),
        )
    }
    
    // Always allow HTTPS to AWS services
    s.SecurityGroup.AddEgressRule(
        awsec2.Peer_AnyIpv4(),
        awsec2.Port_Tcp(jsii.Number(443)),
        jsii.String("Allow HTTPS to AWS services"),
        jsii.Bool(false),
    )
}

func (s *EnhancedSecurity) configureWAF(props *EnhancedSecurityProps) {
    // Create WAF rules
    rules := []awswafv2.CfnWebACL_RuleProperty{}
    
    // Rate limiting rule
    rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
        Name:     jsii.String("RateLimitRule"),
        Priority: jsii.Number(1),
        Statement: &awswafv2.CfnWebACL_StatementProperty{
            RateBasedStatement: &awswafv2.CfnWebACL_RateBasedStatementProperty{
                Limit:              jsii.Number(2000),
                AggregateKeyType:   jsii.String("IP"),
            },
        },
        Action: &awswafv2.CfnWebACL_RuleActionProperty{
            Block: &awswafv2.CfnWebACL_BlockActionProperty{
                CustomResponse: &awswafv2.CfnWebACL_CustomResponseProperty{
                    ResponseCode: jsii.Number(429),
                    CustomResponseBodyKey: jsii.String("RateLimitExceeded"),
                },
            },
        },
        VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
            SampledRequestsEnabled:   jsii.Bool(true),
            CloudWatchMetricsEnabled: jsii.Bool(true),
            MetricName:              jsii.String("RateLimitRule"),
        },
    })
    
    // SQL injection protection
    rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
        Name:     jsii.String("SQLiProtection"),
        Priority: jsii.Number(2),
        Statement: &awswafv2.CfnWebACL_StatementProperty{
            ManagedRuleGroupStatement: &awswafv2.CfnWebACL_ManagedRuleGroupStatementProperty{
                VendorName: jsii.String("AWS"),
                Name:       jsii.String("AWSManagedRulesSQLiRuleSet"),
            },
        },
        OverrideAction: &awswafv2.CfnWebACL_OverrideActionProperty{
            None: &struct{}{},
        },
        VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
            SampledRequestsEnabled:   jsii.Bool(true),
            CloudWatchMetricsEnabled: jsii.Bool(true),
            MetricName:              jsii.String("SQLiProtection"),
        },
    })
    
    // Known bad inputs
    rules = append(rules, awswafv2.CfnWebACL_RuleProperty{
        Name:     jsii.String("KnownBadInputs"),
        Priority: jsii.Number(3),
        Statement: &awswafv2.CfnWebACL_StatementProperty{
            ManagedRuleGroupStatement: &awswafv2.CfnWebACL_ManagedRuleGroupStatementProperty{
                VendorName: jsii.String("AWS"),
                Name:       jsii.String("AWSManagedRulesKnownBadInputsRuleSet"),
            },
        },
        OverrideAction: &awswafv2.CfnWebACL_OverrideActionProperty{
            None: &struct{}{},
        },
        VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
            SampledRequestsEnabled:   jsii.Bool(true),
            CloudWatchMetricsEnabled: jsii.Bool(true),
            MetricName:              jsii.String("KnownBadInputs"),
        },
    })
    
    // Create custom response bodies
    customResponseBodies := map[string]awswafv2.CfnWebACL_CustomResponseBodyProperty{
        "RateLimitExceeded": {
            ContentType: jsii.String("APPLICATION_JSON"),
            Content:     jsii.String(`{"error": "rate_limit_exceeded", "message": "Too many requests"}`),
        },
    }
    
    // Create WAF
    s.WAF = awswafv2.NewCfnWebACL(s.Construct, jsii.String("WebACL"), &awswafv2.CfnWebACLProps{
        Scope:               jsii.String("REGIONAL"),
        DefaultAction:       &awswafv2.CfnWebACL_DefaultActionProperty{Allow: &struct{}{}},
        Rules:               &rules,
        CustomResponseBodies: customResponseBodies,
        VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
            SampledRequestsEnabled:   jsii.Bool(true),
            CloudWatchMetricsEnabled: jsii.Bool(true),
            MetricName:              jsii.String("LiftWAF"),
        },
    })
}

func (s *EnhancedSecurity) createSecrets(props *EnhancedSecurityProps) {
    for _, secretConfig := range props.Secrets {
        secret := awssecretsmanager.NewSecret(s.Construct, jsii.String(secretConfig.Name), &awssecretsmanager.SecretProps{
            Description: jsii.String(secretConfig.Description),
            GenerateSecretString: &awssecretsmanager.SecretStringGenerator{
                SecretStringTemplate: jsii.String(secretConfig.Template),
                GenerateStringKey:    jsii.String(secretConfig.GenerateKey),
                ExcludeCharacters:    jsii.String(secretConfig.ExcludeChars),
                PasswordLength:       jsii.Number(secretConfig.Length),
            },
            RemovalPolicy: awscdk.RemovalPolicy_RETAIN,
        })
        
        // Enable rotation if configured
        if secretConfig.EnableRotation {
            secret.AddRotationSchedule(jsii.String("RotationSchedule"), &awssecretsmanager.RotationScheduleProps{
                AutomaticallyAfter: awscdk.Duration_Days(jsii.Number(30)),
            })
        }
        
        s.Secrets[secretConfig.Name] = secret
    }
}
```

## Task 3: Implement Service Discovery

### Implementation Requirements:

```go
// pkg/cdk/patterns/microservice_complete.go
package patterns

type MicroserviceStackProps struct {
    ServiceName       *string
    CodePath         *string
    EnableServiceMesh *bool
    ServiceDiscovery  *ServiceDiscoveryConfig
    LoadBalancer      *LoadBalancerConfig
    AutoScaling       *AutoScalingConfig
}

type ServiceDiscoveryConfig struct {
    Namespace    *string
    ServiceName  *string
    HealthCheck  *HealthCheckConfig
}

func NewMicroserviceStack(scope constructs.Construct, id *string, props *MicroserviceStackProps) awscdk.Stack {
    stack := awscdk.NewStack(scope, id, &props.StackProps)
    
    // Create VPC with proper networking
    vpc := awsec2.NewVpc(stack, jsii.String("VPC"), &awsec2.VpcProps{
        MaxAzs:           jsii.Number(3),
        NatGateways:      jsii.Number(1),
        SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
            {
                Name:       jsii.String("Public"),
                SubnetType: awsec2.SubnetType_PUBLIC,
                CidrMask:   jsii.Number(24),
            },
            {
                Name:       jsii.String("Private"),
                SubnetType: awsec2.SubnetType_PRIVATE_WITH_NAT,
                CidrMask:   jsii.Number(24),
            },
        },
    })
    
    // Create service discovery namespace
    namespace := awsservicediscovery.NewPrivateDnsNamespace(stack, jsii.String("Namespace"), &awsservicediscovery.PrivateDnsNamespaceProps{
        Name: props.ServiceDiscovery.Namespace,
        Vpc:  vpc,
    })
    
    // Create ECS cluster
    cluster := awsecs.NewCluster(stack, jsii.String("Cluster"), &awsecs.ClusterProps{
        Vpc:                    vpc,
        ContainerInsights:      jsii.Bool(true),
        DefaultCloudMapNamespace: namespace,
    })
    
    // Create task definition
    taskDef := awsecs.NewFargateTaskDefinition(stack, jsii.String("TaskDef"), &awsecs.FargateTaskDefinitionProps{
        MemoryLimitMiB: jsii.Number(512),
        Cpu:           jsii.Number(256),
    })
    
    // Add container with proper configuration
    container := taskDef.AddContainer(jsii.String("Container"), &awsecs.ContainerDefinitionOptions{
        Image: awsecs.ContainerImage_FromAsset(props.CodePath, &awsecs.AssetImageProps{
            Platform: awsecs.Platform_LINUX_ARM64,
        }),
        Logging: awsecs.LogDrivers_AwsLogs(&awsecs.AwsLogDriverProps{
            StreamPrefix: jsii.String(*props.ServiceName),
            LogRetention: awslogs.RetentionDays_ONE_WEEK,
        }),
        Environment: &map[string]*string{
            "SERVICE_NAME": props.ServiceName,
            "NAMESPACE":    props.ServiceDiscovery.Namespace,
        },
        HealthCheck: &awsecs.HealthCheck{
            Command:     &[]*string{jsii.String("CMD-SHELL"), jsii.String("curl -f http://localhost:8080/health || exit 1")},
            Interval:    awscdk.Duration_Seconds(jsii.Number(30)),
            Timeout:     awscdk.Duration_Seconds(jsii.Number(5)),
            Retries:     jsii.Number(3),
            StartPeriod: awscdk.Duration_Seconds(jsii.Number(60)),
        },
    })
    
    // Create service with service discovery
    service := awsecs.NewFargateService(stack, jsii.String("Service"), &awsecs.FargateServiceProps{
        Cluster:              cluster,
        TaskDefinition:       taskDef,
        DesiredCount:        jsii.Number(2),
        AssignPublicIp:      jsii.Bool(false),
        CloudMapOptions: &awsecs.CloudMapOptions{
            Name:              props.ServiceDiscovery.ServiceName,
            DnsRecordType:     awsservicediscovery.DnsRecordType_A,
            DnsTtl:            awscdk.Duration_Seconds(jsii.Number(10)),
            FailureThreshold:  jsii.Number(2),
        },
        CircuitBreaker: &awsecs.DeploymentCircuitBreaker{
            Rollback: jsii.Bool(true),
        },
        EnableLogging: jsii.Bool(true),
    })
    
    // Configure auto-scaling
    scaling := service.AutoScaleTaskCount(&awsapplicationautoscaling.EnableScalingProps{
        MinCapacity: jsii.Number(2),
        MaxCapacity: jsii.Number(10),
    })
    
    scaling.ScaleOnCpuUtilization(jsii.String("CpuScaling"), &awsecs.CpuUtilizationScalingProps{
        TargetUtilizationPercent: jsii.Number(70),
        ScaleInCooldown:         awscdk.Duration_Seconds(jsii.Number(60)),
        ScaleOutCooldown:        awscdk.Duration_Seconds(jsii.Number(60)),
    })
    
    scaling.ScaleOnMemoryUtilization(jsii.String("MemoryScaling"), &awsecs.MemoryUtilizationScalingProps{
        TargetUtilizationPercent: jsii.Number(80),
    })
    
    // Create Application Load Balancer if configured
    if props.LoadBalancer != nil && *props.LoadBalancer.Enabled {
        alb := awselasticloadbalancingv2.NewApplicationLoadBalancer(stack, jsii.String("ALB"), &awselasticloadbalancingv2.ApplicationLoadBalancerProps{
            Vpc:              vpc,
            InternetFacing:   jsii.Bool(true),
            Http2Enabled:     jsii.Bool(true),
            IdleTimeout:      awscdk.Duration_Seconds(jsii.Number(30)),
        })
        
        // Add listener
        listener := alb.AddListener(jsii.String("Listener"), &awselasticloadbalancingv2.BaseApplicationListenerProps{
            Port:     jsii.Number(443),
            Protocol: awselasticloadbalancingv2.ApplicationProtocol_HTTPS,
            Certificates: &[]awselasticloadbalancingv2.IListenerCertificate{
                awselasticloadbalancingv2.ListenerCertificate_FromCertificateManager(props.LoadBalancer.Certificate),
            },
            DefaultAction: awselasticloadbalancingv2.ListenerAction_FixedResponse(jsii.Number(404), &awselasticloadbalancingv2.FixedResponseOptions{
                ContentType: jsii.String("text/plain"),
                MessageBody: jsii.String("Not Found"),
            }),
        })
        
        // Add target group
        targetGroup := listener.AddTargets(jsii.String("ServiceTarget"), &awselasticloadbalancingv2.AddApplicationTargetsProps{
            Port:                jsii.Number(8080),
            Protocol:            awselasticloadbalancingv2.ApplicationProtocol_HTTP,
            Targets:             &[]awselasticloadbalancingv2.IApplicationLoadBalancerTarget{service},
            HealthCheck: &awselasticloadbalancingv2.HealthCheck{
                Path:                jsii.String("/health"),
                HealthyHttpCodes:    jsii.String("200"),
                Interval:            awscdk.Duration_Seconds(jsii.Number(30)),
                Timeout:             awscdk.Duration_Seconds(jsii.Number(5)),
                HealthyThresholdCount: jsii.Number(2),
                UnhealthyThresholdCount: jsii.Number(3),
            },
            DeregistrationDelay: awscdk.Duration_Seconds(jsii.Number(30)),
        })
    }
    
    // Output service discovery endpoint
    awscdk.NewCfnOutput(stack, jsii.String("ServiceEndpoint"), &awscdk.CfnOutputProps{
        Value: jsii.String(fmt.Sprintf("%s.%s", *props.ServiceDiscovery.ServiceName, *namespace.NamespaceName())),
        Description: jsii.String("Service discovery endpoint"),
    })
    
    return stack
}
```

## Task 4: Complete Compliance Features

### Implementation Requirements:

```go
// pkg/compliance/gdpr_complete.go
package compliance

import (
    "github.com/pay-theory/lift/pkg/dynamorm"
    "github.com/pay-theory/lift/pkg/models"
)

type GDPRService struct {
    db              *dynamorm.DynamORMWrapper
    encryptionKey   string
    auditLogger     AuditLogger
}

// Data deletion implementation
func (g *GDPRService) DeleteUserData(ctx context.Context, userID string) error {
    // Start audit trail
    auditID := g.auditLogger.StartOperation(ctx, "GDPR_DELETE", userID)
    defer g.auditLogger.CompleteOperation(ctx, auditID)
    
    // Get all tables that might contain user data
    tables := g.getUserDataTables()
    
    // Delete from each table
    for _, table := range tables {
        if err := g.deleteFromTable(ctx, table, userID); err != nil {
            g.auditLogger.LogError(ctx, auditID, "Failed to delete from table", map[string]interface{}{
                "table": table,
                "error": err.Error(),
            })
            return err
        }
    }
    
    // Delete from S3 if applicable
    if err := g.deleteUserFiles(ctx, userID); err != nil {
        return err
    }
    
    // Create deletion record for compliance
    deletionRecord := &models.DataDeletionRecord{
        UserID:      userID,
        DeletedAt:   time.Now(),
        DeletedBy:   g.getCurrentUser(ctx),
        TablesCleared: tables,
        Status:      "completed",
    }
    
    if err := g.db.Create(deletionRecord); err != nil {
        return err
    }
    
    return nil
}

// Data export implementation
func (g *GDPRService) ExportUserData(ctx context.Context, userID string) (*UserDataExport, error) {
    export := &UserDataExport{
        UserID:      userID,
        RequestedAt: time.Now(),
        Data:        make(map[string]interface{}),
    }
    
    // Collect from all tables
    tables := g.getUserDataTables()
    for _, table := range tables {
        data, err := g.collectFromTable(ctx, table, userID)
        if err != nil {
            return nil, err
        }
        export.Data[table] = data
    }
    
    // Encrypt the export
    encrypted, err := g.encryptExport(export)
    if err != nil {
        return nil, err
    }
    
    // Store encrypted export with expiration
    exportRecord := &models.DataExportRecord{
        UserID:      userID,
        ExportID:    uuid.New().String(),
        EncryptedData: encrypted,
        ExpiresAt:   time.Now().Add(7 * 24 * time.Hour), // 7 days
        Status:      "ready",
    }
    
    if err := g.db.Create(exportRecord); err != nil {
        return nil, err
    }
    
    export.ExportID = exportRecord.ExportID
    return export, nil
}

// Consent management
func (g *GDPRService) UpdateConsent(ctx context.Context, userID string, consent ConsentUpdate) error {
    // Validate consent categories
    if err := g.validateConsentCategories(consent); err != nil {
        return err
    }
    
    // Create consent record with versioning
    consentRecord := &models.ConsentRecord{
        UserID:      userID,
        Version:     g.getNextConsentVersion(ctx, userID),
        Consents:    consent.Categories,
        IPAddress:   g.getClientIP(ctx),
        UserAgent:   g.getUserAgent(ctx),
        Timestamp:   time.Now(),
        LegalBasis:  consent.LegalBasis,
    }
    
    if err := g.db.Create(consentRecord); err != nil {
        return err
    }
    
    // Update processing based on consent
    return g.updateProcessingRules(ctx, userID, consent)
}

// SOC2 compliance logging
type SOC2Logger struct {
    db *dynamorm.DynamORMWrapper
}

func (s *SOC2Logger) LogAccessControl(ctx context.Context, event AccessControlEvent) error {
    record := &models.AccessControlLog{
        EventID:      uuid.New().String(),
        Timestamp:    time.Now(),
        UserID:       event.UserID,
        Resource:     event.Resource,
        Action:       event.Action,
        Result:       event.Result,
        IPAddress:    event.IPAddress,
        SessionID:    event.SessionID,
        // SOC2 specific fields
        RiskScore:    s.calculateRiskScore(event),
        Anomalous:    s.detectAnomaly(event),
        RetentionDate: time.Now().Add(7 * 365 * 24 * time.Hour), // 7 year retention
    }
    
    return s.db.Create(record)
}

func (s *SOC2Logger) LogDataModification(ctx context.Context, event DataModificationEvent) error {
    record := &models.DataModificationLog{
        EventID:      uuid.New().String(),
        Timestamp:    time.Now(),
        UserID:       event.UserID,
        TableName:    event.TableName,
        RecordID:     event.RecordID,
        Operation:    event.Operation,
        OldValues:    s.redactSensitive(event.OldValues),
        NewValues:    s.redactSensitive(event.NewValues),
        ChangeReason: event.ChangeReason,
        ApprovedBy:   event.ApprovedBy,
        // Cryptographic proof of integrity
        Hash:         s.calculateHash(event),
        PreviousHash: s.getPreviousHash(ctx, event.TableName),
    }
    
    return s.db.Create(record)
}
```

## Testing Requirements

### Integration Tests:
```go
func TestCloudWatchMetrics(t *testing.T) {
    // Deploy test stack
    app := awscdk.NewApp(nil)
    stack := NewTestStack(app, "TestStack")
    
    // Create monitoring
    monitoring := constructs.NewEnhancedMonitoring(stack, jsii.String("Monitoring"), &constructs.EnhancedMonitoringProps{
        Resource: testFunction,
    })
    
    // Verify metrics are created
    assert.NotNil(t, monitoring.Metrics["Requests"])
    assert.NotNil(t, monitoring.Metrics["Errors"])
    assert.NotNil(t, monitoring.Dashboard)
    
    // Test alarm configuration
    assert.Equal(t, 10.0, *monitoring.Alarms["HighErrorRate"].Threshold)
}

func TestServiceDiscovery(t *testing.T) {
    // Create service with discovery
    service := createTestService()
    
    // Verify DNS registration
    endpoint := service.CloudMapService.ServiceName()
    assert.Contains(t, *endpoint, "test-service")
    
    // Test health checks
    health := service.HealthCheck()
    assert.Equal(t, "/health", *health.Path)
}
```

### Security Scanning:
- Run AWS Security Hub checks
- Validate least privilege IAM policies
- Scan for exposed secrets
- Verify encryption at rest and in transit

## Success Criteria

1. **All metrics** show real data from AWS CloudWatch
2. **Dashboards** update in real-time with actual metrics
3. **Security groups** follow least privilege principle
4. **WAF rules** block malicious traffic
5. **Service discovery** enables inter-service communication
6. **Compliance features** meet GDPR and SOC2 requirements
7. **All CDK stacks** deploy successfully without errors