# AWS Account ID Auto-Detection

## Overview

The Lift framework can automatically detect your AWS account ID when using `WithPartnerErrorNotifications` for SNS error notifications. This eliminates the need to manually configure the `AWS_ACCOUNT_ID` environment variable.

## How It Works

The auto-detection uses a **fallback strategy**:

1. **First**: Check `AWS_ACCOUNT_ID` environment variable (fastest, no API call)
2. **Fallback**: Call `STS GetCallerIdentity` API (works in any AWS environment)

The account ID is detected **once** and cached for the lifetime of the Lambda function process.

## Usage

### With Environment Variable (Fastest)

```go
// Set environment variable:
// AWS_ACCOUNT_ID=123456789012

logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithPartnerErrorNotifications(snsClient))
```

### With Auto-Detection

```go
// Required environment variables:
// - PARTNER=mypartner
// - STAGE=production
// - AWS_REGION=us-east-1
// AWS_ACCOUNT_ID is auto-detected if not set

logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithPartnerErrorNotifications(snsClient))
```

## Performance Impact

- **First invocation**: ~150ms for STS API call (only if `AWS_ACCOUNT_ID` not set)
- **Subsequent invocations**: 0ms (cached by Lambda runtime)
- **With env var set**: 0ms (no STS call)

## SNS Topic Format

The auto-detection enables the partner SNS topic format:

```
arn:aws:sns:{region}:{account}:cns-{partner}-{stage}
```

## Error Handling

If the account ID cannot be determined (no env var + STS call fails):
- `WithPartnerErrorNotifications` returns `nil`
- Logger continues to work normally
- Error logs appear in CloudWatch Logs
- SNS notifications are disabled (graceful degradation)

## Compatibility

✅ **Backwards Compatible**: Existing code that sets `AWS_ACCOUNT_ID` continues to work  
✅ **Lambda-Safe**: Cached across invocations by Lambda runtime  
✅ **Multi-Environment**: Works in Lambda, ECS, EC2, local development

## Implementation Details

See:
- `/pkg/observability/aws_helpers.go` - Account ID detection logic
- `/pkg/observability/aws_helpers_test.go` - Test coverage
- `/pkg/observability/sns_options.go` - Integration with SNS notifier
