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

// LiftAPIProps defines properties for creating a Lift HTTP API Gateway (v2)
type LiftAPIProps struct {
	APICommonProps
	// API Key configuration
	RequireApiKey *bool
	// Request/Response validation models
	RequestValidators map[string]*RequestValidator
	// Default authorizer for all routes (HTTP API specific)
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

	builder := newLiftAPIBuilder(this, props)
	return builder.build()
}

// liftAPIBuilder builds Lift API components
type liftAPIBuilder struct {
	construct constructs.Construct
	props     *LiftAPIProps
}

// newLiftAPIBuilder creates a new Lift API builder
func newLiftAPIBuilder(construct constructs.Construct, props *LiftAPIProps) *liftAPIBuilder {
	return &liftAPIBuilder{
		construct: construct,
		props:     props,
	}
}

// build constructs the complete Lift API
func (b *liftAPIBuilder) build() *LiftAPI {
	// Create log group for access logging
	logGroup := b.createLogGroup()

	// Create HTTP API
	httpApi := b.createHttpAPI()

	// Create stage
	stage := b.createStage(httpApi, logGroup)

	// Configure custom domain
	b.configureDomain(httpApi, stage)

	return &LiftAPI{
		Construct: b.construct,
		HttpAPI:   httpApi,
		Stage:     stage,
		LogGroup:  logGroup,
	}
}

// createLogGroup creates the access log group if needed
func (b *liftAPIBuilder) createLogGroup() awslogs.ILogGroup {
	if b.props.EnableAccessLogging == nil || !*b.props.EnableAccessLogging {
		return nil
	}

	return CreateAPILogGroup(b.construct, b.props.Name, b.props.AccessLogGroup)
}

// createHttpAPI creates the HTTP API with CORS configuration
func (b *liftAPIBuilder) createHttpAPI() awsapigatewayv2.HttpApi {
	apiProps := &awsapigatewayv2.HttpApiProps{
		ApiName:     b.props.Name,
		Description: b.props.Description,
		// Disable auto-deployment to prevent default stage from being created
		// We'll create our own stage with proper configuration
		CreateDefaultStage: jsii.Bool(false),
	}

	// Configure CORS if enabled
	if b.props.EnableCORS != nil && *b.props.EnableCORS {
		apiProps.CorsPreflight = b.createCORSConfig()
	}

	// Set default authorizer if provided
	if b.props.DefaultAuthorizer != nil {
		apiProps.DefaultAuthorizer = b.props.DefaultAuthorizer
	}

	return awsapigatewayv2.NewHttpApi(b.construct, jsii.String("HttpApi"), apiProps)
}

// createCORSConfig creates CORS preflight configuration
func (b *liftAPIBuilder) createCORSConfig() *awsapigatewayv2.CorsPreflightOptions {
	// Use custom origins if provided, otherwise default to wildcard
	allowOrigins := &[]*string{jsii.String("*")}
	if b.props.AllowOrigins != nil {
		allowOrigins = b.props.AllowOrigins
	}

	return &awsapigatewayv2.CorsPreflightOptions{
		AllowOrigins: allowOrigins,
		AllowMethods: &[]awsapigatewayv2.CorsHttpMethod{
			awsapigatewayv2.CorsHttpMethod_GET,
			awsapigatewayv2.CorsHttpMethod_POST,
			awsapigatewayv2.CorsHttpMethod_PUT,
			awsapigatewayv2.CorsHttpMethod_DELETE,
			awsapigatewayv2.CorsHttpMethod_OPTIONS,
		},
		AllowHeaders:  CORSHeaders(),
		ExposeHeaders: CORSExposeHeaders(),
		MaxAge:        awscdk.Duration_Hours(jsii.Number(24)),
	}
}

