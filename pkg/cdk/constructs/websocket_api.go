package constructs

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// WebSocketRouteConfig defines configuration for WebSocket routes
type WebSocketRouteConfig struct {
	// Route key (e.g., "$connect", "$disconnect", "$default", "custom")
	RouteKey *string
	// Lambda function for this route
	Function awslambda.IFunction
	// Whether this route requires authorization
	RequireAuthorization *bool
	// Custom authorizer for this route
	Authorizer awsapigatewayv2.IWebSocketRouteAuthorizer
}

// WebSocketAPIProps defines properties for a WebSocket API
type WebSocketAPIProps struct {
	// API name
	ApiName *string
	// API description
	Description *string
	// Route selection expression (default: "$request.body.action")
	RouteSelectionExpression *string
	
	// Lambda function properties for handlers
	FunctionProps awslambda.FunctionProps
	
	// Connection management table properties (uses DynamORM)
	ConnectionTableProps *ConnectionTableProps
	// Enable automatic connection management
	EnableConnectionManagement *bool
	
	// WebSocket route configurations
	Routes []*WebSocketRouteConfig
	
	// Default route function (for unmatched routes)
	DefaultRouteFunction awslambda.IFunction
	
	// Connect route function ($connect)
	ConnectRouteFunction awslambda.IFunction
	
	// Disconnect route function ($disconnect)
	DisconnectRouteFunction awslambda.IFunction
	
	// Stage configuration
	StageName *string
	// Auto deploy stage
	AutoDeploy *bool
	
	// Access logging
	EnableAccessLogging *bool
	AccessLogGroup awslogs.ILogGroup
	
	// Throttling
	ThrottleRateLimit *float64
	ThrottleBurstLimit *float64
	
	// Default authorizer for all routes
	DefaultAuthorizer awsapigatewayv2.IWebSocketRouteAuthorizer
	
	// Lift-specific settings
	EnableTracing     *bool
	EnableMultiTenant *bool
	EnableMonitoring  *bool
	EnableDeadLetterQueue *bool
}

// WebSocketAPI represents a WebSocket API Gateway with Lambda integration
type WebSocketAPI struct {
	constructs.Construct
	
	// The WebSocket API
	WebSocketApi awsapigatewayv2.WebSocketApi
	
	// The stage
	Stage awsapigatewayv2.WebSocketStage
	
	// Lambda functions for different routes
	ConnectFunction    *LiftFunction
	DisconnectFunction *LiftFunction
	DefaultFunction    *LiftFunction
	
	// Connection management table (DynamORM-based)
	ConnectionTable *ConnectionTable
	
	// Routes map
	Routes map[string]awsapigatewayv2.WebSocketRoute
	
	// Access log group
	AccessLogGroup awslogs.ILogGroup
}

