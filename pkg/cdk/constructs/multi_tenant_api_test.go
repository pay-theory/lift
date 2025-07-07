package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
)

// Helper function to create test resources
func createTestResources(t *testing.T, stackName string) (awscdk.Stack, awscognito.IUserPool) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String(stackName), nil)

	userPool := awscognito.NewUserPool(stack, jsii.String("TestUserPool"), &awscognito.UserPoolProps{
		UserPoolName: jsii.String("test-pool"),
	})

	return stack, userPool
}

func TestMultiTenantAPI(t *testing.T) {

	t.Run("creates API with basic configuration", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack1")

		// Create MultiTenantAPI
		api := NewMultiTenantAPI(stack, "TestAPI", &MultiTenantAPIProps{
			APIName:    jsii.String("test-api"),
			Code:       awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:    jsii.String("bootstrap"),
			Runtime:    awslambda.Runtime_PROVIDED_AL2(),
			MemorySize: jsii.Number(512),
			Timeout:    awscdk.Duration_Seconds(jsii.Number(30)),
			UserPool:   userPool,
		})

		assert.NotNil(t, api)
		assert.NotNil(t, api.API)
		assert.NotNil(t, api.Table)

		// Verify template
		template := assertions.Template_FromStack(stack, nil)

		// Check API Gateway exists
		template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Api"), map[string]interface{}{
			"Name":     "test-api",
			"ProtocolType": "HTTP",
		})

		// Check table exists with standard pk/sk naming
		template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
			"TableName": "test-api-table",
			"BillingMode": "PAY_PER_REQUEST",
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
	})

	t.Run("creates rate limit table when enabled", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack2")

		// Create MultiTenantAPI with rate limiting
		api := NewMultiTenantAPI(stack, "TestAPIRateLimit", &MultiTenantAPIProps{
			APIName:                  jsii.String("test-api-rate"),
			Code:                     awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:                  jsii.String("bootstrap"),
			Runtime:                  awslambda.Runtime_PROVIDED_AL2(),
			UserPool:                 userPool,
			EnableTenantRateLimiting: jsii.Bool(true),
			TenantRateLimit:          jsii.Number(500),
		})

		assert.NotNil(t, api.RateLimitTable)

		// Verify template
		template := assertions.Template_FromStack(stack, nil)

		// Check rate limit table exists
		template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
			"TableName": "test-api-rate-rate-limits",
			"TimeToLiveSpecification": map[string]interface{}{
				"AttributeName": "ttl",
				"Enabled": true,
			},
		})
	})

	t.Run("configures JWT tenant isolation", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack3")

		// Create MultiTenantAPI with JWT isolation
		NewMultiTenantAPI(stack, "TestAPIJWT", &MultiTenantAPIProps{
			APIName:                  jsii.String("test-api-jwt"),
			Code:                     awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:                  jsii.String("bootstrap"),
			Runtime:                  awslambda.Runtime_PROVIDED_AL2(),
			UserPool:                 userPool,
			EnableJWTTenantIsolation: jsii.Bool(true),
			TenantIDClaim:            jsii.String("custom:org_id"),
		})

		// Verify environment variables
		template := assertions.Template_FromStack(stack, nil)
		// Find the Lambda function and check it has the expected environment variables
		templateJSON := template.ToJSON()
		jsonMap := *templateJSON
		resources := jsonMap["Resources"].(map[string]interface{})
		
		// Find the Lambda function resource
		var foundEnvVars bool
		for _, resource := range resources {
			resourceMap := resource.(map[string]interface{})
			if resourceMap["Type"] == "AWS::Lambda::Function" {
				props := resourceMap["Properties"].(map[string]interface{})
				if env, ok := props["Environment"].(map[string]interface{}); ok {
					if vars, ok := env["Variables"].(map[string]interface{}); ok {
						if vars["TENANT_ISOLATION_MODE"] == "jwt" && 
						   vars["TENANT_ID_CLAIM"] == "custom:org_id" &&
						   vars["LIFT_MULTI_TENANT"] == "true" {
							foundEnvVars = true
							break
						}
					}
				}
			}
		}
		assert.True(t, foundEnvVars, "Lambda function should have tenant isolation environment variables")
	})

	t.Run("configures header tenant isolation", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack4")

		// Create MultiTenantAPI with header isolation
		NewMultiTenantAPI(stack, "TestAPIHeader", &MultiTenantAPIProps{
			APIName:                     jsii.String("test-api-header"),
			Code:                        awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:                     jsii.String("bootstrap"),
			Runtime:                     awslambda.Runtime_PROVIDED_AL2(),
			UserPool:                    userPool,
			EnableHeaderTenantIsolation: jsii.Bool(true),
			TenantIDHeader:              jsii.String("X-Org-ID"),
		})

		// Verify environment variables
		template := assertions.Template_FromStack(stack, nil)
		// Find the Lambda function and check it has the expected environment variables
		templateJSON := template.ToJSON()
		jsonMap := *templateJSON
		resources := jsonMap["Resources"].(map[string]interface{})
		
		// Find the Lambda function resource
		var foundEnvVars bool
		for _, resource := range resources {
			resourceMap := resource.(map[string]interface{})
			if resourceMap["Type"] == "AWS::Lambda::Function" {
				props := resourceMap["Properties"].(map[string]interface{})
				if env, ok := props["Environment"].(map[string]interface{}); ok {
					if vars, ok := env["Variables"].(map[string]interface{}); ok {
						if vars["TENANT_ISOLATION_MODE"] == "header" && 
						   vars["TENANT_ID_HEADER"] == "X-Org-ID" &&
						   vars["LIFT_MULTI_TENANT"] == "true" {
							foundEnvVars = true
							break
						}
					}
				}
			}
		}
		assert.True(t, foundEnvVars, "Lambda function should have header tenant isolation environment variables")
	})

	t.Run("configures path-based tenant isolation", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack5")

		// Create MultiTenantAPI with path isolation
		NewMultiTenantAPI(stack, "TestAPIPath", &MultiTenantAPIProps{
			APIName:                   jsii.String("test-api-path"),
			Code:                      awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:                   jsii.String("bootstrap"),
			Runtime:                   awslambda.Runtime_PROVIDED_AL2(),
			UserPool:                  userPool,
			EnablePathTenantIsolation: jsii.Bool(true),
		})

		// Verify route configuration
		template := assertions.Template_FromStack(stack, nil)
		template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Route"), map[string]interface{}{
			"RouteKey": "ANY /tenants/{tenantId}/{proxy+}",
		})
	})

	t.Run("creates WAF when enabled", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack6")

		// Create MultiTenantAPI with WAF
		api := NewMultiTenantAPI(stack, "TestAPIWAF", &MultiTenantAPIProps{
			APIName:   jsii.String("test-api-waf"),
			Code:      awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:   jsii.String("bootstrap"),
			Runtime:   awslambda.Runtime_PROVIDED_AL2(),
			UserPool:  userPool,
			EnableWAF: jsii.Bool(true),
		})

		assert.NotNil(t, api.WebACL)

		// Verify template
		template := assertions.Template_FromStack(stack, nil)

		// Check WAF WebACL exists
		template.HasResourceProperties(jsii.String("AWS::WAFv2::WebACL"), map[string]interface{}{
			"Name":  "test-api-waf-waf",
			"Scope": "REGIONAL",
		})

		// Check WAF association exists
		template.HasResourceProperties(jsii.String("AWS::WAFv2::WebACLAssociation"), map[string]interface{}{})
	})

	t.Run("configures access logging when enabled", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack7")

		// Create MultiTenantAPI with access logging
		NewMultiTenantAPI(stack, "TestAPILogs", &MultiTenantAPIProps{
			APIName:             jsii.String("test-api-logs"),
			Code:                awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:             jsii.String("bootstrap"),
			Runtime:             awslambda.Runtime_PROVIDED_AL2(),
			UserPool:            userPool,
			EnableAccessLogging: jsii.Bool(true),
		})

		// Verify template
		template := assertions.Template_FromStack(stack, nil)

		// Check log group exists
		template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]interface{}{
			"LogGroupName": "/aws/apigateway/test-api-logs/access",
		})
	})

	t.Run("configures custom domain", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack8")

		// Create MultiTenantAPI with custom domain
		NewMultiTenantAPI(stack, "TestAPIDomain", &MultiTenantAPIProps{
			APIName:  jsii.String("test-api-domain"),
			Code:     awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:  jsii.String("bootstrap"),
			Runtime:  awslambda.Runtime_PROVIDED_AL2(),
			UserPool: userPool,
			CustomDomain: &CustomDomainConfig{
				DomainName:     "api.example.com",
				CertificateArn: "arn:aws:acm:us-east-1:123456789012:certificate/abc",
				BasePath:       "v1",
			},
		})

		// Verify template
		template := assertions.Template_FromStack(stack, nil)

		// Check domain name exists
		template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::DomainName"), map[string]interface{}{
			"DomainName": "api.example.com",
		})

		// Check API mapping exists
		template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::ApiMapping"), map[string]interface{}{
			"ApiMappingKey": "v1",
		})
	})

	t.Run("configures CORS", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack9")

		// Create MultiTenantAPI with CORS
		NewMultiTenantAPI(stack, "TestAPICORS", &MultiTenantAPIProps{
			APIName:  jsii.String("test-api-cors"),
			Code:     awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:  jsii.String("bootstrap"),
			Runtime:  awslambda.Runtime_PROVIDED_AL2(),
			UserPool: userPool,
			CorsConfig: &CorsConfig{
				AllowOrigins:     []string{"https://example.com"},
				AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
				AllowHeaders:     []string{"Content-Type", "Authorization"},
				AllowCredentials: true,
				MaxAge:           awscdk.Duration_Hours(jsii.Number(1)),
			},
		})

		// Verify template
		template := assertions.Template_FromStack(stack, nil)

		// Check API has CORS configuration
		template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Api"), map[string]interface{}{
			"CorsConfiguration": map[string]interface{}{
				"AllowOrigins":     []interface{}{"https://example.com"},
				"AllowMethods":     []interface{}{"GET", "POST", "PUT", "DELETE"},
				"AllowHeaders":     []interface{}{"Content-Type", "Authorization"},
				"AllowCredentials": true,
			},
		})
	})

	t.Run("configures throttling", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack10")

		// Create MultiTenantAPI with throttling
		NewMultiTenantAPI(stack, "TestAPIThrottle", &MultiTenantAPIProps{
			APIName:  jsii.String("test-api-throttle"),
			Code:     awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:  jsii.String("bootstrap"),
			Runtime:  awslambda.Runtime_PROVIDED_AL2(),
			UserPool: userPool,
			ThrottleConfig: &ThrottleConfig{
				RateLimit:  100,
				BurstLimit: 200,
			},
		})

		// Verify template
		template := assertions.Template_FromStack(stack, nil)

		// Check stage has throttle settings
		template.HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Stage"), map[string]interface{}{
			"ThrottleSettings": map[string]interface{}{
				"RateLimit":  100,
				"BurstLimit": 200,
			},
		})
	})

	t.Run("passes environment variables to Lambda", func(t *testing.T) {
		stack, userPool := createTestResources(t, "TestStack11")

		// Create MultiTenantAPI with custom environment
		NewMultiTenantAPI(stack, "TestAPIEnv", &MultiTenantAPIProps{
			APIName:  jsii.String("test-api-env"),
			Code:     awslambda.Code_FromAsset(jsii.String("."), nil),
			Handler:  jsii.String("bootstrap"),
			Runtime:  awslambda.Runtime_PROVIDED_AL2(),
			UserPool: userPool,
			Environment: map[string]*string{
				"CUSTOM_VAR": jsii.String("custom-value"),
			},
		})

		// Verify environment variables
		template := assertions.Template_FromStack(stack, nil)
		// Find the Lambda function and check it has the expected environment variables
		templateJSON := template.ToJSON()
		jsonMap := *templateJSON
		resources := jsonMap["Resources"].(map[string]interface{})
		
		// Find the Lambda function resource
		var foundEnvVars bool
		for _, resource := range resources {
			resourceMap := resource.(map[string]interface{})
			if resourceMap["Type"] == "AWS::Lambda::Function" {
				props := resourceMap["Properties"].(map[string]interface{})
				if env, ok := props["Environment"].(map[string]interface{}); ok {
					if vars, ok := env["Variables"].(map[string]interface{}); ok {
						if vars["CUSTOM_VAR"] == "custom-value" {
							foundEnvVars = true
							break
						}
					}
				}
			}
		}
		assert.True(t, foundEnvVars, "Lambda function should have custom environment variables")
	})
}

