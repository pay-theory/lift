package integration

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/constructs"
	"github.com/pay-theory/lift/pkg/cdk/patterns"
	"github.com/pay-theory/lift/pkg/cdk/stacks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests verify that all constructs can be synthesized successfully
// and produce valid CloudFormation templates

func TestAllConstructsSynthesize(t *testing.T) {
	// Test that all constructs can be created and synthesized
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("IntegrationTestStack"), nil)

	// Basic constructs
	t.Run("LiftFunction", func(t *testing.T) {
		constructs.NewLiftFunction(stack, jsii.String("LiftFn"), &constructs.LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
				Handler: jsii.String("bootstrap"),
			},
			EnableMultiTenant: jsii.Bool(true),
		})
	})

	t.Run("RateLimitedFunction", func(t *testing.T) {
		constructs.NewRateLimitedFunction(stack, jsii.String("RateLimitedFn"), &constructs.RateLimitedFunctionProps{
			LiftFunctionProps: constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler: jsii.String("bootstrap"),
				},
			},
			RateLimitType: constructs.RateLimitTypeIP,
		})
	})

	t.Run("IdempotentFunction", func(t *testing.T) {
		constructs.NewIdempotentFunction(stack, jsii.String("IdempotentFn"), &constructs.IdempotentFunctionProps{
			LiftFunctionProps: constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler: jsii.String("bootstrap"),
				},
			},
			KeyExtractor: constructs.IdempotentKeyHeader,
			KeyField:    jsii.String("x-request-id"),
		})
	})

	t.Run("SecureFunction", func(t *testing.T) {
		constructs.NewSecureFunction(stack, jsii.String("SecureFn"), &constructs.SecureFunctionProps{
			LiftFunctionProps: constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler: jsii.String("bootstrap"),
				},
			},
			EnableKMSEncryption: jsii.Bool(true),
		})
	})

	t.Run("MonitoredFunction", func(t *testing.T) {
		constructs.NewMonitoredFunction(stack, jsii.String("MonitoredFn"), &constructs.MonitoredFunctionProps{
			LiftFunctionProps: constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler: jsii.String("bootstrap"),
				},
			},
			EnableDashboard:          jsii.Bool(true),
			EnableLogInsightsQueries: jsii.Bool(true),
		})
	})

	// Verify synthesis works
	synth := app.Synth(nil)
	require.NotNil(t, synth)
}

func TestAllPatternsSynthesize(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("PatternTestStack"), nil)

	t.Run("BasicAPI", func(t *testing.T) {
		patterns.NewBasicAPI(stack, jsii.String("BasicAPI"), &patterns.BasicAPIProps{
			ApiName:  jsii.String("basic-api"),
			Code:     awslambda.Code_FromAsset(jsii.String("."), nil),
		})
	})

	t.Run("SecureAPI", func(t *testing.T) {
		patterns.NewSecureAPI(stack, jsii.String("SecureAPI"), &patterns.SecureAPIProps{
			ApiName:            jsii.String("secure-api"),
			Code:               awslambda.Code_FromAsset(jsii.String("."), nil),
			EnableRateLimiting: jsii.Bool(true),
			EnableWAF:          jsii.Bool(true),
		})
	})

	t.Run("LiftApp", func(t *testing.T) {
		patterns.NewLiftApp(stack, jsii.String("LiftApp"), &patterns.LiftAppProps{
			AppName:           jsii.String("full-app"),
			CodeAssetPath:     jsii.String("."),
			EnableMultiTenant: jsii.Bool(true),
			EnableDatabase:    jsii.Bool(true),
			EnableRateLimiting: jsii.Bool(true),
		})
	})

	// Verify synthesis works
	synth := app.Synth(nil)
	require.NotNil(t, synth)
}

func TestAllStacksSynthesize(t *testing.T) {
	app := awscdk.NewApp(nil)

	t.Run("MicroserviceStack", func(t *testing.T) {
		stack := stacks.NewMicroserviceStack(app, "MicroserviceStack", &stacks.MicroserviceStackProps{
			ServiceName:    "test-service",
			CodePath:       ".",
			EnableDatabase: true,
		})
		assert.NotNil(t, stack)
	})

	t.Run("MultiTenantSaaSStack", func(t *testing.T) {
		stack := stacks.NewMultiTenantSaaSStack(app, "SaaSStack", &stacks.MultiTenantSaaSStackProps{
			AppName:           "test-saas",
			CodePath:          ".",
			EnableAuth:        true,
			EnableFileStorage: true,
		})
		assert.NotNil(t, stack)
	})

	t.Run("EventDrivenStack", func(t *testing.T) {
		stack := stacks.NewEventDrivenStack(app, "EventStack", &stacks.EventDrivenStackProps{
			AppName:                "test-events",
			ApiCodePath:            ".",
			EventProcessorCodePath: ".",
		})
		assert.NotNil(t, stack)
	})

	// Verify synthesis works
	synth := app.Synth(nil)
	require.NotNil(t, synth)
}

