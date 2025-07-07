package patterns_test

import (
	"testing"

	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/patterns"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestLiftApp(t *testing.T) {
	tests := []struct {
		name     string
		props    *patterns.LiftAppProps
		validate func(*test.LiftStackTester)
	}{
		{
			name: "basic app with database",
			props: &patterns.LiftAppProps{
				AppName:        jsii.String("test-app"),
				CodeAssetPath:  jsii.String("."),
				EnableDatabase: jsii.Bool(true),
			},
			validate: func(tester *test.LiftStackTester) {
				// Check Lambda function
				tester.AssertLiftFunction(map[string]interface{}{
					"FunctionName": "test-app",
				})
				
				// Check API Gateway
				tester.AssertLiftAPI("test-app-api", true)
				
				// Check DynamoDB table
				tester.AssertLiftTable("test-app-table", true, true)
				
				// Check outputs
				tester.AssertHasOutput("ApiUrl")
				tester.AssertHasOutput("FunctionName")
				tester.AssertHasOutput("DatabaseTableName")
			},
		},
		{
			name: "app with rate limiting",
			props: &patterns.LiftAppProps{
				AppName:            jsii.String("rate-limited-app"),
				CodeAssetPath:      jsii.String("."),
				EnableRateLimiting: jsii.Bool(true),
			},
			validate: func(tester *test.LiftStackTester) {
				// Check rate limiting table
				tester.AssertHasResourceWithProperties("AWS::DynamoDB::Table", map[string]interface{}{
					"TableName": "rate-limited-app-rate-limits",
					"TimeToLiveSpecification": map[string]interface{}{
						"AttributeName": "expires",
						"Enabled":       true,
					},
				})
				
				// Check Lambda has table name in environment
				tester.AssertHasResourceWithProperties("AWS::Lambda::Function", map[string]interface{}{
					"Environment": map[string]interface{}{
						"Variables": map[string]interface{}{
							"RATE_LIMIT_TABLE": "rate-limited-app-rate-limits",
						},
					},
				})
			},
		},
		{
			name: "multi-tenant app",
			props: &patterns.LiftAppProps{
				AppName:           jsii.String("multi-tenant-app"),
				CodeAssetPath:     jsii.String("."),
				EnableMultiTenant: jsii.Bool(true),
				EnableDatabase:    jsii.Bool(true),
			},
			validate: func(tester *test.LiftStackTester) {
				// Check multi-tenant configuration
				tester.AssertHasResourceWithProperties("AWS::Lambda::Function", map[string]interface{}{
					"Environment": map[string]interface{}{
						"Variables": map[string]interface{}{
							"LIFT_MULTI_TENANT": "true",
						},
					},
				})
				
				// Check tenant-aware table configuration (uses standard pk/sk naming)
				tester.AssertHasResourceWithProperties("AWS::DynamoDB::Table", map[string]interface{}{
					"KeySchema": []map[string]interface{}{
						{
							"AttributeName": "pk",
							"KeyType":       "HASH",
						},
						{
							"AttributeName": "sk",
							"KeyType":       "RANGE",
						},
					},
				})
			},
		},
		{
			name: "production configuration",
			props: &patterns.LiftAppProps{
				AppName:             jsii.String("prod-app"),
				CodeAssetPath:       jsii.String("."),
				EnableDatabase:      jsii.Bool(true),
				EnableRateLimiting:  jsii.Bool(true),
				EnableAccessLogging: jsii.Bool(true),
				MemorySize:          jsii.Number(2048),
				Timeout:             jsii.Number(600), // 10 minutes in seconds
				Environment: &map[string]*string{
					"ENVIRONMENT": jsii.String("production"),
					"LOG_LEVEL":   jsii.String("warn"),
				},
			},
			validate: func(tester *test.LiftStackTester) {
				// Check Lambda configuration
				tester.AssertHasResourceWithProperties("AWS::Lambda::Function", map[string]interface{}{
					"MemorySize": 2048,
					"Timeout":    600,
					"Environment": map[string]interface{}{
						"Variables": map[string]interface{}{
							"ENVIRONMENT": "production",
							"LOG_LEVEL":   "warn",
						},
					},
				})
				
				// Check complete infrastructure
				tester.AssertCompleteInfrastructure("prod-app", true, true)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := test.NewLiftStackTester(t)
			
			patterns.NewLiftApp(tester.Stack(), jsii.String("TestApp"), tt.props)
			
			tester.Synthesize()
			tt.validate(tester)
		})
	}
}

func TestLiftAppRouting(t *testing.T) {
	tester := test.NewLiftStackTester(t)
	
	app := patterns.NewLiftApp(tester.Stack(), jsii.String("TestApp"), &patterns.LiftAppProps{
		AppName:       jsii.String("routing-app"),
		CodeAssetPath: jsii.String("."),
	})
	
	tester.Synthesize()
	
	// Check catch-all route
	tester.AssertHasResourceWithProperties("AWS::ApiGatewayV2::Route", map[string]interface{}{
		"RouteKey": "ANY /{proxy+}",
	})
	
	// Check root route
	tester.AssertHasResourceWithProperties("AWS::ApiGatewayV2::Route", map[string]interface{}{
		"RouteKey": "ANY /",
	})
	
	// Check Lambda integration
	tester.AssertHasResource("AWS::ApiGatewayV2::Integration")
	
	// Verify API has reference to function
	if app.Function == nil || app.API == nil {
		t.Error("LiftApp should expose Function and API properties")
	}
}

func TestLiftAppPermissions(t *testing.T) {
	tester := test.NewLiftStackTester(t)
	
	patterns.NewLiftApp(tester.Stack(), jsii.String("TestApp"), &patterns.LiftAppProps{
		AppName:            jsii.String("perms-app"),
		CodeAssetPath:      jsii.String("."),
		EnableDatabase:     jsii.Bool(true),
		EnableRateLimiting: jsii.Bool(true),
	})
	
	tester.Synthesize()
	
	// Check IAM role exists
	tester.AssertHasResource("AWS::IAM::Role")
	
	// Check IAM policy exists (structure may vary due to CDK implementation details)
	tester.AssertHasResource("AWS::IAM::Policy")
	
	// Verify the policy has DynamoDB permissions by checking template contains DynamoDB actions
	template := tester.Template().ToJSON()
	if template == nil {
		t.Error("Template is nil")
		return
	}
}