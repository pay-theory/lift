package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
)

// NOTE: Using test helper functions from sqs_processor_test.go

func TestWebSocketAPI_DefaultConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName: jsii.String("test-websocket-api"),
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	template := synthesizeTemplate(stack)

	// Verify WebSocket API is created
	assertResourceExists(t, template, "AWS::ApiGatewayV2::Api", map[string]interface{}{
		"Name":                     "test-websocket-api",
		"Description":              "Lift WebSocket API with DynamORM",
		"ProtocolType":             "WEBSOCKET",
		"RouteSelectionExpression": "$request.body.action",
	})

	// Verify stage is created
	assertResourceExists(t, template, "AWS::ApiGatewayV2::Stage", map[string]interface{}{
		"StageName":  "prod",
		"AutoDeploy": true,
	})

	// Verify connection table is created with standard PK/SK structure (uppercase as per ConnectionTable implementation)
	assertResourceExists(t, template, "AWS::DynamoDB::Table", map[string]interface{}{
		"TableName": "test-websocket-api-connections",
		"KeySchema": []interface{}{
			map[string]interface{}{
				"AttributeName": "PK",
				"KeyType":       "HASH",
			},
			map[string]interface{}{
				"AttributeName": "SK",
				"KeyType":       "RANGE",
			},
		},
		"BillingMode": "PAY_PER_REQUEST",
		"PointInTimeRecoverySpecification": map[string]interface{}{
			"PointInTimeRecoveryEnabled": true,
		},
		"TimeToLiveSpecification": map[string]interface{}{
			"AttributeName": "ttl",
			"Enabled":       true,
		},
	})

	// Verify Lambda functions are created
	functions := findResourcesByType(template, "AWS::Lambda::Function")
	if len(functions) != 3 {
		t.Errorf("Expected 3 Lambda functions, got %d", len(functions))
	}

	// Verify routes are created
	routes := findResourcesByType(template, "AWS::ApiGatewayV2::Route")
	expectedRoutes := []string{"$connect", "$disconnect", "$default"}
	if len(routes) != len(expectedRoutes) {
		t.Errorf("Expected %d routes, got %d", len(expectedRoutes), len(routes))
	}

	// Verify log group is created
	assertResourceExists(t, template, "AWS::Logs::LogGroup", map[string]interface{}{
		"LogGroupName":    "/aws/apigateway/websocket/test-websocket-api",
		"RetentionInDays": 30,
	})

	// Verify construct properties
	if wsApi.WebSocketApi == nil {
		t.Error("WebSocket API should not be nil")
	}
	if wsApi.Stage == nil {
		t.Error("Stage should not be nil")
	}
	if wsApi.ConnectionTable == nil {
		t.Error("Connection table should not be nil")
	}
	if wsApi.ConnectFunction == nil {
		t.Error("Connect function should not be nil")
	}
	if wsApi.DisconnectFunction == nil {
		t.Error("Disconnect function should not be nil")
	}
	if wsApi.DefaultFunction == nil {
		t.Error("Default function should not be nil")
	}
}

func TestWebSocketAPI_CustomConfiguration(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName:                  jsii.String("custom-api"),
		Description:              jsii.String("Custom WebSocket API"),
		RouteSelectionExpression: jsii.String("$request.body.type"),
		StageName:                jsii.String("dev"),
		AutoDeploy:               jsii.Bool(false),
		EnableAccessLogging:      jsii.Bool(false),
		EnableMultiTenant:        jsii.Bool(false),
		ThrottleRateLimit:        jsii.Number(1000),
		ThrottleBurstLimit:       jsii.Number(2000),
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("custom-websocket"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
			Timeout:      awscdk.Duration_Seconds(jsii.Number(60)),
		},
	})

	template := synthesizeTemplate(stack)

	// Verify custom API configuration
	assertResourceExists(t, template, "AWS::ApiGatewayV2::Api", map[string]interface{}{
		"Name":                     "custom-api",
		"Description":              "Custom WebSocket API",
		"RouteSelectionExpression": "$request.body.type",
	})

	// Verify custom stage configuration
	assertResourceExists(t, template, "AWS::ApiGatewayV2::Stage", map[string]interface{}{
		"StageName":  "dev",
		"AutoDeploy": false,
	})

	// Verify no log group is created when access logging is disabled
	logGroups := findResourcesByType(template, "AWS::Logs::LogGroup")
	// Only Lambda log groups should exist, not API Gateway access log group
	for _, logGroup := range logGroups {
		if props, ok := logGroup["Properties"].(map[string]interface{}); ok {
			if logGroupName, ok := props["LogGroupName"].(string); ok {
				if logGroupName == "/aws/apigateway/websocket/custom-api" {
					t.Error("Access log group should not be created when access logging is disabled")
				}
			}
		}
	}

	// Verify connection table without GSIs (multi-tenant disabled)
	tables := findResourcesByType(template, "AWS::DynamoDB::Table")
	if len(tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(tables))
	}

	// Check that table has basic structure but no GSIs for multi-tenant
	table := tables[0]
	if props, ok := table["Properties"].(map[string]interface{}); ok {
		if gsis, exists := props["GlobalSecondaryIndexes"]; exists {
			// Should not have GSIs when multi-tenant is disabled
			if gsiList, ok := gsis.([]interface{}); ok && len(gsiList) > 0 {
				t.Error("Table should not have GSIs when multi-tenant is disabled")
			}
		}
	}

	// Verify custom function timeout
	functions := findResourcesByType(template, "AWS::Lambda::Function")
	for _, function := range functions {
		if props, ok := function["Properties"].(map[string]interface{}); ok {
			if timeout, ok := props["Timeout"].(float64); ok && timeout != 60 {
				t.Errorf("Expected function timeout to be 60 seconds, got %f", timeout)
			}
		}
	}

	if wsApi == nil {
		t.Error("WebSocketAPI should not be nil")
	}
}

