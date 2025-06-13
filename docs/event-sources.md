# Event Sources

Lift automatically detects and adapts various AWS Lambda event sources into a unified request format. This guide covers all supported event sources and how to work with them.

## Overview

Lift's event adapter system provides:

- **Automatic Detection**: Identifies event type without configuration
- **Unified Interface**: Same handler signature for all events
- **Type Safety**: Strongly typed access to event data
- **Zero Configuration**: Works out of the box

## API Gateway (HTTP/REST)

The most common event source for web APIs.

### API Gateway V2 (HTTP API)

```go
// Automatic routing for HTTP methods
app.GET("/users", getUsers)
app.POST("/users", createUser)
app.PUT("/users/:id", updateUser)
app.DELETE("/users/:id", deleteUser)

func getUsers(ctx *lift.Context) error {
    // Query parameters
    page := ctx.Query("page")
    limit := ctx.Query("limit")
    
    // Headers
    auth := ctx.Header("Authorization")
    
    // Return JSON
    return ctx.JSON(users)
}

func updateUser(ctx *lift.Context) error {
    // Path parameters
    userID := ctx.Param("id")
    
    // Parse body
    var updates UserUpdate
    if err := ctx.ParseJSON(&updates); err != nil {
        return lift.BadRequest("Invalid request body")
    }
    
    // Update user...
    return ctx.JSON(updatedUser)
}
```

### API Gateway V1 (REST API)

```go
// V1 supports same patterns with additional features
app.GET("/users", handleUsers)

func handleUsers(ctx *lift.Context) error {
    // Stage variables (V1 specific)
    stage := ctx.Request.Metadata["stage"].(string)
    
    // Request context (V1 specific)
    requestContext := ctx.Request.Metadata["requestContext"].(map[string]interface{})
    apiID := requestContext["apiId"].(string)
    
    return ctx.JSON(response)
}
```

### Request/Response Features

```go
// Status codes
ctx.Status(201).JSON(created)
ctx.NoContent() // 204

// Headers
ctx.Header("X-Request-ID", requestID)
ctx.Header("Cache-Control", "max-age=3600")

// Cookies (through headers)
ctx.Header("Set-Cookie", "session=abc123; HttpOnly; Secure")

// Binary responses
imageData := loadImage()
ctx.Header("Content-Type", "image/png").Binary(imageData)
```

## WebSocket API

Full WebSocket support for real-time applications.

### Connection Management

```go
// Connection lifecycle
app.Handle("CONNECT", "/connect", handleConnect)
app.Handle("DISCONNECT", "/disconnect", handleDisconnect)
app.Handle("MESSAGE", "/message", handleMessage)
app.Handle("MESSAGE", "/chat", handleChat)

func handleConnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    connectionID := wsCtx.ConnectionID()
    
    // Extract auth from query params (WebSocket standard)
    token := ctx.Query("token")
    if !validateToken(token) {
        return lift.Unauthorized("Invalid token")
    }
    
    // Store connection
    storeConnection(connectionID, extractUserID(token))
    
    return ctx.JSON(map[string]string{
        "message": "Connected successfully",
    })
}

func handleDisconnect(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    removeConnection(wsCtx.ConnectionID())
    return nil
}
```

### Sending Messages

```go
func handleMessage(ctx *lift.Context) error {
    wsCtx, _ := ctx.AsWebSocket()
    
    var msg ChatMessage
    if err := ctx.ParseJSON(&msg); err != nil {
        return err
    }
    
    // Send to specific connection
    err := wsCtx.SendMessage(targetConnectionID, msg)
    
    // Broadcast to all connections
    connections := getActiveConnections()
    err = wsCtx.BroadcastMessage(connections, msg)
    
    // Send back to sender
    err = wsCtx.Reply(map[string]string{
        "status": "message sent",
    })
    
    return err
}
```

### WebSocket Context Methods

```go
type WebSocketContext interface {
    ConnectionID() string
    RouteKey() string
    EventType() string
    RequestID() string
    DomainName() string
    Stage() string
    
    // Messaging
    SendMessage(connectionID string, message interface{}) error
    BroadcastMessage(connectionIDs []string, message interface{}) error
    Reply(message interface{}) error
}
```

