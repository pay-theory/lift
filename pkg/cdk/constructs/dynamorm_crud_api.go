package constructs

import (
	"fmt"
	"strings"
	
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// CRUDOperation defines CRUD operations
type CRUDOperation string

const (
	CRUDOperation_CREATE CRUDOperation = "CREATE"
	CRUDOperation_READ   CRUDOperation = "READ"
	CRUDOperation_UPDATE CRUDOperation = "UPDATE"
	CRUDOperation_DELETE CRUDOperation = "DELETE"
	CRUDOperation_LIST   CRUDOperation = "LIST"
	CRUDOperation_SEARCH CRUDOperation = "SEARCH"
)

// ValidationStrategy defines validation approach
type ValidationStrategy string

const (
	ValidationStrategy_STRICT    ValidationStrategy = "STRICT"
	ValidationStrategy_PERMISSIVE ValidationStrategy = "PERMISSIVE"
	ValidationStrategy_CUSTOM    ValidationStrategy = "CUSTOM"
)

// AuthorizationStrategy defines authorization approach
type AuthorizationStrategy string

const (
	AuthorizationStrategy_NONE     AuthorizationStrategy = "NONE"
	AuthorizationStrategy_API_KEY  AuthorizationStrategy = "API_KEY"
	AuthorizationStrategy_JWT      AuthorizationStrategy = "JWT"
	AuthorizationStrategy_COGNITO  AuthorizationStrategy = "COGNITO"
	AuthorizationStrategy_CUSTOM   AuthorizationStrategy = "CUSTOM"
)

// DynamORMCRUDAPIProps defines properties for DynamORM CRUD API
type DynamORMCRUDAPIProps struct {
	// Required: The DynamORM table for CRUD operations
	DynamORMTable *DynamORMTable

	// API configuration
	APIName        *string
	APIDescription *string
	StageName      *string

	// Entity configuration
	EntityName     *string              // Name of the entity (e.g., "User", "Order")
	EntityResource *string              // API resource path (e.g., "/users", "/orders")
	PrimaryKey     *string              // Primary key field name
	SortKey        *string              // Sort key field name (optional)

	// CRUD operations to enable
	EnabledOperations []CRUDOperation

	// Validation configuration
	ValidationStrategy ValidationStrategy
	ValidationSchema   *string             // JSON schema for validation
	RequiredFields     []string            // Required fields for create/update

	// Authorization configuration
	AuthorizationStrategy AuthorizationStrategy
	CognitoUserPool      *string            // Cognito User Pool ID
	JWTSecret           *string            // JWT secret for validation

	// Multi-tenant configuration
	EnableMultiTenant    *bool              // Enable multi-tenant support
	TenantAttribute      *string            // Tenant attribute name
	TenantFromAuth       *bool              // Extract tenant from auth context

	// Pagination configuration
	EnablePagination     *bool              // Enable pagination for list operations
	DefaultPageSize      *int               // Default page size
	MaxPageSize          *int               // Maximum page size

	// Search configuration
	EnableSearch         *bool              // Enable search operations
	SearchableFields     []string           // Fields that can be searched
	SearchIndexes        []string           // GSI names for search

	// Caching configuration
	EnableCaching        *bool              // Enable response caching
	CacheConfig          *DynamORMCacheProps

	// Monitoring configuration
	EnableMetrics        *bool              // Enable CloudWatch metrics
	EnableTracing        *bool              // Enable X-Ray tracing
	EnableDetailedMetrics *bool             // Enable detailed monitoring

	// Performance configuration
	BatchSize            *int               // Batch size for list operations
	TimeoutSeconds       *int               // Lambda timeout in seconds

	// CORS configuration
	EnableCORS           *bool              // Enable CORS
	CORSOrigins          []string           // Allowed origins
	CORSMethods          []string           // Allowed methods
	CORSHeaders          []string           // Allowed headers

	// Rate limiting
	EnableRateLimit      *bool              // Enable rate limiting
	RateLimit            *int               // Requests per minute
	BurstLimit           *int               // Burst limit

	// Tags
	Tags                 *map[string]*string
}

// DynamORMCRUDAPI provides a complete CRUD API for DynamORM tables
type DynamORMCRUDAPI struct {
	constructs.Construct

	// The DynamORM table
	Table *DynamORMTable

	// API Gateway REST API
	API awsapigateway.RestApi

	// Lambda functions for different operations
	CreateFunction *LiftFunction
	ReadFunction   *LiftFunction
	UpdateFunction *LiftFunction
	DeleteFunction *LiftFunction
	ListFunction   *LiftFunction
	SearchFunction *LiftFunction

	// API Gateway resources
	EntityResource awsapigateway.Resource
	ItemResource   awsapigateway.Resource

	// Optional cache
	Cache *DynamORMCache

	// Configuration
	props *DynamORMCRUDAPIProps

	// CloudWatch metrics
	Metrics map[string]awscloudwatch.Metric

	// IAM execution role
	ExecutionRole awsiam.Role
}

// validateCRUDAPIProps validates the required properties for DynamORMCRUDAPI
func validateCRUDAPIProps(props *DynamORMCRUDAPIProps) error {
	if props == nil {
		return fmt.Errorf("DynamORMCRUDAPIProps cannot be nil")
	}
	if props.DynamORMTable == nil {
		return fmt.Errorf("DynamORMTable is required for DynamORMCRUDAPI")
	}
	return nil
}

// NewDynamORMCRUDAPI creates a new DynamORM CRUD API construct
func NewDynamORMCRUDAPI(scope constructs.Construct, id *string, props *DynamORMCRUDAPIProps) *DynamORMCRUDAPI {
	this := &DynamORMCRUDAPI{}
	constructs.NewConstruct_Override(this, scope, id)

	// Validate required properties
	if err := validateCRUDAPIProps(props); err != nil {
		// For CDK constructs, panic is acceptable during construction with clear error messages
		panic(fmt.Sprintf("DynamORMCRUDAPI validation failed: %v", err))
	}

	props = this.applyDefaults(props)
	this.props = props
	this.Table = props.DynamORMTable

	// Create API Gateway
	this.createAPI()

	// Create IAM execution role
	this.createExecutionRole()

	// Create Lambda functions for enabled operations
	this.createLambdaFunctions()

	// Create API resources and methods
	this.createAPIResources()

	// Create cache if enabled
	if props.EnableCaching != nil && *props.EnableCaching {
		this.createCache()
	}

	// Set up monitoring if enabled
	if props.EnableMetrics != nil && *props.EnableMetrics {
		this.createCRUDMetrics()
	}

	return this
}

// applyDefaults applies default values to CRUD API properties
func (c *DynamORMCRUDAPI) applyDefaults(props *DynamORMCRUDAPIProps) *DynamORMCRUDAPIProps {
	if props.APIName == nil {
		if props.EntityName != nil {
			props.APIName = jsii.String(fmt.Sprintf("%s-api", *props.EntityName))
		} else {
			props.APIName = jsii.String("dynamorm-crud-api")
		}
	}
	if props.APIDescription == nil {
		props.APIDescription = jsii.String("CRUD API generated by DynamORM")
	}
	if props.StageName == nil {
		props.StageName = jsii.String("prod")
	}
	if props.EntityName == nil {
		props.EntityName = jsii.String("Item")
	}
	if props.EntityResource == nil {
		props.EntityResource = jsii.String(fmt.Sprintf("%ss", strings.ToLower(*props.EntityName)))
	}
	if props.PrimaryKey == nil {
		props.PrimaryKey = jsii.String("id")
	}
	if props.EnabledOperations == nil {
		props.EnabledOperations = []CRUDOperation{
			CRUDOperation_CREATE,
			CRUDOperation_READ,
			CRUDOperation_UPDATE,
			CRUDOperation_DELETE,
			CRUDOperation_LIST,
		}
	}
	if props.ValidationStrategy == "" {
		props.ValidationStrategy = ValidationStrategy_STRICT
	}
	if props.AuthorizationStrategy == "" {
		props.AuthorizationStrategy = AuthorizationStrategy_API_KEY
	}
	if props.EnableMultiTenant == nil {
		props.EnableMultiTenant = jsii.Bool(false)
	}
	if props.TenantAttribute == nil {
		props.TenantAttribute = jsii.String("TenantID")
	}
	if props.TenantFromAuth == nil {
		props.TenantFromAuth = jsii.Bool(true)
	}
	if props.EnablePagination == nil {
		props.EnablePagination = jsii.Bool(true)
	}
	if props.DefaultPageSize == nil {
		i := 25
		props.DefaultPageSize = &i
	}
	if props.MaxPageSize == nil {
		i := 100
		props.MaxPageSize = &i
	}
	if props.EnableSearch == nil {
		props.EnableSearch = jsii.Bool(false)
	}
	if props.EnableCaching == nil {
		props.EnableCaching = jsii.Bool(false)
	}
	if props.EnableMetrics == nil {
		props.EnableMetrics = jsii.Bool(true)
	}
	if props.EnableTracing == nil {
		props.EnableTracing = jsii.Bool(true)
	}
	if props.EnableDetailedMetrics == nil {
		props.EnableDetailedMetrics = jsii.Bool(false)
	}
	if props.BatchSize == nil {
		i := 25
		props.BatchSize = &i
	}
	if props.TimeoutSeconds == nil {
		i := 30
		props.TimeoutSeconds = &i
	}
	if props.EnableCORS == nil {
		props.EnableCORS = jsii.Bool(true)
	}
	if props.CORSOrigins == nil {
		props.CORSOrigins = []string{"*"}
	}
	if props.CORSMethods == nil {
		props.CORSMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if props.CORSHeaders == nil {
		props.CORSHeaders = []string{"Content-Type", "X-Amz-Date", "Authorization", "X-Api-Key"}
	}
	if props.EnableRateLimit == nil {
		props.EnableRateLimit = jsii.Bool(false)
	}
	if props.RateLimit == nil {
		i := 1000 // 1000 requests per minute
		props.RateLimit = &i
	}
	if props.BurstLimit == nil {
		i := 2000 // 2000 burst limit
		props.BurstLimit = &i
	}

	return props
}

// createAPI creates the API Gateway REST API
func (c *DynamORMCRUDAPI) createAPI() {
	apiProps := &awsapigateway.RestApiProps{
		RestApiName:  c.props.APIName,
		Description:  c.props.APIDescription,
		EndpointTypes: &[]awsapigateway.EndpointType{
			awsapigateway.EndpointType_REGIONAL,
		},
		DeployOptions: &awsapigateway.StageOptions{
			StageName:   c.props.StageName,
			MetricsEnabled: jsii.Bool(true),
			TracingEnabled: c.props.EnableTracing,
		},
	}

	// Configure CORS if enabled
	if c.props.EnableCORS != nil && *c.props.EnableCORS {
		apiProps.DefaultCorsPreflightOptions = &awsapigateway.CorsOptions{
			AllowOrigins: jsii.Strings(c.props.CORSOrigins...),
			AllowMethods: jsii.Strings(c.props.CORSMethods...),
			AllowHeaders: jsii.Strings(c.props.CORSHeaders...),
		}
	}

	c.API = awsapigateway.NewRestApi(c, jsii.String("API"), apiProps)

	// Add API key for authentication if needed
	if c.props.AuthorizationStrategy == AuthorizationStrategy_API_KEY {
		apiKey := c.API.AddApiKey(jsii.String("ApiKey"), &awsapigateway.ApiKeyOptions{
			ApiKeyName: jsii.String(fmt.Sprintf("%s-api-key", *c.props.APIName)),
		})
		
		usagePlan := c.API.AddUsagePlan(jsii.String("UsagePlan"), &awsapigateway.UsagePlanProps{
			Name: jsii.String(fmt.Sprintf("%s-usage-plan", *c.props.APIName)),
			Throttle: &awsapigateway.ThrottleSettings{
				RateLimit:  jsii.Number(float64(*c.props.RateLimit)),
				BurstLimit: jsii.Number(float64(*c.props.BurstLimit)),
			},
		})
		
		usagePlan.AddApiKey(apiKey, nil)
		usagePlan.AddApiStage(&awsapigateway.UsagePlanPerApiStage{
			Api:   c.API,
			Stage: c.API.DeploymentStage(),
		})
	}
}

// createExecutionRole creates IAM execution role for Lambda functions
func (c *DynamORMCRUDAPI) createExecutionRole() {
	c.ExecutionRole = awsiam.NewRole(c, jsii.String("ExecutionRole"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		ManagedPolicies: &[]awsiam.IManagedPolicy{
			awsiam.ManagedPolicy_FromAwsManagedPolicyName(jsii.String("service-role/AWSLambdaBasicExecutionRole")),
		},
	})

	// Grant DynamORM permissions
	c.Table.AddDynamORMPermissions(c.ExecutionRole)

	// Grant X-Ray permissions if tracing is enabled
	if c.props.EnableTracing != nil && *c.props.EnableTracing {
		c.Table.AddXRayPermissions(c.ExecutionRole)
	}

	// Grant CloudWatch permissions for metrics
	c.ExecutionRole.AddToPrincipalPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: &[]*string{
			jsii.String("cloudwatch:PutMetricData"),
		},
		Resources: &[]*string{
			jsii.String("*"),
		},
		Conditions: &map[string]interface{}{
			"StringEquals": map[string]interface{}{
				"cloudwatch:namespace": []string{"DynamORM/CRUD"},
			},
		},
	}))
}

