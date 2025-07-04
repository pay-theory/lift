# Event Adapters: Multi-Source Event Handling with Lift

**This is the RECOMMENDED pattern for building Lambda functions that handle multiple AWS event sources.**

## What is This Example?

This example demonstrates the **STANDARD approach** for building event-driven serverless functions with Lift. It shows the **preferred patterns** for handling multiple AWS event sources in a single Lambda function with automatic event detection and routing.

## Why Use Multi-Source Event Adapters?

✅ **USE this pattern when:**
- Building event-driven serverless architectures
- Need one Lambda to handle multiple AWS event sources
- Want automatic event type detection and routing
- Require consistent event processing patterns
- Building microservices that respond to various triggers

❌ **DON'T USE when:**
- Building simple HTTP-only APIs (use basic patterns)
- Need dedicated functions for each event type
- Processing real-time streams (use Kinesis-specific patterns)
- Single event source applications

## Quick Start

```go
// This is the CORRECT way to handle multiple event sources
package main

import "github.com/pay-theory/lift/pkg/lift"

func main() {
    app := lift.New()
    
    // HTTP API routes (API Gateway)
    app.GET("/hello", httpHandler)
    
    // Event-driven handlers
    app.Handle("POST", "/sqs", sqsHandler)           // SQS messages
    app.Handle("POST", "/s3", s3Handler)             // S3 events
    app.Handle("POST", "/eventbridge", ebHandler)    // EventBridge events
    app.Handle("POST", "/scheduled", scheduledHandler) // CloudWatch Events
    
    // REQUIRED: Start with multi-source support
    app.Start()
}

// INCORRECT: Separate Lambda functions for each event type
// This approach requires more infrastructure and complexity
```

## Core Event Handling Patterns

### 1. Event Type Detection (AUTOMATIC Pattern)

**Purpose:** Automatically detect and route different AWS event sources
**When to use:** All multi-source Lambda functions

```go
// CORRECT: Lift automatically detects event types
app.Handle("POST", "/sqs", func(ctx *lift.Context) error {
    // Lift automatically sets ctx.Request.TriggerType = lift.TriggerSQS
    if ctx.Request.TriggerType != lift.TriggerSQS {
        return ctx.Status(400).JSON(map[string]string{
            "error": "Expected SQS trigger"
        })
    }
    
    // Process SQS records
    for _, record := range ctx.Request.Records {
        // Handle each SQS message
        processMessage(record.Body)
    }
    
    return ctx.JSON(map[string]any{
        "processed": len(ctx.Request.Records),
        "source":    ctx.Request.Source,
    })
})

// INCORRECT: Manual event parsing
// app.Handle("POST", "/sqs", func(ctx *lift.Context) error {
//     var sqsEvent events.SQSEvent  // Manual AWS SDK parsing
//     if err := json.Unmarshal(rawEvent, &sqsEvent); err != nil {
//         return err  // Error-prone and verbose
//     }
//     // ... manual processing
// })
```

### 2. SQS Message Processing (STANDARD Pattern)

**Purpose:** Process SQS messages with batch handling
**When to use:** Queue-based event processing

```go
// CORRECT: Batch SQS message processing
app.Handle("POST", "/sqs", func(ctx *lift.Context) error {
    ctx.Logger().Info("Processing SQS batch", 
        "count", len(ctx.Request.Records))
    
    var processed int
    var failed int
    
    for _, record := range ctx.Request.Records {
        if err := processSQSMessage(record); err != nil {
            ctx.Logger().Error("Failed to process message", 
                "messageId", record.MessageID,
                "error", err)
            failed++
            continue
        }
        processed++
    }
    
    return ctx.JSON(map[string]any{
        "processed": processed,
        "failed":    failed,
        "total":     len(ctx.Request.Records),
    })
})

// INCORRECT: Processing one message at a time
// This doesn't take advantage of SQS batch processing
```

### 3. S3 Event Processing (PREFERRED Pattern)