func TestWebSocketAPI_CustomConnectionTable(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName: jsii.String("test-api"),
		ConnectionTableProps: &ConnectionTableProps{
			TableName: jsii.String("custom-connections"),
		},
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	template := synthesizeTemplate(stack)

	// Verify the custom table was created
	assertResourceExists(t, template, "AWS::DynamoDB::Table", map[string]interface{}{
		"TableName": "custom-connections",
	})

	// Verify construct has a connection table
	if wsApi.ConnectionTable == nil {
		t.Error("WebSocketAPI should have a connection table")
	}
	if wsApi.ConnectionTable.Table == nil {
		t.Error("ConnectionTable should have a DynamoDB table")
	}
}

func TestWebSocketAPI_DisabledConnectionManagement(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName:                    jsii.String("test-api"),
		EnableConnectionManagement: jsii.Bool(false),
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	template := synthesizeTemplate(stack)

	// Should not create connection table
	tables := findResourcesByType(template, "AWS::DynamoDB::Table")
	if len(tables) != 0 {
		t.Errorf("Expected 0 tables when connection management is disabled, got %d", len(tables))
	}

	// Verify no connection table in construct
	if wsApi.ConnectionTable != nil {
		t.Error("Connection table should be nil when connection management is disabled")
	}
}

func TestWebSocketAPI_CustomRoutes(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Create custom function for custom route
	customFunction := NewLiftFunction(stack, jsii.String("CustomFunction"), &LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("custom-route-handler"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
	})

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName: jsii.String("test-api"),
		Routes: []*WebSocketRouteConfig{
			{
				RouteKey: jsii.String("sendMessage"),
				Function: customFunction.Function,
			},
			{
				RouteKey: jsii.String("joinRoom"),
				Function: customFunction.Function,
			},
		},
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	template := synthesizeTemplate(stack)

	// Should have 5 routes total: $connect, $disconnect, $default + 2 custom
	routes := findResourcesByType(template, "AWS::ApiGatewayV2::Route")
	if len(routes) != 5 {
		t.Errorf("Expected 5 routes, got %d", len(routes))
	}

	// Verify custom routes are in construct
	if len(wsApi.Routes) != 5 {
		t.Errorf("Expected 5 routes in construct, got %d", len(wsApi.Routes))
	}

	if _, exists := wsApi.Routes["sendMessage"]; !exists {
		t.Error("sendMessage route should exist")
	}
	if _, exists := wsApi.Routes["joinRoom"]; !exists {
		t.Error("joinRoom route should exist")
	}
}

func TestWebSocketAPI_CustomTableProps(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName: jsii.String("test-api"),
		ConnectionTableProps: &ConnectionTableProps{
			TableName: jsii.String("custom-table-name"),
		},
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	template := synthesizeTemplate(stack)

	// Verify custom table configuration
	assertResourceExists(t, template, "AWS::DynamoDB::Table", map[string]interface{}{
		"TableName":   "custom-table-name",
		"BillingMode": "PAY_PER_REQUEST", // Default billing mode for LiftTable
	})

	if wsApi.ConnectionTable == nil {
		t.Error("Connection table should not be nil")
	}
}

