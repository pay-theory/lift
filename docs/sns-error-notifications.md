# SNS Error Notifications

<!-- AI Training: How to configure Lift's SNS error notification options -->

The Lift framework provides multiple methods for configuring SNS error notifications.

## Methods

### 1. `WithEnvironmentErrorNotifications` (Recommended)

**Use this for:** All services that need SNS error notifications

**What it does:** Reads the SNS topic ARN from the `ERROR_NOTIFICATION_SNS_TOPIC_ARN` environment variable.

**Example:**

```go
// Set environment variable:
// export ERROR_NOTIFICATION_SNS_TOPIC_ARN="arn:aws:sns:us-east-1:123456789012:my-error-topic"

cfg, err := config.LoadDefaultConfig(context.Background())
if err != nil {
    log.Fatalf("Failed to load AWS config: %v", err)
}

snsClient := sns.NewFromConfig(cfg)

logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithEnvironmentErrorNotifications(snsClient))
```

**Required:**
- `ERROR_NOTIFICATION_SNS_TOPIC_ARN` environment variable

**Benefits:**
- ✅ Simple configuration via environment variable
- ✅ Works with any SNS topic ARN
- ✅ Easy to configure per-environment

---

### 2. `WithPartnerErrorNotifications` (Auto-Detection)

**Use this for:** Services that follow the `cns-{partner}-{stage}` topic naming convention

**What it does:** Auto-detects the AWS account ID and constructs the topic ARN dynamically.

**Topic pattern:** `arn:aws:sns:{region}:{account}:cns-{partner}-{stage}`

**Example:**

```go
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithPartnerErrorNotifications(snsClient))
```

**Required environment variables:**
- `PARTNER` - Partner identifier
- `STAGE` - Deployment stage
- `AWS_REGION` - AWS region
- `AWS_ACCOUNT_ID` - (Optional) Auto-detected via STS if not set

**Benefits:**
- ✅ Automatic account ID detection
- ✅ Dynamic topic ARN construction
- ✅ Works in any AWS environment

---

### 3. `WithErrorNotifications` (Custom Topic)

**Use this for:** Complete control over the SNS topic ARN

**Example:**
```go
topicARN := "arn:aws:sns:us-east-1:123456789012:my-custom-topic"
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithErrorNotifications(snsClient, topicARN))
```

**Benefits:**
- ✅ Full control over SNS topic
- ✅ Useful for testing or custom infrastructure

---

## Decision Tree

```
Do you have an SNS topic ARN to use?
├─ YES → Use WithEnvironmentErrorNotifications or WithErrorNotifications
└─ NO → Do you follow the cns-{partner}-{stage} naming convention?
    ├─ YES → Use WithPartnerErrorNotifications
    └─ NO → Create an SNS topic first
```

## IAM Requirements

Your Lambda role needs:

```json
{
  "Effect": "Allow",
  "Action": ["sns:Publish"],
  "Resource": "arn:aws:sns:{region}:{account}:{topic-name}"
}
```

For auto-detection (`WithPartnerErrorNotifications`), optionally add:
```json
{
  "Effect": "Allow",
  "Action": ["sts:GetCallerIdentity"],
  "Resource": "*"
}
```

## Testing

```go
// Trigger an error log
logger.Error("Test error for SNS notification", map[string]any{
    "test_type": "sns_validation",
})
```

Check:
1. CloudWatch Logs for the error entry
2. SNS topic for the notification
3. Downstream subscribers (Slack, email, etc.)

## See Also

- [AWS Account ID Auto-Detection](./aws-account-id-auto-detection.md)
- [Zap Logger with SNS Notifications Example](../examples/zap-sns-logging/)