// createLambdaFunctions creates Lambda functions for enabled CRUD operations
func (c *DynamORMCRUDAPI) createLambdaFunctions() {
	// Common environment variables
	commonEnv := c.getCommonEnvironmentVariables()

	// Common function properties
	commonProps := &LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
			Architecture: awslambda.Architecture_ARM_64(),
			Timeout:      awscdk.Duration_Seconds(jsii.Number(*c.props.TimeoutSeconds)),
			MemorySize:   jsii.Number(512),
			Role:         c.ExecutionRole,
			Environment:  commonEnv,
			Code:         awslambda.Code_FromInline(jsii.String(GenerateCRUDHandlerCode("default"))),
		},
		EnableTracing:     c.props.EnableTracing,
		EnableMultiTenant: c.props.EnableMultiTenant,
	}

	for _, operation := range c.props.EnabledOperations {
		switch operation {
		case CRUDOperation_CREATE:
			c.CreateFunction = c.createOperationFunction("Create", "create", commonProps)
		case CRUDOperation_READ:
			c.ReadFunction = c.createOperationFunction("Read", "read", commonProps)
		case CRUDOperation_UPDATE:
			c.UpdateFunction = c.createOperationFunction("Update", "update", commonProps)
		case CRUDOperation_DELETE:
			c.DeleteFunction = c.createOperationFunction("Delete", "delete", commonProps)
		case CRUDOperation_LIST:
			c.ListFunction = c.createOperationFunction("List", "list", commonProps)
		case CRUDOperation_SEARCH:
			if c.props.EnableSearch != nil && *c.props.EnableSearch {
				c.SearchFunction = c.createOperationFunction("Search", "search", commonProps)
			}
		}
	}
}

