package test

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

// TestStack provides utilities for testing CDK stacks
type TestStack struct {
	t        *testing.T
	app      awscdk.App
	stack    awscdk.Stack
	template assertions.Template
}

// NewTestStack creates a new test stack
func NewTestStack() *TestStack {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	return &TestStack{
		app:   app,
		stack: stack,
	}
}

// NewTestStackWithTesting creates a new test stack with testing context
func NewTestStackWithTesting(t *testing.T) *TestStack {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	return &TestStack{
		t:     t,
		app:   app,
		stack: stack,
	}
}

// Stack returns the CDK stack for adding constructs
func (ts *TestStack) Stack() awscdk.Stack {
	return ts.stack
}

// Synthesize synthesizes the stack and creates a template for assertions
func (ts *TestStack) Synthesize() {
	ts.template = assertions.Template_FromStack(ts.stack, nil)
}

// Template returns the synthesized template for assertions
func (ts *TestStack) Template() assertions.Template {
	if ts.template == nil {
		ts.Synthesize()
	}
	return ts.template
}

// AssertHasResource asserts that a resource of the given type exists
func (ts *TestStack) AssertHasResource(resourceType string) {
	ts.Template().HasResource(jsii.String(resourceType), &map[string]interface{}{})
}

// AssertHasResourceWithProperties asserts that a resource exists with specific properties
func (ts *TestStack) AssertHasResourceWithProperties(resourceType string, props map[string]interface{}) {
	ts.Template().HasResourceProperties(jsii.String(resourceType), &props)
}

// AssertResourceCount asserts the count of resources of a specific type
func (ts *TestStack) AssertResourceCount(resourceType string, count float64) {
	ts.Template().ResourceCountIs(jsii.String(resourceType), jsii.Number(count))
}

// AssertHasOutput asserts that a stack output exists
func (ts *TestStack) AssertHasOutput(outputName string) {
	outputs := ts.Template().FindOutputs(jsii.String("*"), nil)
	if outputs == nil {
		ts.t.Errorf("No outputs found in stack")
		return
	}

	if _, ok := (*outputs)[outputName]; !ok {
		ts.t.Errorf("Output %s not found in stack", outputName)
	}
}

// AssertLambdaFunction asserts Lambda function properties
func (ts *TestStack) AssertLambdaFunction(functionName string, runtime string, architecture string) {
	ts.Template().HasResourceProperties(jsii.String("AWS::Lambda::Function"), &map[string]interface{}{
		"FunctionName":  functionName,
		"Runtime":       runtime,
		"Architectures": []string{architecture},
	})
}

// AssertDynamoDBTable asserts DynamoDB table properties
func (ts *TestStack) AssertDynamoDBTable(tableName string, billingMode string) {
	ts.Template().HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &map[string]interface{}{
		"TableName":   tableName,
		"BillingMode": billingMode,
	})
}

// AssertAPIGateway asserts API Gateway properties
func (ts *TestStack) AssertAPIGateway(apiName string) {
	ts.Template().HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Api"), &map[string]interface{}{
		"Name":         apiName,
		"ProtocolType": "HTTP",
	})
}

// GetResource retrieves a resource by logical ID
func (ts *TestStack) GetResource(logicalId string) map[string]interface{} {
	templateJSON := ts.Template().ToJSON()
	if templateMap, ok := (*templateJSON)["Resources"].(map[string]interface{}); ok {
		if resource, ok := templateMap[logicalId].(map[string]interface{}); ok {
			return resource
		}
	}
	return nil
}

// PrintTemplate prints the synthesized template for debugging
func (ts *TestStack) PrintTemplate() {
	template := ts.Template().ToJSON()
	jsonBytes, err := json.MarshalIndent(*template, "", "  ")
	if err != nil && ts.t != nil {
		ts.t.Logf("Failed to marshal template: %v", err)
		return
	}
	if ts.t != nil {
		ts.t.Logf("Stack Template:\n%s", string(jsonBytes))
	}
}

// LiftStackTester provides specialized testing for Lift CDK constructs
type LiftStackTester struct {
	*TestStack
}