## SQS (Simple Queue Service)

Process messages from SQS queues with automatic batching support.

### Basic Queue Processing

```go
app.Handle("SQS", "/process-orders", processOrders)

func processOrders(ctx *lift.Context) error {
    // SQS sends batches of messages
    for _, record := range ctx.Request.Records {
        // Type assertion for SQS record
        sqsRecord := record.(map[string]interface{})
        
        // Message body
        body := sqsRecord["body"].(string)
        
        // Message attributes
        attrs := sqsRecord["messageAttributes"].(map[string]interface{})
        
        // Process message
        var order Order
        if err := json.Unmarshal([]byte(body), &order); err != nil {
            // Log error but continue processing other messages
            ctx.Logger.Error("Failed to parse order", map[string]interface{}{
                "error": err,
                "body":  body,
            })
            continue
        }
        
        // Process order
        if err := processOrder(order); err != nil {
            // Return error to retry this message
            return err
        }
    }
    
    return nil
}
```

### Advanced Queue Features

```go
func processMessages(ctx *lift.Context) error {
    var errors []error
    
    for i, record := range ctx.Request.Records {
        sqsRecord := record.(map[string]interface{})
        
        // Message metadata
        messageID := sqsRecord["messageId"].(string)
        receiptHandle := sqsRecord["receiptHandle"].(string)
        
        // FIFO queue specific
        if sequenceNumber, ok := sqsRecord["sequenceNumber"].(string); ok {
            // Handle FIFO message
        }
        
        // Process with error handling
        if err := processSingleMessage(sqsRecord); err != nil {
            errors = append(errors, fmt.Errorf("message %s: %w", messageID, err))
        }
    }
    
    // Partial batch failure (Lambda 2.0)
    if len(errors) > 0 {
        return lift.PartialBatchFailure(errors)
    }
    
    return nil
}
```

## S3 Events

Handle S3 object events for file processing pipelines.

### Object Created/Removed

```go
app.Handle("S3", "/process-upload", handleS3Upload)

func handleS3Upload(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        s3Record := record.(map[string]interface{})
        
        // Event details
        eventName := s3Record["eventName"].(string) // e.g., "s3:ObjectCreated:Put"
        
        // S3 object details
        s3Data := s3Record["s3"].(map[string]interface{})
        bucket := s3Data["bucket"].(map[string]interface{})
        object := s3Data["object"].(map[string]interface{})
        
        bucketName := bucket["name"].(string)
        objectKey := object["key"].(string)
        objectSize := object["size"].(float64)
        
        // Process based on event type
        switch {
        case strings.HasPrefix(eventName, "s3:ObjectCreated:"):
            err := processNewFile(bucketName, objectKey, int64(objectSize))
        case strings.HasPrefix(eventName, "s3:ObjectRemoved:"):
            err := handleFileDeleted(bucketName, objectKey)
        }
        
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### File Type Processing

```go
func processUpload(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        s3Record := extractS3Record(record)
        key := s3Record.ObjectKey
        
        // Route by file type
        switch {
        case strings.HasSuffix(key, ".jpg"), strings.HasSuffix(key, ".png"):
            err := processImage(s3Record)
        case strings.HasSuffix(key, ".pdf"):
            err := processPDF(s3Record)
        case strings.HasSuffix(key, ".csv"):
            err := processCSV(s3Record)
        default:
            ctx.Logger.Warn("Unknown file type", map[string]interface{}{
                "key": key,
            })
        }
    }
    
    return nil
}
```

## EventBridge

Handle custom application events and scheduled events.

### Custom Events

```go
app.Handle("EventBridge", "/user-events", handleUserEvents)

