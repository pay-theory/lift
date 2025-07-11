package constructs

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// LiftAPIProps defines properties for creating a Lift API Gateway
type LiftAPIProps struct {
	// Name of the API
	Name *string
	// Description of the API
	Description *string
	// Enable CORS
	EnableCORS *bool
	// Custom domain name
	DomainName *string
	// Certificate ARN for custom domain
	CertificateArn *string
	// Enable access logging
	EnableAccessLogging *bool
	// CloudWatch log group for access logs
	AccessLogGroup awslogs.ILogGroup
	// Throttle settings
	ThrottleRateLimit  *float64
	ThrottleBurstLimit *float64
	// Stage name (defaults to $default)
	StageName *string
	// Enable detailed CloudWatch metrics
	EnableDetailedMetrics *bool
	// API Key configuration
	RequireApiKey *bool
	// Request/Response validation models
	RequestValidators map[string]*RequestValidator
	// Default authorizer for all routes
	DefaultAuthorizer awsapigatewayv2.IHttpRouteAuthorizer
}

// RequestValidator defines validation rules for API requests
type RequestValidator struct {
	// Validate request body
	ValidateBody *bool
	// Validate request parameters
	ValidateParameters *bool
	// JSON schema for body validation
	BodySchema interface{}
}

// LiftAPI is an API Gateway HTTP API construct for Lift applications
type LiftAPI struct {
	constructs.Construct
	HttpAPI  awsapigatewayv2.HttpApi
	Stage    awsapigatewayv2.IHttpStage
	LogGroup awslogs.ILogGroup
}

// GetResourceName returns the API name
func (l *LiftAPI) GetResourceName() *string {
	return l.HttpAPI.ApiId()
}

// NewLiftAPI creates a new API Gateway HTTP API optimized for Lift
func NewLiftAPI(scope constructs.Construct, id *string, props *LiftAPIProps) *LiftAPI {
	this := constructs.NewConstruct(scope, id)

	// Create log group for access logs if enabled
	var logGroup awslogs.ILogGroup
	if props.EnableAccessLogging != nil && *props.EnableAccessLogging {
		if props.AccessLogGroup != nil {
			logGroup = props.AccessLogGroup
		} else {
			logGroup = awslogs.NewLogGroup(this, jsii.String("AccessLogs"), &awslogs.LogGroupProps{
				LogGroupName:  jsii.String("/aws/apigateway/" + *props.Name),
				Retention:     awslogs.RetentionDays_ONE_WEEK,
				RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			})
		}
	}

	// Create HTTP API with Lift defaults
	apiProps := &awsapigatewayv2.HttpApiProps{
		ApiName:     props.Name,
		Description: props.Description,
	}

	// Configure CORS if enabled
	if props.EnableCORS != nil && *props.EnableCORS {
		apiProps.CorsPreflight = &awsapigatewayv2.CorsPreflightOptions{
			AllowOrigins: &[]*string{jsii.String("*")},
			AllowMethods: &[]awsapigatewayv2.CorsHttpMethod{
				awsapigatewayv2.CorsHttpMethod_GET,
				awsapigatewayv2.CorsHttpMethod_POST,
				awsapigatewayv2.CorsHttpMethod_PUT,
				awsapigatewayv2.CorsHttpMethod_DELETE,
				awsapigatewayv2.CorsHttpMethod_OPTIONS,
			},
			AllowHeaders: &[]*string{
				jsii.String("Content-Type"),
				jsii.String("Authorization"),
				jsii.String("X-Tenant-ID"),
				jsii.String("X-Request-ID"),
				jsii.String("X-Api-Key"),
			},
			ExposeHeaders: &[]*string{
				jsii.String("X-Request-ID"),
				jsii.String("X-Rate-Limit-Limit"),
				jsii.String("X-Rate-Limit-Remaining"),
				jsii.String("X-Rate-Limit-Reset"),
			},
			MaxAge: awscdk.Duration_Hours(jsii.Number(24)),
		}
	}

	// Set default authorizer if provided
	if props.DefaultAuthorizer != nil {
		apiProps.DefaultAuthorizer = props.DefaultAuthorizer
	}

	httpApi := awsapigatewayv2.NewHttpApi(this, jsii.String("HttpApi"), apiProps)

	// Create or get stage
	var stage awsapigatewayv2.IHttpStage
	stageName := "$default"
	if props.StageName != nil {
		stageName = *props.StageName
	}

	// Check if we need a custom stage
	needCustomStage := stageName != "$default" ||
		props.ThrottleRateLimit != nil ||
		props.ThrottleBurstLimit != nil ||
		props.EnableAccessLogging != nil && *props.EnableAccessLogging ||
		props.EnableDetailedMetrics != nil && *props.EnableDetailedMetrics

	if !needCustomStage {
		stage = httpApi.DefaultStage()
	} else {
		// Create custom stage with configuration
		stageProps := &awsapigatewayv2.HttpStageProps{
			HttpApi:    httpApi,
			StageName:  jsii.String(stageName),
			AutoDeploy: jsii.Bool(true),
		}

		// Configure throttling
		if props.ThrottleRateLimit != nil || props.ThrottleBurstLimit != nil {
			throttleSettings := &awsapigatewayv2.ThrottleSettings{}
			if props.ThrottleRateLimit != nil {
				throttleSettings.RateLimit = props.ThrottleRateLimit
			}
			if props.ThrottleBurstLimit != nil {
				throttleSettings.BurstLimit = props.ThrottleBurstLimit
			}
			stageProps.Throttle = throttleSettings
		}

		stage = awsapigatewayv2.NewHttpStage(this, jsii.String("Stage"), stageProps)
	}

	// Configure access logging
	if logGroup != nil {
		accessLogSettings := &awsapigatewayv2.CfnStage_AccessLogSettingsProperty{
			DestinationArn: logGroup.LogGroupArn(),
			Format:         jsii.String(`$context.requestId $context.requestTime "$context.httpMethod $context.path $context.protocol" $context.status $context.responseLength $context.error.message $context.error.responseType`),
		}

		cfnStage := stage.Node().DefaultChild().(awsapigatewayv2.CfnStage)
		cfnStage.SetAccessLogSettings(accessLogSettings)

		// Grant write permissions to API Gateway service
		logGroup.Grant(awsiam.NewServicePrincipal(jsii.String("apigateway.amazonaws.com"), nil), jsii.String("logs:PutLogEvents"))
	}

	// Configure detailed metrics
	if props.EnableDetailedMetrics != nil && *props.EnableDetailedMetrics {
		cfnStage := stage.Node().DefaultChild().(awsapigatewayv2.CfnStage)
		cfnStage.AddPropertyOverride(jsii.String("DetailedMetricsEnabled"), jsii.Bool(true))
	}

	// Configure custom domain if provided
	if props.DomainName != nil && props.CertificateArn != nil {
		// Create certificate from ARN
		cert := awscertificatemanager.Certificate_FromCertificateArn(this, jsii.String("Certificate"), props.CertificateArn)

		domainName := awsapigatewayv2.NewDomainName(this, jsii.String("DomainName"), &awsapigatewayv2.DomainNameProps{
			DomainName:  props.DomainName,
			Certificate: cert,
		})

		awsapigatewayv2.NewApiMapping(this, jsii.String("ApiMapping"), &awsapigatewayv2.ApiMappingProps{
			Api:        httpApi,
			DomainName: domainName,
			Stage:      stage,
		})
	}

	return &LiftAPI{
		Construct: this,
		HttpAPI:   httpApi,
		Stage:     stage,
		LogGroup:  logGroup,
	}
}