// createOperationFunction creates a Lambda function for a specific CRUD operation
func (c *DynamORMCRUDAPI) createOperationFunction(name, handler string, baseProps *LiftFunctionProps) *LiftFunction {
	props := *baseProps // Copy the struct
	props.FunctionProps.FunctionName = jsii.String(fmt.Sprintf("%s-%s-%s", *c.props.APIName, *c.props.EntityName, handler))
	props.FunctionProps.Handler = jsii.String("index.handler")
	
	// Set operation-specific code
	props.FunctionProps.Code = awslambda.Code_FromInline(jsii.String(GenerateCRUDHandlerCode(handler)))
	
	// Set operation-specific environment variables
	env := make(map[string]*string)
	for k, v := range *props.FunctionProps.Environment {
		env[k] = v
	}
	env["CRUD_OPERATION"] = jsii.String(handler)
	props.FunctionProps.Environment = &env
	
	return NewLiftFunction(c, jsii.String(fmt.Sprintf("%sFunction", name)), &props)
}

// getCommonEnvironmentVariables returns common environment variables for all functions
func (c *DynamORMCRUDAPI) getCommonEnvironmentVariables() *map[string]*string {
	env := make(map[string]*string)

	// DynamORM table configuration
	tableEnv := c.Table.GetEnvironmentVariables()
	for k, v := range *tableEnv {
		env[k] = v
	}

	// CRUD API configuration
	env["CRUD_API_ENABLED"] = jsii.String("true")
	env["CRUD_ENTITY_NAME"] = c.props.EntityName
	env["CRUD_PRIMARY_KEY"] = c.props.PrimaryKey
	if c.props.SortKey != nil {
		env["CRUD_SORT_KEY"] = c.props.SortKey
	}

	// Validation configuration
	env["CRUD_VALIDATION_STRATEGY"] = jsii.String(string(c.props.ValidationStrategy))
	if c.props.ValidationSchema != nil {
		env["CRUD_VALIDATION_SCHEMA"] = c.props.ValidationSchema
	}
	if len(c.props.RequiredFields) > 0 {
		fields := ""
		for i, field := range c.props.RequiredFields {
			if i > 0 {
				fields += ","
			}
			fields += field
		}
		env["CRUD_REQUIRED_FIELDS"] = jsii.String(fields)
	}

	// Authorization configuration
	env["CRUD_AUTH_STRATEGY"] = jsii.String(string(c.props.AuthorizationStrategy))
	if c.props.CognitoUserPool != nil {
		env["CRUD_COGNITO_USER_POOL"] = c.props.CognitoUserPool
	}
	if c.props.JWTSecret != nil {
		env["CRUD_JWT_SECRET"] = c.props.JWTSecret
	}

	// Multi-tenant configuration
	if c.props.EnableMultiTenant != nil && *c.props.EnableMultiTenant {
		env["CRUD_MULTI_TENANT"] = jsii.String("true")
		env["CRUD_TENANT_ATTRIBUTE"] = c.props.TenantAttribute
		if c.props.TenantFromAuth != nil && *c.props.TenantFromAuth {
			env["CRUD_TENANT_FROM_AUTH"] = jsii.String("true")
		}
	}

	// Pagination configuration
	if c.props.EnablePagination != nil && *c.props.EnablePagination {
		env["CRUD_PAGINATION_ENABLED"] = jsii.String("true")
		env["CRUD_DEFAULT_PAGE_SIZE"] = jsii.String(fmt.Sprintf("%d", *c.props.DefaultPageSize))
		env["CRUD_MAX_PAGE_SIZE"] = jsii.String(fmt.Sprintf("%d", *c.props.MaxPageSize))
	}

	// Search configuration
	if c.props.EnableSearch != nil && *c.props.EnableSearch {
		env["CRUD_SEARCH_ENABLED"] = jsii.String("true")
		if len(c.props.SearchableFields) > 0 {
			fields := ""
			for i, field := range c.props.SearchableFields {
				if i > 0 {
					fields += ","
				}
				fields += field
			}
			env["CRUD_SEARCHABLE_FIELDS"] = jsii.String(fields)
		}
		if len(c.props.SearchIndexes) > 0 {
			indexes := ""
			for i, index := range c.props.SearchIndexes {
				if i > 0 {
					indexes += ","
				}
				indexes += index
			}
			env["CRUD_SEARCH_INDEXES"] = jsii.String(indexes)
		}
	}

	// Performance configuration
	env["CRUD_BATCH_SIZE"] = jsii.String(fmt.Sprintf("%d", *c.props.BatchSize))

	// Metrics configuration
	if c.props.EnableMetrics != nil && *c.props.EnableMetrics {
		env["CRUD_METRICS_ENABLED"] = jsii.String("true")
	}

	return &env
}

