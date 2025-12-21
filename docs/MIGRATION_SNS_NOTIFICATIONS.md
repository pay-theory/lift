# Migration Guide: SNS Error Notifications

## Summary

This guide covers migration to the new SNS error notification API.

## Breaking Change

`WithDefaultErrorNotifications` has been replaced with `WithEnvironmentErrorNotifications`.

### Migration

**Before:**
```go
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithDefaultErrorNotifications(snsClient))
```

**After:**
```go
// Option 1: Environment variable (recommended)
// Set: ERROR_NOTIFICATION_SNS_TOPIC_ARN=arn:aws:sns:us-east-1:123456789012:my-topic
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithEnvironmentErrorNotifications(snsClient))

// Option 2: Explicit ARN
topicARN := os.Getenv("ERROR_NOTIFICATION_SNS_TOPIC_ARN")
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithErrorNotifications(snsClient, topicARN))

// Option 3: Partner-based (auto-detects account)
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithPartnerErrorNotifications(snsClient))
```

## Environment Variables

### New API
| Variable | Description |
|----------|-------------|
| `ERROR_NOTIFICATION_SNS_TOPIC_ARN` | Full SNS topic ARN |

### Partner Notifications API
| Variable | Description |
|----------|-------------|
| `PARTNER` | Partner identifier |
| `STAGE` | Deployment stage |
| `AWS_REGION` | AWS region |
| `AWS_ACCOUNT_ID` | (Optional) Auto-detected via STS |

## Verification Checklist

- [ ] Updated code to use new API
- [ ] Set `ERROR_NOTIFICATION_SNS_TOPIC_ARN` environment variable
- [ ] Lambda role has `sns:Publish` permission
- [ ] Tested error notifications work

## IAM Requirements

```json
{
  "Effect": "Allow",
  "Action": ["sns:Publish"],
  "Resource": "arn:aws:sns:{region}:{account}:{topic-name}"
}
```

## See Also

- [SNS Error Notifications](./SNS_ERROR_NOTIFICATIONS.md)
- [AWS Account ID Auto-Detection](./AWS_ACCOUNT_ID_AUTO_DETECTION.md)
