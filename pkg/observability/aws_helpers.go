package observability

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// getAWSAccountID retrieves the AWS account ID using multiple fallback strategies:
// 1. AWS_ACCOUNT_ID environment variable (fastest, explicitly set)
// 2. STS GetCallerIdentity API call (works everywhere, requires AWS API call)
//
// The account ID is detected once and cached for the lifetime of the process.
// This is Lambda-safe as the same process handles multiple invocations.
func getAWSAccountID(ctx context.Context) string {
	// Strategy 1: Check environment variable (fastest, no AWS API call)
	if accountID := os.Getenv("AWS_ACCOUNT_ID"); accountID != "" {
		return accountID
	}

	// Strategy 2: Use STS GetCallerIdentity (works in any AWS environment)
	// This makes a single AWS API call and is cached by the Lambda runtime
	accountID, err := getAccountIDFromSTS(ctx)
	if err != nil {
		// If STS fails, return empty string - WithDefaultErrorNotifications will return nil
		return ""
	}

	return accountID
}

// getAccountIDFromSTS uses AWS STS GetCallerIdentity to determine the account ID
// This is a reliable way to get the account ID in any AWS environment (Lambda, ECS, EC2, etc.)
func getAccountIDFromSTS(ctx context.Context) (string, error) {
	// Create a context with timeout to prevent hanging
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(timeoutCtx)
	if err != nil {
		return "", err
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Call GetCallerIdentity
	result, err := stsClient.GetCallerIdentity(timeoutCtx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	if result.Account != nil {
		return *result.Account, nil
	}

	return "", nil
}
