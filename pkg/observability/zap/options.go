package zap

import (
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/pay-theory/lift/pkg/observability"
)

// WithSNSNotifier creates a ZapLoggerOptions with the specified SNS notifier
func WithSNSNotifier(notifier *observability.SNSNotifier) ZapLoggerOptions {
	return ZapLoggerOptions{
		Notifier: notifier,
	}
}

// WithDefaultErrorNotifications creates a ZapLoggerOptions with default SNS notifications for errors.
// This uses the centralized cross-account SNS topic pattern used by all Pay Theory services.
// This is the recommended method for all Lift-based services.
func WithDefaultErrorNotifications(snsClient *sns.Client) ZapLoggerOptions {
	notifier := observability.WithDefaultErrorNotifications(snsClient)

	return ZapLoggerOptions{
		Notifier: notifier,
	}
}

// WithPartnerErrorNotifications creates a ZapLoggerOptions with partner-specific SNS notifications.
// Use this only if you need a partner-specific SNS topic instead of the centralized cross-account topic.
// Most services should use WithDefaultErrorNotifications instead.
func WithPartnerErrorNotifications(snsClient *sns.Client) ZapLoggerOptions {
	notifier := observability.WithPartnerErrorNotifications(snsClient)

	return ZapLoggerOptions{
		Notifier: notifier,
	}
}

// WithErrorNotifications creates a ZapLoggerOptions with SNS notifications for errors
func WithErrorNotifications(snsClient *sns.Client, topicARN string) ZapLoggerOptions {
	notifier := observability.WithErrorNotifications(snsClient, topicARN)

	return ZapLoggerOptions{
		Notifier: notifier,
	}
}