// NewWebSocketAPI creates a new WebSocket API construct using DynamORM
func NewWebSocketAPI(scope constructs.Construct, id *string, props *WebSocketAPIProps) *WebSocketAPI {
	this := &WebSocketAPI{}
	constructs.NewConstruct_Override(this, scope, id)
	
	// Set defaults
	if props == nil {
		props = &WebSocketAPIProps{}
	}
	
	apiName := "WebSocketAPI"
	if props.ApiName != nil {
		apiName = *props.ApiName
	}
	
	description := "Lift WebSocket API with DynamORM"
	if props.Description != nil {
		description = *props.Description
	}
	
	routeSelectionExpression := "$request.body.action"
	if props.RouteSelectionExpression != nil {
		routeSelectionExpression = *props.RouteSelectionExpression
	}
	
	stageName := "prod"
	if props.StageName != nil {
		stageName = *props.StageName
	}
	
	enableConnectionManagement := true
	if props.EnableConnectionManagement != nil {
		enableConnectionManagement = *props.EnableConnectionManagement
	}
	
	autoDeploy := true
	if props.AutoDeploy != nil {
		autoDeploy = *props.AutoDeploy
	}
	
	enableAccessLogging := true
	if props.EnableAccessLogging != nil {
		enableAccessLogging = *props.EnableAccessLogging
	}
	
	// Create access log group if access logging is enabled
	if enableAccessLogging {
		if props.AccessLogGroup != nil {
			this.AccessLogGroup = props.AccessLogGroup
		} else {
			this.AccessLogGroup = awslogs.NewLogGroup(this, jsii.String("AccessLogGroup"), &awslogs.LogGroupProps{
				LogGroupName:  jsii.String(fmt.Sprintf("/aws/apigateway/websocket/%s", apiName)),
				Retention:     awslogs.RetentionDays_ONE_MONTH,
				RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
			})
		}
	}
	
	// Create WebSocket API
	apiProps := &awsapigatewayv2.WebSocketApiProps{
		ApiName:                  jsii.String(apiName),
		Description:              jsii.String(description),
		RouteSelectionExpression: jsii.String(routeSelectionExpression),
	}
	
	// Add default authorizer if provided
	if props.DefaultAuthorizer != nil {
		apiProps.DefaultRouteOptions = &awsapigatewayv2.WebSocketRouteOptions{
			Authorizer: props.DefaultAuthorizer,
		}
	}
	
	this.WebSocketApi = awsapigatewayv2.NewWebSocketApi(this, jsii.String("WebSocketApi"), apiProps)
	
	// Create connection management table using DynamORM if enabled
	if enableConnectionManagement {
		// Set defaults for connection table
		connectionTableProps := &ConnectionTableProps{}
		if props.ConnectionTableProps != nil {
			connectionTableProps = props.ConnectionTableProps
		}
		
		// Set table name based on API name if not provided
		if connectionTableProps.TableName == nil {
			connectionTableProps.TableName = jsii.String(fmt.Sprintf("%s-connections", apiName))
		}
		
		// Enable indexes based on multi-tenant setting
		if connectionTableProps.EnableUserIndex == nil {
			connectionTableProps.EnableUserIndex = jsii.Bool(true)
		}
		if connectionTableProps.EnableTenantIndex == nil && props.EnableMultiTenant != nil {
			connectionTableProps.EnableTenantIndex = props.EnableMultiTenant
		}
		
		// Create the DynamORM-based connection table
		this.ConnectionTable = NewConnectionTable(this, jsii.String("ConnectionTable"), connectionTableProps)
	}
	
	// Initialize routes map
	this.Routes = make(map[string]awsapigatewayv2.WebSocketRoute)
	
	// Create Lambda functions for standard routes
	this.createStandardFunctions(props)
	
	// Create standard routes using functions
	connectFunction := props.ConnectRouteFunction
	if connectFunction == nil && this.ConnectFunction != nil {
		connectFunction = this.ConnectFunction.Function
	}
	
	disconnectFunction := props.DisconnectRouteFunction
	if disconnectFunction == nil && this.DisconnectFunction != nil {
		disconnectFunction = this.DisconnectFunction.Function
	}
	
	defaultFunction := props.DefaultRouteFunction
	if defaultFunction == nil && this.DefaultFunction != nil {
		defaultFunction = this.DefaultFunction.Function
	}
	
	// Add standard routes
	if connectFunction != nil {
		this.AddRoute("$connect", connectFunction, &WebSocketRouteConfig{
			RouteKey: jsii.String("$connect"),
			Function: connectFunction,
		})
	}
	
	if disconnectFunction != nil {
		this.AddRoute("$disconnect", disconnectFunction, &WebSocketRouteConfig{
			RouteKey: jsii.String("$disconnect"),
			Function: disconnectFunction,
		})
	}
	
	if defaultFunction != nil {
		this.AddRoute("$default", defaultFunction, &WebSocketRouteConfig{
			RouteKey: jsii.String("$default"),
			Function: defaultFunction,
		})
	}
	
	// Add custom routes if provided
	if props.Routes != nil {
		for _, routeConfig := range props.Routes {
			if routeConfig.RouteKey != nil && routeConfig.Function != nil {
				this.AddRoute(*routeConfig.RouteKey, routeConfig.Function, routeConfig)
			}
		}
	}
	
	// Create stage
	stageProps := &awsapigatewayv2.WebSocketStageProps{
		WebSocketApi: this.WebSocketApi,
		StageName:    jsii.String(stageName),
		AutoDeploy:   jsii.Bool(autoDeploy),
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
	
	this.Stage = awsapigatewayv2.NewWebSocketStage(this, jsii.String("Stage"), stageProps)
	
	// Grant API Gateway permissions to invoke Lambda functions
	this.grantApiGatewayInvokePermissions()
	
	// Set up environment variables for Lambda functions
	this.setupEnvironmentVariables()
	
	return this
}

// createStandardFunctions creates Lambda functions for standard WebSocket routes
func (w *WebSocketAPI) createStandardFunctions(props *WebSocketAPIProps) {
	// Create base function props with Lift optimizations
	baseFunctionProps := &LiftFunctionProps{
		FunctionProps: props.FunctionProps,
		EnableTracing: props.EnableTracing,
		EnableMultiTenant: props.EnableMultiTenant,
		EnableDeadLetterQueue: props.EnableDeadLetterQueue,
	}
	
	// Set defaults for WebSocket functions
	if baseFunctionProps.FunctionProps.Runtime == nil {
		baseFunctionProps.FunctionProps.Runtime = awslambda.Runtime_PROVIDED_AL2023()
	}
	if baseFunctionProps.FunctionProps.Architecture == nil {
		baseFunctionProps.FunctionProps.Architecture = awslambda.Architecture_ARM_64()
	}
	if baseFunctionProps.FunctionProps.Timeout == nil {
		baseFunctionProps.FunctionProps.Timeout = awscdk.Duration_Seconds(jsii.Number(30))
	}
	
	// Create connect function
	connectProps := *baseFunctionProps
	connectProps.FunctionProps.FunctionName = jsii.String("websocket-connect")
	if props.FunctionProps.FunctionName != nil {
		connectProps.FunctionProps.FunctionName = jsii.String(*props.FunctionProps.FunctionName + "-connect")
	}
	w.ConnectFunction = NewLiftFunction(w, jsii.String("ConnectFunction"), &connectProps)
	
	// Create disconnect function
	disconnectProps := *baseFunctionProps
	disconnectProps.FunctionProps.FunctionName = jsii.String("websocket-disconnect")
	if props.FunctionProps.FunctionName != nil {
		disconnectProps.FunctionProps.FunctionName = jsii.String(*props.FunctionProps.FunctionName + "-disconnect")
	}
	w.DisconnectFunction = NewLiftFunction(w, jsii.String("DisconnectFunction"), &disconnectProps)
	
	// Create default function
	defaultProps := *baseFunctionProps
	defaultProps.FunctionProps.FunctionName = jsii.String("websocket-default")
	if props.FunctionProps.FunctionName != nil {
		defaultProps.FunctionProps.FunctionName = jsii.String(*props.FunctionProps.FunctionName + "-default")
	}
	w.DefaultFunction = NewLiftFunction(w, jsii.String("DefaultFunction"), &defaultProps)
}

// AddRoute adds a new route to the WebSocket API
func (w *WebSocketAPI) AddRoute(routeKey string, function awslambda.IFunction, config *WebSocketRouteConfig) awsapigatewayv2.WebSocketRoute {
	// Create Lambda integration
	integration := awsapigatewayv2integrations.NewWebSocketLambdaIntegration(
		jsii.String(fmt.Sprintf("%sIntegration", routeKey)),
		function,
		nil,
	)
	
	// Build route options
	routeOptions := &awsapigatewayv2.WebSocketRouteOptions{
		Integration: integration,
	}
	
	// Add authorizer if specified
	if config != nil && config.Authorizer != nil {
		routeOptions.Authorizer = config.Authorizer
	}
	
	// Create the route
	route := w.WebSocketApi.AddRoute(jsii.String(routeKey), routeOptions)
	
	// Grant permissions
	apiGatewayPrincipal := awsiam.NewServicePrincipal(
		jsii.String("apigateway.amazonaws.com"),
		&awsiam.ServicePrincipalOpts{
			Conditions: &map[string]interface{}{
				"ArnLike": map[string]interface{}{
					"aws:SourceArn": fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId()),
				},
			},
		},
	)
	function.GrantInvoke(apiGatewayPrincipal)
	
	// Store in routes map
	w.Routes[routeKey] = route
	
	return route
}

