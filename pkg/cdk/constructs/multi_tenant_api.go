package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2authorizers"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awswafv2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// MultiTenantAPIProps defines properties for MultiTenantAPI
type MultiTenantAPIProps struct {
	// API name
	APIName *string

	// Lambda function code
	Code awslambda.Code

	// Lambda function handler
	Handler *string

	// Lambda function runtime
	Runtime awslambda.Runtime

	// Lambda function memory size
	MemorySize *float64

	// Lambda function timeout
	Timeout awscdk.Duration

	// Existing DynamoDB table (optional - will create one if not provided)
	Table awsdynamodb.ITable

	// Table name for new table (used if Table is not provided)
	TableName *string

	// Cognito user pool for authentication
	UserPool awscognito.IUserPool

	// Optional user pool client
	UserPoolClient awscognito.IUserPoolClient

	// Enable per-tenant rate limiting
	EnableTenantRateLimiting *bool

	// Rate limit per tenant (requests per 5 minutes)
	TenantRateLimit *float64

	// Enable tenant isolation via JWT claims
	EnableJWTTenantIsolation *bool

	// JWT claim for tenant ID
	TenantIDClaim *string

	// Enable header-based tenant isolation
	EnableHeaderTenantIsolation *bool

	// Header name for tenant ID
	TenantIDHeader *string

	// Enable path-based tenant isolation
	EnablePathTenantIsolation *bool

	// Enable WAF protection
	EnableWAF *bool

	// Enable detailed CloudWatch logging
	EnableDetailedLogging *bool

	// Enable request/response logging
	EnableAccessLogging *bool

	// Custom domain configuration
	CustomDomain *CustomDomainConfig

	// CORS configuration
	CorsConfig *CorsConfig

	// Throttling configuration
	ThrottleConfig *ThrottleConfig

	// Environment variables to pass to Lambda
	Environment map[string]*string
}

// CustomDomainConfig defines custom domain settings
type CustomDomainConfig struct {
	DomainName    string
	CertificateArn string
	BasePath      string
}

// CorsConfig defines CORS settings
type CorsConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	MaxAge           awscdk.Duration
	AllowCredentials bool
}

// ThrottleConfig defines throttling settings
type ThrottleConfig struct {
	RateLimit  float64
	BurstLimit float64
}

// MultiTenantAPI creates an API Gateway with multi-tenant support
type MultiTenantAPI struct {
	constructs.Construct
	API            awsapigatewayv2.HttpApi
	Function       *LiftFunction
	Table          awsdynamodb.ITable
	RateLimitTable awsdynamodb.ITable
	WebACL         awswafv2.CfnWebACL
}

