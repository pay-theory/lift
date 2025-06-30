package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

const (
	// DefaultSNSTargetARN is the default SNS target for error notifications
	DefaultSNSTargetARN = "arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-paytheory"
)

// WithErrorNotifications creates CloudWatch logger options with SNS error notifications
func WithErrorNotifications(snsClient *sns.Client, topicARN string) CloudWatchLoggerOptions {
	if topicARN == "" {
		topicARN = DefaultSNSTargetARN
	}
	
	notifier := NewSNSNotifier(SNSConfig{
		Client:   snsClient,
		TopicARN: topicARN,
	})
	
	return CloudWatchLoggerOptions{
		Notifier: notifier,
	}
}

// WithDefaultErrorNotifications creates CloudWatch logger options with default SNS error notifications
func WithDefaultErrorNotifications(snsClient *sns.Client) CloudWatchLoggerOptions {
	return WithErrorNotifications(snsClient, DefaultSNSTargetARN)
}