// createAPIResources creates API Gateway resources and methods
func (c *DynamORMCRUDAPI) createAPIResources() {
	// Create entity resource (e.g., /users)
	c.EntityResource = c.API.Root().AddResource(jsii.String(*c.props.EntityResource), nil)

	// Create item resource (e.g., /users/{id})
	c.ItemResource = c.EntityResource.AddResource(jsii.String(fmt.Sprintf("{%s}", *c.props.PrimaryKey)), nil)

	// Configure authorization
	var authorizer awsapigateway.IAuthorizer
	if c.props.AuthorizationStrategy == AuthorizationStrategy_COGNITO && c.props.CognitoUserPool != nil {
		// TODO: Add Cognito authorizer when CDK supports it properly
		// For now, use API key authentication
	}

	// Add methods for enabled operations
	for _, operation := range c.props.EnabledOperations {
		switch operation {
		case CRUDOperation_CREATE:
			if c.CreateFunction != nil {
				c.addMethod(c.EntityResource, "POST", c.CreateFunction, authorizer, c.props.AuthorizationStrategy == AuthorizationStrategy_API_KEY)
			}
		case CRUDOperation_READ:
			if c.ReadFunction != nil {
				c.addMethod(c.ItemResource, "GET", c.ReadFunction, authorizer, c.props.AuthorizationStrategy == AuthorizationStrategy_API_KEY)
			}
		case CRUDOperation_UPDATE:
			if c.UpdateFunction != nil {
				c.addMethod(c.ItemResource, "PUT", c.UpdateFunction, authorizer, c.props.AuthorizationStrategy == AuthorizationStrategy_API_KEY)
			}
		case CRUDOperation_DELETE:
			if c.DeleteFunction != nil {
				c.addMethod(c.ItemResource, "DELETE", c.DeleteFunction, authorizer, c.props.AuthorizationStrategy == AuthorizationStrategy_API_KEY)
			}
		case CRUDOperation_LIST:
			if c.ListFunction != nil {
				c.addMethod(c.EntityResource, "GET", c.ListFunction, authorizer, c.props.AuthorizationStrategy == AuthorizationStrategy_API_KEY)
			}
		case CRUDOperation_SEARCH:
			if c.SearchFunction != nil {
				searchResource := c.EntityResource.AddResource(jsii.String("search"), nil)
				c.addMethod(searchResource, "GET", c.SearchFunction, authorizer, c.props.AuthorizationStrategy == AuthorizationStrategy_API_KEY)
			}
		}
	}
}