// NewMultiTenantAPI creates a new multi-tenant API construct
func NewMultiTenantAPI(scope constructs.Construct, id string, props *MultiTenantAPIProps) *MultiTenantAPI {
	this := constructs.NewConstruct(scope, &id)

	// Set defaults
	if props.TenantRateLimit == nil {
		props.TenantRateLimit = jsii.Number(1000)
	}
	if props.TenantIDClaim == nil {
		props.TenantIDClaim = jsii.String("custom:tenant_id")
	}
	if props.TenantIDHeader == nil {
		props.TenantIDHeader = jsii.String("X-Tenant-ID")
	}

	// Use existing table or create a DynamORM-compatible one
	var table awsdynamodb.ITable
	if props.Table != nil {
		table = props.Table
	} else {
		// Create a DynamORM-compatible multi-tenant table
		tableName := props.TableName
		if tableName == nil {
			tableName = jsii.String(fmt.Sprintf("%s-table", *props.APIName))
		}
		
		liftTable := NewLiftTable(this, jsii.String("Table"), &LiftTableProps{
			TableName:         tableName,
			EnableMultiTenant: jsii.Bool(true),
			EnableStreams:     jsii.Bool(true),
			EnablePointInTimeRecovery: jsii.Bool(true),
		})
		table = liftTable.Table
	}

	// Create rate limit table if enabled
	var rateLimitTable awsdynamodb.ITable
	if props.EnableTenantRateLimiting != nil && *props.EnableTenantRateLimiting {
		rateLimitTableConstruct := NewLiftTable(this, jsii.String("RateLimitTable"), &LiftTableProps{
			TableName:           jsii.String(fmt.Sprintf("%s-rate-limits", *props.APIName)),
			TimeToLiveAttribute: jsii.String("ttl"),
		})
		rateLimitTable = rateLimitTableConstruct.Table
	}

	// Create the Lambda function with multi-tenant configuration
	functionEnv := make(map[string]*string)
	if props.Environment != nil {
		for k, v := range props.Environment {
			functionEnv[k] = v
		}
	}
	
	// Add tenant-specific environment variables
	functionEnv["DYNAMODB_TABLE_NAME"] = table.TableName()
	functionEnv["TENANT_ISOLATION_MODE"] = jsii.String(getTenantIsolationMode(props))
	functionEnv["LIFT_MULTI_TENANT"] = jsii.String("true")
	if props.EnableJWTTenantIsolation != nil && *props.EnableJWTTenantIsolation {
		functionEnv["TENANT_ID_CLAIM"] = props.TenantIDClaim
	}
	if props.EnableHeaderTenantIsolation != nil && *props.EnableHeaderTenantIsolation {
		functionEnv["TENANT_ID_HEADER"] = props.TenantIDHeader
	}
	if props.EnableTenantRateLimiting != nil && *props.EnableTenantRateLimiting {
		functionEnv["RATE_LIMIT_TABLE_NAME"] = rateLimitTable.TableName()
		functionEnv["TENANT_RATE_LIMIT"] = jsii.String(fmt.Sprintf("%.0f", *props.TenantRateLimit))
	}

	// Create the Lambda function
	liftFunction := NewLiftFunction(this, jsii.String("Function"), &LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String(fmt.Sprintf("%s-function", *props.APIName)),
			Description:  jsii.String(fmt.Sprintf("Multi-tenant API function for %s", *props.APIName)),
			Code:         props.Code,
			Handler:      props.Handler,
			Runtime:      props.Runtime,
			MemorySize:   props.MemorySize,
			Timeout:      props.Timeout,
			Environment:  &functionEnv,
		},
		EnableTracing:     jsii.Bool(true),
		EnableMultiTenant: jsii.Bool(true),
	})

	// Grant permissions
	table.GrantReadWriteData(liftFunction.Function)
	if rateLimitTable != nil {
		rateLimitTable.GrantReadWriteData(liftFunction.Function)
	}

	// Create authorizer
	var authorizer awsapigatewayv2.IHttpRouteAuthorizer
	if props.UserPool != nil {
		// Create user pool client if not provided
		userPoolClient := props.UserPoolClient
		if userPoolClient == nil {
			userPoolClient = props.UserPool.AddClient(jsii.String(fmt.Sprintf("%sAPIClient", *props.APIName)), &awscognito.UserPoolClientOptions{
				UserPoolClientName: jsii.String(fmt.Sprintf("%s-api-client", *props.APIName)),
				GenerateSecret:     jsii.Bool(false),
				AuthFlows: &awscognito.AuthFlow{
					UserPassword: jsii.Bool(true),
					UserSrp:      jsii.Bool(true),
				},
			})
		}

		authorizer = awsapigatewayv2authorizers.NewHttpJwtAuthorizer(
			jsii.String("JwtAuthorizer"),
			jsii.String(fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", 
				*awscdk.Stack_Of(this).Region(), 
				*props.UserPool.UserPoolId())),
			&awsapigatewayv2authorizers.HttpJwtAuthorizerProps{
				JwtAudience: &[]*string{userPoolClient.UserPoolClientId()},
			},
		)
	}

	// Create HTTP API
	var corsConfig *awsapigatewayv2.CorsPreflightOptions
	if props.CorsConfig != nil {
		// Convert string methods to CorsHttpMethod
		var corsMethods *[]awsapigatewayv2.CorsHttpMethod
		if len(props.CorsConfig.AllowMethods) > 0 {
			methods := make([]awsapigatewayv2.CorsHttpMethod, 0)
			for _, method := range props.CorsConfig.AllowMethods {
				switch method {
				case "GET":
					methods = append(methods, awsapigatewayv2.CorsHttpMethod_GET)
				case "POST":
					methods = append(methods, awsapigatewayv2.CorsHttpMethod_POST)
				case "PUT":
					methods = append(methods, awsapigatewayv2.CorsHttpMethod_PUT)
				case "DELETE":
					methods = append(methods, awsapigatewayv2.CorsHttpMethod_DELETE)
				case "OPTIONS":
					methods = append(methods, awsapigatewayv2.CorsHttpMethod_OPTIONS)
				case "HEAD":
					methods = append(methods, awsapigatewayv2.CorsHttpMethod_HEAD)
				case "PATCH":
					methods = append(methods, awsapigatewayv2.CorsHttpMethod_PATCH)
				}
			}
			corsMethods = &methods
		}
		
		corsConfig = &awsapigatewayv2.CorsPreflightOptions{
			AllowOrigins:     jsii.Strings(props.CorsConfig.AllowOrigins...),
			AllowMethods:     corsMethods,
			AllowHeaders:     jsii.Strings(props.CorsConfig.AllowHeaders...),
			ExposeHeaders:    jsii.Strings(props.CorsConfig.ExposeHeaders...),
			MaxAge:           props.CorsConfig.MaxAge,
			AllowCredentials: jsii.Bool(props.CorsConfig.AllowCredentials),
		}
	}

	api := awsapigatewayv2.NewHttpApi(this, jsii.String("API"), &awsapigatewayv2.HttpApiProps{
		ApiName:      props.APIName,
		CorsPreflight: corsConfig,
		DefaultAuthorizer: authorizer,
		DisableExecuteApiEndpoint: func() *bool {
			if props.CustomDomain != nil {
				return jsii.Bool(true)
			}
			return jsii.Bool(false)
		}(),
	})

	// Configure throttling
	if props.ThrottleConfig != nil {
		stage := api.DefaultStage().Node().DefaultChild().(awsapigatewayv2.CfnStage)
		stage.AddPropertyOverride(jsii.String("ThrottleSettings"), map[string]interface{}{
			"RateLimit":  props.ThrottleConfig.RateLimit,
			"BurstLimit": props.ThrottleConfig.BurstLimit,
		})
	}

	// Enable access logging
	if props.EnableAccessLogging != nil && *props.EnableAccessLogging {
		logGroup := awslogs.NewLogGroup(this, jsii.String("AccessLogs"), &awslogs.LogGroupProps{
			LogGroupName:  jsii.String(fmt.Sprintf("/aws/apigateway/%s/access", *props.APIName)),
			Retention:     awslogs.RetentionDays_ONE_MONTH,
			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		})

		stage := api.DefaultStage().Node().DefaultChild().(awsapigatewayv2.CfnStage)
		stage.SetAccessLogSettings(&awsapigatewayv2.CfnStage_AccessLogSettingsProperty{
			DestinationArn: logGroup.LogGroupArn(),
			Format: jsii.String(`$context.requestId $context.identity.sourceIp $context.requestTime $context.httpMethod $context.path $context.status $context.error.message $context.error.responseType $context.authorizer.claims.sub $context.authorizer.claims['custom:tenant_id']`),
		})
	}

	// Configure custom domain
	if props.CustomDomain != nil {
		domain := awsapigatewayv2.NewDomainName(this, jsii.String("Domain"), &awsapigatewayv2.DomainNameProps{
			DomainName: jsii.String(props.CustomDomain.DomainName),
			Certificate: awscertificatemanager.Certificate_FromCertificateArn(
				this,
				jsii.String("Certificate"),
				jsii.String(props.CustomDomain.CertificateArn),
			),
		})

		awsapigatewayv2.NewApiMapping(this, jsii.String("Mapping"), &awsapigatewayv2.ApiMappingProps{
			Api:        api,
			DomainName: domain,
			ApiMappingKey: jsii.String(props.CustomDomain.BasePath),
		})
	}

	// Create Lambda integration
	integration := awsapigatewayv2integrations.NewHttpLambdaIntegration(
		jsii.String("LambdaIntegration"),
		liftFunction.Function,
		&awsapigatewayv2integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: awsapigatewayv2.PayloadFormatVersion_VERSION_2_0(),
		},
	)

	// Add routes based on tenant isolation mode
	if props.EnablePathTenantIsolation != nil && *props.EnablePathTenantIsolation {
		// Path-based routing: /tenants/{tenantId}/*
		api.AddRoutes(&awsapigatewayv2.AddRoutesOptions{
			Path:        jsii.String("/tenants/{tenantId}/{proxy+}"),
			Methods:     &[]awsapigatewayv2.HttpMethod{awsapigatewayv2.HttpMethod_ANY},
			Integration: integration,
			Authorizer:  authorizer,
		})
	} else {
		// Standard routing with tenant ID in JWT or header
		api.AddRoutes(&awsapigatewayv2.AddRoutesOptions{
			Path:        jsii.String("/{proxy+}"),
			Methods:     &[]awsapigatewayv2.HttpMethod{awsapigatewayv2.HttpMethod_ANY},
			Integration: integration,
			Authorizer:  authorizer,
		})
	}

	// Create WAF if enabled
	var webACL awswafv2.CfnWebACL
	if props.EnableWAF != nil && *props.EnableWAF {
		webACL = createWAF(this, props)
		
		// Associate WAF with API
		awswafv2.NewCfnWebACLAssociation(this, jsii.String("WAFAssociation"), &awswafv2.CfnWebACLAssociationProps{
			ResourceArn: jsii.String(fmt.Sprintf("arn:aws:apigatewayv2:%s:%s:apis/%s/stages/%s",
				*awscdk.Stack_Of(this).Region(),
				*awscdk.Stack_Of(this).Account(),
				*api.ApiId(),
				*api.DefaultStage().StageName(),
			)),
			WebAclArn: webACL.AttrArn(),
		})
	}

	// Enable detailed CloudWatch logging
	if props.EnableDetailedLogging != nil && *props.EnableDetailedLogging {
		api.DefaultStage().Node().DefaultChild().(awsapigatewayv2.CfnStage).SetDefaultRouteSettings(&awsapigatewayv2.CfnStage_RouteSettingsProperty{
			LoggingLevel:          jsii.String("INFO"),
			DetailedMetricsEnabled: jsii.Bool(true),
			DataTraceEnabled:      jsii.Bool(true),
		})
	}

	return &MultiTenantAPI{
		Construct:      this,
		API:            api,
		Function:       liftFunction,
		Table:          table,
		RateLimitTable: rateLimitTable,
		WebACL:         webACL,
	}
}

