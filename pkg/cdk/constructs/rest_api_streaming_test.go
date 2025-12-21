package constructs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestLiftRestAPI_EnablesStreamingIntegration(t *testing.T) {
	stack := test.NewTestStack()

	timeoutSeconds := 15 * 60
	api := NewLiftRestAPI(stack.Stack(), jsii.String("TestRestAPI"), &LiftRestAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-rest-api"),
		},
		EnableStreaming:  jsii.Bool(true),
		StreamingTimeout: &timeoutSeconds,
	})

	fn := awslambda.NewFunction(stack.Stack(), jsii.String("StreamFn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
	})

	api.AddLambdaIntegration(jsii.String("/stream"), jsii.String("GET"), fn)

	template := assertions.Template_FromStack(stack.Stack(), nil)
	raw, err := json.Marshal(template.ToJSON())
	require.NoError(t, err)

	templateJSON := string(raw)
	assert.Contains(t, templateJSON, "response-streaming-invocations")
	assert.Contains(t, templateJSON, "2021-11-15/functions")
	assert.Contains(t, templateJSON, "\"ResponseTransferMode\":\"STREAM\"")
	assert.Contains(t, templateJSON, "\"TimeoutInMillis\":900000")
}

func TestLiftRestAPI_AllowsMixedStreamingAndBufferedMethods(t *testing.T) {
	stack := test.NewTestStack()

	propsTimeoutSeconds := 10 * 60
	api := NewLiftRestAPI(stack.Stack(), jsii.String("TestRestAPI"), &LiftRestAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-rest-api"),
		},
		EnableStreaming:  jsii.Bool(false),
		StreamingTimeout: &propsTimeoutSeconds,
	})

	fn := awslambda.NewFunction(stack.Stack(), jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
	})

	api.AddLambdaIntegrationWithOptions(jsii.String("/stream"), jsii.String("GET"), fn, &IntegrationOptions{
		EnableStreaming: jsii.Bool(true),
	})
	api.AddLambdaIntegration(jsii.String("/buffered"), jsii.String("GET"), fn)

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGateway::Method"), jsii.Number(2))

	methods := template.FindResources(jsii.String("AWS::ApiGateway::Method"), nil)
	require.NotNil(t, methods)

	streamingFound := false
	bufferedFound := false
	for _, res := range *methods {
		propsAny, ok := (*res)["Properties"]
		require.True(t, ok, "expected Properties on method resource")

		props, ok := propsAny.(map[string]any)
		require.True(t, ok, "expected method Properties to be an object")

		integrationAny, ok := props["Integration"]
		require.True(t, ok, "expected Integration on method Properties")

		integration, ok := integrationAny.(map[string]any)
		require.True(t, ok, "expected Integration to be an object")

		uriJSON, err := json.Marshal(integration["Uri"])
		require.NoError(t, err)
		uriStr := string(uriJSON)

		switch {
		case strings.Contains(uriStr, "response-streaming-invocations"):
			assert.Equal(t, "STREAM", integration["ResponseTransferMode"])
			assert.Equal(t, float64(600000), integration["TimeoutInMillis"])
			streamingFound = true
		case strings.Contains(uriStr, "/invocations"):
			_, hasTransferMode := integration["ResponseTransferMode"]
			assert.False(t, hasTransferMode, "buffered integration should not set ResponseTransferMode")
			_, hasTimeout := integration["TimeoutInMillis"]
			assert.False(t, hasTimeout, "buffered integration should not set TimeoutInMillis")
			bufferedFound = true
		}
	}

	assert.True(t, streamingFound, "expected to find a streaming method integration")
	assert.True(t, bufferedFound, "expected to find a buffered method integration")
}