// GrantConnectionManagement grants permissions to manage WebSocket connections
func (w *WebSocketAPI) GrantConnectionManagement(grantee awsiam.IGrantable) awsiam.Grant {
	// Grant API Gateway management permissions
	apiPolicy := awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: jsii.Strings(
			"execute-api:ManageConnections",
			"execute-api:Invoke",
		),
		Resources: jsii.Strings(fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId())),
	})
	
	grantee.GrantPrincipal().AddToPrincipalPolicy(apiPolicy)
	
	// Grant connection table permissions using DynamORM methods
	if w.ConnectionTable != nil {
		w.ConnectionTable.AddDynamORMPermissions(grantee)
	}
	
	// Return a simple grant
	return awsiam.Grant_AddToPrincipal(&awsiam.GrantOnPrincipalOptions{
		Grantee: grantee,
		Actions: &[]*string{jsii.String("execute-api:ManageConnections")},
		ResourceArns: &[]*string{jsii.String(fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId()))},
	})
}

// GrantApiInvoke grants permission to invoke the WebSocket API
func (w *WebSocketAPI) GrantApiInvoke(grantee awsiam.IGrantable) awsiam.Grant {
	// Add policy statement for API invoke permissions
	apiPolicy := awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: jsii.Strings("execute-api:Invoke"),
		Resources: jsii.Strings(fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId())),
	})
	
	grantee.GrantPrincipal().AddToPrincipalPolicy(apiPolicy)
	
	return awsiam.Grant_AddToPrincipal(&awsiam.GrantOnPrincipalOptions{
		Grantee: grantee,
		Actions: &[]*string{jsii.String("execute-api:Invoke")},
		ResourceArns: &[]*string{jsii.String(fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId()))},
	})
}

