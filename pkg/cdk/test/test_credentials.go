package test

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// TestCredentials provides credentials for local testing
// These should only be used with local testing tools like DynamoDB Local
const (
	// DefaultTestAccessKeyID is used for local testing only
	DefaultTestAccessKeyID = "local-test-key"
	// DefaultTestSecretAccessKey is used for local testing only
	DefaultTestSecretAccessKey = "local-test-secret"
)

// GetTestCredentials returns appropriate credentials for testing
// For local testing (e.g., DynamoDB Local), it returns dummy credentials
// For real AWS testing, it returns nil to use default credential chain
func GetTestCredentials(isLocal bool) aws.CredentialsProvider {
	if !isLocal {
		// Use default AWS credential chain for real AWS
		return nil
	}

	// For local testing, check if custom credentials are provided
	accessKey := os.Getenv("LOCAL_TEST_AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("LOCAL_TEST_AWS_SECRET_ACCESS_KEY")

	if accessKey == "" {
		accessKey = DefaultTestAccessKeyID
	}
	if secretKey == "" {
		secretKey = DefaultTestSecretAccessKey
	}

	return aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			Source:          "LocalTestCredentials",
		}, nil
	})
}

// IsLocalTesting checks if we're running against local services
func IsLocalTesting() bool {
	// Check common environment variables for local testing
	return os.Getenv("DYNAMODB_LOCAL") == "true" ||
		os.Getenv("USE_LOCAL_STACK") == "true" ||
		os.Getenv("AWS_SAM_LOCAL") == "true" ||
		os.Getenv("TEST_ENV") == "local"
}
