package constructs

import (
	"fmt"
	"strings"

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


	// Connection management table properties (uses DynamORM)
	ConnectionTableProps *ConnectionTableProps
	// Enable automatic connection management
	EnableConnectionManagement *bool

	// WebSocket route configurations
	Routes []*WebSocketRouteConfig

	// Default route function (for unmatched routes) - REQUIRED
	DefaultRouteFunction awslambda.IFunction

	// Connect route function ($connect) - REQUIRED
	ConnectRouteFunction awslambda.IFunction

	// Disconnect route function ($disconnect) - REQUIRED
	DisconnectRouteFunction awslambda.IFunction

	// Stage configuration
	StageName *string
	// Auto deploy stage
	AutoDeploy *bool

	// Access logging
	EnableAccessLogging *bool
	AccessLogGroup      awslogs.ILogGroup

	// Throttling
	ThrottleRateLimit  *float64
	ThrottleBurstLimit *float64

	// Default authorizer for all routes
	DefaultAuthorizer awsapigatewayv2.IWebSocketRouteAuthorizer

	// Lift-specific settings
	EnableTracing         *bool
	EnableMultiTenant     *bool
	EnableMonitoring      *bool
	EnableDeadLetterQueue *bool
}

// WebSocketAPI represents a WebSocket API Gateway with Lambda integration
type WebSocketAPI struct {
	constructs.Construct

	// The WebSocket API
	WebSocketApi awsapigatewayv2.WebSocketApi

	// The stage
	Stage awsapigatewayv2.WebSocketStage

	// Lambda functions for different routes - REMOVED: Functions must be created externally
	// ConnectFunction    *LiftFunction
	// DisconnectFunction *LiftFunction
	// DefaultFunction    *LiftFunction

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

	this.WebSocketApi = awsapigatewayv2.NewWebSocketApi(this, jsii.String("Api"), apiProps) // Shorter ID

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

		// GSIs for user and tenant indexes are now defined in DynamORM models
		// Example model:
		// type Connection struct {
		//     PK     string `dynamorm:"pk"`                    // connection#{id}
		//     SK     string `dynamorm:"sk"`                    // metadata
		//     UserID string `dynamorm:"index:user-index,pk"`   // For user queries
		//     TenantID string `dynamorm:"index:tenant-index,pk"` // For tenant queries (if multi-tenant)
		// }

		// Create the DynamORM-based connection table with minimal ID
		this.ConnectionTable = NewConnectionTable(this, jsii.String("T"), connectionTableProps) // Minimal ID
	}

	// Validate required functions
	if props.ConnectRouteFunction == nil {
		panic("ConnectRouteFunction is required. Create Lambda function externally and pass via props to avoid long CloudFormation resource names.")
	}
	if props.DisconnectRouteFunction == nil {
		panic("DisconnectRouteFunction is required. Create Lambda function externally and pass via props to avoid long CloudFormation resource names.")
	}
	if props.DefaultRouteFunction == nil {
		panic("DefaultRouteFunction is required. Create Lambda function externally and pass via props to avoid long CloudFormation resource names.")
	}

	// Initialize routes map
	this.Routes = make(map[string]awsapigatewayv2.WebSocketRoute)

	// Use provided functions
	connectFunction := props.ConnectRouteFunction
	disconnectFunction := props.DisconnectRouteFunction
	defaultFunction := props.DefaultRouteFunction

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

	// Environment variables are no longer set automatically
	// Use GetWebSocketURL(), GetConnectionTableName(), etc. to get values for your functions

	return this
}

// REMOVED: createStandardFunctions - Functions must now be created externally to avoid deep nesting

// AddRoute adds a new route to the WebSocket API
func (w *WebSocketAPI) AddRoute(routeKey string, function awslambda.IFunction, config *WebSocketRouteConfig) awsapigatewayv2.WebSocketRoute {
	// Sanitize route key for naming
	sanitizedName := strings.ReplaceAll(routeKey, "$", "")
	sanitizedName = strings.ReplaceAll(sanitizedName, "/", "")

	// Create Lambda integration - use minimal ID
	// Use single letter for standard routes to minimize nesting
	shortId := ""
	switch routeKey {
	case "$connect":
		shortId = "C"
	case "$disconnect":
		shortId = "D"
	case "$default":
		shortId = "X"
	default:
		// For custom routes, use first letter or two
		if len(sanitizedName) > 0 {
			shortId = string(sanitizedName[0])
		} else {
			shortId = "R"
		}
	}

	integration := awsapigatewayv2integrations.NewWebSocketLambdaIntegration(
		jsii.String(shortId),
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

	// Grant connection table permissions
	if w.ConnectionTable != nil {
		w.ConnectionTable.GrantConnectionManagement(grantee)
	}

	// Return a simple grant
	return awsiam.Grant_AddToPrincipal(&awsiam.GrantOnPrincipalOptions{
		Grantee:      grantee,
		Actions:      &[]*string{jsii.String("execute-api:ManageConnections")},
		ResourceArns: &[]*string{jsii.String(fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId()))},
	})
}

// GrantApiInvoke grants permission to invoke the WebSocket API
func (w *WebSocketAPI) GrantApiInvoke(grantee awsiam.IGrantable) awsiam.Grant {
	// Add policy statement for API invoke permissions
	apiPolicy := awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:    awsiam.Effect_ALLOW,
		Actions:   jsii.Strings("execute-api:Invoke"),
		Resources: jsii.Strings(fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId())),
	})

	grantee.GrantPrincipal().AddToPrincipalPolicy(apiPolicy)

	return awsiam.Grant_AddToPrincipal(&awsiam.GrantOnPrincipalOptions{
		Grantee:      grantee,
		Actions:      &[]*string{jsii.String("execute-api:Invoke")},
		ResourceArns: &[]*string{jsii.String(fmt.Sprintf("arn:aws:execute-api:*:*:%s/*/*", *w.WebSocketApi.ApiId()))},
	})
}

// REMOVED: AddEnvironmentVariable - Functions are now managed externally
// Add environment variables directly to the Lambda functions you create

// grantApiGatewayInvokePermissions grants API Gateway permission to invoke Lambda functions
func (w *WebSocketAPI) grantApiGatewayInvokePermissions() {
	// Permissions are now created automatically by WebSocketLambdaIntegration
	// when routes are added. This avoids duplicate permissions.
}

// REMOVED: setupEnvironmentVariables - Set environment variables directly on your Lambda functions
// Use GetWebSocketURL(), GetConnectionTableName(), etc. to get values to set on your functions

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