func TestLiftRestAPI_GlobalStreamingCanBeDisabledPerMethod(t *testing.T) {
	stack := test.NewTestStack()

	api := NewLiftRestAPI(stack.Stack(), jsii.String("TestRestAPI"), &LiftRestAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-rest-api"),
		},
		EnableStreaming: jsii.Bool(true),
	})

	fn := awslambda.NewFunction(stack.Stack(), jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
	})

	api.AddLambdaIntegration(jsii.String("/stream"), jsii.String("GET"), fn)
	api.AddLambdaIntegrationWithOptions(jsii.String("/buffered"), jsii.String("GET"), fn, &IntegrationOptions{
		EnableStreaming: jsii.Bool(false),
	})

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGateway::Method"), jsii.Number(2))

	methods := template.FindResources(jsii.String("AWS::ApiGateway::Method"), nil)
	require.NotNil(t, methods)

	streamingFound := false
	bufferedFound := false
	for _, res := range *methods {
		propsAny, ok := (*res)["Properties"]
		require.True(t, ok, "expected Properties on method resource")

		props, ok := propsAny.(map[string]any)
		require.True(t, ok, "expected method Properties to be an object")

		integrationAny, ok := props["Integration"]
		require.True(t, ok, "expected Integration on method Properties")

		integration, ok := integrationAny.(map[string]any)
		require.True(t, ok, "expected Integration to be an object")

		uriJSON, err := json.Marshal(integration["Uri"])
		require.NoError(t, err)
		uriStr := string(uriJSON)

		switch {
		case strings.Contains(uriStr, "response-streaming-invocations"):
			assert.Equal(t, "STREAM", integration["ResponseTransferMode"])
			assert.Equal(t, float64(900000), integration["TimeoutInMillis"])
			streamingFound = true
		case strings.Contains(uriStr, "/invocations"):
			_, hasTransferMode := integration["ResponseTransferMode"]
			assert.False(t, hasTransferMode, "buffered integration should not set ResponseTransferMode")
			_, hasTimeout := integration["TimeoutInMillis"]
			assert.False(t, hasTimeout, "buffered integration should not set TimeoutInMillis")
			bufferedFound = true
		}
	}

	assert.True(t, streamingFound, "expected to find a streaming method integration")
	assert.True(t, bufferedFound, "expected to find a buffered method integration")
}

func TestLiftRestAPI_PerMethodStreamingTimeoutOverridesAndClamps(t *testing.T) {
	stack := test.NewTestStack()

	propsTimeoutSeconds := 1
	api := NewLiftRestAPI(stack.Stack(), jsii.String("TestRestAPI"), &LiftRestAPIProps{
		APICommonProps: APICommonProps{
			Name: jsii.String("test-rest-api"),
		},
		EnableStreaming:  jsii.Bool(false),
		StreamingTimeout: &propsTimeoutSeconds,
	})

	fn := awslambda.NewFunction(stack.Stack(), jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("."), nil),
	})

	shortTimeoutSeconds := 5
	longTimeoutSeconds := 9999
	api.AddLambdaIntegrationWithOptions(jsii.String("/short"), jsii.String("GET"), fn, &IntegrationOptions{
		EnableStreaming:         jsii.Bool(true),
		StreamingTimeoutSeconds: &shortTimeoutSeconds,
	})
	api.AddLambdaIntegrationWithOptions(jsii.String("/clamped"), jsii.String("GET"), fn, &IntegrationOptions{
		EnableStreaming:         jsii.Bool(true),
		StreamingTimeoutSeconds: &longTimeoutSeconds,
	})
	api.AddLambdaIntegration(jsii.String("/buffered"), jsii.String("GET"), fn)

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::ApiGateway::Method"), jsii.Number(3))

	methods := template.FindResources(jsii.String("AWS::ApiGateway::Method"), nil)
	require.NotNil(t, methods)

	shortFound := false
	clampedFound := false
	bufferedFound := false

	for _, res := range *methods {
		propsAny, ok := (*res)["Properties"]
		require.True(t, ok, "expected Properties on method resource")

		props, ok := propsAny.(map[string]any)
		require.True(t, ok, "expected method Properties to be an object")

		integrationAny, ok := props["Integration"]
		require.True(t, ok, "expected Integration on method Properties")

		integration, ok := integrationAny.(map[string]any)
		require.True(t, ok, "expected Integration to be an object")

		uriJSON, err := json.Marshal(integration["Uri"])
		require.NoError(t, err)
		uriStr := string(uriJSON)

		switch {
		case strings.Contains(uriStr, "response-streaming-invocations"):
			assert.Equal(t, "STREAM", integration["ResponseTransferMode"])
			switch integration["TimeoutInMillis"] {
			case float64(5000):
				shortFound = true
			case float64(900000):
				clampedFound = true
			default:
				assert.Fail(t, "unexpected streaming TimeoutInMillis", "got %v", integration["TimeoutInMillis"])
			}
		case strings.Contains(uriStr, "/invocations"):
			_, hasTransferMode := integration["ResponseTransferMode"]
			assert.False(t, hasTransferMode, "buffered integration should not set ResponseTransferMode")
			_, hasTimeout := integration["TimeoutInMillis"]
			assert.False(t, hasTimeout, "buffered integration should not set TimeoutInMillis")
			bufferedFound = true
		}
	}

	assert.True(t, shortFound, "expected to find a streaming method with per-method timeout override")
	assert.True(t, clampedFound, "expected to find a streaming method with clamped timeout")
	assert.True(t, bufferedFound, "expected to find a buffered method integration")
}
