package deployment

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// InfrastructureProvider defines the supported IaC providers
type InfrastructureProvider string

const (
	ProviderPulumi         InfrastructureProvider = "pulumi"
	ProviderTerraform      InfrastructureProvider = "terraform"
	ProviderCloudFormation InfrastructureProvider = "cloudformation"
	ProviderCDK            InfrastructureProvider = "cdk"
)

// InfrastructureTemplate represents a complete infrastructure template
type InfrastructureTemplate struct {
	Provider    InfrastructureProvider `json:"provider"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Resources   map[string]Resource    `json:"resources"`
	Outputs     map[string]Output      `json:"outputs"`
	Parameters  map[string]Parameter   `json:"parameters"`
	Metadata    map[string]any `json:"metadata"`
	Tags        map[string]string      `json:"tags"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Resource represents an infrastructure resource
type Resource struct {
	Type         string                 `json:"type"`
	Name         string                 `json:"name"`
	Properties   map[string]any `json:"properties"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Condition    string                 `json:"condition,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Tags         map[string]string      `json:"tags,omitempty"`
}

// Output represents a template output
type Output struct {
	Value       any `json:"value"`
	Description string      `json:"description"`
	Export      bool        `json:"export,omitempty"`
}

// Parameter represents a template parameter
type Parameter struct {
	Type          string        `json:"type"`
	Description   string        `json:"description"`
	Default       any   `json:"default,omitempty"`
	AllowedValues []any `json:"allowed_values,omitempty"`
	MinLength     *int          `json:"min_length,omitempty"`
	MaxLength     *int          `json:"max_length,omitempty"`
	Pattern       string        `json:"pattern,omitempty"`
}

// InfrastructureConfig holds configuration for infrastructure generation
type InfrastructureConfig struct {
	// Application settings
	ApplicationName string   `json:"application_name"`
	Environment     string   `json:"environment"`
	Region          string   `json:"region"`
	MultiRegion     bool     `json:"multi_region"`
	Regions         []string `json:"regions,omitempty"`

	// Lambda configuration
	Lambda LambdaConfig `json:"lambda"`

	// API Gateway configuration
	APIGateway APIGatewayConfig `json:"api_gateway"`

	// Database configuration
	Database DatabaseConfig `json:"database"`

	// Monitoring configuration
	Monitoring MonitoringConfig `json:"monitoring"`

	// Security configuration
	Security SecurityConfig `json:"security"`

	// Networking configuration
	Networking NetworkingConfig `json:"networking"`

	// Tags and metadata
	Tags     map[string]string      `json:"tags"`
	Metadata map[string]any `json:"metadata"`
}

// LambdaConfig holds Lambda function configuration
type LambdaConfig struct {
	Runtime             string            `json:"runtime"`
	Handler             string            `json:"handler"`
	Timeout             int               `json:"timeout"`
	MemorySize          int               `json:"memory_size"`
	ReservedConcurrency *int              `json:"reserved_concurrency,omitempty"`
	Environment         map[string]string `json:"environment,omitempty"`
	Layers              []string          `json:"layers,omitempty"`
	DeadLetterQueue     bool              `json:"dead_letter_queue"`
	VPCConfig           *VPCConfig        `json:"vpc_config,omitempty"`
}

// APIGatewayConfig holds API Gateway configuration
type APIGatewayConfig struct {
	Type           string               `json:"type"` // REST, HTTP, WebSocket
	StageName      string               `json:"stage_name"`
	CORS           CORSConfig           `json:"cors"`
	Authentication AuthenticationConfig `json:"authentication"`
	Throttling     ThrottlingConfig     `json:"throttling"`
	Caching        CachingConfig        `json:"caching"`
	CustomDomain   *CustomDomainConfig  `json:"custom_domain,omitempty"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type          string           `json:"type"` // DynamoDB, RDS, Aurora
	Tables        []TableConfig    `json:"tables,omitempty"`
	GlobalTables  bool             `json:"global_tables"`
	BackupEnabled bool             `json:"backup_enabled"`
	Encryption    EncryptionConfig `json:"encryption"`
	StreamEnabled bool             `json:"stream_enabled"`
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	CloudWatch CloudWatchConfig `json:"cloudwatch"`
	XRay       XRayConfig       `json:"xray"`
	Alarms     []AlarmConfig    `json:"alarms"`
	Dashboard  DashboardConfig  `json:"dashboard"`
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	IAMRoles       []IAMRoleConfig     `json:"iam_roles"`
	KMSKeys        []KMSKeyConfig      `json:"kms_keys"`
	SecretsManager SecretsConfig       `json:"secrets_manager"`
	WAF            *WAFConfig          `json:"waf,omitempty"`
	VPCEndpoints   []VPCEndpointConfig `json:"vpc_endpoints,omitempty"`
}

// NetworkingConfig holds networking configuration
type NetworkingConfig struct {
	VPC            *VPCConfig            `json:"vpc,omitempty"`
	Subnets        []SubnetConfig        `json:"subnets,omitempty"`
	SecurityGroups []SecurityGroupConfig `json:"security_groups,omitempty"`
	NATGateways    bool                  `json:"nat_gateways"`
	VPCEndpoints   []VPCEndpointConfig   `json:"vpc_endpoints,omitempty"`
}

// Supporting configuration types
type VPCConfig struct {
	CIDR               string   `json:"cidr"`
	EnableDNSSupport   bool     `json:"enable_dns_support"`
	EnableDNSHostnames bool     `json:"enable_dns_hostnames"`
	AvailabilityZones  []string `json:"availability_zones"`
}

type CORSConfig struct {
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
	ExposeHeaders    []string `json:"expose_headers,omitempty"`
	MaxAge           int      `json:"max_age,omitempty"`
	AllowCredentials bool     `json:"allow_credentials"`
}

type AuthenticationConfig struct {
	Type        string             `json:"type"` // JWT, API_KEY, IAM, COGNITO
	Authorizers []AuthorizerConfig `json:"authorizers,omitempty"`
	APIKeys     []APIKeyConfig     `json:"api_keys,omitempty"`
}

type ThrottlingConfig struct {
	BurstLimit int `json:"burst_limit"`
	RateLimit  int `json:"rate_limit"`
}

type CachingConfig struct {
	Enabled    bool   `json:"enabled"`
	TTL        int    `json:"ttl"`
	KeyPattern string `json:"key_pattern,omitempty"`
}

type CustomDomainConfig struct {
	DomainName        string `json:"domain_name"`
	CertificateArn    string `json:"certificate_arn"`
	Route53HostedZone string `json:"route53_hosted_zone,omitempty"`
}

type TableConfig struct {
	Name          string              `json:"name"`
	BillingMode   string              `json:"billing_mode"`
	HashKey       string              `json:"hash_key"`
	RangeKey      string              `json:"range_key,omitempty"`
	Attributes    []AttributeConfig   `json:"attributes"`
	GlobalIndexes []GlobalIndexConfig `json:"global_indexes,omitempty"`
	LocalIndexes  []LocalIndexConfig  `json:"local_indexes,omitempty"`
	StreamEnabled bool                `json:"stream_enabled"`
	BackupEnabled bool                `json:"backup_enabled"`
}

type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	KMSKeyId  string `json:"kms_key_id,omitempty"`
	Algorithm string `json:"algorithm,omitempty"`
}

type CloudWatchConfig struct {
	LogGroups     []LogGroupConfig     `json:"log_groups"`
	MetricFilters []MetricFilterConfig `json:"metric_filters,omitempty"`
	RetentionDays int                  `json:"retention_days"`
}

type XRayConfig struct {
	Enabled       bool    `json:"enabled"`
	SamplingRate  float64 `json:"sampling_rate"`
	TracingConfig string  `json:"tracing_config"` // Active, PassThrough
}

type AlarmConfig struct {
	Name               string   `json:"name"`
	MetricName         string   `json:"metric_name"`
	Namespace          string   `json:"namespace"`
	Statistic          string   `json:"statistic"`
	Threshold          float64  `json:"threshold"`
	ComparisonOperator string   `json:"comparison_operator"`
	EvaluationPeriods  int      `json:"evaluation_periods"`
	Period             int      `json:"period"`
	Actions            []string `json:"actions"`
}

type DashboardConfig struct {
	Name    string            `json:"name"`
	Widgets []DashboardWidget `json:"widgets"`
}

type DashboardWidget struct {
	Type       string                 `json:"type"`
	Title      string                 `json:"title"`
	Properties map[string]any `json:"properties"`
}

// Additional supporting types
type IAMRoleConfig struct {
	Name             string                 `json:"name"`
	AssumeRolePolicy map[string]any `json:"assume_role_policy"`
	Policies         []string               `json:"policies"`
	InlinePolicies   []InlinePolicyConfig   `json:"inline_policies,omitempty"`
}

type KMSKeyConfig struct {
	Alias       string                 `json:"alias"`
	Description string                 `json:"description"`
	Policy      map[string]any `json:"policy,omitempty"`
}

type SecretsConfig struct {
	Secrets []SecretConfig `json:"secrets"`
}

type SecretConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	KMSKeyId    string `json:"kms_key_id,omitempty"`
}

type WAFConfig struct {
	Name  string    `json:"name"`
	Rules []WAFRule `json:"rules"`
}

type WAFRule struct {
	Name      string                 `json:"name"`
	Priority  int                    `json:"priority"`
	Action    string                 `json:"action"`
	Statement map[string]any `json:"statement"`
}

type VPCEndpointConfig struct {
	ServiceName string   `json:"service_name"`
	Type        string   `json:"type"` // Gateway, Interface
	SubnetIds   []string `json:"subnet_ids,omitempty"`
}

type SubnetConfig struct {
	Name             string `json:"name"`
	CIDR             string `json:"cidr"`
	AvailabilityZone string `json:"availability_zone"`
	Type             string `json:"type"` // public, private
}

type SecurityGroupConfig struct {
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	IngressRules []SecurityGroupRule `json:"ingress_rules"`
	EgressRules  []SecurityGroupRule `json:"egress_rules"`
}

type SecurityGroupRule struct {
	Protocol   string   `json:"protocol"`
	FromPort   int      `json:"from_port"`
	ToPort     int      `json:"to_port"`
	CIDRBlocks []string `json:"cidr_blocks,omitempty"`
	SourceSG   string   `json:"source_sg,omitempty"`
}

type AuthorizerConfig struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URI  string `json:"uri,omitempty"`
}

type APIKeyConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type AttributeConfig struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type GlobalIndexConfig struct {
	Name       string           `json:"name"`
	HashKey    string           `json:"hash_key"`
	RangeKey   string           `json:"range_key,omitempty"`
	Projection ProjectionConfig `json:"projection"`
}

type LocalIndexConfig struct {
	Name       string           `json:"name"`
	RangeKey   string           `json:"range_key"`
	Projection ProjectionConfig `json:"projection"`
}

type ProjectionConfig struct {
	Type       string   `json:"type"` // ALL, KEYS_ONLY, INCLUDE
	Attributes []string `json:"attributes,omitempty"`
}

type LogGroupConfig struct {
	Name          string `json:"name"`
	RetentionDays int    `json:"retention_days"`
	KMSKeyId      string `json:"kms_key_id,omitempty"`
}

type MetricFilterConfig struct {
	Name            string `json:"name"`
	LogGroupName    string `json:"log_group_name"`
	FilterPattern   string `json:"filter_pattern"`
	MetricName      string `json:"metric_name"`
	MetricNamespace string `json:"metric_namespace"`
	MetricValue     string `json:"metric_value"`
}

type InlinePolicyConfig struct {
	Name   string                 `json:"name"`
	Policy map[string]any `json:"policy"`
}

// InfrastructureGenerator generates infrastructure templates
type InfrastructureGenerator struct {
	provider InfrastructureProvider
	config   InfrastructureConfig
	mu       sync.RWMutex
}

// NewInfrastructureGenerator creates a new infrastructure generator
func NewInfrastructureGenerator(provider InfrastructureProvider, config InfrastructureConfig) *InfrastructureGenerator {
	return &InfrastructureGenerator{
		provider: provider,
		config:   config,
	}
}

// GenerateTemplate generates a complete infrastructure template
func (ig *InfrastructureGenerator) GenerateTemplate() (*InfrastructureTemplate, error) {
	ig.mu.RLock()
	defer ig.mu.RUnlock()

	template := &InfrastructureTemplate{
		Provider:    ig.provider,
		Name:        fmt.Sprintf("%s-%s", ig.config.ApplicationName, ig.config.Environment),
		Version:     "1.0.0",
		Description: fmt.Sprintf("Infrastructure template for %s in %s environment", ig.config.ApplicationName, ig.config.Environment),
		Resources:   make(map[string]Resource),
		Outputs:     make(map[string]Output),
		Parameters:  make(map[string]Parameter),
		Metadata:    ig.config.Metadata,
		Tags:        ig.config.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Generate resources based on configuration
	if err := ig.generateLambdaResources(template); err != nil {
		return nil, fmt.Errorf("failed to generate Lambda resources: %w", err)
	}

	if err := ig.generateAPIGatewayResources(template); err != nil {
		return nil, fmt.Errorf("failed to generate API Gateway resources: %w", err)
	}

	if err := ig.generateDatabaseResources(template); err != nil {
		return nil, fmt.Errorf("failed to generate database resources: %w", err)
	}

	if err := ig.generateMonitoringResources(template); err != nil {
		return nil, fmt.Errorf("failed to generate monitoring resources: %w", err)
	}

	if err := ig.generateSecurityResources(template); err != nil {
		return nil, fmt.Errorf("failed to generate security resources: %w", err)
	}

	if err := ig.generateNetworkingResources(template); err != nil {
		return nil, fmt.Errorf("failed to generate networking resources: %w", err)
	}

	// Generate outputs
	ig.generateOutputs(template)

	// Generate parameters
	ig.generateParameters(template)

	return template, nil
}

// generateLambdaResources generates Lambda function resources
func (ig *InfrastructureGenerator) generateLambdaResources(template *InfrastructureTemplate) error {
	lambdaConfig := ig.config.Lambda

	// Lambda execution role
	executionRole := Resource{
		Type: "AWS::IAM::Role",
		Name: fmt.Sprintf("%s-lambda-execution-role", ig.config.ApplicationName),
		Properties: map[string]any{
			"AssumeRolePolicyDocument": map[string]any{
				"Version": "2012-10-17",
				"Statement": []map[string]any{
					{
						"Effect": "Allow",
						"Principal": map[string]any{
							"Service": "lambda.amazonaws.com",
						},
						"Action": "sts:AssumeRole",
					},
				},
			},
			"ManagedPolicyArns": []string{
				"arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
			},
		},
		Tags: ig.config.Tags,
	}

	if lambdaConfig.VPCConfig != nil {
		managedPolicies := executionRole.Properties["ManagedPolicyArns"].([]string)
		managedPolicies = append(managedPolicies, "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole")
		executionRole.Properties["ManagedPolicyArns"] = managedPolicies
	}

	template.Resources["LambdaExecutionRole"] = executionRole

	// Lambda function
	lambdaFunction := Resource{
		Type: "AWS::Lambda::Function",
		Name: fmt.Sprintf("%s-function", ig.config.ApplicationName),
		Properties: map[string]any{
			"FunctionName": fmt.Sprintf("%s-%s", ig.config.ApplicationName, ig.config.Environment),
			"Runtime":      lambdaConfig.Runtime,
			"Handler":      lambdaConfig.Handler,
			"Timeout":      lambdaConfig.Timeout,
			"MemorySize":   lambdaConfig.MemorySize,
			"Role":         fmt.Sprintf("${%s.Arn}", "LambdaExecutionRole"),
			"Code": map[string]any{
				"ZipFile": "// Placeholder code - replace with actual deployment package",
			},
		},
		Dependencies: []string{"LambdaExecutionRole"},
		Tags:         ig.config.Tags,
	}

	if lambdaConfig.Environment != nil && len(lambdaConfig.Environment) > 0 {
		lambdaFunction.Properties["Environment"] = map[string]any{
			"Variables": lambdaConfig.Environment,
		}
	}

	if lambdaConfig.ReservedConcurrency != nil {
		lambdaFunction.Properties["ReservedConcurrencyLimit"] = *lambdaConfig.ReservedConcurrency
	}

	if len(lambdaConfig.Layers) > 0 {
		lambdaFunction.Properties["Layers"] = lambdaConfig.Layers
	}

	if lambdaConfig.VPCConfig != nil {
		lambdaFunction.Properties["VpcConfig"] = map[string]any{
			"SecurityGroupIds": []string{"${SecurityGroup.GroupId}"},
			"SubnetIds":        []string{"${PrivateSubnet1.SubnetId}", "${PrivateSubnet2.SubnetId}"},
		}
		lambdaFunction.Dependencies = append(lambdaFunction.Dependencies, "SecurityGroup", "PrivateSubnet1", "PrivateSubnet2")
	}

	template.Resources["LambdaFunction"] = lambdaFunction

	// Dead Letter Queue if enabled
	if lambdaConfig.DeadLetterQueue {
		dlq := Resource{
			Type: "AWS::SQS::Queue",
			Name: fmt.Sprintf("%s-dlq", ig.config.ApplicationName),
			Properties: map[string]any{
				"QueueName":                fmt.Sprintf("%s-%s-dlq", ig.config.ApplicationName, ig.config.Environment),
				"MessageRetentionPeriod":   1209600, // 14 days
				"VisibilityTimeoutSeconds": 60,
			},
			Tags: ig.config.Tags,
		}

		template.Resources["DeadLetterQueue"] = dlq

		// Update Lambda function to use DLQ
		lambdaProps := lambdaFunction.Properties
		lambdaProps["DeadLetterConfig"] = map[string]any{
			"TargetArn": fmt.Sprintf("${%s.Arn}", "DeadLetterQueue"),
		}
		lambdaFunction.Dependencies = append(lambdaFunction.Dependencies, "DeadLetterQueue")
		template.Resources["LambdaFunction"] = lambdaFunction
	}

	return nil
}

// generateAPIGatewayResources generates API Gateway resources
func (ig *InfrastructureGenerator) generateAPIGatewayResources(template *InfrastructureTemplate) error {
	apiConfig := ig.config.APIGateway

	var apiResource Resource

	switch apiConfig.Type {
	case "HTTP":
		apiResource = Resource{
			Type: "AWS::ApiGatewayV2::Api",
			Name: fmt.Sprintf("%s-api", ig.config.ApplicationName),
			Properties: map[string]any{
				"Name":         fmt.Sprintf("%s-%s-api", ig.config.ApplicationName, ig.config.Environment),
				"ProtocolType": "HTTP",
				"Description":  fmt.Sprintf("HTTP API for %s", ig.config.ApplicationName),
			},
			Tags: ig.config.Tags,
		}

		// CORS configuration for HTTP API
		if len(apiConfig.CORS.AllowOrigins) > 0 {
			apiResource.Properties["CorsConfiguration"] = map[string]any{
				"AllowOrigins":     apiConfig.CORS.AllowOrigins,
				"AllowMethods":     apiConfig.CORS.AllowMethods,
				"AllowHeaders":     apiConfig.CORS.AllowHeaders,
				"ExposeHeaders":    apiConfig.CORS.ExposeHeaders,
				"MaxAge":           apiConfig.CORS.MaxAge,
				"AllowCredentials": apiConfig.CORS.AllowCredentials,
			}
		}

	default: // REST API
		apiResource = Resource{
			Type: "AWS::ApiGateway::RestApi",
			Name: fmt.Sprintf("%s-api", ig.config.ApplicationName),
			Properties: map[string]any{
				"Name":        fmt.Sprintf("%s-%s-api", ig.config.ApplicationName, ig.config.Environment),
				"Description": fmt.Sprintf("REST API for %s", ig.config.ApplicationName),
				"EndpointConfiguration": map[string]any{
					"Types": []string{"REGIONAL"},
				},
			},
			Tags: ig.config.Tags,
		}
	}

	template.Resources["APIGateway"] = apiResource

	// API Gateway deployment
	deployment := Resource{
		Type: "AWS::ApiGateway::Deployment",
		Name: fmt.Sprintf("%s-deployment", ig.config.ApplicationName),
		Properties: map[string]any{
			"RestApiId": fmt.Sprintf("${%s.RestApiId}", "APIGateway"),
			"StageName": apiConfig.StageName,
		},
		Dependencies: []string{"APIGateway"},
		Tags:         ig.config.Tags,
	}

	template.Resources["APIDeployment"] = deployment

	// Lambda permission for API Gateway
	lambdaPermission := Resource{
		Type: "AWS::Lambda::Permission",
		Name: fmt.Sprintf("%s-lambda-permission", ig.config.ApplicationName),
		Properties: map[string]any{
			"FunctionName": fmt.Sprintf("${%s.FunctionName}", "LambdaFunction"),
			"Action":       "lambda:InvokeFunction",
			"Principal":    "apigateway.amazonaws.com",
			"SourceArn":    fmt.Sprintf("${%s.ExecutionArn}/*/*", "APIGateway"),
		},
		Dependencies: []string{"LambdaFunction", "APIGateway"},
	}

	template.Resources["LambdaPermission"] = lambdaPermission

	return nil
}

// generateDatabaseResources generates database resources
func (ig *InfrastructureGenerator) generateDatabaseResources(template *InfrastructureTemplate) error {
	dbConfig := ig.config.Database

	if dbConfig.Type == "DynamoDB" {
		for _, tableConfig := range dbConfig.Tables {
			tableName := fmt.Sprintf("DynamoTable%s", cases.Title(language.English).String(tableConfig.Name))

			table := Resource{
				Type: "AWS::DynamoDB::Table",
				Name: fmt.Sprintf("%s-%s", ig.config.ApplicationName, tableConfig.Name),
				Properties: map[string]any{
					"TableName":            fmt.Sprintf("%s-%s-%s", ig.config.ApplicationName, ig.config.Environment, tableConfig.Name),
					"BillingMode":          tableConfig.BillingMode,
					"AttributeDefinitions": ig.generateAttributeDefinitions(tableConfig.Attributes),
					"KeySchema":            ig.generateKeySchema(tableConfig.HashKey, tableConfig.RangeKey),
				},
				Tags: ig.config.Tags,
			}

			if tableConfig.StreamEnabled {
				table.Properties["StreamSpecification"] = map[string]any{
					"StreamViewType": "NEW_AND_OLD_IMAGES",
				}
			}

			if dbConfig.Encryption.Enabled {
				table.Properties["SSESpecification"] = map[string]any{
					"SSEEnabled": true,
				}
				if dbConfig.Encryption.KMSKeyId != "" {
					table.Properties["SSESpecification"].(map[string]any)["KMSMasterKeyId"] = dbConfig.Encryption.KMSKeyId
				}
			}

			if tableConfig.BackupEnabled {
				table.Properties["PointInTimeRecoverySpecification"] = map[string]any{
					"PointInTimeRecoveryEnabled": true,
				}
			}

			// Global Secondary Indexes
			if len(tableConfig.GlobalIndexes) > 0 {
				gsis := make([]map[string]any, 0, len(tableConfig.GlobalIndexes))
				for _, gsi := range tableConfig.GlobalIndexes {
					gsiDef := map[string]any{
						"IndexName": gsi.Name,
						"KeySchema": ig.generateKeySchema(gsi.HashKey, gsi.RangeKey),
						"Projection": map[string]any{
							"ProjectionType": gsi.Projection.Type,
						},
					}

					if gsi.Projection.Type == "INCLUDE" && len(gsi.Projection.Attributes) > 0 {
						gsiDef["Projection"].(map[string]any)["NonKeyAttributes"] = gsi.Projection.Attributes
					}

					gsis = append(gsis, gsiDef)
				}
				table.Properties["GlobalSecondaryIndexes"] = gsis
			}

			template.Resources[tableName] = table
		}
	}

	return nil
}

// generateMonitoringResources generates monitoring resources
func (ig *InfrastructureGenerator) generateMonitoringResources(template *InfrastructureTemplate) error {
	monConfig := ig.config.Monitoring

	// CloudWatch Log Groups
	for _, logGroup := range monConfig.CloudWatch.LogGroups {
		logGroupName := fmt.Sprintf("LogGroup%s", cases.Title(language.English).String(strings.ReplaceAll(logGroup.Name, "-", "")))

		logGroupResource := Resource{
			Type: "AWS::Logs::LogGroup",
			Name: logGroup.Name,
			Properties: map[string]any{
				"LogGroupName":    fmt.Sprintf("/aws/lambda/%s-%s", ig.config.ApplicationName, ig.config.Environment),
				"RetentionInDays": logGroup.RetentionDays,
			},
			Tags: ig.config.Tags,
		}

		if logGroup.KMSKeyId != "" {
			logGroupResource.Properties["KmsKeyId"] = logGroup.KMSKeyId
		}

		template.Resources[logGroupName] = logGroupResource
	}

	// CloudWatch Alarms
	for _, alarm := range monConfig.Alarms {
		alarmName := fmt.Sprintf("Alarm%s", cases.Title(language.English).String(strings.ReplaceAll(alarm.Name, "-", "")))

		alarmResource := Resource{
			Type: "AWS::CloudWatch::Alarm",
			Name: alarm.Name,
			Properties: map[string]any{
				"AlarmName":          fmt.Sprintf("%s-%s-%s", ig.config.ApplicationName, ig.config.Environment, alarm.Name),
				"AlarmDescription":   fmt.Sprintf("Alarm for %s", alarm.MetricName),
				"MetricName":         alarm.MetricName,
				"Namespace":          alarm.Namespace,
				"Statistic":          alarm.Statistic,
				"Threshold":          alarm.Threshold,
				"ComparisonOperator": alarm.ComparisonOperator,
				"EvaluationPeriods":  alarm.EvaluationPeriods,
				"Period":             alarm.Period,
			},
			Tags: ig.config.Tags,
		}

		if len(alarm.Actions) > 0 {
			alarmResource.Properties["AlarmActions"] = alarm.Actions
		}

		template.Resources[alarmName] = alarmResource
	}

	return nil
}

// generateSecurityResources generates security resources
func (ig *InfrastructureGenerator) generateSecurityResources(template *InfrastructureTemplate) error {
	secConfig := ig.config.Security

	// IAM Roles
	for _, role := range secConfig.IAMRoles {
		roleName := fmt.Sprintf("IAMRole%s", cases.Title(language.English).String(strings.ReplaceAll(role.Name, "-", "")))

		roleResource := Resource{
			Type: "AWS::IAM::Role",
			Name: role.Name,
			Properties: map[string]any{
				"RoleName":                 fmt.Sprintf("%s-%s-%s", ig.config.ApplicationName, ig.config.Environment, role.Name),
				"AssumeRolePolicyDocument": role.AssumeRolePolicy,
				"ManagedPolicyArns":        role.Policies,
			},
			Tags: ig.config.Tags,
		}

		if len(role.InlinePolicies) > 0 {
			policies := make([]map[string]any, 0, len(role.InlinePolicies))
			for _, policy := range role.InlinePolicies {
				policies = append(policies, map[string]any{
					"PolicyName":     policy.Name,
					"PolicyDocument": policy.Policy,
				})
			}
			roleResource.Properties["Policies"] = policies
		}

		template.Resources[roleName] = roleResource
	}

	// KMS Keys
	for _, key := range secConfig.KMSKeys {
		keyName := fmt.Sprintf("KMSKey%s", cases.Title(language.English).String(strings.ReplaceAll(key.Alias, "-", "")))

		keyResource := Resource{
			Type: "AWS::KMS::Key",
			Name: key.Alias,
			Properties: map[string]any{
				"Description": key.Description,
				"KeyPolicy": map[string]any{
					"Version": "2012-10-17",
					"Statement": []map[string]any{
						{
							"Effect": "Allow",
							"Principal": map[string]any{
								"AWS": fmt.Sprintf("arn:aws:iam::${AWS::AccountId}:root"),
							},
							"Action":   "kms:*",
							"Resource": "*",
						},
					},
				},
			},
			Tags: ig.config.Tags,
		}

		if key.Policy != nil {
			keyResource.Properties["KeyPolicy"] = key.Policy
		}

		template.Resources[keyName] = keyResource

		// KMS Alias
		aliasResource := Resource{
			Type: "AWS::KMS::Alias",
			Name: fmt.Sprintf("%s-alias", key.Alias),
			Properties: map[string]any{
				"AliasName":   fmt.Sprintf("alias/%s-%s-%s", ig.config.ApplicationName, ig.config.Environment, key.Alias),
				"TargetKeyId": fmt.Sprintf("${%s.KeyId}", keyName),
			},
			Dependencies: []string{keyName},
		}

		template.Resources[fmt.Sprintf("%sAlias", keyName)] = aliasResource
	}

	return nil
}

// generateNetworkingResources generates networking resources
func (ig *InfrastructureGenerator) generateNetworkingResources(template *InfrastructureTemplate) error {
	netConfig := ig.config.Networking

	if netConfig.VPC != nil {
		// VPC
		vpc := Resource{
			Type: "AWS::EC2::VPC",
			Name: fmt.Sprintf("%s-vpc", ig.config.ApplicationName),
			Properties: map[string]any{
				"CidrBlock":          netConfig.VPC.CIDR,
				"EnableDnsSupport":   netConfig.VPC.EnableDNSSupport,
				"EnableDnsHostnames": netConfig.VPC.EnableDNSHostnames,
			},
			Tags: ig.config.Tags,
		}

		template.Resources["VPC"] = vpc

		// Internet Gateway
		igw := Resource{
			Type:       "AWS::EC2::InternetGateway",
			Name:       fmt.Sprintf("%s-igw", ig.config.ApplicationName),
			Properties: map[string]any{},
			Tags:       ig.config.Tags,
		}

		template.Resources["InternetGateway"] = igw

		// VPC Gateway Attachment
		vpcGwAttachment := Resource{
			Type: "AWS::EC2::VPCGatewayAttachment",
			Name: fmt.Sprintf("%s-vpc-gw-attachment", ig.config.ApplicationName),
			Properties: map[string]any{
				"VpcId":             fmt.Sprintf("${%s.VpcId}", "VPC"),
				"InternetGatewayId": fmt.Sprintf("${%s.InternetGatewayId}", "InternetGateway"),
			},
			Dependencies: []string{"VPC", "InternetGateway"},
		}

		template.Resources["VPCGatewayAttachment"] = vpcGwAttachment

		// Subnets
		for i, subnet := range netConfig.Subnets {
			subnetName := fmt.Sprintf("%sSubnet%d", cases.Title(language.English).String(subnet.Type), i+1)

			subnetResource := Resource{
				Type: "AWS::EC2::Subnet",
				Name: subnet.Name,
				Properties: map[string]any{
					"VpcId":            fmt.Sprintf("${%s.VpcId}", "VPC"),
					"CidrBlock":        subnet.CIDR,
					"AvailabilityZone": subnet.AvailabilityZone,
				},
				Dependencies: []string{"VPC"},
				Tags:         ig.config.Tags,
			}

			if subnet.Type == "public" {
				subnetResource.Properties["MapPublicIpOnLaunch"] = true
			}

			template.Resources[subnetName] = subnetResource
		}

		// Security Groups
		for _, sg := range netConfig.SecurityGroups {
			sgName := fmt.Sprintf("SecurityGroup%s", cases.Title(language.English).String(strings.ReplaceAll(sg.Name, "-", "")))

			sgResource := Resource{
				Type: "AWS::EC2::SecurityGroup",
				Name: sg.Name,
				Properties: map[string]any{
					"GroupName":        fmt.Sprintf("%s-%s-%s", ig.config.ApplicationName, ig.config.Environment, sg.Name),
					"GroupDescription": sg.Description,
					"VpcId":            fmt.Sprintf("${%s.VpcId}", "VPC"),
				},
				Dependencies: []string{"VPC"},
				Tags:         ig.config.Tags,
			}

			if len(sg.IngressRules) > 0 {
				ingressRules := make([]map[string]any, 0, len(sg.IngressRules))
				for _, rule := range sg.IngressRules {
					ingressRule := map[string]any{
						"IpProtocol": rule.Protocol,
						"FromPort":   rule.FromPort,
						"ToPort":     rule.ToPort,
					}

					if len(rule.CIDRBlocks) > 0 {
						ingressRule["CidrIp"] = rule.CIDRBlocks[0] // Simplified for single CIDR
					}

					if rule.SourceSG != "" {
						ingressRule["SourceSecurityGroupId"] = rule.SourceSG
					}

					ingressRules = append(ingressRules, ingressRule)
				}
				sgResource.Properties["SecurityGroupIngress"] = ingressRules
			}

			template.Resources[sgName] = sgResource
		}
	}

	return nil
}

// generateOutputs generates template outputs
func (ig *InfrastructureGenerator) generateOutputs(template *InfrastructureTemplate) {
	// Lambda Function ARN
	template.Outputs["LambdaFunctionArn"] = Output{
		Value:       fmt.Sprintf("${%s.Arn}", "LambdaFunction"),
		Description: "ARN of the Lambda function",
		Export:      true,
	}

	// API Gateway URL
	template.Outputs["APIGatewayURL"] = Output{
		Value:       fmt.Sprintf("https://${%s.RestApiId}.execute-api.${AWS::Region}.amazonaws.com/${%s.StageName}", "APIGateway", "APIDeployment"),
		Description: "URL of the API Gateway",
		Export:      true,
	}

	// DynamoDB Table Names
	for _, tableConfig := range ig.config.Database.Tables {
		tableName := fmt.Sprintf("DynamoTable%s", cases.Title(language.English).String(tableConfig.Name))
		outputName := fmt.Sprintf("%sTableName", cases.Title(language.English).String(tableConfig.Name))

		template.Outputs[outputName] = Output{
			Value:       fmt.Sprintf("${%s.TableName}", tableName),
			Description: fmt.Sprintf("Name of the %s DynamoDB table", tableConfig.Name),
			Export:      true,
		}
	}
}

// generateParameters generates template parameters
func (ig *InfrastructureGenerator) generateParameters(template *InfrastructureTemplate) {
	template.Parameters["Environment"] = Parameter{
		Type:          "String",
		Description:   "Environment name",
		Default:       ig.config.Environment,
		AllowedValues: []any{"dev", "staging", "prod"},
	}

	template.Parameters["ApplicationName"] = Parameter{
		Type:        "String",
		Description: "Name of the application",
		Default:     ig.config.ApplicationName,
		MinLength:   &[]int{1}[0],
		MaxLength:   &[]int{64}[0],
	}
}

// Helper functions
func (ig *InfrastructureGenerator) generateAttributeDefinitions(attributes []AttributeConfig) []map[string]any {
	definitions := make([]map[string]any, 0, len(attributes))
	for _, attr := range attributes {
		definitions = append(definitions, map[string]any{
			"AttributeName": attr.Name,
			"AttributeType": attr.Type,
		})
	}
	return definitions
}

func (ig *InfrastructureGenerator) generateKeySchema(hashKey, rangeKey string) []map[string]any {
	schema := []map[string]any{
		{
			"AttributeName": hashKey,
			"KeyType":       "HASH",
		},
	}

	if rangeKey != "" {
		schema = append(schema, map[string]any{
			"AttributeName": rangeKey,
			"KeyType":       "RANGE",
		})
	}

	return schema
}

// ExportTemplate exports the template in the specified format
func (ig *InfrastructureGenerator) ExportTemplate(template *InfrastructureTemplate, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(template, "", "  ")
	case "yaml":
		// For YAML export, you would use a YAML library
		// For now, return JSON
		return json.MarshalIndent(template, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// ValidateTemplate validates the generated template
func (ig *InfrastructureGenerator) ValidateTemplate(template *InfrastructureTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if len(template.Resources) == 0 {
		return fmt.Errorf("template must contain at least one resource")
	}

	// Validate resource dependencies
	resourceNames := make(map[string]bool)
	for name := range template.Resources {
		resourceNames[name] = true
	}

	for name, resource := range template.Resources {
		for _, dep := range resource.Dependencies {
			if !resourceNames[dep] {
				return fmt.Errorf("resource %s has invalid dependency: %s", name, dep)
			}
		}
	}

	return nil
}
