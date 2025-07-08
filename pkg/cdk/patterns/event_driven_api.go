package patterns

import (
	"fmt"
	
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

// EventDrivenAPIProps defines properties for an event-driven API pattern
type EventDrivenAPIProps struct {
	// Application name
	AppName *string
	
	// API configuration
	ApiName *string
	Description *string
	EnableCORS *bool
	EnableAccessLogging *bool
	ThrottleRateLimit *float64
	ThrottleBurstLimit *float64
	
	// Lambda function configuration
	FunctionProps awslambda.FunctionProps
	MemorySize *float64
	Timeout *float64
	Environment *map[string]*string
	
	// EventBridge configuration
	EventBusName *string
	EventSource *string
	DetailType *string
	
	// Request tracking configuration
	RequestTrackingTableProps *liftconstructs.RequestTrackingTableProps
	EnableRequestTracking *bool
	RequestRetentionDays *float64
	
	// Lift-specific settings
	EnableTracing *bool
	EnableMultiTenant *bool
	EnableMonitoring *bool
	EnableDeadLetterQueue *bool
}

// EventDrivenAPI represents an API Gateway + EventBridge pattern for async processing
type EventDrivenAPI struct {
	constructs.Construct
	
	// The HTTP API
	API *liftconstructs.LiftAPI
	
	// API handler function
	APIFunction *liftconstructs.LiftFunction
	
	// EventBridge handler
	EventHandler *liftconstructs.EventBridgeHandler
	
	// Request tracking table (DynamORM-based)
	RequestTrackingTable *liftconstructs.RequestTrackingTable
}

// NewEventDrivenAPI creates a new event-driven API pattern using DynamORM
func NewEventDrivenAPI(scope constructs.Construct, id *string, props *EventDrivenAPIProps) *EventDrivenAPI {
	this := &EventDrivenAPI{}
	constructs.NewConstruct_Override(this, scope, id)
	
	// Set defaults
	if props == nil {
		props = &EventDrivenAPIProps{}
	}
	
	appName := "event-driven-api"
	if props.AppName != nil {
		appName = *props.AppName
	}
	
	apiName := appName + "-api"
	if props.ApiName != nil {
		apiName = *props.ApiName
	}
	
	eventBusName := "default"
	if props.EventBusName != nil {
		eventBusName = *props.EventBusName
	}
	
	eventSource := appName
	if props.EventSource != nil {
		eventSource = *props.EventSource
	}
	
	detailType := "APIRequest"
	if props.DetailType != nil {
		detailType = *props.DetailType
	}
	
	enableRequestTracking := true
	if props.EnableRequestTracking != nil {
		enableRequestTracking = *props.EnableRequestTracking
	}
	
	// Create request tracking table using DynamORM if enabled
	if enableRequestTracking {
		requestTrackingProps := &liftconstructs.RequestTrackingTableProps{
			TableName: jsii.String(appName + "-requests"),
			// GSIs for correlation, status, and user indexes are now defined in DynamORM models
			// Example model:
			// type Request struct {
			//     PK            string `dynamorm:"pk"`                          // request#{request_id}
			//     SK            string `dynamorm:"sk"`                          // metadata
			//     CorrelationID string `dynamorm:"index:correlation-index,pk"`  // For correlation queries
			//     Status        string `dynamorm:"index:status-index,pk"`       // For status queries
			//     UserID        string `dynamorm:"index:user-index,pk"`         // For user queries
			// }
		}
		
		// Override with user-provided props
		if props.RequestTrackingTableProps != nil {
			requestTrackingProps = props.RequestTrackingTableProps
		}
		
		this.RequestTrackingTable = liftconstructs.NewRequestTrackingTable(this, jsii.String("RequestTracking"), requestTrackingProps)
	}
	
	// Create API handler function
	apiEnv := make(map[string]*string)
	if props.Environment != nil {
		for k, v := range *props.Environment {
			apiEnv[k] = v
		}
	}
	
	// Add environment variables for event-driven pattern
	apiEnv["EVENT_BUS_NAME"] = jsii.String(eventBusName)
	apiEnv["EVENT_SOURCE"] = jsii.String(eventSource)
	apiEnv["EVENT_DETAIL_TYPE"] = jsii.String(detailType)
	if this.RequestTrackingTable != nil {
		apiEnv["REQUEST_TRACKING_TABLE"] = this.RequestTrackingTable.GetTableName()
		apiEnv["REQUEST_TRACKING_TABLE_ARN"] = this.RequestTrackingTable.GetTableArn()
		
		// GSI names are now determined by DynamORM model struct tags
		// The index names in the model would be like "correlation-index", "status-index", etc.
	}
	
	// Create API function
	apiFunctionProps := props.FunctionProps
	apiFunctionProps.FunctionName = jsii.String(appName + "-api-handler")
	apiFunctionProps.Environment = &apiEnv
	if props.MemorySize != nil {
		apiFunctionProps.MemorySize = props.MemorySize
	}
	if props.Timeout != nil {
		apiFunctionProps.Timeout = awscdk.Duration_Seconds(props.Timeout)
	}
	
	this.APIFunction = liftconstructs.NewLiftFunction(this, jsii.String("APIFunction"), &liftconstructs.LiftFunctionProps{
		FunctionProps:         apiFunctionProps,
		EnableTracing:        props.EnableTracing,
		EnableMultiTenant:    props.EnableMultiTenant,
		EnableDeadLetterQueue: props.EnableDeadLetterQueue,
	})
	
	// Grant permissions to API function
	if this.RequestTrackingTable != nil {
		this.RequestTrackingTable.GrantReadWrite(this.APIFunction.Function)
	}
	
	// Create HTTP API
	this.API = liftconstructs.NewLiftAPI(this, jsii.String("API"), &liftconstructs.LiftAPIProps{
		Name:                  jsii.String(apiName),
		Description:           jsii.String("Event-driven API with async processing"),
		EnableCORS:            props.EnableCORS,
		EnableAccessLogging:   props.EnableAccessLogging,
		ThrottleRateLimit:     props.ThrottleRateLimit,
		ThrottleBurstLimit:    props.ThrottleBurstLimit,
	})
	
	// Add routes to API
	this.API.AddLambdaRoute(jsii.String("/submit"), "POST", this.APIFunction.Function)
	this.API.AddLambdaRoute(jsii.String("/status/{requestId}"), "GET", this.APIFunction.Function)
	
	// Create EventBridge handler
	eventEnv := make(map[string]*string)
	if props.Environment != nil {
		for k, v := range *props.Environment {
			eventEnv[k] = v
		}
	}
	
	// Add environment variables for event processing
	if this.RequestTrackingTable != nil {
		eventEnv["REQUEST_TRACKING_TABLE"] = this.RequestTrackingTable.GetTableName()
		eventEnv["REQUEST_TRACKING_TABLE_ARN"] = this.RequestTrackingTable.GetTableArn()
	}
	
	// Create event processing function
	eventFunctionProps := props.FunctionProps
	eventFunctionProps.FunctionName = jsii.String(appName + "-event-processor")
	eventFunctionProps.Environment = &eventEnv
	if props.MemorySize != nil {
		eventFunctionProps.MemorySize = props.MemorySize
	}
	if props.Timeout != nil {
		eventFunctionProps.Timeout = awscdk.Duration_Seconds(props.Timeout)
	}
	
	eventHandler, err := liftconstructs.NewEventBridgeHandler(this, jsii.String("EventHandler"), &liftconstructs.EventBridgeHandlerProps{
		FunctionProps:         eventFunctionProps,
		EnableTracing:        props.EnableTracing,
		EnableMultiTenant:    props.EnableMultiTenant,
		EnableDeadLetterQueue: props.EnableDeadLetterQueue,
		RuleProps: &awsevents.RuleProps{
			RuleName:    jsii.String(appName + "-processor-rule"),
			Description: jsii.String("Process async API requests"),
			EventPattern: &awsevents.EventPattern{
				Source:     &[]*string{jsii.String(eventSource)},
				DetailType: &[]*string{jsii.String(detailType)},
			},
		},
	})
	if err != nil {
		// Log error and create a minimal setup
		fmt.Printf("Warning: Failed to create EventBridge handler: %v\n", err)
		// Set to nil to indicate failure
		this.EventHandler = nil
	} else {
		this.EventHandler = eventHandler
	}
	
	// Grant permissions to event handler
	if this.RequestTrackingTable != nil && this.EventHandler != nil {
		this.RequestTrackingTable.GrantReadWrite(this.EventHandler.Function.Function)
	}
	
	// Enable monitoring if requested
	if props.EnableMonitoring != nil && *props.EnableMonitoring {
		this.enableMonitoring(props)
	}
	
	return this
}

// enableMonitoring adds CloudWatch alarms and metrics
func (e *EventDrivenAPI) enableMonitoring(props *EventDrivenAPIProps) {
	// Basic monitoring implementation with Lambda function metrics only
	if e.APIFunction != nil {
		function := e.APIFunction.GetFunction()
		
		// Function error alarm
		awscloudwatch.NewAlarm(e, jsii.String("FunctionErrorAlarm"), &awscloudwatch.AlarmProps{
			AlarmName: jsii.String("api-function-errors"),
			Metric: function.MetricErrors(&awscloudwatch.MetricOptions{
				Period: awscdk.Duration_Minutes(jsii.Number(5)),
			}),
			Threshold: jsii.Number(5),
			EvaluationPeriods: jsii.Number(2),
		})
	}
}

// GetAPIEndpoint returns the API endpoint URL
func (e *EventDrivenAPI) GetAPIEndpoint() *string {
	return e.API.GetUrl()
}

// GetRequestTrackingTableName returns the request tracking table name
func (e *EventDrivenAPI) GetRequestTrackingTableName() *string {
	if e.RequestTrackingTable != nil {
		return e.RequestTrackingTable.GetTableName()
	}
	return nil
}

// GrantRequestTrackingAccess grants read/write access to the request tracking table
func (e *EventDrivenAPI) GrantRequestTrackingAccess(grantee awslambda.IFunction) {
	if e.RequestTrackingTable != nil {
		e.RequestTrackingTable.GrantReadWrite(grantee)
	}
}

// AddAPIRoute adds a new route to the API
func (e *EventDrivenAPI) AddAPIRoute(path *string, method string, handler awslambda.IFunction) {
	httpMethod := awsapigatewayv2.HttpMethod(method)
	e.API.AddLambdaRoute(path, httpMethod, handler)
}