func TestWebSocketAPI_EnvironmentVariables(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName:   jsii.String("test-api"),
		StageName: jsii.String("dev"),
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Add custom environment variable
	wsApi.AddEnvironmentVariable("CUSTOM_VAR", "custom_value")

	template := synthesizeTemplate(stack)

	// Find Lambda functions and verify environment variables
	functions := findResourcesByType(template, "AWS::Lambda::Function")
	for _, function := range functions {
		if props, ok := function["Properties"].(map[string]interface{}); ok {
			if env, ok := props["Environment"].(map[string]interface{}); ok {
				if variables, ok := env["Variables"].(map[string]interface{}); ok {
					// Check for required environment variables
					expectedVars := []string{
						"WEBSOCKET_API_ID",
						"WEBSOCKET_STAGE",
						"CONNECTION_TABLE_NAME",
						"CONNECTION_TABLE_ARN",
						"CUSTOM_VAR",
					}

					for _, expectedVar := range expectedVars {
						if _, exists := variables[expectedVar]; !exists {
							t.Errorf("Expected environment variable %s not found", expectedVar)
						}
					}

					// Verify custom variable value
					if customVar, ok := variables["CUSTOM_VAR"].(string); !ok || customVar != "custom_value" {
						t.Errorf("Expected CUSTOM_VAR to be 'custom_value', got %v", variables["CUSTOM_VAR"])
					}
				}
			}
		}
	}

	if wsApi == nil {
		t.Error("WebSocketAPI should not be nil")
	}
}

func TestWebSocketAPI_HelperMethods(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName: jsii.String("test-api"),
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Test GetConnectionTableName
	tableName := wsApi.GetConnectionTableName()
	if tableName == nil {
		t.Error("GetConnectionTableName should not return nil")
	}
	// Note: Table name will be a CDK token during testing, not the actual name

	// Test GetWebSocketURL
	wsUrl := wsApi.GetWebSocketURL()
	if wsUrl == nil {
		t.Error("GetWebSocketURL should not return nil")
	}
	// URL should contain API ID and stage
	expectedPattern := "wss://"
	if len(*wsUrl) < len(expectedPattern) || (*wsUrl)[:len(expectedPattern)] != expectedPattern {
		t.Errorf("WebSocket URL should start with 'wss://', got '%s'", *wsUrl)
	}

	// Test GrantConnectionManagement
	grantee := wsApi.ConnectFunction.Function
	grant := wsApi.GrantConnectionManagement(grantee)
	if grant == nil {
		t.Error("GrantConnectionManagement should return a grant")
	}

	// Test GrantApiInvoke
	grant = wsApi.GrantApiInvoke(grantee)
	if grant == nil {
		t.Error("GrantApiInvoke should return a grant")
	}
}

// TODO: Re-enable when AddRoute is fixed
/*
func TestWebSocketAPI_AddRouteMethod(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		ApiName: jsii.String("test-api"),
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	// Create and add a custom route
	customFunction := NewLiftFunction(stack, jsii.String("CustomFunction"), &LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("custom-handler"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
	})

	route := wsApi.AddRoute("customAction", customFunction.Function, &WebSocketRouteConfig{
		RouteKey: jsii.String("customAction"),
		Function: customFunction.Function,
	})

	// Verify route was added
	if route == nil {
		t.Error("AddRoute should return a route")
	}

	if _, exists := wsApi.Routes["customAction"]; !exists {
		t.Error("Custom route should be added to Routes map")
	}

	// Should now have 4 routes: $connect, $disconnect, $default, customAction
	if len(wsApi.Routes) != 4 {
		t.Errorf("Expected 4 routes after adding custom route, got %d", len(wsApi.Routes))
	}

	template := synthesizeTemplate(stack)
	routes := findResourcesByType(template, "AWS::ApiGatewayV2::Route")
	if len(routes) != 4 {
		t.Errorf("Expected 4 routes in CloudFormation template, got %d", len(routes))
	}
}
*/

func TestWebSocketAPI_MinimalProps(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	// Test with minimal required props
	wsApi := NewWebSocketAPI(stack, jsii.String("TestWebSocketAPI"), &WebSocketAPIProps{
		FunctionProps: awslambda.FunctionProps{
			Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler: jsii.String("index.handler"),
			Runtime: awslambda.Runtime_NODEJS_18_X(),
		},
	})

	template := synthesizeTemplate(stack)

	// Should create basic resources with defaults
	assertResourceExists(t, template, "AWS::ApiGatewayV2::Api", map[string]interface{}{
		"Name":        "WebSocketAPI",
		"Description": "Lift WebSocket API with DynamORM",
	})

	if wsApi == nil {
		t.Error("WebSocketAPI should not be nil")
	}
}
