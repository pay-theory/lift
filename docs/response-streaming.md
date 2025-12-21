# Response Streaming & SSE (Server-Sent Events)

<!-- AI Training: This document teaches the canonical Lift pattern for server-to-client streaming via API Gateway REST API (v1). -->
**Lift supports Server-Sent Events (SSE) using AWS Lambda response streaming when deployed behind API Gateway REST API (v1). This enables long-lived HTTP responses (up to 15 minutes) that stream incremental events to the client.**

## When to Use SSE Streaming

Use SSE streaming when you need:
- ✅ One-way server → client updates (progress, notifications, live feeds)
- ✅ Simple browser/client support over plain HTTP
- ✅ A single request that streams multiple events over time

Do **NOT** use SSE streaming when you need:
- ❌ Bidirectional messaging (use WebSockets instead)
- ❌ Background/async processing with continuation (use SQS/EventBridge + worker Lambdas, or Lift’s streamer patterns)
- ❌ HTTP API v2-only deployments (SSE streaming is implemented for REST API v1 streaming integrations)

## Requirements

### 1) Build for Lambda response streaming

AWS Lambda response streaming in Go requires compiling with:

```bash
# CORRECT: required for response streaming types (and recommended for production Lambda builds)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go
```

### 2) Use API Gateway REST API (v1) with streaming enabled

Lift’s CDK `LiftRestAPI` construct can configure the required integration settings automatically.

## Quick Start (Canonical Pattern)

### Server: Lift handler that streams SSE

```go
package main

import (
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
)

func main() {
	app := lift.New()
	app.Use(middleware.RequestID(), middleware.Logger(), middleware.Recover())

	// CORRECT: SSE endpoint implemented as a streaming response.
	_ = app.GET("/events", func(ctx *lift.Context) error {
		eventChan := make(chan lift.SSEEvent, 8)

		// Produce events asynchronously.
		go func() {
			defer close(eventChan)

			eventChan <- lift.SSEEvent{Event: "status", Data: "connected"}

			for i := 0; i < 3; i++ {
				eventChan <- lift.SSEEvent{
					Event: "progress",
					Data:  time.Now().UTC().Format(time.RFC3339),
					ID:    "tick",
				}
				time.Sleep(250 * time.Millisecond)
			}

			eventChan <- lift.SSEEvent{Event: "done", Data: "complete"}
		}()

		return lift.SSEResponse(ctx, eventChan)
	})

	lambda.Start(app.HandleRequest)
}
```

**What Lift does:**
- Sets `Content-Type: text/event-stream`
- Formats each `SSEEvent` into SSE wire format (`event:`, `id:`, `retry:`, `data:`)
- Returns an AWS Lambda streaming response type so the runtime can stream bytes to the client

### Client: Minimal browser example

```js
// CORRECT: browser-native SSE
const es = new EventSource("/events");
es.addEventListener("progress", (e) => console.log("progress", e.data));
es.addEventListener("done", (e) => { console.log("done", e.data); es.close(); });
```

## CDK Deployment (LiftRestAPI)

### CORRECT: REST API v1 + streaming enabled

```go
package infra

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
)

func NewStreamingAPIStack(scope constructs.Construct, id *string) awscdk.Stack {
	stack := awscdk.NewStack(scope, id, nil)

	fn := awslambda.NewFunction(stack, jsii.String("Fn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("./dist"), nil),
	})

	timeoutSeconds := 15 * 60
	api := liftconstructs.NewLiftRestAPI(stack, jsii.String("RestAPI"), &liftconstructs.LiftRestAPIProps{
		APICommonProps: liftconstructs.APICommonProps{
			Name:       jsii.String("my-rest-api"),
			StageName:  jsii.String("prod"),
			EnableCORS: jsii.Bool(true),
		},

		// Enable API Gateway REST API response streaming.
		EnableStreaming:  jsii.Bool(true),
		StreamingTimeout: &timeoutSeconds, // seconds, up to 15 minutes
	})

	api.AddLambdaIntegration(jsii.String("/events"), jsii.String("GET"), fn)

	return stack
}
```

**What the construct configures:**
- `ResponseTransferMode: STREAM` on REST API method integrations
- Streaming Lambda invocation URI: `.../2021-11-15/functions/{arn}/response-streaming-invocations`
- Optional longer integration timeout (up to 15 minutes)

### INCORRECT: HTTP API v2 expecting streaming SSE

```go
// INCORRECT: LiftAPI (HTTP API v2) does not configure REST API v1 streaming integrations.
api := constructs.NewLiftAPI(stack, jsii.String("HttpAPI"), &constructs.LiftAPIProps{
	APICommonProps: constructs.APICommonProps{Name: jsii.String("my-http-api")},
})
```

## Relationship to Streamer / WebSockets

- **SSE streaming**: one-way (server → client) over HTTP, simplest client support.
- **WebSockets**: two-way (client ↔ server) messaging; see `docs/streamer-guide.md`.
- **Streamer async patterns**: useful when work must continue after the request lifecycle or needs durable fanout; SSE is not durable by itself.

## Next: Detailed APIs and Edge Cases

- Lift API: see `docs/api-reference.md` for `lift.SSEResponse` and `lift.SSEEvent`.
- CDK API: see `docs/cdk-api-reference.md` for `LiftRestAPI` streaming settings.
- Troubleshooting: see `docs/troubleshooting.md` for common streaming/SSE pitfalls.
