# SQS Large Payloads via S3 (Lift)

AWS SQS message bodies have a hard limit of **256KB**. Lift supports an “extended payload” pattern where oversized message bodies are stored in S3 and SQS carries a small pointer envelope.

## Table of Contents

- [How It Works](#how-it-works)
- [CDK: Enable Large Payloads](#cdk-enable-large-payloads)
- [Go: Producer Offload Pattern](#go-producer-offload-pattern)
- [Go + Lift: Consumer Hydrate + Delete](#go--lift-consumer-hydrate--delete)
- [Configuration Reference](#configuration-reference)
- [Operational Notes](#operational-notes)

## How It Works

### Offload Rule (No Compression)

- If the serialized SQS message body is **> 256KB**, Lift offloads it to S3 (**no gzip**) and sends a pointer envelope through SQS instead.
- Threshold is fixed at `> 256KB` (`sqslargepayload.SQSMaxMessageBytes`).

### Pointer Envelope Schema

When offloaded, the SQS message body becomes JSON:

```json
{
  "type": "lift:sqs-large-payload:v1",
  "bucket": "myapp-live-sqs-large-payload",
  "key": "sqs/orders-queue-live/messages/01J.../payload.json",
  "sha256": "optional",
  "bytes": 123456
}
```

Notes:
- The envelope is **self-describing** (`bucket` + `key`) so consumers don’t need bucket env vars to hydrate/delete.
- `sha256` is optional; Lift verifies it by default when present.

### Delete-On-Success + TTL Fallback

- Consumers **explicitly delete** the S3 object after successful processing.
- The bucket is configured with an S3 lifecycle **expiration TTL** (default: **7 days**) as a safety net for leak cleanup.

## CDK: Enable Large Payloads

Enable the feature on `constructs.SQSProcessor` via `SQSProcessorProps.LargePayload`:

```go
processor := constructs.NewSQSProcessor(stack, jsii.String("Jobs"), &constructs.SQSProcessorProps{
    FunctionProps: awslambda.FunctionProps{
        FunctionName: jsii.String("jobs-processor"),
        Code:         awslambda.Code_FromAsset(jsii.String("./dist"), nil),
        Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
    },
    LargePayload: &constructs.SQSLargePayloadProps{}, // opt-in
})

// For wiring producers:
// - bucket:  processor.LargePayloadBucket
// - prefix:  *processor.LargePayloadPrefix
// - grants:  processor.GrantLargePayloadPublish(fn) / processor.GrantLargePayloadConsume(fn)
_ = processor
```

### Producer Wiring (Recommended)

The consumer Lambda (the processor function) automatically receives `s3:GetObject` + `s3:DeleteObject` scoped to the payload prefix.

For a producer Lambda that offloads payloads, grant write permissions and pass the bucket/prefix to your producer code:

```go
producerFn := constructs.NewLiftFunction(stack, jsii.String("Producer"), &constructs.LiftFunctionProps{
    FunctionProps: awslambda.FunctionProps{
        FunctionName: jsii.String("jobs-producer"),
        Code:         awslambda.Code_FromAsset(jsii.String("./dist-producer"), nil),
        Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
    },
})

processor.GrantLargePayloadPublish(producerFn.Function)

producerFn.Function.AddEnvironment(jsii.String("LIFT_SQS_LARGE_PAYLOAD_BUCKET"), processor.LargePayloadBucket.BucketName(), nil)
producerFn.Function.AddEnvironment(jsii.String("LIFT_SQS_LARGE_PAYLOAD_PREFIX"), processor.LargePayloadPrefix, nil)
```

Lift also supports deterministic bucket naming when the naming context is available:
- Required: `APP_NAME`, `STAGE`
- Optional: `PARTNER`

If you want producers to derive bucket names at runtime (instead of injecting bucket env vars), ensure the bucket is created with a deterministic name (or explicitly set `BucketProps.BucketName`) and use `pkg/naming` in producer code.

## Go: Producer Offload Pattern

Use `pkg/services/sqs_large_payload` to offload when needed and send either the original body or an envelope as the SQS message body.

```go
import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/sqs"
    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
    sqslargepayload "github.com/pay-theory/lift/pkg/services/sqs_large_payload"
)

func randomHex(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}

func send(ctx context.Context, queueURL, bucket, prefix string, payload []byte) error {
    awsCfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return err
    }

    s3Store := sqslargepayload.NewS3Store(s3.NewFromConfig(awsCfg))

    // Pick a unique key under the configured prefix.
    key := fmt.Sprintf("%s%s.json", prefix, randomHex(16))

    body, _, err := sqslargepayload.OffloadIfNeeded(
        ctx,
        s3Store,
        sqslargepayload.Ref{Bucket: bucket, Key: key},
        payload,
        sqslargepayload.DefaultOptions(),
    )
    if err != nil {
        return err
    }

    _, err = sqs.NewFromConfig(awsCfg).SendMessage(ctx, &sqs.SendMessageInput{
        QueueUrl:    &queueURL,
        MessageBody: awsString(string(body)),
        MessageAttributes: map[string]types.MessageAttributeValue{
            "contentType": {DataType: awsString("String"), StringValue: awsString("application/json")},
        },
    })
    return err
}

func awsString(s string) *string { return &s }
```

## Go + Lift: Consumer Hydrate + Delete

In a Lift SQS handler, use `sqslargepayload.BatchProcessor` to hydrate a mixed batch (inline + envelopes), build `BatchItemFailures`, and delete S3 objects only for successful messages.

Important: the Lambda return value for partial batch failure is `events.SQSEventResponse` (not an API Gateway proxy response). Lift handlers that return a response must use the `(any, error)` signature and return an `events.SQSEventResponse` value.

```go
import (
    "context"
    "encoding/json"
    "errors"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/pay-theory/lift/pkg/lift"
    sqslargepayload "github.com/pay-theory/lift/pkg/services/sqs_large_payload"
)

type Job struct {
    ID string `json:"id"`
}

func main() {
    app := lift.New()

    // Create S3Store once and reuse it.
    awsCfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        panic(err)
    }
    store := sqslargepayload.NewS3Store(s3.NewFromConfig(awsCfg))

    processor := sqslargepayload.BatchProcessor{
        Store:   store,
        Options: sqslargepayload.DefaultOptions(),
    }

    _ = app.SQS("*", func(ctx *lift.Context) (any, error) {
        records, err := ctx.SQSRecords()
        if err != nil {
            return events.SQSEventResponse{}, err
        }

        return processor.Process(ctx.Context, records, func(ctx context.Context, msg sqslargepayload.Message) error {
            var job Job
            if err := json.Unmarshal(msg.Payload, &job); err != nil {
                return err
            }
            if job.ID == "" {
                return errors.New("missing job.id")
            }
            return nil
        })
    })

    lift.Start(app)
}
```

## Configuration Reference

### `constructs.SQSLargePayloadProps`

```go
type SQSLargePayloadProps struct {
    Enabled        *bool
    ExistingBucket awss3.IBucket
    BucketProps    *awss3.BucketProps
    Prefix         *string
    Expiration     awscdk.Duration // default: 7 days (TTL fallback)
}
```

### Defaults

- Bucket secure defaults: block public access, S3-managed encryption, SSL enforced.
- Lifecycle expiration rule is applied (default: 7 days). When Lift creates a shared default bucket, the rule is scoped to the derived prefix.
- `constructs.SQSProcessor` defaults `ReportBatchItemFailures` to `true`.

## Operational Notes

- This pattern is compatible with at-least-once delivery; keep handlers idempotent.
- If S3 delete fails, TTL will eventually clean up; you can set `BatchProcessor.FailOnDeleteError` if you prefer deletes to mark the message as failed.
- If your producer needs deterministic bucket naming without explicit env injection, set `APP_NAME` and `STAGE` for both synth and runtime (or explicitly set `BucketProps.BucketName`).
