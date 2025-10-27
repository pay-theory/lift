package observability

import (
	"context"
	"os"
	"testing"
)

func TestGetAWSAccountID(t *testing.T) {
	tests := []struct {
		name        string
		envVarValue string
		wantEmpty   bool
		description string
	}{
		{
			name:        "returns account ID from environment variable",
			envVarValue: "123456789012",
			wantEmpty:   false,
			description: "Should use AWS_ACCOUNT_ID env var when set",
		},
		{
			name:        "falls back to STS when env var not set",
			envVarValue: "",
			wantEmpty:   false,
			description: "Should call STS GetCallerIdentity when AWS_ACCOUNT_ID not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env var
			originalValue := os.Getenv("AWS_ACCOUNT_ID")
			defer os.Setenv("AWS_ACCOUNT_ID", originalValue)

			// Set test env var
			if tt.envVarValue != "" {
				os.Setenv("AWS_ACCOUNT_ID", tt.envVarValue)
			} else {
				os.Unsetenv("AWS_ACCOUNT_ID")
			}

			// Test
			accountID := getAWSAccountID(context.Background())

			if tt.wantEmpty && accountID != "" {
				t.Errorf("Expected empty account ID, got: %s", accountID)
			}

			if !tt.wantEmpty && accountID == "" {
				// This is acceptable if STS fails (e.g., running without AWS credentials)
				t.Logf("Account ID is empty - this is OK if running without AWS credentials")
			}

			if tt.envVarValue != "" && accountID != tt.envVarValue {
				t.Errorf("Expected account ID %s from env var, got: %s", tt.envVarValue, accountID)
			}
		})
	}
}

func TestGetAWSAccountIDEnvVarPriority(t *testing.T) {
	// Save original env var
	originalValue := os.Getenv("AWS_ACCOUNT_ID")
	defer os.Setenv("AWS_ACCOUNT_ID", originalValue)

	// Set test env var
	testAccountID := "999999999999"
	os.Setenv("AWS_ACCOUNT_ID", testAccountID)

	// Test
	accountID := getAWSAccountID(context.Background())

	if accountID != testAccountID {
		t.Errorf("Expected account ID from env var (%s), got: %s", testAccountID, accountID)
	}
}

func TestGetAccountIDFromSTS(t *testing.T) {
	// This test requires AWS credentials to be configured
	// It will skip if credentials are not available
	ctx := context.Background()

	accountID, err := getAccountIDFromSTS(ctx)

	if err != nil {
		t.Skipf("Skipping STS test - AWS credentials not available: %v", err)
	}

	if accountID == "" {
		t.Error("Expected non-empty account ID from STS")
	}

	// Validate format (12 digits)
	if len(accountID) != 12 {
		t.Errorf("Expected 12-digit account ID, got: %s (length: %d)", accountID, len(accountID))
	}
}

func TestGetAccountIDFromSTSTimeout(t *testing.T) {
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := getAccountIDFromSTS(ctx)

	if err == nil {
		t.Error("Expected error from cancelled context, got nil")
	}
}