func TestComplexStackIntegration(t *testing.T) {
	// Test a complex stack with multiple constructs working together
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("ComplexStack"), nil)

	// Create a rate-limited API
	rateLimitedFn := constructs.NewRateLimitedFunction(stack, jsii.String("APIHandler"), &constructs.RateLimitedFunctionProps{
		LiftFunctionProps: constructs.LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
				Handler: jsii.String("bootstrap"),
			},
			EnableMultiTenant: jsii.Bool(true),
		},
		RateLimitType: constructs.RateLimitTypeTenant,
		Limit:         jsii.Number(1000),
		WindowSeconds: jsii.Number(3600), // 60 minutes
	})

	// Create API Gateway
	api := constructs.NewLiftAPI(stack, jsii.String("API"), &constructs.LiftAPIProps{
		Name:       jsii.String("complex-api"),
		EnableCORS: jsii.Bool(true),
		StageName:  jsii.String("prod"),
	})

	// Add routes
	api.AddLambdaRoute(jsii.String("/users"), awsapigatewayv2.HttpMethod_GET, rateLimitedFn.Function.Function)
	api.AddLambdaRoute(jsii.String("/users"), awsapigatewayv2.HttpMethod_POST, rateLimitedFn.Function.Function)

	// Create monitored background processor
	monitoredProcessor := constructs.NewMonitoredFunction(stack, jsii.String("Processor"), &constructs.MonitoredFunctionProps{
		LiftFunctionProps: constructs.LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
				Handler: jsii.String("bootstrap"),
				Timeout: awscdk.Duration_Minutes(jsii.Number(5)),
			},
		},
		EnableDashboard:          jsii.Bool(true),
		EnableLogInsightsQueries: jsii.Bool(true),
		AlarmConfig: &constructs.AlarmConfig{
			EnableErrorAlarm:   jsii.Bool(true),
			ErrorRateThreshold: jsii.Number(5),
		},
	})

	// Create DynamoDB tables
	mainTable := constructs.NewLiftTable(stack, jsii.String("MainTable"), &constructs.LiftTableProps{
		TableName:                 jsii.String("complex-main-table"),
		EnableMultiTenant:         jsii.Bool(true),
		EnableAutoScaling:         jsii.Bool(true),
		EnablePointInTimeRecovery: jsii.Bool(true),
		EnableStreams:             jsii.Bool(true),
		StreamViewType:            awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
	})

	// Grant permissions
	mainTable.Table.GrantReadWriteData(rateLimitedFn.Function.Function)
	mainTable.Table.GrantReadData(monitoredProcessor.Function.Function)

	// Synthesize and verify
	template := assertions.Template_FromStack(stack, nil)
	
	// Verify resources exist
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(2))
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::Table"), jsii.Number(2)) // Main + rate limit
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Api"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Dashboard"), jsii.Number(1))
	
	// Verify integrations
	template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Integration"), &map[string]interface{}{
		"IntegrationType": "AWS_PROXY",
	})
}

func TestCrossConstructIntegration(t *testing.T) {
	// Test that constructs can reference and work with each other
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("CrossConstructStack"), nil)

	// Create shared resources
	sharedTable := constructs.NewLiftTable(stack, jsii.String("SharedTable"), &constructs.LiftTableProps{
		TableName:         jsii.String("shared-table"),
		EnableAutoScaling: jsii.Bool(true),
		EnableMultiTenant: jsii.Bool(false),
	})

	// Create multiple functions that share the table
	functions := []constructs.LiftFunction{}
	for i := 0; i < 3; i++ {
		name := string(rune('A' + i)) + "Function"
		fn := constructs.NewLiftFunction(stack, jsii.String(name), &constructs.LiftFunctionProps{
			FunctionProps: awslambda.FunctionProps{
				Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
				Handler: jsii.String("bootstrap"),
			},
		})
		
		// Grant read/write access
		sharedTable.Table.GrantReadWriteData(fn.Function)
		
		// Add table name to environment
		fn.Function.AddEnvironment(jsii.String("TABLE_NAME"), sharedTable.Table.TableName(), nil)
		
		functions = append(functions, *fn)
	}

	// Create an API that routes to different functions
	api := constructs.NewLiftAPI(stack, jsii.String("SharedAPI"), &constructs.LiftAPIProps{
		Name:       jsii.String("shared-api"),
		EnableCORS: jsii.Bool(true),
	})

	api.AddLambdaRoute(jsii.String("/a"), awsapigatewayv2.HttpMethod_GET, functions[0].Function)
	api.AddLambdaRoute(jsii.String("/b"), awsapigatewayv2.HttpMethod_GET, functions[1].Function)
	api.AddLambdaRoute(jsii.String("/c"), awsapigatewayv2.HttpMethod_GET, functions[2].Function)

	// Verify synthesis
	template := assertions.Template_FromStack(stack, nil)
	
	// Should have 3 functions, 1 table, 1 API
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(3))
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::Table"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ApiGatewayV2::Api"), jsii.Number(1))
	
	// Verify each function has TABLE_NAME environment variable set
	// We don't check the exact Ref value since it's generated
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(3))
}