// createStage creates the API stage with configuration
func (b *liftAPIBuilder) createStage(httpApi awsapigatewayv2.HttpApi, logGroup awslogs.ILogGroup) awsapigatewayv2.IHttpStage {
	stageName := "$default"
	if b.props.StageName != nil {
		stageName = *b.props.StageName
	}

	// Always create a custom stage since we disabled CreateDefaultStage in the API
	// This ensures we have full control over stage configuration (logging, throttling, metrics)
	stage := b.createCustomStage(httpApi, stageName)

	// Configure access logging
	b.configureAccessLogging(stage, logGroup)

	return stage
}

// createCustomStage creates a custom stage with throttling
func (b *liftAPIBuilder) createCustomStage(httpApi awsapigatewayv2.HttpApi, stageName string) awsapigatewayv2.IHttpStage {
	stageProps := &awsapigatewayv2.HttpStageProps{
		HttpApi:    httpApi,
		StageName:  jsii.String(stageName),
		AutoDeploy: jsii.Bool(true),
	}

	// Configure throttling if specified
	if b.props.ThrottleRateLimit != nil || b.props.ThrottleBurstLimit != nil {
		stageProps.Throttle = b.createThrottleSettings()
	}

	return awsapigatewayv2.NewHttpStage(b.construct, jsii.String("Stage"), stageProps)
}

// createThrottleSettings creates throttle configuration
func (b *liftAPIBuilder) createThrottleSettings() *awsapigatewayv2.ThrottleSettings {
	throttleSettings := &awsapigatewayv2.ThrottleSettings{}

	if b.props.ThrottleRateLimit != nil {
		throttleSettings.RateLimit = b.props.ThrottleRateLimit
	}
	if b.props.ThrottleBurstLimit != nil {
		throttleSettings.BurstLimit = b.props.ThrottleBurstLimit
	}

	return throttleSettings
}

// configureAccessLogging configures access logging for the stage
func (b *liftAPIBuilder) configureAccessLogging(stage awsapigatewayv2.IHttpStage, logGroup awslogs.ILogGroup) {
	if logGroup == nil {
		return
	}

	accessLogSettings := &awsapigatewayv2.CfnStage_AccessLogSettingsProperty{
		DestinationArn: logGroup.LogGroupArn(),
		Format:         jsii.String(`$context.requestId $context.requestTime "$context.httpMethod $context.path $context.protocol" $context.status $context.responseLength $context.error.message $context.error.responseType`),
	}

	if defaultChild := stage.Node().DefaultChild(); defaultChild != nil {
		if cfnStage, ok := defaultChild.(awsapigatewayv2.CfnStage); ok {
			cfnStage.SetAccessLogSettings(accessLogSettings)
		}
	}

	// Grant write permissions to API Gateway service
	logGroup.Grant(awsiam.NewServicePrincipal(jsii.String("apigateway.amazonaws.com"), nil), jsii.String("logs:PutLogEvents"))
}

// configureDomain configures custom domain mapping if provided
func (b *liftAPIBuilder) configureDomain(httpApi awsapigatewayv2.HttpApi, stage awsapigatewayv2.IHttpStage) {
	if b.props.DomainName == nil || b.props.CertificateArn == nil {
		return
	}

	// Create certificate from ARN
	cert := awscertificatemanager.Certificate_FromCertificateArn(b.construct, jsii.String("Certificate"), b.props.CertificateArn)

	domainName := awsapigatewayv2.NewDomainName(b.construct, jsii.String("DomainName"), &awsapigatewayv2.DomainNameProps{
		DomainName:  b.props.DomainName,
		Certificate: cert,
	})

	awsapigatewayv2.NewApiMapping(b.construct, jsii.String("ApiMapping"), &awsapigatewayv2.ApiMappingProps{
		Api:        httpApi,
		DomainName: domainName,
		Stage:      stage,
	})
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

// GetUrl returns the URL of the API stage
func (api *LiftAPI) GetUrl() *string {
	// Always use the stage URL since Lift creates a custom stage
	return api.Stage.Url()
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
