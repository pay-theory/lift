# Event-Driven API Pattern

The EventDrivenAPI pattern combines API Gateway with EventBridge to create asynchronous APIs with support for webhooks, callbacks, and event-driven processing. This pattern is ideal for long-running operations, webhook integrations, and microservice architectures.

## Overview

The EventDrivenAPI pattern provides:
- Asynchronous request processing with EventBridge
- Request tracking and status management
- Webhook support for external integrations
- Callback patterns for completion notifications
- Async validation capabilities
- Built-in retry and error handling

## Basic Usage

```go
import (
    "github.com/pay-theory/lift/pkg/cdk/patterns"
)

api := patterns.NewEventDrivenAPI(stack, jsii.String("MyAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("async-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
})
```

## Configuration Options

### Core Properties

| Property | Type | Description | Default |
|----------|------|-------------|---------|
| APIName | *string | Name of the API (used for resource naming) | Required |
| APICodeAssetPath | *string | Path to the API Lambda code | Required |
| EventProcessorCodeAssetPath | *string | Path to the event processor Lambda code | Required |
| CallbackHandlerCodeAssetPath | *string | Path to the callback handler Lambda code | nil |
| AsyncTimeoutMinutes | *float64 | Timeout for async operations | 5 |
| Environment | *map[string]*string | Additional environment variables | nil |

### Feature Flags

| Property | Type | Description |
|----------|------|-------------|
| EnableWebhooks | *bool | Enable webhook support |
| EnableCallbacks | *bool | Enable callback pattern support |
| EnableAsyncValidation | *bool | Enable async request validation |
| EnableRequestTracking | *bool | Enable request status tracking |
| EnableDLQ | *bool | Enable dead letter queue for failed events |

## Request Tracking

Enable request tracking to monitor async operation status:

```go
api := patterns.NewEventDrivenAPI(stack, jsii.String("MyAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("async-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableRequestTracking:       jsii.Bool(true),
})

// The tracking table includes:
// - Primary key: requestId
// - GSI 1: userId-timestamp for user queries
// - GSI 2: status-timestamp for status monitoring
// - TTL attribute for automatic cleanup
```

### Request Tracking Schema

| Attribute | Type | Description |
|-----------|------|-------------|
| requestId | String | Unique request identifier |
| userId | String | User who initiated the request |
| status | String | Current status (pending, processing, completed, failed) |
| timestamp | Number | Request initiation timestamp |
| result | Map | Processing result (when completed) |
| error | Map | Error details (when failed) |
| ttl | Number | Time-to-live for automatic deletion |

## Webhook Support

Enable webhooks for external system integration:

```go
api := patterns.NewEventDrivenAPI(stack, jsii.String("MyAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("webhook-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableWebhooks:              jsii.Bool(true),
    WebhookQueueProps: &awssqs.QueueProps{
        VisibilityTimeout: awscdk.Duration_Minutes(jsii.Number(10)),
        RetentionPeriod:   awscdk.Duration_Days(jsii.Number(14)),
    },
})

// Environment variables set:
// - WEBHOOKS_ENABLED=true
// - WEBHOOK_QUEUE_URL=<queue-url>
```

## Callback Pattern

Enable callbacks for completion notifications:

```go
api := patterns.NewEventDrivenAPI(stack, jsii.String("MyAPI"), &patterns.EventDrivenAPIProps{
    APIName:                      jsii.String("callback-api"),
    APICodeAssetPath:             jsii.String("./dist/api"),
    EventProcessorCodeAssetPath:  jsii.String("./dist/processor"),
    CallbackHandlerCodeAssetPath: jsii.String("./dist/callback"),
    EnableCallbacks:              jsii.Bool(true),
    EnableRequestTracking:        jsii.Bool(true),
})

// Environment variables set:
// - CALLBACKS_ENABLED=true
// - CALLBACK_URL=<api-url>/callback
// - CALLBACK_LAMBDA_ARN=<callback-function-arn>
```

## Async Validation

Enable asynchronous request validation:

```go
api := patterns.NewEventDrivenAPI(stack, jsii.String("MyAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("validated-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableAsyncValidation:       jsii.Bool(true),
})

// Creates a separate EventBridge rule for validation events:
// - Source: api.<api-name>
// - Detail Type: Validation Request
```