**Purpose:** Handle S3 object lifecycle events
**When to use:** File processing, data pipeline triggers

```go
// CORRECT: S3 event processing with error handling
app.Handle("POST", "/s3", func(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        bucket := record.S3.Bucket.Name
        key := record.S3.Object.Key
        eventType := record.EventName
        
        ctx.Logger().Info("Processing S3 event",
            "bucket", bucket,
            "key", key,
            "eventType", eventType)
        
        // Route based on event type
        switch eventType {
        case "ObjectCreated:Put", "ObjectCreated:Post":
            if err := processNewObject(bucket, key); err != nil {
                return fmt.Errorf("failed to process new object: %w", err)
            }
        case "ObjectRemoved:Delete":
            if err := handleObjectDeletion(bucket, key); err != nil {
                return fmt.Errorf("failed to handle deletion: %w", err)
            }
        default:
            ctx.Logger().Warn("Unhandled S3 event type", "eventType", eventType)
        }
    }
    
    return ctx.JSON(map[string]any{
        "message": "S3 events processed",
        "count":   len(ctx.Request.Records),
    })
})
```

### 4. EventBridge Processing (STANDARD Pattern)

**Purpose:** Handle custom application events and AWS service events
**When to use:** Event-driven microservices, workflow orchestration

```go
// CORRECT: EventBridge event processing with type routing
app.Handle("POST", "/eventbridge", func(ctx *lift.Context) error {
    source := ctx.Request.Source
    detailType := ctx.Request.DetailType
    detail := ctx.Request.Detail
    
    ctx.Logger().Info("Processing EventBridge event",
        "source", source,
        "detailType", detailType)
    
    // Route based on event source and type
    switch source {
    case "myapp.orders":
        return handleOrderEvent(detailType, detail)
    case "myapp.payments":
        return handlePaymentEvent(detailType, detail)
    case "aws.s3":
        return handleS3ServiceEvent(detailType, detail)
    default:
        return fmt.Errorf("unhandled event source: %s", source)
    }
})

// Helper functions for different event types
func handleOrderEvent(detailType string, detail map[string]any) error {
    switch detailType {
    case "Order Placed":
        return processNewOrder(detail)
    case "Order Cancelled":
        return processOrderCancellation(detail)
    default:
        return fmt.Errorf("unhandled order event: %s", detailType)
    }
}
```

### 5. Scheduled Event Processing (STANDARD Pattern)

**Purpose:** Handle CloudWatch Events scheduled triggers
**When to use:** Cron jobs, periodic maintenance tasks

```go
// CORRECT: Scheduled event with task identification
app.Handle("POST", "/scheduled", func(ctx *lift.Context) error {
    // Extract rule name from ARN
    var ruleName string
    if len(ctx.Request.Resources) > 0 {
        // Parse ARN: arn:aws:events:region:account:rule/rule-name
        arn := ctx.Request.Resources[0]
        parts := strings.Split(arn, "/")
        if len(parts) > 1 {
            ruleName = parts[len(parts)-1]
        }
    }
    
    ctx.Logger().Info("Processing scheduled event", "rule", ruleName)
    
    // Route based on rule name
    switch ruleName {
    case "daily-cleanup":
        return performDailyCleanup()
    case "hourly-metrics":
        return generateHourlyMetrics()
    case "weekly-reports":
        return generateWeeklyReports()
    default:
        return fmt.Errorf("unhandled scheduled rule: %s", ruleName)
    }
})
```

## Supported Event Sources

### ✅ Fully Supported

- **API Gateway v1/v2** - HTTP REST and WebSocket APIs
- **SQS** - Standard and FIFO queues
- **S3** - Object lifecycle events
- **EventBridge** - Custom and AWS service events
- **CloudWatch Events** - Scheduled events and rules
- **DynamoDB Streams** - Table change events
- **Kinesis** - Stream processing events

### 📋 Event Source Mapping

