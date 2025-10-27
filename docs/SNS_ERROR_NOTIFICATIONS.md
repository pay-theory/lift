# SNS Error Notifications

The Lift framework provides three methods for configuring SNS error notifications, each suited for different use cases.

## Methods

### 1. `WithDefaultErrorNotifications` (Recommended - Use This!)

**Use this for:** All Pay Theory services (kernel services, partner services, any Lift-based service)

**What it does:** Publishes error logs to the centralized cross-account SNS topics in the main Pay Theory account (805600764437), matching the pattern used by all Python services.

**Topic pattern:** `arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-{stage}`

**Example:**

```go
// Initialize AWS config for SNS
cfg, err := config.LoadDefaultConfig(context.Background())
if err != nil {
    log.Fatalf("Failed to load AWS config: %v", err)
}

// Create SNS client
snsClient := sns.NewFromConfig(cfg)

// Create logger with default error notifications
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithDefaultErrorNotifications(snsClient))
```

**Required environment variables:**

- `STAGE` - Deployment stage (paytheory, paytheorylab, paytheorystudy)

**Benefits:**

- ✅ Centralized error monitoring across ALL Pay Theory services
- ✅ Matches existing Python services pattern
- ✅ No manual SNS topic management required
- ✅ Cross-account publishing handled automatically via IAM policies
- ✅ Simple - only requires STAGE environment variable

---

### 2. `WithPartnerErrorNotifications` (Rarely Needed)

**Use this for:** Services that need partner-specific SNS topics instead of centralized monitoring

**What it does:** Auto-detects the AWS account ID and creates a partner-specific SNS topic ARN.

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
- `AWS_ACCOUNT_ID` - (Optional) AWS account ID - auto-detected via STS if not set

**Auto-detection:**

The AWS account ID is auto-detected using:
1. `AWS_ACCOUNT_ID` environment variable (if set)
2. STS GetCallerIdentity API call (fallback)

**Benefits:**

- ✅ Automatic account ID detection
- ✅ Partner-specific error notifications
- ✅ Works in any AWS environment

**Note:** Most services should use `WithDefaultErrorNotifications` for centralized monitoring instead.

---

### 3. `WithErrorNotifications` (Custom Topic)

**Use this for:** Complete control over the SNS topic ARN

**What it does:** Uses an explicitly provided SNS topic ARN.

**Example:**
```go
customTopicARN := "arn:aws:sns:us-east-1:123456789012:my-custom-topic"
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithErrorNotifications(snsClient, customTopicARN))
```

**Benefits:**
- ✅ Full control over SNS topic
- ✅ Can point to any topic in any account
- ✅ Useful for testing or custom infrastructure

---

## Decision Tree

```
Do you need centralized error monitoring? (99% of services)
├─ YES → Use WithDefaultErrorNotifications ✅
└─ NO → Do you need partner-specific SNS topics?
    ├─ YES → Use WithPartnerErrorNotifications
    └─ NO → Do you have a custom SNS topic?
        ├─ YES → Use WithErrorNotifications
        └─ NO → Use WithDefaultErrorNotifications
```

**TL;DR:** Use `WithDefaultErrorNotifications` unless you have a specific reason not to.

## IAM Requirements

### For Centralized Monitoring (WithDefaultErrorNotifications)

Your Lambda role needs the `kernel-common-service-policy` (or similar) which includes:

```json
{
  "Effect": "Allow",
  "Action": ["sns:Publish"],
  "Resource": "*"
}
```

This allows cross-account publishing to the centralized SNS topics.

### For Partner-Specific Topics (WithPartnerErrorNotifications)

Your Lambda role needs:
```json
{
  "Effect": "Allow",
  "Action": ["sns:Publish"],
  "Resource": "arn:aws:sns:{region}:{account}:cns-{partner}-{stage}"
}
```

Plus optionally (for auto-detection):
```json
{
  "Effect": "Allow",
  "Action": ["sts:GetCallerIdentity"],
  "Resource": "*"
}
```

## Testing

To test SNS error notifications:

```go
// Trigger an error log
logger.Error("Test error for SNS notification", map[string]any{
    "test_type": "sns_validation",
    "environment": os.Getenv("STAGE"),
})
```

Check:
1. CloudWatch Logs for the error entry
2. SNS topic for the notification
3. Downstream subscribers (Slack, email, etc.)

## Migration from Python Services

If migrating from Python services:

**Python pattern:**
```python
send_notification('arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-paytheory', alert_object)
```

**Go (Lift) equivalent:**
```go
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithDefaultErrorNotifications(snsClient))
```

Both publish to the same centralized SNS topics for consistent error monitoring.

## See Also

- [AWS Account ID Auto-Detection](./AWS_ACCOUNT_ID_AUTO_DETECTION.md)
- [Zap Logger with SNS Notifications Example](../examples/zap-sns-logging/)