## Custom Event Patterns

Configure custom event patterns for processing:

```go
api := patterns.NewEventDrivenAPI(stack, jsii.String("MyAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("custom-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EventPattern: &awseventbridge.EventPattern{
        Source:     &[]*string{jsii.String("custom.source")},
        DetailType: &[]*string{jsii.String("Order"), jsii.String("Payment")},
        Detail: &map[string]interface{}{
            "status": &[]*string{jsii.String("new"), jsii.String("pending")},
        },
    },
    RetryPolicy: &awseventbridge.RetryPolicy{
        MaximumRetryAttempts: jsii.Number(5),
        MaximumEventAge:      awscdk.Duration_Hours(jsii.Number(1)),
    },
})
```

## Using Existing Resources

Integrate with existing AWS resources:

```go
existingBus := awseventbridge.EventBus_FromEventBusName(stack, jsii.String("Bus"), jsii.String("my-bus"))
existingTable := awsdynamodb.Table_FromTableName(stack, jsii.String("Table"), jsii.String("my-table"))
existingQueue := awssqs.Queue_FromQueueArn(stack, jsii.String("Queue"), jsii.String("arn:aws:sqs:..."))

api := patterns.NewEventDrivenAPI(stack, jsii.String("MyAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("integrated-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EventBus:                    existingBus,
    RequestTrackingTable:        existingTable,
    WebhookQueue:                existingQueue,
    EnableRequestTracking:       jsii.Bool(true),
    EnableWebhooks:              jsii.Bool(true),
})
```

## Environment Variables

The pattern automatically sets environment variables:

### Core Variables
- `API_NAME`: Name of the API
- `ASYNC_TIMEOUT_MINS`: Timeout for async operations
- `EVENT_BUS_NAME`: EventBridge bus name
- `EVENT_BUS_ARN`: EventBridge bus ARN
- `API_BASE_URL`: Base URL of the API

### Feature-Specific Variables
- `REQUEST_TRACKING_ENABLED`: "true" if tracking is enabled
- `REQUEST_TRACKING_TABLE_NAME`: DynamoDB table name
- `REQUEST_TRACKING_TABLE_ARN`: DynamoDB table ARN
- `CALLBACKS_ENABLED`: "true" if callbacks are enabled
- `CALLBACK_URL`: URL for callbacks
- `CALLBACK_LAMBDA_ARN`: ARN of callback Lambda
- `WEBHOOKS_ENABLED`: "true" if webhooks are enabled
- `WEBHOOK_QUEUE_URL`: SQS queue URL for webhooks
- `ASYNC_VALIDATION_ENABLED`: "true" if validation is enabled

## Helper Methods

The pattern provides helper methods:

```go
// Grant permissions
api.GrantPutEvents(myFunction)
api.GrantTrackingTableRead(myFunction)
api.GrantTrackingTableWrite(myFunction)

// Get resources
webhookQueue := api.GetWebhookQueue()

// Access metrics
asyncRequests := api.MetricAsyncRequests()
callbacks := api.MetricCallbacks()
webhooks := api.MetricWebhooks()
```

## Common Patterns

### Long-Running Operations

```go
// API endpoint initiates processing
func handleRequest(ctx *lift.Context) error {
    requestId := uuid.New().String()
    
    // Store initial request status
    err := trackRequest(requestId, "pending", ctx.UserID())
    if err != nil {
        return err
    }
    
    // Send event for processing
    event := map[string]interface{}{
        "requestId": requestId,
        "userId":    ctx.UserID(),
        "data":      ctx.Request.Body,
    }
    
    err = sendEvent("ProcessRequest", event)
    if err != nil {
        return err
    }
    
    return ctx.JSON(map[string]string{
        "requestId": requestId,
        "status":    "accepted",
    })
}

// Event processor handles the work
func processEvent(event events.EventBridgeEvent) error {
    var request ProcessRequest
    json.Unmarshal(event.Detail, &request)
    
    // Update status
    updateRequestStatus(request.RequestId, "processing")
    
    // Do the work
    result, err := performLongOperation(request)
    
    if err != nil {
        updateRequestStatus(request.RequestId, "failed", err)
        return err
    }
    
    // Update completion
    updateRequestStatus(request.RequestId, "completed", result)
    
    // Send callback if configured
    if callbacksEnabled {
        sendCallback(request.RequestId, result)
    }
    
    return nil
}
```