```go
// Lift automatically maps these trigger types:
ctx.Request.TriggerType == lift.TriggerAPIGateway    // HTTP requests
ctx.Request.TriggerType == lift.TriggerSQS          // SQS messages
ctx.Request.TriggerType == lift.TriggerS3           // S3 events
ctx.Request.TriggerType == lift.TriggerEventBridge  // EventBridge events
ctx.Request.TriggerType == lift.TriggerScheduled    // CloudWatch Events
ctx.Request.TriggerType == lift.TriggerDynamoStream // DynamoDB Streams
ctx.Request.TriggerType == lift.TriggerKinesis      // Kinesis streams
```

## What This Example Teaches

### ✅ Best Practices Demonstrated

1. **ALWAYS check trigger type** - Validate expected event source before processing
2. **USE batch processing** - Handle multiple records efficiently in SQS/S3 events
3. **IMPLEMENT error handling** - Log failures and return appropriate responses
4. **ROUTE by event attributes** - Use source, detail-type, or event name for routing
5. **LOG with context** - Include relevant event metadata in all log messages

### 🚫 Critical Anti-Patterns Avoided

1. **Manual event parsing** - Error-prone and requires AWS SDK knowledge
2. **Single event processing** - Doesn't leverage batch capabilities
3. **Generic error handling** - Makes debugging difficult
4. **Missing event validation** - Can lead to processing wrong event types
5. **Hardcoded event routing** - Difficult to maintain and extend

### 📊 Performance Benefits

- **Event Detection**: Automatic - no manual parsing required
- **Batch Processing**: Native support for SQS/S3 batches
- **Memory Usage**: Minimal overhead per event type
- **Cold Start**: <15ms additional overhead for multi-source support

## Testing Event Sources

```go
// Test different event types locally
func TestEventSources(t *testing.T) {
    app := testing.NewTestApp()
    setupEventHandlers(app.App())
    
    // Test SQS event
    sqsEvent := map[string]any{
        "Records": []any{
            map[string]any{
                "eventSource": "aws:sqs",
                "body":        `{"test": "data"}`,
                "messageId":   "test-123",
            },
        },
    }
    
    response := app.HandleRequest(context.Background(), sqsEvent)
    assert.Equal(t, 200, response.StatusCode)
    
    // Test EventBridge event
    ebEvent := map[string]any{
        "source":      "myapp.orders",
        "detail-type": "Order Placed",
        "detail":      map[string]any{"orderId": "123"},
    }
    
    response = app.HandleRequest(context.Background(), ebEvent)
    assert.Equal(t, 200, response.StatusCode)
}
```

## Deployment Configuration

### Serverless Framework Example

```yaml
# serverless.yml
functions:
  eventHandler:
    handler: bootstrap
    events:
      # API Gateway
      - http:
          path: /{proxy+}
          method: ANY
      # SQS
      - sqs:
          arn: arn:aws:sqs:us-east-1:123456789:my-queue
      # S3
      - s3:
          bucket: my-bucket
          event: s3:ObjectCreated:*
      # EventBridge
      - eventBridge:
          pattern:
            source: ["myapp.orders"]
      # Scheduled
      - schedule:
          rate: rate(1 hour)
```

## Next Steps

After mastering event adapters:

1. **Multi-Event Handler** → See `examples/multi-event-handler/`
2. **Production Patterns** → See `examples/production-api/`
3. **WebSocket Events** → See `examples/websocket-demo/`
4. **Observability** → See `examples/observability-demo/`

## Common Issues

### Issue: "Wrong trigger type detected"
**Cause:** Event routing misconfiguration
**Solution:** Check your serverless.yml event mappings

### Issue: "Records not populated"
**Cause:** Event doesn't have Records field (EventBridge, Scheduled)
**Solution:** Use appropriate fields for each event type

### Issue: "Handler not triggered"
**Cause:** Missing event source configuration
**Solution:** Ensure proper IAM permissions and event source mapping

This example provides the foundation for event-driven serverless architectures - master these patterns to build scalable, multi-source Lambda functions.