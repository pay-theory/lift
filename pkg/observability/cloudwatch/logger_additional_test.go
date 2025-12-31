package cloudwatch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/pay-theory/lift/pkg/observability"
	"github.com/stretchr/testify/require"
)

type mockSNSClient struct {
	mu        sync.Mutex
	published []*sns.PublishInput
	err       error
}

func (m *mockSNSClient) Publish(_ context.Context, params *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *params
	m.published = append(m.published, &clone)
	if m.err != nil {
		return nil, m.err
	}
	return &sns.PublishOutput{}, nil
}

func (m *mockSNSClient) publishCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.published)
}

func TestCloudWatchLogger_DebugWarnAndSNSNotification(t *testing.T) {
	mockLogs := NewMockCloudWatchLogsClient()
	mockSNS := &mockSNSClient{err: errors.New("sns error")}
	notifier := observability.NewSNSNotifier(observability.SNSConfig{
		Client:   mockSNS,
		TopicARN: "arn:aws:sns:us-east-1:123456789012:topic",
	})

	config := observability.LoggerConfig{
		LogGroup:      "test-log-group",
		LogStream:     "test-log-stream",
		BatchSize:     1,
		FlushInterval: 25 * time.Millisecond,
		BufferSize:    10,
	}

	logger, err := NewCloudWatchLogger(config, mockLogs, CloudWatchLoggerOptions{Notifier: notifier})
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Debug("debug message", map[string]any{"a": "b"})
	logger.Warn("warn message")
	logger.Error("error message", map[string]any{"token": "secret"})

	require.Eventually(t, func() bool {
		return mockSNS.publishCount() > 0
	}, time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		stats := logger.GetStats()
		return stats.ErrorCount > 0 && stats.LastError != ""
	}, time.Second, 10*time.Millisecond)

	// Also cover notifier helpers that build options.
	opts := WithErrorNotifications(nil, "arn:aws:sns:us-east-1:123456789012:topic2")
	require.NotNil(t, opts.Notifier)
	require.Equal(t, "arn:aws:sns:us-east-1:123456789012:topic2", opts.Notifier.GetTopicARN())

	t.Setenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:123456789012:topic3")
	envOpts := WithEnvironmentErrorNotifications(nil)
	require.NotNil(t, envOpts.Notifier)
	require.Equal(t, "arn:aws:sns:us-east-1:123456789012:topic3", envOpts.Notifier.GetTopicARN())
}