### Webhook Integration

```go
// Register webhook endpoint
func registerWebhook(ctx *lift.Context) error {
    var webhook WebhookConfig
    ctx.ParseRequest(&webhook)
    
    // Store webhook configuration
    err := storeWebhookConfig(webhook)
    if err != nil {
        return err
    }
    
    return ctx.JSON(map[string]string{
        "webhookId": webhook.ID,
        "status":    "registered",
    })
}

// Process events and send webhooks
func processWithWebhook(event events.EventBridgeEvent) error {
    // Process the event
    result := processBusinessLogic(event)
    
    // Get registered webhooks
    webhooks := getWebhooksForEventType(event.DetailType)
    
    for _, webhook := range webhooks {
        // Send to webhook queue for reliable delivery
        sendToWebhookQueue(webhook, result)
    }
    
    return nil
}

// Webhook processor with retries
func processWebhookQueue(event events.SQSEvent) error {
    for _, record := range event.Records {
        var webhook WebhookMessage
        json.Unmarshal([]byte(record.Body), &webhook)
        
        // Send with exponential backoff
        err := sendWebhookWithRetry(webhook)
        if err != nil {
            // Message will return to queue for retry
            return err
        }
    }
    return nil
}
```

### Status Polling

```go
// Status endpoint for polling
func getRequestStatus(ctx *lift.Context) error {
    requestId := ctx.Param("requestId")
    
    status, err := queryRequestStatus(requestId)
    if err != nil {
        return lift.NotFound("Request not found")
    }
    
    return ctx.JSON(status)
}

// Batch status check
func getBatchStatus(ctx *lift.Context) error {
    var requestIds []string
    ctx.ParseRequest(&requestIds)
    
    statuses, err := queryBatchStatus(requestIds)
    if err != nil {
        return err
    }
    
    return ctx.JSON(statuses)
}
```

## Advanced Patterns

### Event Orchestration

```go
api := patterns.NewEventDrivenAPI(stack, jsii.String("OrchestrationAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("orchestration-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/orchestrator"),
    EnableRequestTracking:       jsii.Bool(true),
    EventPattern: &awseventbridge.EventPattern{
        Source: &[]*string{
            jsii.String("order.service"),
            jsii.String("payment.service"),
            jsii.String("shipping.service"),
        },
    },
})

// Orchestrator tracks multi-step workflows
func orchestrateWorkflow(event events.EventBridgeEvent) error {
    switch event.Source {
    case "order.service":
        return handleOrderEvent(event)
    case "payment.service":
        return handlePaymentEvent(event)
    case "shipping.service":
        return handleShippingEvent(event)
    }
    return nil
}
```

### Rate-Limited Processing

```go
api := patterns.NewEventDrivenAPI(stack, jsii.String("RateLimitedAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("rate-limited-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableWebhooks:              jsii.Bool(true),
    APIProps: &constructs.LiftAPIProps{
        // Configure API-level rate limiting
        ThrottleSettings: &apigateway.ThrottleSettings{
            BurstLimit: jsii.Number(100),
            RateLimit:  jsii.Number(50),
        },
    },
    // Configure event processor concurrency
    Environment: &map[string]*string{
        "MAX_CONCURRENT_REQUESTS": jsii.String("10"),
    },
})
```

### Multi-Region Deployment

```go
// Primary region
primaryAPI := patterns.NewEventDrivenAPI(primaryStack, jsii.String("PrimaryAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("global-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableRequestTracking:       jsii.Bool(true),
})

// Secondary region with cross-region event bus
secondaryAPI := patterns.NewEventDrivenAPI(secondaryStack, jsii.String("SecondaryAPI"), &patterns.EventDrivenAPIProps{
    APIName:                     jsii.String("global-api"),
    APICodeAssetPath:            jsii.String("./dist/api"),
    EventProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EventBus:                    primaryAPI.EventBus, // Share event bus
})
```

