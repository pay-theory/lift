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

const (
	errorLevel   = "ERROR"
	unknownValue = "UNKNOWN"
	timeFormat   = "2006-01-02T15:04:05.000000Z"
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

type lambdaExecutionContext struct {
	region   string
	account  string
	function string
}

// AlertConfig contains alert configuration
type AlertConfig struct {
	AlertType       string `json:"alert_type"`
	AlertTargetType string `json:"alert_target_type"`
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
	if logEntry.Level != errorLevel {
		return nil
	}

	functionName := getEnvOrDefault("AWS_LAMBDA_FUNCTION_NAME", unknownValue)
	execCtx := extractLambdaExecutionContext(functionName)
	formattedMessage := formatLogMessage(logEntry)

	notification := n.buildNotification(logEntry, execCtx, formattedMessage)
	applyNotificationOverrides(&notification, logEntry.Fields)

	messageJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal SNS notification: %w", err)
	}

	if _, err = n.snsClient.Publish(ctx, &sns.PublishInput{
		TargetArn: aws.String(n.targetARN),
		Message:   aws.String(string(messageJSON)),
	}); err != nil {
		return fmt.Errorf("failed to publish to CNS via SNS: %w", err)
	}

	return nil
}

func (n *SNSNotifier) buildNotification(logEntry *LogEntry, execCtx lambdaExecutionContext, message string) SNSNotificationMessage {
	return SNSNotificationMessage{
		AlertConfig: AlertConfig{
			AlertType:       "LiftError",
			AlertTargetType: "SLACK",
		},
		LogTime:    logEntry.Timestamp.UTC().Format(timeFormat),
		Partner:    getEnvOrDefault("PARTNER", unknownValue),
		Stage:      getEnvOrDefault("STAGE", unknownValue),
		AWSRegion:  execCtx.region,
		AWSAccount: execCtx.account,
		Severity:   errorLevel,
		Function:   execCtx.function,
		Subsystem:  execCtx.function,
		Message:    message,
	}
}

func extractLambdaExecutionContext(functionName string) lambdaExecutionContext {
	ctxInfo := lambdaExecutionContext{
		region:   unknownValue,
		account:  unknownValue,
		function: unknownValue,
	}

	if lambdaARN := os.Getenv("AWS_LAMBDA_FUNCTION_ARN"); lambdaARN != "" {
		if region, account, fn := parseLambdaARN(lambdaARN); region != "" {
			ctxInfo.region = region
			ctxInfo.account = account
			ctxInfo.function = fn
		}
	}

	if region := os.Getenv("AWS_REGION"); region != "" {
		ctxInfo.region = region
	}
	if account := os.Getenv("AWS_ACCOUNT_ID"); account != "" {
		ctxInfo.account = account
	}
	if functionName != unknownValue {
		ctxInfo.function = functionName
	}

	return ctxInfo
}

func parseLambdaARN(arn string) (string, string, string) {
	parts := strings.Split(arn, ":")
	if len(parts) < 7 {
		return "", "", ""
	}
	return parts[3], parts[4], parts[6]
}

func formatLogMessage(logEntry *LogEntry) string {
	return fmt.Sprintf("%s | %s | %s", errorLevel, logEntry.Message, formatLogFieldsJSON(logEntry.Fields))
}

func formatLogFieldsJSON(fields map[string]any) string {
	if len(fields) == 0 {
		return "{}"
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func applyNotificationOverrides(notification *SNSNotificationMessage, fields map[string]any) {
	if len(fields) == 0 {
		return
	}
	if env, ok := fields["environment"].(string); ok {
		notification.Environment = env
	}
	if svc, ok := fields["service"].(string); ok {
		notification.Service = svc
	}
	if funcName, ok := fields["function_name"].(string); ok {
		notification.Function = funcName
		notification.Subsystem = funcName
	} else if funcName, ok := fields["function"].(string); ok {
		notification.Function = funcName
		notification.Subsystem = funcName
	}
	if region, ok := fields["aws_region"].(string); ok {
		notification.AWSRegion = region
	} else if region, ok := fields["region"].(string); ok {
		notification.AWSRegion = region
	}
	if account, ok := fields["account_id"].(string); ok {
		notification.AWSAccount = account
	} else if account, ok := fields["aws_account"].(string); ok {
		notification.AWSAccount = account
	}
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