// NewLiftStackTester creates a new Lift-specific stack tester
func NewLiftStackTester(t *testing.T) *LiftStackTester {
	return &LiftStackTester{
		TestStack: NewTestStackWithTesting(t),
	}
}

// AssertLiftFunction asserts Lift Lambda function configuration
func (lst *LiftStackTester) AssertLiftFunction(props map[string]interface{}) {
	defaultProps := map[string]interface{}{
		"Runtime":       "provided.al2023",
		"Handler":       "bootstrap",
		"Architectures": []string{"arm64"},
	}

	// Merge provided props with defaults
	for k, v := range props {
		defaultProps[k] = v
	}

	lst.Template().HasResourceProperties(jsii.String("AWS::Lambda::Function"), &defaultProps)
}

// AssertLiftAPI asserts Lift API Gateway configuration
func (lst *LiftStackTester) AssertLiftAPI(apiName string, hasCORS bool) {
	lst.Template().HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Api"), &map[string]interface{}{
		"Name":         apiName,
		"ProtocolType": "HTTP",
	})

	if hasCORS {
		lst.Template().HasResourceProperties(jsii.String("AWS::ApiGatewayV2::Api"), &map[string]interface{}{
			"CorsConfiguration": map[string]interface{}{
				"AllowOrigins": []string{"*"},
				"AllowMethods": []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				"AllowHeaders": []string{"Content-Type", "Authorization", "X-Tenant-ID", "X-Request-ID", "X-Api-Key"},
			},
		})
	}
}

// AssertLiftTable asserts Lift DynamoDB table configuration
func (lst *LiftStackTester) AssertLiftTable(tableName string, _ bool, hasStreams bool) {
	tableProps := map[string]interface{}{
		"TableName": tableName,
		"AttributeDefinitions": []map[string]string{
			{"AttributeName": "pk", "AttributeType": "S"},
			{"AttributeName": "sk", "AttributeType": "S"},
		},
		"KeySchema": []map[string]string{
			{"AttributeName": "pk", "KeyType": "HASH"},
			{"AttributeName": "sk", "KeyType": "RANGE"},
		},
	}

	// Note: GSIs are now handled by DynamORM through struct tags at runtime,
	// so we don't expect GSI attributes in the CDK template anymore

	if hasStreams {
		tableProps["StreamSpecification"] = map[string]interface{}{
			"StreamViewType": "NEW_AND_OLD_IMAGES",
		}
	}

	lst.Template().HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), &tableProps)
}

// AssertCompleteInfrastructure asserts a complete Lift app infrastructure
func (lst *LiftStackTester) AssertCompleteInfrastructure(appName string, hasDatabase bool, hasRateLimiting bool) {
	// Lambda function
	lst.AssertLiftFunction(map[string]interface{}{
		"FunctionName": appName,
	})

	// API Gateway
	lst.AssertLiftAPI(appName+"-api", true)

	// API routes
	lst.AssertHasResource("AWS::ApiGatewayV2::Route")
	lst.AssertHasResource("AWS::ApiGatewayV2::Integration")

	// Database table
	if hasDatabase {
		lst.AssertLiftTable(appName+"-table", true, true)
	}

	// Rate limiting table
	if hasRateLimiting {
		lst.AssertHasResourceWithProperties("AWS::DynamoDB::Table", map[string]interface{}{
			"TableName": appName + "-rate-limits",
		})
	}

	// IAM permissions
	lst.AssertHasResource("AWS::IAM::Role")
	lst.AssertHasResource("AWS::IAM::Policy")

	// CloudWatch logs
	lst.AssertHasResource("AWS::Logs::LogGroup")
}

// MockCDKContext provides a mock CDK context for testing
type MockCDKContext struct {
	Values map[string]interface{}
}

// NewMockCDKContext creates a new mock CDK context
func NewMockCDKContext() *MockCDKContext {
	return &MockCDKContext{
		Values: make(map[string]interface{}),
	}
}

// Set sets a context value
func (m *MockCDKContext) Set(key string, value interface{}) {
	m.Values[key] = value
}

// Get gets a context value
func (m *MockCDKContext) Get(key string) interface{} {
	return m.Values[key]
}