func handleUserEvents(ctx *lift.Context) error {
    // EventBridge event structure
    event := ctx.Request.Metadata["detail"].(map[string]interface{})
    eventType := ctx.Request.Metadata["detail-type"].(string)
    source := ctx.Request.Metadata["source"].(string)
    
    switch eventType {
    case "UserCreated":
        var userCreated UserCreatedEvent
        mapstructure.Decode(event, &userCreated)
        return handleUserCreated(userCreated)
        
    case "UserDeleted":
        var userDeleted UserDeletedEvent
        mapstructure.Decode(event, &userDeleted)
        return handleUserDeleted(userDeleted)
        
    default:
        return fmt.Errorf("unknown event type: %s", eventType)
    }
}
```

### Event Patterns

```go
// EventBridge rule pattern example:
// {
//   "source": ["myapp.users"],
//   "detail-type": ["UserCreated", "UserUpdated"]
// }

func handleUserEvent(ctx *lift.Context) error {
    // Access pattern-matched fields
    account := ctx.Request.Metadata["account"].(string)
    region := ctx.Request.Metadata["region"].(string)
    time := ctx.Request.Metadata["time"].(string)
    
    // Process event...
    return nil
}
```

## Scheduled Events (CloudWatch Events)

Handle periodic tasks and cron jobs.

### Basic Schedule

```go
app.Handle("Scheduled", "/daily-report", generateDailyReport)

func generateDailyReport(ctx *lift.Context) error {
    // Scheduled event metadata
    scheduledTime := ctx.Request.Metadata["time"].(string)
    
    ctx.Logger.Info("Generating daily report", map[string]interface{}{
        "scheduled_time": scheduledTime,
    })
    
    // Generate report...
    report := generateReport()
    
    // Send via email/S3/etc
    return sendReport(report)
}
```

### Rate vs Cron

```go
// Rate: rate(5 minutes)
app.Handle("Scheduled", "/health-check", performHealthCheck)

// Cron: cron(0 12 * * ? *) - Daily at noon UTC
app.Handle("Scheduled", "/daily-backup", performBackup)

func performHealthCheck(ctx *lift.Context) error {
    // Check service health
    results := checkAllServices()
    
    // Alert if issues found
    if !results.Healthy {
        return alertOncall(results)
    }
    
    return nil
}
```

## DynamoDB Streams

Process changes to DynamoDB tables.

```go
app.Handle("DynamoDBStream", "/user-changes", handleUserChanges)

func handleUserChanges(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        streamRecord := record.(map[string]interface{})
        
        eventName := streamRecord["eventName"].(string)
        dynamodb := streamRecord["dynamodb"].(map[string]interface{})
        
        switch eventName {
        case "INSERT":
            newImage := dynamodb["NewImage"].(map[string]interface{})
            handleNewUser(newImage)
            
        case "MODIFY":
            oldImage := dynamodb["OldImage"].(map[string]interface{})
            newImage := dynamodb["NewImage"].(map[string]interface{})
            handleUserUpdate(oldImage, newImage)
            
        case "REMOVE":
            oldImage := dynamodb["OldImage"].(map[string]interface{})
            handleUserDeletion(oldImage)
        }
    }
    
    return nil
}
```

## Kinesis Streams

Process streaming data from Kinesis.

```go
app.Handle("Kinesis", "/clickstream", processClickstream)

func processClickstream(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        kinesisRecord := record.(map[string]interface{})
        
        // Decode base64 data
        data := kinesisRecord["kinesis"].(map[string]interface{})["data"].(string)
        decoded, _ := base64.StdEncoding.DecodeString(data)
        
        // Process click event
        var clickEvent ClickEvent
        json.Unmarshal(decoded, &clickEvent)
        
        // Aggregate or forward
        processClick(clickEvent)
    }
    
    return nil
}
```

## SNS (Simple Notification Service)

Handle notifications from SNS topics.

```go
app.Handle("SNS", "/notifications", handleNotification)

