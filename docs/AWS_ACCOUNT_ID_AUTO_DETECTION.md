# AWS Account ID Auto-Detection

## Overview

The Lift framework now automatically detects your AWS account ID when using `WithDefaultErrorNotifications` for SNS error notifications. This eliminates the need to manually configure the `AWS_ACCOUNT_ID` environment variable.

## How It Works

The auto-detection uses a **fallback strategy**:

1. **First**: Check `AWS_ACCOUNT_ID` environment variable (fastest, no API call)
2. **Fallback**: Call `STS GetCallerIdentity` API (works in any AWS environment)

The account ID is detected **once** and cached for the lifetime of the Lambda function process.

## Usage

### Before (Required Manual Configuration)

```go
// Required environment variables:
// - PARTNER=qakernel
// - STAGE=paytheory
// - AWS_REGION=us-east-1
// - AWS_ACCOUNT_ID=058264189048  ❌ Had to set this manually

logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithDefaultErrorNotifications(snsClient))
```

### After (Auto-Detection)

```go
// Required environment variables:
// - PARTNER=qakernel
// - STAGE=paytheory
// - AWS_REGION=us-east-1
// - AWS_ACCOUNT_ID=058264189048  ✅ Optional - auto-detected if not set

logger, err := zap.NewZapLogger(loggerConfig,
    zap.WithDefaultErrorNotifications(snsClient))
```

## Performance Impact

- **First invocation**: ~150ms for STS API call (only if `AWS_ACCOUNT_ID` not set)
- **Subsequent invocations**: 0ms (cached by Lambda runtime)
- **With env var set**: 0ms (no STS call)

## SNS Topic Format

The auto-detection enables the standard Pay Theory SNS topic format:

```
arn:aws:sns:{region}:{account}:cns-{partner}-{stage}
```

Example for K3 paytheory:
```
arn:aws:sns:us-east-1:058264189048:cns-qakernel-paytheory
```

## Error Handling

If the account ID cannot be determined (no env var + STS call fails):
- `WithDefaultErrorNotifications` returns `nil`
- Logger continues to work normally
- Error logs appear in CloudWatch Logs
- SNS notifications are disabled (graceful degradation)

## Compatibility

✅ **Backwards Compatible**: Existing code that sets `AWS_ACCOUNT_ID` continues to work
✅ **No Breaking Changes**: Auto-detection is a fallback, not a replacement
✅ **Lambda-Safe**: Cached across invocations by Lambda runtime
✅ **Multi-Environment**: Works in Lambda, ECS, EC2, local development

## Example: K3 Integration

```go
func main() {
    // Create lift app
    app := lift.New()

    // Check if we're running on AWS Lambda
    if app.IsLambda() {
        // Initialize AWS config for SNS
        cfg, err := config.LoadDefaultConfig(context.Background())
        if err != nil {
            log.Fatalf("Failed to load AWS config: %v", err)
        }

        // Create SNS client
        snsClient := sns.NewFromConfig(cfg)

        // Create Zap logger with SNS error notifications
        // AWS_ACCOUNT_ID is auto-detected!
        logger, err = zap.NewZapLogger(loggerConfig,
            zap.WithDefaultErrorNotifications(snsClient))
        if err != nil {
            log.Fatalf("Failed to create zap logger with SNS: %v", err)
        }
    }

    // Use logger
    app.WithLogger(logger)
}
```

## Testing

Run the tests to verify auto-detection:

```bash
go test ./pkg/observability -v -run TestGetAWSAccountID
```

## Implementation Details

See:
- `/pkg/observability/aws_helpers.go` - Account ID detection logic
- `/pkg/observability/aws_helpers_test.go` - Test coverage
- `/pkg/observability/sns_options.go` - Integration with SNS notifier
