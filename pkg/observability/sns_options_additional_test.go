package observability

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSNSOptions_ConstructorsAndARNHelpers(t *testing.T) {
	t.Setenv("AWS_ACCOUNT_ID", "123456789012") // avoid STS fallback in getAWSAccountID
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("PARTNER", "partner")
	t.Setenv("STAGE", "lab")

	// WithSNSNotifier is a thin wrapper.
	require.Nil(t, WithSNSNotifier(nil))

	t.Setenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN", "")
	require.Nil(t, WithEnvironmentErrorNotifications(nil))

	t.Setenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:123456789012:error-topic")
	n := WithEnvironmentErrorNotifications(nil)
	require.NotNil(t, n)
	require.Equal(t, "arn:aws:sns:us-east-1:123456789012:error-topic", n.GetTopicARN())

	pn := WithPartnerErrorNotifications(nil)
	require.NotNil(t, pn)
	require.Contains(t, pn.GetTopicARN(), ":cns-partner-lab")

	en := WithErrorNotifications(nil, "arn:aws:sns:us-east-1:123456789012:explicit-topic")
	require.NotNil(t, en)
	require.Equal(t, "arn:aws:sns:us-east-1:123456789012:explicit-topic", en.GetTopicARN())
}

func TestSNSNotifier_EnvAndFormatHelpers(t *testing.T) {
	require.Equal(t, "default", getEnvOrDefault("DOES_NOT_EXIST", "default"))
	t.Setenv("FOO", "bar")
	require.Equal(t, "bar", getEnvOrDefault("FOO", "default"))

	require.Equal(t, "{}", formatLogFieldsJSON(nil))
	require.Equal(t, "{}", formatLogFieldsJSON(map[string]any{"bad": make(chan int)}))
	require.Contains(t, formatLogFieldsJSON(map[string]any{"k": "v"}), `"k":"v"`)

	msg := formatLogMessage(&LogEntry{
		Level:     "ERROR",
		Message:   "boom",
		Fields:    map[string]any{"k": "v"},
		Timestamp: time.Unix(1, 0).UTC(),
	})
	require.Contains(t, msg, "ERROR")
	require.Contains(t, msg, "boom")

	notification := &SNSNotificationMessage{}
	applyNotificationOverrides(notification, map[string]any{
		"environment": "env",
		"service":     "svc",
		"function":    "fn",
		"region":      "r1",
		"aws_account": "a1",
	})
	require.Equal(t, "env", notification.Environment)
	require.Equal(t, "svc", notification.Service)
	require.Equal(t, "fn", notification.Function)
	require.Equal(t, "fn", notification.Subsystem)
	require.Equal(t, "r1", notification.AWSRegion)
	require.Equal(t, "a1", notification.AWSAccount)
}

func TestLambdaARNParsingAndContextExtraction(t *testing.T) {
	region, account, fn := parseLambdaARN("bad-arn")
	require.Equal(t, "", region)
	require.Equal(t, "", account)
	require.Equal(t, "", fn)

	region, account, fn = parseLambdaARN("arn:aws:lambda:us-east-1:123456789012:function:my-func")
	require.Equal(t, "us-east-1", region)
	require.Equal(t, "123456789012", account)
	require.Equal(t, "my-func", fn)

	// Prefer explicit environment overrides.
	t.Setenv("AWS_LAMBDA_FUNCTION_ARN", "arn:aws:lambda:us-east-1:111111111111:function:arn-func")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ACCOUNT_ID", "999999999999")

	ctxInfo := extractLambdaExecutionContext("explicit-func")
	require.Equal(t, "us-west-2", ctxInfo.region)
	require.Equal(t, "999999999999", ctxInfo.account)
	require.Equal(t, "explicit-func", ctxInfo.function)

	// Fall back to parsing when function name is unknown.
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_ACCOUNT_ID", "")
	ctxInfo = extractLambdaExecutionContext(unknownValue)
	require.Equal(t, "us-east-1", ctxInfo.region)
	require.Equal(t, "111111111111", ctxInfo.account)
	require.Equal(t, "arn-func", ctxInfo.function)

	// Defensive: ensure no panics when context is unused.
	_ = context.Background()
}