func handleNotification(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        snsRecord := record.(map[string]interface{})
        sns := snsRecord["Sns"].(map[string]interface{})
        
        message := sns["Message"].(string)
        subject := sns["Subject"].(string)
        topicArn := sns["TopicArn"].(string)
        
        // Message attributes
        attrs := sns["MessageAttributes"].(map[string]interface{})
        
        // Process notification
        switch topicArn {
        case orderTopicArn:
            processOrderNotification(message)
        case alertTopicArn:
            processAlert(message, attrs)
        }
    }
    
    return nil
}
```

## Event Source Metadata

Access raw event data when needed:

```go
func handleGenericEvent(ctx *lift.Context) error {
    // Event source type
    trigger := ctx.Request.TriggerType
    
    // Raw event metadata
    metadata := ctx.Request.Metadata
    
    // Common metadata fields
    if requestID, ok := metadata["requestId"].(string); ok {
        ctx.Logger.Info("Processing request", map[string]interface{}{
            "request_id": requestID,
            "trigger":    trigger,
        })
    }
    
    // Source-specific handling
    switch trigger {
    case lift.TriggerAPIGateway:
        stage := metadata["stage"].(string)
        // API Gateway specific logic
        
    case lift.TriggerSQS:
        queueArn := metadata["eventSourceARN"].(string)
        // SQS specific logic
    }
    
    return nil
}
```

## Multi-Event Lambda

Handle multiple event sources in one Lambda:

```go
func main() {
    app := lift.New()
    
    // HTTP API endpoints
    app.GET("/api/users", getUsers)
    app.POST("/api/users", createUser)
    
    // Async processing
    app.Handle("SQS", "/process-orders", processOrders)
    app.Handle("S3", "/process-uploads", processUploads)
    
    // Scheduled tasks
    app.Handle("Scheduled", "/daily-report", dailyReport)
    
    lambda.Start(app.HandleRequest)
}
```

## Best Practices

### 1. Event Source Detection

Let Lift automatically detect event sources:

```go
// DON'T: Manual type checking
func handler(ctx context.Context, event interface{}) error {
    switch e := event.(type) {
    case events.APIGatewayProxyRequest:
        // Handle API Gateway
    case events.SQSEvent:
        // Handle SQS
    }
}

// DO: Use Lift's routing
app.GET("/users", handleUsers)
app.Handle("SQS", "/queue", handleQueue)
```

### 2. Error Handling by Source

Different event sources have different error semantics:

```go
// API Gateway - Return user-friendly errors
func handleAPI(ctx *lift.Context) error {
    if err := validateInput(); err != nil {
        return lift.BadRequest("Invalid input: " + err.Error())
    }
    return ctx.JSON(response)
}

// SQS - Return error to retry message
func handleSQS(ctx *lift.Context) error {
    if err := processMessage(); err != nil {
        return err // Message will be retried
    }
    return nil
}

// S3 - Log and continue for batch
func handleS3(ctx *lift.Context) error {
    for _, record := range ctx.Request.Records {
        if err := processFile(record); err != nil {
            ctx.Logger.Error("Failed to process file", map[string]interface{}{
                "error": err,
            })
            // Continue processing other files
        }
    }
    return nil
}
```

### 3. Batch Processing

Handle batch events efficiently:

```go
func processBatch(ctx *lift.Context) error {
    // Process in parallel for better performance
    var wg sync.WaitGroup
    errors := make(chan error, len(ctx.Request.Records))
    
    for _, record := range ctx.Request.Records {
        wg.Add(1)
        go func(r interface{}) {
            defer wg.Done()
            if err := processRecord(r); err != nil {
                errors <- err
            }
        }(record)
    }
    
    wg.Wait()
    close(errors)
    
    // Collect errors
    var errs []error
    for err := range errors {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return lift.PartialBatchFailure(errs)
    }
    
    return nil
}
```

### 4. Event Source Configuration

Configure your Lambda appropriately for each event source:

```yaml
# serverless.yml example
functions:
  api:
    handler: bin/api
    events:
      - httpApi: '*'
    
  processor:
    handler: bin/processor
    reservedConcurrency: 10
    events:
      - sqs:
          arn: ${QueueArn}
          batchSize: 25
          maximumBatchingWindowInSeconds: 5
      
  scheduled:
    handler: bin/scheduled
    events:
      - schedule: rate(1 hour)
```

## Summary

Lift's event source support provides:

- **Unified Interface**: Same programming model for all events
- **Type Safety**: Strongly typed access to event data
- **Auto-Detection**: No configuration needed
- **Best Practices**: Built-in patterns for each source

This allows you to focus on your business logic rather than event parsing and handling. 