// addMethod adds a method to an API Gateway resource
func (c *DynamORMCRUDAPI) addMethod(resource awsapigateway.IResource, httpMethod string, function *LiftFunction, authorizer awsapigateway.IAuthorizer, requireApiKey bool) {
	integration := awsapigateway.NewLambdaIntegration(function.Function, &awsapigateway.LambdaIntegrationOptions{
		RequestTemplates: &map[string]*string{
			"application/json": jsii.String(`{
				"body": $input.json('$'),
				"headers": {
					#foreach($header in $input.params().header.keySet())
					"$header": "$util.escapeJavaScript($input.params().header.get($header))"
					#if($foreach.hasNext),#end
					#end
				},
				"pathParameters": {
					#foreach($param in $input.params().path.keySet())
					"$param": "$util.escapeJavaScript($input.params().path.get($param))"
					#if($foreach.hasNext),#end
					#end
				},
				"queryStringParameters": {
					#foreach($queryParam in $input.params().querystring.keySet())
					"$queryParam": "$util.escapeJavaScript($input.params().querystring.get($queryParam))"
					#if($foreach.hasNext),#end
					#end
				}
			}`),
		},
		IntegrationResponses: &[]*awsapigateway.IntegrationResponse{
			{
				StatusCode: jsii.String("200"),
				ResponseTemplates: &map[string]*string{
					"application/json": jsii.String("$input.json('$')"),
				},
			},
			{
				StatusCode:      jsii.String("400"),
				SelectionPattern: jsii.String(".*\\[BadRequest\\].*"),
				ResponseTemplates: &map[string]*string{
					"application/json": jsii.String(`{"error": "$input.path('$.errorMessage')"}`),
				},
			},
			{
				StatusCode:      jsii.String("404"),
				SelectionPattern: jsii.String(".*\\[NotFound\\].*"),
				ResponseTemplates: &map[string]*string{
					"application/json": jsii.String(`{"error": "$input.path('$.errorMessage')"}`),
				},
			},
			{
				StatusCode:      jsii.String("500"),
				SelectionPattern: jsii.String(".*\\[InternalError\\].*"),
				ResponseTemplates: &map[string]*string{
					"application/json": jsii.String(`{"error": "Internal server error"}`),
				},
			},
		},
	})

	methodOptions := &awsapigateway.MethodOptions{
		ApiKeyRequired: jsii.Bool(requireApiKey),
		MethodResponses: &[]*awsapigateway.MethodResponse{
			{
				StatusCode: jsii.String("200"),
				ResponseModels: &map[string]awsapigateway.IModel{
					"application/json": awsapigateway.Model_EMPTY_MODEL(),
				},
			},
			{
				StatusCode: jsii.String("400"),
				ResponseModels: &map[string]awsapigateway.IModel{
					"application/json": awsapigateway.Model_ERROR_MODEL(),
				},
			},
			{
				StatusCode: jsii.String("404"),
				ResponseModels: &map[string]awsapigateway.IModel{
					"application/json": awsapigateway.Model_ERROR_MODEL(),
				},
			},
			{
				StatusCode: jsii.String("500"),
				ResponseModels: &map[string]awsapigateway.IModel{
					"application/json": awsapigateway.Model_ERROR_MODEL(),
				},
			},
		},
	}

	if authorizer != nil {
		methodOptions.Authorizer = authorizer
	}

	resource.AddMethod(jsii.String(httpMethod), integration, methodOptions)
}

