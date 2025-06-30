package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/pay-theory/lift/pkg/observability"
)

const (
	// DefaultSNSTargetARN is the default SNS target for error notifications
	DefaultSNSTargetARN = "arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-paytheory"
)

// WithErrorNotifications configures the CloudWatch logger to send error notifications to SNS
func WithErrorNotifications(snsClient *sns.Client) CloudWatchLoggerOptions {
	return CloudWatchLoggerOptions{
		SNSClient: snsClient,
	}
}

// EnableErrorNotifications is a helper to enable SNS error notifications with default settings
func EnableErrorNotifications(config *observability.LoggerConfig) {
	config.EnableSNSNotifications = true
	if config.SNSTopicARN == "" {
		config.SNSTopicARN = DefaultSNSTargetARN
	}
}