// AddEnvironmentVariable adds an environment variable to all WebSocket functions
func (w *WebSocketAPI) AddEnvironmentVariable(key string, value string) {
	if w.ConnectFunction != nil {
		w.ConnectFunction.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
	}
	if w.DisconnectFunction != nil {
		w.DisconnectFunction.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
	}
	if w.DefaultFunction != nil {
		w.DefaultFunction.Function.AddEnvironment(jsii.String(key), jsii.String(value), nil)
	}
}

// grantApiGatewayInvokePermissions grants API Gateway permission to invoke Lambda functions
func (w *WebSocketAPI) grantApiGatewayInvokePermissions() {
	apiGatewayPrincipal := awsiam.NewServicePrincipal(jsii.String("apigateway.amazonaws.com"), &awsiam.ServicePrincipalOpts{})
	
	if w.ConnectFunction != nil {
		w.ConnectFunction.Function.GrantInvoke(apiGatewayPrincipal)
	}
	if w.DisconnectFunction != nil {
		w.DisconnectFunction.Function.GrantInvoke(apiGatewayPrincipal)
	}
	if w.DefaultFunction != nil {
		w.DefaultFunction.Function.GrantInvoke(apiGatewayPrincipal)
	}
}

// setupEnvironmentVariables sets up common environment variables for WebSocket functions
func (w *WebSocketAPI) setupEnvironmentVariables() {
	// WebSocket API URL
	wsUrl := fmt.Sprintf("wss://%s.execute-api.%s.amazonaws.com/%s",
		*w.WebSocketApi.ApiId(),
		*w.WebSocketApi.Stack().Region(),
		*w.Stage.StageName(),
	)
	w.AddEnvironmentVariable("WEBSOCKET_API_URL", wsUrl)
	w.AddEnvironmentVariable("WEBSOCKET_API_ID", *w.WebSocketApi.ApiId())
	w.AddEnvironmentVariable("WEBSOCKET_STAGE", *w.Stage.StageName())
	
	// Connection table - using DynamORM table
	if w.ConnectionTable != nil {
		w.AddEnvironmentVariable("CONNECTION_TABLE_NAME", *w.ConnectionTable.Table.TableName())
		w.AddEnvironmentVariable("CONNECTION_TABLE_ARN", *w.ConnectionTable.Table.TableArn())
		
		// Add DynamORM-specific environment variables
		if w.ConnectionTable.GetUserIndexName() != nil {
			w.AddEnvironmentVariable("CONNECTION_USER_INDEX", *w.ConnectionTable.GetUserIndexName())
		}
		if w.ConnectionTable.GetTenantIndexName() != nil {
			w.AddEnvironmentVariable("CONNECTION_TENANT_INDEX", *w.ConnectionTable.GetTenantIndexName())
		}
	}
	
	// Access log group
	if w.AccessLogGroup != nil {
		w.AddEnvironmentVariable("ACCESS_LOG_GROUP", *w.AccessLogGroup.LogGroupName())
	}
}

// GetConnectionTableName returns the connection table name
func (w *WebSocketAPI) GetConnectionTableName() *string {
	if w.ConnectionTable != nil {
		return w.ConnectionTable.Table.TableName()
	}
	return nil
}

// GetWebSocketURL returns the WebSocket URL
func (w *WebSocketAPI) GetWebSocketURL() *string {
	url := fmt.Sprintf("wss://%s.execute-api.%s.amazonaws.com/%s",
		*w.WebSocketApi.ApiId(),
		*w.WebSocketApi.Stack().Region(),
		*w.Stage.StageName(),
	)
	return jsii.String(url)
}