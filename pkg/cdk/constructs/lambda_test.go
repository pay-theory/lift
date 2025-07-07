package constructs_test

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/constructs"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestLiftFunction(t *testing.T) {
	tests := []struct {
		name     string
		props    *constructs.LiftFunctionProps
		validate func(*test.LiftStackTester)
	}{
		{
			name: "default configuration",
			props: &constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler: jsii.String("bootstrap"),
				},
			},
			validate: func(tester *test.LiftStackTester) {
				tester.AssertLiftFunction(map[string]interface{}{
					"Handler":     "bootstrap",
					"Runtime":     "provided.al2023",
					"MemorySize":  512,
					"Timeout":     30,
					"Architectures": []string{"arm64"},
				})
			},
		},
		{
			name: "with tracing enabled",
			props: &constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler: jsii.String("bootstrap"),
				},
				EnableTracing: jsii.Bool(true),
			},
			validate: func(tester *test.LiftStackTester) {
				tester.AssertHasResourceWithProperties("AWS::Lambda::Function", map[string]interface{}{
					"TracingConfig": map[string]interface{}{
						"Mode": "Active",
					},
				})
			},
		},
		{
			name: "with multi-tenant enabled",
			props: &constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler: jsii.String("bootstrap"),
				},
				EnableMultiTenant: jsii.Bool(true),
			},
			validate: func(tester *test.LiftStackTester) {
				tester.AssertHasResourceWithProperties("AWS::Lambda::Function", map[string]interface{}{
					"Environment": map[string]interface{}{
						"Variables": map[string]interface{}{
							"LIFT_MULTI_TENANT": "true",
							"LIFT_VERSION":      "1.0.0",
						},
					},
				})
			},
		},
		{
			name: "with custom memory and timeout",
			props: &constructs.LiftFunctionProps{
				FunctionProps: awslambda.FunctionProps{
					Code:       awslambda.Code_FromAsset(jsii.String("."), nil),
					Handler:    jsii.String("bootstrap"),
					MemorySize: jsii.Number(1024),
					Timeout:    awscdk.Duration_Minutes(jsii.Number(5)),
				},
			},
			validate: func(tester *test.LiftStackTester) {
				tester.AssertHasResourceWithProperties("AWS::Lambda::Function", map[string]interface{}{
					"MemorySize": 1024,
					"Timeout":    300,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := test.NewLiftStackTester(t)
			
			constructs.NewLiftFunction(tester.Stack(), jsii.String("TestFunction"), tt.props)
			
			tester.Synthesize()
			tt.validate(tester)
		})
	}
}

func TestLiftFunctionEnvironmentVariables(t *testing.T) {
	tester := test.NewLiftStackTester(t)
	
	customEnv := map[string]*string{
		"CUSTOM_VAR": jsii.String("custom_value"),
		"LOG_LEVEL":  jsii.String("debug"),
	}
	
	constructs.NewLiftFunction(tester.Stack(), jsii.String("TestFunction"), &constructs.LiftFunctionProps{
		FunctionProps: awslambda.FunctionProps{
			Code:        awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:     jsii.String("bootstrap"),
			Environment: &customEnv,
		},
		EnableMultiTenant: jsii.Bool(true),
	})
	
	tester.Synthesize()
	
	// Verify all environment variables are present
	tester.AssertHasResourceWithProperties("AWS::Lambda::Function", map[string]interface{}{
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"CUSTOM_VAR":        "custom_value",
				"LOG_LEVEL":         "debug",
				"LIFT_VERSION":      "1.0.0",
				"LIFT_MULTI_TENANT": "true",
			},
		},
	})
}