func TestGetTenantIsolationMode(t *testing.T) {
	tests := []struct {
		name     string
		props    *MultiTenantAPIProps
		expected string
	}{
		{
			name: "no isolation",
			props: &MultiTenantAPIProps{
				EnableJWTTenantIsolation:    jsii.Bool(false),
				EnableHeaderTenantIsolation: jsii.Bool(false),
				EnablePathTenantIsolation:   jsii.Bool(false),
			},
			expected: "none",
		},
		{
			name: "JWT isolation",
			props: &MultiTenantAPIProps{
				EnableJWTTenantIsolation: jsii.Bool(true),
			},
			expected: "jwt",
		},
		{
			name: "header isolation",
			props: &MultiTenantAPIProps{
				EnableHeaderTenantIsolation: jsii.Bool(true),
			},
			expected: "header",
		},
		{
			name: "path isolation",
			props: &MultiTenantAPIProps{
				EnablePathTenantIsolation: jsii.Bool(true),
			},
			expected: "path",
		},
		{
			name: "multiple modes - returns first",
			props: &MultiTenantAPIProps{
				EnableJWTTenantIsolation:    jsii.Bool(true),
				EnableHeaderTenantIsolation: jsii.Bool(true),
			},
			expected: "jwt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTenantIsolationMode(tt.props)
			assert.Equal(t, tt.expected, result)
		})
	}
}