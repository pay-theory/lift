package observability

import (
	"context"
	"fmt"
	"os"
)

// WithSNSNotifier creates an option with the specified SNS notifier
func WithSNSNotifier(notifier *SNSNotifier) interface{} {
	return notifier
}

// WithEnvironmentErrorNotifications creates an SNS notifier using the topic ARN
// from the ERROR_NOTIFICATION_SNS_TOPIC_ARN environment variable.
//
// This is the recommended method for configuring SNS error notifications.
// Returns nil if the environment variable is not set.
//
// Example usage:
//
//	export ERROR_NOTIFICATION_SNS_TOPIC_ARN="arn:aws:sns:us-east-1:123456789012:my-error-topic"
//	notifier := observability.WithEnvironmentErrorNotifications(snsClient)
func WithEnvironmentErrorNotifications(snsClient SNSClient) *SNSNotifier {
	topicARN := os.Getenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN")
	if topicARN == "" {
		return nil
	}
	return WithErrorNotifications(snsClient, topicARN)
}

// WithPartnerErrorNotifications creates an SNS notifier with partner-specific configuration.
// It builds the SNS topic ARN using the partner-specific format: cns-{partner}-{stage}
//
// The AWS account ID is auto-detected using multiple strategies:
// 1. AWS_ACCOUNT_ID environment variable (if explicitly set)
// 2. STS GetCallerIdentity API call (works in any AWS environment)
//
// Returns nil if required environment variables are not set or account ID cannot be determined.
//
// Use this only if you need a partner-specific SNS topic instead of the centralized
// cross-account topic. Most services should use WithDefaultErrorNotifications instead.
func WithPartnerErrorNotifications(snsClient SNSClient) *SNSNotifier {
	partner := os.Getenv("PARTNER")
	stage := os.Getenv("STAGE")
	region := os.Getenv("AWS_REGION")

	// Auto-detect AWS account ID using fallback strategies
	accountID := getAWSAccountID(context.Background())

	if partner == "" || stage == "" || region == "" || accountID == "" {
		// Return nil if environment variables are not set or account ID cannot be determined
		return nil
	}

	// Build the partner-specific SNS topic ARN: arn:aws:sns:{region}:{account}:cns-{partner}-{stage}
	topicARN := fmt.Sprintf("arn:aws:sns:%s:%s:cns-%s-%s", region, accountID, partner, stage)

	return WithErrorNotifications(snsClient, topicARN)
}

// WithErrorNotifications creates an SNS notifier with the specified topic ARN
func WithErrorNotifications(snsClient SNSClient, topicARN string) *SNSNotifier {
	config := SNSConfig{
		Client:   snsClient,
		TopicARN: topicARN,
	}

	return NewSNSNotifier(config)
}