// Helper function to determine tenant isolation mode
func getTenantIsolationMode(props *MultiTenantAPIProps) string {
	modes := []string{}
	if props.EnableJWTTenantIsolation != nil && *props.EnableJWTTenantIsolation {
		modes = append(modes, "jwt")
	}
	if props.EnableHeaderTenantIsolation != nil && *props.EnableHeaderTenantIsolation {
		modes = append(modes, "header")
	}
	if props.EnablePathTenantIsolation != nil && *props.EnablePathTenantIsolation {
		modes = append(modes, "path")
	}
	if len(modes) == 0 {
		return "none"
	}
	return modes[0] // Return first enabled mode
}

// createWAF creates a WAF WebACL with multi-tenant rules
func createWAF(scope constructs.Construct, props *MultiTenantAPIProps) awswafv2.CfnWebACL {
	return awswafv2.NewCfnWebACL(scope, jsii.String("WAF"), &awswafv2.CfnWebACLProps{
		Scope: jsii.String("REGIONAL"),
		DefaultAction: &awswafv2.CfnWebACL_DefaultActionProperty{
			Allow: &awswafv2.CfnWebACL_AllowActionProperty{},
		},
		Name: jsii.String(fmt.Sprintf("%s-waf", *props.APIName)),
		Rules: &[]interface{}{
			// Rate limiting rule
			&awswafv2.CfnWebACL_RuleProperty{
				Name:     jsii.String("RateLimitRule"),
				Priority: jsii.Number(1),
				Statement: &awswafv2.CfnWebACL_StatementProperty{
					RateBasedStatement: &awswafv2.CfnWebACL_RateBasedStatementProperty{
						Limit:              jsii.Number(2000),
						AggregateKeyType:    jsii.String("IP"),
					},
				},
				Action: &awswafv2.CfnWebACL_RuleActionProperty{
					Block: &awswafv2.CfnWebACL_BlockActionProperty{},
				},
				VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
					SampledRequestsEnabled:   jsii.Bool(true),
					CloudWatchMetricsEnabled: jsii.Bool(true),
					MetricName:               jsii.String("RateLimitRule"),
				},
			},
			// SQL injection protection
			&awswafv2.CfnWebACL_RuleProperty{
				Name:     jsii.String("SQLiRule"),
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
					MetricName:               jsii.String("SQLiRule"),
				},
			},
			// XSS protection
			&awswafv2.CfnWebACL_RuleProperty{
				Name:     jsii.String("XSSRule"),
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
					MetricName:               jsii.String("XSSRule"),
				},
			},
		},
		VisibilityConfig: &awswafv2.CfnWebACL_VisibilityConfigProperty{
			SampledRequestsEnabled:   jsii.Bool(true),
			CloudWatchMetricsEnabled: jsii.Bool(true),
			MetricName:               jsii.String(fmt.Sprintf("%s-waf", *props.APIName)),
		},
	})
}

// GrantInvoke grants invoke permissions to the API
func (m *MultiTenantAPI) GrantInvoke(grantee awsiam.IGrantable) awsiam.Grant {
	return awsiam.Grant_AddToPrincipal(&awsiam.GrantOnPrincipalOptions{
		Grantee: grantee,
		Actions: &[]*string{jsii.String("execute-api:Invoke")},
		ResourceArns: &[]*string{m.API.ArnForExecuteApi(jsii.String("*"), jsii.String("*"), jsii.String("*"))},
	})
}

// AddTenantMetrics adds CloudWatch metrics for tenant usage
func (m *MultiTenantAPI) AddTenantMetrics(tenantId string) {
	// Implementation would add CloudWatch custom metrics
	// This is a placeholder for tenant-specific metrics
}