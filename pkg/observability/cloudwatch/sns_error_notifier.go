package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/pay-theory/lift/pkg/observability"
)

// WithErrorNotifications creates CloudWatch logger options with SNS error notifications
func WithErrorNotifications(snsClient *sns.Client, topicARN string) CloudWatchLoggerOptions {
	notifier := observability.WithErrorNotifications(snsClient, topicARN)

	return CloudWatchLoggerOptions{
		Notifier: notifier,
	}
}

// WithEnvironmentErrorNotifications creates CloudWatch logger options with SNS error notifications
// using the topic ARN from the ERROR_NOTIFICATION_SNS_TOPIC_ARN environment variable.
func WithEnvironmentErrorNotifications(snsClient *sns.Client) CloudWatchLoggerOptions {
	notifier := observability.WithEnvironmentErrorNotifications(snsClient)

	return CloudWatchLoggerOptions{
		Notifier: notifier,
	}
}