## Best Practices

### 1. Request Design
- Use UUIDs for request IDs
- Include correlation IDs for tracing
- Set appropriate TTLs for tracking records
- Implement idempotency for retries

### 2. Event Processing
- Keep event payloads small (<256KB)
- Use S3 for large payloads with presigned URLs
- Implement proper error handling and retries
- Monitor DLQ for failed events

### 3. Performance
- Configure appropriate Lambda memory/timeout
- Use SQS for webhook delivery (reliability)
- Implement circuit breakers for external calls
- Cache frequently accessed data

### 4. Security
- Validate all incoming requests
- Use API keys or authentication
- Encrypt sensitive data in events
- Implement request signing for webhooks

## Monitoring

Monitor your event-driven API:

```go
// CloudWatch Dashboard
dashboard := cloudwatch.NewDashboard(stack, jsii.String("APIDashboard"), &cloudwatch.DashboardProps{
    DashboardName: jsii.String("event-driven-api"),
})

dashboard.AddWidgets(
    cloudwatch.NewGraphWidget(&cloudwatch.GraphWidgetProps{
        Title: jsii.String("Async Requests"),
        Left: &[]cloudwatch.IMetric{
            api.MetricAsyncRequests(),
        },
    }),
    cloudwatch.NewGraphWidget(&cloudwatch.GraphWidgetProps{
        Title: jsii.String("Processing Time"),
        Left: &[]cloudwatch.IMetric{
            api.EventProcessor.Function.MetricDuration(),
        },
    }),
)

// Alarms
cloudwatch.NewAlarm(stack, jsii.String("FailedRequests"), &cloudwatch.AlarmProps{
    Metric:            api.EventProcessor.Function.MetricErrors(),
    Threshold:         jsii.Number(10),
    EvaluationPeriods: jsii.Number(1),
})
```

## Complete Example

```go
// E-commerce order processing API
orderAPI := patterns.NewEventDrivenAPI(stack, jsii.String("OrderAPI"), &patterns.EventDrivenAPIProps{
    APIName:                      jsii.String("order-api"),
    APICodeAssetPath:             jsii.String("./dist/order-api"),
    EventProcessorCodeAssetPath:  jsii.String("./dist/order-processor"),
    CallbackHandlerCodeAssetPath: jsii.String("./dist/order-callback"),
    EnableRequestTracking:        jsii.Bool(true),
    EnableCallbacks:              jsii.Bool(true),
    EnableWebhooks:               jsii.Bool(true),
    EnableAsyncValidation:        jsii.Bool(true),
    AsyncTimeoutMinutes:          jsii.Number(30),
    
    APIProps: &constructs.LiftAPIProps{
        EnableCORS:     jsii.Bool(true),
        DomainName:     jsii.String("api.example.com"),
        APIKeyRequired: jsii.Bool(true),
    },
    
    EventPattern: &awseventbridge.EventPattern{
        Source:     &[]*string{jsii.String("order.api")},
        DetailType: &[]*string{
            jsii.String("Order Created"),
            jsii.String("Payment Processed"),
            jsii.String("Shipment Dispatched"),
        },
    },
    
    Environment: &map[string]*string{
        "PAYMENT_API_URL":    jsii.String("https://payment.api.com"),
        "INVENTORY_API_URL":  jsii.String("https://inventory.api.com"),
        "NOTIFICATION_EMAIL": jsii.String("orders@example.com"),
    },
})

// Grant permissions to external services
paymentService := lambda.Function_FromFunctionArn(stack, jsii.String("PaymentService"), paymentArn)
orderAPI.GrantPutEvents(paymentService)
orderAPI.GrantTrackingTableRead(paymentService)
```

## Troubleshooting

### Events Not Processing
1. Check EventBridge rule is active
2. Verify Lambda has permissions
3. Check event pattern matches
4. Review CloudWatch logs

### Webhooks Failing
1. Check SQS queue for messages
2. Verify webhook URLs are valid
3. Review retry configuration
4. Check DLQ for failures

### Request Tracking Issues
1. Verify DynamoDB table exists
2. Check TTL configuration
3. Monitor table throttling
4. Review GSI usage
