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

// WithDefaultErrorNotifications creates an SNS notifier using the centralized
// cross-account topic pattern used by all Pay Theory services (matching Python services).
//
// This publishes to the main Pay Theory account (805600764437) for centralized error monitoring
// across all Pay Theory services and partners. The topic ARN format is:
// arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-{stage}
//
// Supported stages: paytheory, paytheorylab, paytheorystudy
// Defaults to paytheory for unknown stages.
//
// This is the recommended method for all Lift-based services.
func WithDefaultErrorNotifications(snsClient SNSClient) *SNSNotifier {
	stage := os.Getenv("STAGE")

	// Default to paytheory if stage not set
	if stage == "" {
		stage = "paytheory"
	}

	// Map stage to the appropriate cross-account SNS topic
	var topicARN string
	switch stage {
	case "paytheorylab":
		topicARN = "arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-paytheorylab"
	case "paytheorystudy":
		topicARN = "arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-paytheorystudy"
	default:
		// Default to paytheory for production and unknown stages
		topicARN = "arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-paytheory"
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