// createCache creates cache if caching is enabled
func (c *DynamORMCRUDAPI) createCache() {
	if c.props.CacheConfig == nil {
		c.props.CacheConfig = &DynamORMCacheProps{
			CacheStrategy:         CacheStrategy_IN_MEMORY,
			InvalidationStrategy:  CacheInvalidationStrategy_TTL,
			DefaultTTL:           awscdk.Duration_Minutes(jsii.Number(15)),
			EnableTenantIsolation: c.props.EnableMultiTenant,
			TenantAttribute:       c.props.TenantAttribute,
		}
	}

	c.props.CacheConfig.DynamORMTable = c.Table
	c.Cache = NewDynamORMCache(c, jsii.String("Cache"), c.props.CacheConfig)

	// Grant cache access to all functions
	functions := []*LiftFunction{
		c.CreateFunction, c.ReadFunction, c.UpdateFunction,
		c.DeleteFunction, c.ListFunction, c.SearchFunction,
	}

	for _, function := range functions {
		if function != nil {
			c.Cache.GrantCacheAccess(function.Function)
			
			// Add cache environment variables
			cacheEnv := c.Cache.GetEnvironmentVariables()
			for k, v := range *cacheEnv {
				function.Function.AddEnvironment(jsii.String(k), v, nil)
			}
		}
	}
}

