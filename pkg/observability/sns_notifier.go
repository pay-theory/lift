package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSConfig contains configuration for SNS notifications
type SNSConfig struct {
	Client   SNSClient
	TopicARN string
}

// SNSNotifier handles sending error notifications to AWS SNS
type SNSNotifier struct {
	snsClient SNSClient
	targetARN string
}

// SNSNotificationMessage represents the structure sent to SNS
type SNSNotificationMessage struct {
	AlertConfig AlertConfig `json:"alert_config"`
	LogTime     string      `json:"log_time"`
	Environment string      `json:"environment,omitempty"`
	Service     string      `json:"service,omitempty"`
	Partner     string      `json:"partner"`
	Stage       string      `json:"stage"`
	AWSRegion   string      `json:"aws_region"`
	AWSAccount  string      `json:"aws_account"`
	Function    string      `json:"function"`
	Subsystem   string      `json:"subsystem"`
	Severity    string      `json:"severity"`
	Message     string      `json:"message"`
}

// AlertConfig contains alert configuration
type AlertConfig struct {
	AlertType       string   `json:"alert_type"`
	AlertTargetType []string `json:"alert_target_type"`
}

// NewSNSNotifier creates a new SNS notifier from configuration
func NewSNSNotifier(config SNSConfig) *SNSNotifier {
	return &SNSNotifier{
		snsClient: config.Client,
		targetARN: config.TopicARN,
	}
}

// GetTopicARN returns the configured SNS topic ARN
func (n *SNSNotifier) GetTopicARN() string {
	return n.targetARN
}

// NotifyError sends an error notification to SNS when an error is logged
func (n *SNSNotifier) NotifyError(ctx context.Context, logEntry *LogEntry) error {
	// Only notify for ERROR level logs
	if logEntry.Level != "ERROR" {
		return nil
	}

	// Get function name once to use for both Function and Subsystem
	functionName := getEnvOrDefault("AWS_LAMBDA_FUNCTION_NAME", "UNKNOWN")

	// Get AWS account ID from Lambda context ARN 
	awsRegion := "UNKNOWN"
	awsAccount := "UNKNOWN"
	lambdaFunction := "UNKNOWN"

	// Try to get Lambda ARN from environment (Lambda sets this automatically)
	if lambdaARN := os.Getenv("AWS_LAMBDA_FUNCTION_ARN"); lambdaARN != "" {
		// Parse ARN: arn:aws:lambda:region:account:function:name
		arnParts := strings.Split(lambdaARN, ":")
		if len(arnParts) >= 7 {
			awsRegion = arnParts[3]
			awsAccount = arnParts[4]
			lambdaFunction = arnParts[6]
		}
	}

	// Override with explicit env vars if set
	if region := os.Getenv("AWS_REGION"); region != "" {
		awsRegion = region
	}
	if account := os.Getenv("AWS_ACCOUNT_ID"); account != "" {
		awsAccount = account
	}
	if functionName != "UNKNOWN" {
		lambdaFunction = functionName
	}

	// Create JSON string representation of the log fields 
	var strBody string
	if len(logEntry.Fields) > 0 {
		fieldsJSON, err := json.Marshal(logEntry.Fields)
		if err != nil {
			strBody = "{}"
		} else {
			strBody = string(fieldsJSON)
		}
	} else {
		strBody = "{}"
	}

	// Format message: "ERROR | message | json_body"
	formattedMessage := fmt.Sprintf("ERROR | %s | %s", logEntry.Message, strBody)

	// Build the notification message
	notification := SNSNotificationMessage{
		AlertConfig: AlertConfig{
			AlertType:       "ERROR",
			AlertTargetType: []string{"SLACK"},
		},
		LogTime:    logEntry.Timestamp.UTC().Format("2006-01-02T15:04:05.000000Z"),
		Partner:    getEnvOrDefault("PARTNER", "UNKNOWN"),
		Stage:      getEnvOrDefault("STAGE", "UNKNOWN"),
		AWSRegion:  awsRegion,
		AWSAccount: awsAccount,
		Severity:   "ERROR",
		Function:   lambdaFunction,
		Subsystem:  lambdaFunction, // Set to same value as Function
		Message:    formattedMessage,
	}

	// Add environment and service from fields if available
	if env, ok := logEntry.Fields["environment"].(string); ok {
		notification.Environment = env
	}
	if svc, ok := logEntry.Fields["service"].(string); ok {
		notification.Service = svc
	}

	// Override function name if provided in fields
	if funcName, ok := logEntry.Fields["function_name"].(string); ok {
		notification.Function = funcName
		notification.Subsystem = funcName // Keep subsystem same as function
	} else if funcName, ok := logEntry.Fields["function"].(string); ok {
		notification.Function = funcName
		notification.Subsystem = funcName // Keep subsystem same as function
	}

	// Override AWS region if provided in fields
	if region, ok := logEntry.Fields["aws_region"].(string); ok {
		notification.AWSRegion = region
	} else if region, ok := logEntry.Fields["region"].(string); ok {
		notification.AWSRegion = region
	}

	// Override AWS account if provided in fields
	// Check for account_id from Lift context first (this comes from Context.AccountID())
	if account, ok := logEntry.Fields["account_id"].(string); ok {
		notification.AWSAccount = account
	} else if account, ok := logEntry.Fields["aws_account"].(string); ok {
		notification.AWSAccount = account
	}

	// Marshal the notification to JSON
	messageJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal SNS notification: %w", err)
	}

	// Wrap message to match SNS publishing format
	wrappedMessage := map[string]string{
		"default": "Default Message",
		"lambda":  string(messageJSON),
	}
	wrappedJSON, err := json.Marshal(wrappedMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal wrapped SNS message: %w", err)
	}

	// Publish to SNS with MessageStructure='json'
	_, err = n.snsClient.Publish(ctx, &sns.PublishInput{
		TargetArn:        aws.String(n.targetARN),
		Message:          aws.String(string(wrappedJSON)),
		MessageStructure: aws.String("json"),
	})

	if err != nil {
		return fmt.Errorf("failed to publish to CNS via SNS: %w", err)
	}

	return nil
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
