# Migration Guide: SNS Error Notifications

## Summary

The Lift framework now supports centralized SNS error notifications matching the Python services pattern. All Pay Theory services (in any account) use the same centralized cross-account SNS topics.

## What Changed

### Before (v1.0.56)
- `WithDefaultErrorNotifications` required `AWS_ACCOUNT_ID` environment variable
- If `AWS_ACCOUNT_ID` was not set, SNS notifications silently failed (returned `nil`)
- Services like bin-lookup-service had SNS configured but it wasn't working

### After (Current)
- `WithDefaultErrorNotifications` uses centralized cross-account SNS topics
- Only requires `STAGE` environment variable
- Works for ALL services in ANY account (kernel, partner, etc.)
- Auto-detection available via `WithPartnerErrorNotifications` if needed

## Migration Steps

### For Most Services (99% of cases)

**No code changes needed!** Just ensure you have the `STAGE` environment variable set.

```go
// This continues to work - now uses centralized SNS topics
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithDefaultErrorNotifications(snsClient))
```

**Before:**
- Required: `PARTNER`, `STAGE`, `AWS_REGION`, `AWS_ACCOUNT_ID`
- Result: SNS notifications silently failed if `AWS_ACCOUNT_ID` not set

**After:**
- Required: `STAGE` only
- Result: SNS notifications work out-of-the-box

### For Services Using Partner-Specific Topics (Rare)

If you explicitly need partner-specific SNS topics instead of centralized monitoring:

```go
// Use this instead of WithDefaultErrorNotifications
logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithPartnerErrorNotifications(snsClient))
```

**Requires:**
- `PARTNER`, `STAGE`, `AWS_REGION`
- `AWS_ACCOUNT_ID` (optional - auto-detected via STS if not set)

## Verification

### Check Current Behavior

1. **bin-lookup-service** (and similar services):
   ```bash
   # Check if AWS_ACCOUNT_ID is set
   aws lambda get-function --function-name bin-lookup-austin-paytheory \
     --query "Configuration.Environment.Variables.AWS_ACCOUNT_ID"

   # If it returns null, SNS notifications were NOT working
   ```

2. **K3** (after deployment):
   ```bash
   # Test SNS notification
   curl https://k3.qakernel.paytheory.com/test/sns

   # Check CloudWatch Logs for error entry
   aws logs tail /aws/lambda/k3-qakernel-paytheory --since 1m --filter-pattern "TEST"

   # Check SNS topic for notification (if you have access)
   ```

## SNS Topic Structure

All services publish to:
```
arn:aws:sns:us-east-1:805600764437:global-logs-publisher-topic-{stage}
```

**Stages:**
- `paytheory` (production)
- `paytheorylab` (lab/testing)
- `paytheorystudy` (study/development)

## IAM Requirements

Your Lambda execution role needs:
```json
{
  "Effect": "Allow",
  "Action": ["sns:Publish"],
  "Resource": "*"
}
```

This is typically provided by:
- `kernel-common-service-policy` (for kernel services)
- Similar managed policies for partner services

## Backward Compatibility

✅ **Non-Breaking Change**
- Services using `WithDefaultErrorNotifications` continue to work
- Previous implementation was non-functional (required `AWS_ACCOUNT_ID` which was never set)
- No code changes required for most services

## Testing Checklist

- [ ] Service builds successfully
- [ ] `STAGE` environment variable is set
- [ ] Lambda role has `sns:Publish` permission
- [ ] Error logs appear in CloudWatch Logs
- [ ] SNS notifications are sent (check downstream subscribers)
- [ ] No errors in Lambda logs related to SNS

## Rollback

If you need to rollback:
1. Pin Lift to previous version: `github.com/pay-theory/lift v1.0.56`
2. Set `AWS_ACCOUNT_ID` environment variable (SNS will still not work, but no errors)

## Support

- Documentation: `/docs/SNS_ERROR_NOTIFICATIONS.md`
- Examples: `/examples/zap-sns-logging/`
- AWS Account ID Auto-Detection: `/docs/AWS_ACCOUNT_ID_AUTO_DETECTION.md`
