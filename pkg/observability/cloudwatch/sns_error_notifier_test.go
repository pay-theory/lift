package cloudwatch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudWatchSNSErrorNotifierOptions(t *testing.T) {
	opts := WithErrorNotifications(nil, "arn:aws:sns:us-east-1:123456789012:topic")
	require.NotNil(t, opts.Notifier)
	require.Equal(t, "arn:aws:sns:us-east-1:123456789012:topic", opts.Notifier.GetTopicARN())

	t.Run("env var not set returns nil notifier", func(t *testing.T) {
		t.Setenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN", "")
		opts := WithEnvironmentErrorNotifications(nil)
		require.Nil(t, opts.Notifier)
	})

	t.Run("env var set returns notifier", func(t *testing.T) {
		t.Setenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:123456789012:topic2")
		opts := WithEnvironmentErrorNotifications(nil)
		require.NotNil(t, opts.Notifier)
		require.Equal(t, "arn:aws:sns:us-east-1:123456789012:topic2", opts.Notifier.GetTopicARN())
	})
}