// createCRUDMetrics creates CloudWatch metrics for CRUD operations
func (c *DynamORMCRUDAPI) createCRUDMetrics() {
	c.Metrics = make(map[string]awscloudwatch.Metric)
	apiName := *c.props.APIName

	// API Gateway metrics
	c.Metrics["APIRequests"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGateway"),
		MetricName: jsii.String("Count"),
		DimensionsMap: &map[string]*string{
			"ApiName": jsii.String(apiName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	c.Metrics["APILatency"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGateway"),
		MetricName: jsii.String("Latency"),
		DimensionsMap: &map[string]*string{
			"ApiName": jsii.String(apiName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	c.Metrics["APIErrors"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("AWS/ApiGateway"),
		MetricName: jsii.String("4XXError"),
		DimensionsMap: &map[string]*string{
			"ApiName": jsii.String(apiName),
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// CRUD operation metrics
	c.Metrics["CRUDOperations"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/CRUD"),
		MetricName: jsii.String("Operations"),
		DimensionsMap: &map[string]*string{
			"APIName": jsii.String(apiName),
			"EntityName": c.props.EntityName,
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	c.Metrics["CRUDLatency"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/CRUD"),
		MetricName: jsii.String("Latency"),
		DimensionsMap: &map[string]*string{
			"APIName": jsii.String(apiName),
			"EntityName": c.props.EntityName,
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_AVERAGE)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	c.Metrics["CRUDErrors"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
		Namespace:  jsii.String("DynamORM/CRUD"),
		MetricName: jsii.String("Errors"),
		DimensionsMap: &map[string]*string{
			"APIName": jsii.String(apiName),
			"EntityName": c.props.EntityName,
		},
		Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
	})

	// Multi-tenant metrics if enabled
	if c.props.EnableMultiTenant != nil && *c.props.EnableMultiTenant {
		c.Metrics["TenantOperations"] = awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("DynamORM/CRUD/Tenant"),
			MetricName: jsii.String("Operations"),
			DimensionsMap: &map[string]*string{
				"APIName": jsii.String(apiName),
				"EntityName": c.props.EntityName,
				"TenantAttribute": c.props.TenantAttribute,
			},
			Statistic: jsii.String(string(awscloudwatch.Statistic_SUM)),
			Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		})
	}
}

// GetAPI returns the API Gateway REST API
func (c *DynamORMCRUDAPI) GetAPI() awsapigateway.RestApi {
	return c.API
}

// GetTable returns the DynamORM table
func (c *DynamORMCRUDAPI) GetTable() *DynamORMTable {
	return c.Table
}

// GetCache returns the cache (if enabled)
func (c *DynamORMCRUDAPI) GetCache() *DynamORMCache {
	return c.Cache
}

// GetCRUDMetrics returns CRUD CloudWatch metrics
func (c *DynamORMCRUDAPI) GetCRUDMetrics() map[string]awscloudwatch.Metric {
	return c.Metrics
}

// GetExecutionRole returns the IAM execution role
func (c *DynamORMCRUDAPI) GetExecutionRole() awsiam.Role {
	return c.ExecutionRole
}

// GetAPIURL returns the API Gateway URL
func (c *DynamORMCRUDAPI) GetAPIURL() *string {
	return c.API.Url()
}

// GetEntityEndpoint returns the entity endpoint URL
func (c *DynamORMCRUDAPI) GetEntityEndpoint() *string {
	return jsii.String(fmt.Sprintf("%s%s", *c.API.Url(), *c.props.EntityResource))
}