// AddLambdaRoute adds a Lambda function as a route to the API
func (api *LiftAPI) AddLambdaRoute(path *string, method awsapigatewayv2.HttpMethod, fn awslambda.IFunction) {
	api.AddLambdaRouteWithOptions(path, method, fn, nil)
}

// RouteOptions defines options for API routes
type RouteOptions struct {
	// Authorizer for this route
	Authorizer awsapigatewayv2.IHttpRouteAuthorizer
	// Request validation
	RequestValidator *RequestValidator
	// Route-specific throttling
	ThrottleRateLimit  *float64
	ThrottleBurstLimit *float64
}

// AddLambdaRouteWithOptions adds a Lambda function as a route with additional options
func (api *LiftAPI) AddLambdaRouteWithOptions(path *string, method awsapigatewayv2.HttpMethod, fn awslambda.IFunction, options *RouteOptions) {
	integration := awsapigatewayv2integrations.NewHttpLambdaIntegration(
		jsii.String("LambdaIntegration"),
		fn,
		&awsapigatewayv2integrations.HttpLambdaIntegrationProps{
			PayloadFormatVersion: awsapigatewayv2.PayloadFormatVersion_VERSION_2_0(),
		},
	)

	routeOptions := &awsapigatewayv2.AddRoutesOptions{
		Path:        path,
		Methods:     &[]awsapigatewayv2.HttpMethod{method},
		Integration: integration,
	}

	// Apply route-specific options
	if options != nil {
		if options.Authorizer != nil {
			routeOptions.Authorizer = options.Authorizer
		}
	}

	api.HttpAPI.AddRoutes(routeOptions)
}

// AddRoutes adds multiple routes from a route definition map
func (api *LiftAPI) AddRoutes(routes map[string]map[string]awslambda.IFunction) {
	for path, methods := range routes {
		for method, fn := range methods {
			httpMethod := awsapigatewayv2.HttpMethod(method)
			api.AddLambdaRoute(jsii.String(path), httpMethod, fn)
		}
	}
}

// EnableApiKeyAuth enables API key authentication for the API
func (api *LiftAPI) EnableApiKeyAuth() awsapigatewayv2.IHttpRouteAuthorizer {
	// HTTP APIs don't have built-in API key support, so we use a Lambda authorizer
	authorizer := NewAPIKeyAuthorizer(api, jsii.String("APIKeyAuth"), &APIKeyAuthorizerProps{
		APIKeySource:    jsii.String("header"),
		APIKeyParameter: jsii.String("X-API-Key"),
		ResultsCacheTtl: jsii.Number(300), // Cache for 5 minutes
	})

	return authorizer.Authorizer
}

// GetUrl returns the URL of the API
func (api *LiftAPI) GetUrl() *string {
	return api.HttpAPI.Url()
}

// GetArn returns the ARN of the API
func (api *LiftAPI) GetArn() *string {
	return api.HttpAPI.ApiId()
}

// GrantInvoke grants invoke permissions to a principal
func (api *LiftAPI) GrantInvoke(grantee awsiam.IGrantable) awsiam.Grant {
	return awsiam.Grant_AddToPrincipal(&awsiam.GrantOnPrincipalOptions{
		Grantee:      grantee,
		Actions:      &[]*string{jsii.String("execute-api:Invoke")},
		ResourceArns: &[]*string{api.HttpAPI.ArnForExecuteApi(jsii.String("*"), jsii.String("*"), jsii.String("*"))},
	})
}
