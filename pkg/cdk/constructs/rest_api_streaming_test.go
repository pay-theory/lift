package constructs

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
