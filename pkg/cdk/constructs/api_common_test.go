package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/jsii-runtime-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORSHelpersReturnExpectedValues(t *testing.T) {
	headers := CORSHeaders()
	require.NotNil(t, headers)
	headerValues := derefStrings(headers)
	assert.Contains(t, headerValues, "Content-Type")
	assert.Contains(t, headerValues, "Authorization")
	assert.Contains(t, headerValues, "X-Request-ID")

	exposeHeaders := CORSExposeHeaders()
	require.NotNil(t, exposeHeaders)
	exposeValues := derefStrings(exposeHeaders)
	assert.ElementsMatch(t, []string{
		"X-Request-ID",
		"X-Rate-Limit-Limit",
		"X-Rate-Limit-Remaining",
		"X-Rate-Limit-Reset",
	}, exposeValues)

	methods := CORSMethods()
	assert.ElementsMatch(t, []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, methods)
}

func TestSplitPathHandlesVariousPatterns(t *testing.T) {
	tests := map[string][]string{
		"":           {},
		"/":          {},
		"foo":        {"foo"},
		"/foo":       {"foo"},
		"foo/bar":    {"foo", "bar"},
		"/foo/bar/":  {"foo", "bar"},
		"//a//b///c": {"a", "b", "c"},
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, SplitPath(input))
		})
	}
}

func TestCreateAPILogGroupReusesExistingGroup(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)
	existing := awslogs.NewLogGroup(stack, jsii.String("Existing"), nil)

	result := CreateAPILogGroup(stack, jsii.String("my-api"), existing)
	assert.Same(t, existing, result)
}

func TestCreateAPILogGroupCreatesNewGroupWithExpectedName(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	logGroup := CreateAPILogGroup(stack, jsii.String("my-api"), nil)
	require.NotNil(t, logGroup)

	template := assertions.Template_FromStack(stack, nil)
	template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]any{
		"LogGroupName":    "/aws/apigateway/my-api",
		"RetentionInDays": float64(7),
	})
}

func derefStrings(values *[]*string) []string {
	result := make([]string, 0, len(*values))
	for _, v := range *values {
		result = append(result, *v)
	}
	return result
}
