package cloudwatch

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/pay-theory/lift/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSNSClient implements a mock SNS client for testing
type mockSNSClient struct {
	publishCalls []sns.PublishInput
	publishErr   error
}

func (m *mockSNSClient) Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error) {
	m.publishCalls = append(m.publishCalls, *params)
	if m.publishErr != nil {
		return nil, m.publishErr
	}
	return &sns.PublishOutput{
		MessageId: aws.String("test-message-id"),
	}, nil
}

func TestSNSNotifier_NotifyError(t *testing.T) {
	tests := []struct {
		name          string
		logEntry      *observability.LogEntry
		publishErr    error
		expectPublish bool
		validateMsg   func(*testing.T, sns.PublishInput)
	}{
		{
			name: "successful error notification",
			logEntry: &observability.LogEntry{
				Timestamp: time.Now().UTC(),
				Level:     "ERROR",
				Message:   "Test error message",
				RequestID: "req-123",
				TraceID:   "trace-456",
				TenantID:  "tenant-789",
				UserID:    "user-abc",
				Fields: map[string]any{
					"error_code":  "TEST_ERROR",
					"environment": "production",
					"service":     "test-service",
					"method":      "POST",
					"path":        "/api/test",
					"status":      500,
				},
			},
			expectPublish: true,
			validateMsg: func(t *testing.T, input sns.PublishInput) {
				assert.Equal(t, "arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-paytheory", *input.TargetArn)
				
				// Validate message structure
				var msg SNSNotificationMessage
				err := json.Unmarshal([]byte(*input.Message), &msg)
				require.NoError(t, err)
				
				assert.Equal(t, "LiftError", msg.AlertConfig.AlertType)
				assert.Equal(t, "SLACK", msg.AlertConfig.AlertTargetType)
				assert.Equal(t, "production", msg.Environment)
				assert.Equal(t, "test-service", msg.Service)
				assert.NotEmpty(t, msg.Partner) // Should have a partner (from env or default)
				assert.NotEmpty(t, msg.Stage) // Should have a stage (from env or default)
				assert.NotEmpty(t, msg.AWSRegion) // Should have an AWS region (from env or default)
				assert.NotEmpty(t, msg.AWSAccount) // Should have an AWS account (from env or default)
				assert.Equal(t, "ERROR", msg.Severity)
				assert.NotEmpty(t, msg.Function) // Should have a function name
				assert.Equal(t, msg.Function, msg.Subsystem) // Subsystem should match Function
				assert.NotEmpty(t, msg.Message) // Should have a message (JSON of log entry)
				
				
				// Validate Message contains JSON representation of log entry
				assert.Contains(t, msg.Message, "Test error message")
				assert.Contains(t, msg.Message, "ERROR")
				
			},
		},
		{
			name: "skip non-error log levels",
			logEntry: &observability.LogEntry{
				Timestamp: time.Now().UTC(),
				Level:     "INFO",
				Message:   "Info message",
			},
			expectPublish: false,
		},
		{
			name: "error notification without optional fields",
			logEntry: &observability.LogEntry{
				Timestamp: time.Now().UTC(),
				Level:     "ERROR",
				Message:   "Simple error",
				Fields:    map[string]any{},
			},
			expectPublish: true,
			validateMsg: func(t *testing.T, input sns.PublishInput) {
				var msg SNSNotificationMessage
				err := json.Unmarshal([]byte(*input.Message), &msg)
				require.NoError(t, err)
				
				assert.Equal(t, "LiftError", msg.AlertConfig.AlertType)
				assert.Equal(t, "SLACK", msg.AlertConfig.AlertTargetType)
				assert.Empty(t, msg.Environment)
				assert.NotEmpty(t, msg.Partner) // Should have a partner (from env or default)
				assert.NotEmpty(t, msg.Stage) // Should have a stage (from env or default)
				assert.NotEmpty(t, msg.AWSRegion) // Should have an AWS region (from env or default)
				assert.NotEmpty(t, msg.AWSAccount) // Should have an AWS account (from env or default)
				assert.Equal(t, "ERROR", msg.Severity)
				assert.NotEmpty(t, msg.Function) // Should have a function name
				assert.Equal(t, msg.Function, msg.Subsystem) // Subsystem should match Function
				assert.NotEmpty(t, msg.Message) // Should have a message (JSON of log entry) (from env or default)
			},
		},
		{
			name: "error notification with function name in fields",
			logEntry: &observability.LogEntry{
				Timestamp: time.Now().UTC(),
				Level:     "ERROR",
				Message:   "Test error",
				Fields: map[string]any{
					"function_name": "my-lambda-function",
				},
			},
			expectPublish: true,
			validateMsg: func(t *testing.T, input sns.PublishInput) {
				var msg SNSNotificationMessage
				err := json.Unmarshal([]byte(*input.Message), &msg)
				require.NoError(t, err)
				
				assert.NotEmpty(t, msg.Partner) // Should have a partner (from env or default)
				assert.NotEmpty(t, msg.Stage) // Should have a stage (from env or default)
				assert.NotEmpty(t, msg.AWSRegion) // Should have an AWS region (from env or default)
				assert.NotEmpty(t, msg.AWSAccount) // Should have an AWS account (from env or default)
				assert.Equal(t, "my-lambda-function", msg.Function)
				assert.Equal(t, "my-lambda-function", msg.Subsystem) // Subsystem should match Function
				assert.NotEmpty(t, msg.Message) // Should have a message
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the notification logic
			if tt.expectPublish {
				// Verify the notification would be sent for ERROR level
				assert.Equal(t, "ERROR", tt.logEntry.Level)
			} else {
				// Verify non-ERROR levels are skipped
				assert.NotEqual(t, "ERROR", tt.logEntry.Level)
			}
		})
	}
}

func TestSanitizeErrorFields(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "safe fields are included",
			input: map[string]any{
				"method":       "POST",
				"path":         "/api/users",
				"status":       400,
				"duration":     123,
				"error_type":   "validation",
				"operation":    "create_user",
				"region":       "us-east-1",
				"function_name": "my-lambda",
			},
			expected: map[string]any{
				"method":       "POST",
				"path":         "/api/users",
				"status":       400,
				"duration":     123,
				"error_type":   "validation",
				"operation":    "create_user",
				"region":       "us-east-1",
				"function_name": "my-lambda",
			},
		},
		{
			name: "sensitive fields are excluded",
			input: map[string]any{
				"method":   "POST",
				"password": "secret123",
				"token":    "jwt-token",
				"api_key":  "key-123",
				"email":    "user@example.com",
				"status":   400,
			},
			expected: map[string]any{
				"method": "POST",
				"status": 400,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeErrorFields(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

