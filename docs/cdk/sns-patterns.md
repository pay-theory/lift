# SNS Patterns with Lift CDK

This guide demonstrates how to use the SNSProcessor construct to implement pub/sub messaging patterns in your Lift applications.

## Table of Contents
- [Overview](#overview)
- [Basic Usage](#basic-usage)
- [Advanced Configurations](#advanced-configurations)
- [Message Filtering](#message-filtering)
- [FIFO Topics](#fifo-topics)
- [Error Handling](#error-handling)
- [Common Patterns](#common-patterns)
- [Performance Optimization](#performance-optimization)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

The `SNSProcessor` construct provides a complete SNS topic with Lambda processor setup, including:
- Automatic topic creation and configuration
- Lambda function integration with event source mapping
- Dead letter queue (DLQ) support for failed messages
- Message filtering capabilities
- FIFO topic support for ordered processing
- Multi-tenant environment variable injection

## Basic Usage

### Simple SNS Processor

```go
import (
    "github.com/pay-theory/lift/pkg/cdk/constructs"
    "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
)

processor := constructs.NewSNSProcessor(stack, jsii.String("OrderProcessor"), &constructs.SNSProcessorProps{
    FunctionProps: &constructs.LiftFunctionProps{
        Runtime: awslambda.Runtime_PROVIDED_AL2023(),
        Handler: jsii.String("bootstrap"),
        Code:    awslambda.Code_FromAsset(jsii.String("./dist"), nil),
        Environment: &map[string]*string{
            "SERVICE_NAME": jsii.String("order-service"),
        },
    },
})
```

### Publishing Messages

```go
// Grant publish permissions to another Lambda
otherFunction := constructs.NewLiftFunction(stack, jsii.String("Publisher"), publisherProps)
processor.GrantPublish(otherFunction.Function)

// In your Go application code
import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/sns"
)

func publishMessage(topicArn string, message interface{}) error {
    sess := session.Must(session.NewSession())
    snsClient := sns.New(sess)
    
    messageJSON, _ := json.Marshal(message)
    
    _, err := snsClient.Publish(&sns.PublishInput{
        TopicArn: aws.String(topicArn),
        Message:  aws.String(string(messageJSON)),
        MessageAttributes: map[string]*sns.MessageAttributeValue{
            "eventType": {
                DataType:    aws.String("String"),
                StringValue: aws.String("order-created"),
            },
        },
    })
    return err
}
```

## Advanced Configurations

### Custom Topic Configuration

```go
processor := constructs.NewSNSProcessor(stack, jsii.String("NotificationTopic"), &constructs.SNSProcessorProps{
    FunctionProps: functionProps,
    TopicProps: &awssns.TopicProps{
        TopicName:   jsii.String("customer-notifications"),
        DisplayName: jsii.String("Customer Notification Topic"),
    },
    MessageRetentionSeconds: jsii.Number(86400), // 1 day
    DLQProps: &awssqs.QueueProps{
        QueueName:       jsii.String("notifications-dlq"),
        RetentionPeriod: awscdk.Duration_Days(jsii.Number(7)),
    },
})
```

### Using Existing Topics

```go
// Import existing topic
existingTopic := awssns.Topic_FromTopicArn(
    stack,
    jsii.String("ImportedTopic"),
    jsii.String("arn:aws:sns:us-east-1:123456789012:existing-topic"),
)

processor := constructs.NewSNSProcessor(stack, jsii.String("Processor"), &constructs.SNSProcessorProps{
    FunctionProps: functionProps,
    ExistingTopic: existingTopic,
})
```

## Message Filtering

### Attribute-Based Filtering

```go
processor := constructs.NewSNSProcessor(stack, jsii.String("FilteredProcessor"), &constructs.SNSProcessorProps{
    FunctionProps: functionProps,
    FilterPolicy: &map[string]awssns.SubscriptionFilter{
        "eventType": awssns.SubscriptionFilter_StringFilter(&awssns.StringConditions{
            Allowlist: &[]*string{
                jsii.String("order-created"),
                jsii.String("order-updated"),
                jsii.String("order-cancelled"),
            },
        }),
        "customerId": awssns.SubscriptionFilter_ExistsFilter(),
        "orderValue": awssns.SubscriptionFilter_NumericFilter(&awssns.NumericConditions{
            GreaterThan: jsii.Number(100),
            LessThan:    jsii.Number(10000),
        }),
    },
})
```

### Complex Filter Patterns

```go
// Filter for high-value orders from specific regions
filterPolicy := &map[string]awssns.SubscriptionFilter{
    "region": awssns.SubscriptionFilter_StringFilter(&awssns.StringConditions{
        Allowlist: &[]*string{
            jsii.String("us-east-1"),
            jsii.String("eu-west-1"),
        },
    }),
    "orderValue": awssns.SubscriptionFilter_NumericFilter(&awssns.NumericConditions{
        GreaterThanOrEqualTo: jsii.Number(1000),
    }),
    "isPriority": awssns.SubscriptionFilter_StringFilter(&awssns.StringConditions{
        Allowlist: &[]*string{jsii.String("true")},
    }),
}
```

## FIFO Topics

### Ordered Message Processing

```go
processor := constructs.NewSNSProcessor(stack, jsii.String("OrderedProcessor"), &constructs.SNSProcessorProps{
    FunctionProps: functionProps,
    EnableFifo: jsii.Bool(true),
    ContentBasedDeduplication: jsii.Bool(true),
    TopicProps: &awssns.TopicProps{
        TopicName: jsii.String("orders.fifo"), // Must end with .fifo
    },
})

// Publishing to FIFO topic
func publishFifoMessage(topicArn string, message interface{}, groupId string) error {
    sess := session.Must(session.NewSession())
    snsClient := sns.New(sess)
    
    messageJSON, _ := json.Marshal(message)
    
    _, err := snsClient.Publish(&sns.PublishInput{
        TopicArn:               aws.String(topicArn),
        Message:                aws.String(string(messageJSON)),
        MessageGroupId:         aws.String(groupId), // Required for FIFO
        MessageDeduplicationId: aws.String(uuid.New().String()), // Or use content-based
    })
    return err
}
```

## Error Handling

### Dead Letter Queue Configuration

```go
processor := constructs.NewSNSProcessor(stack, jsii.String("ResilientProcessor"), &constructs.SNSProcessorProps{
    FunctionProps: functionProps,
    EnableDLQ: jsii.Bool(true), // Enabled by default
    DLQProps: &awssqs.QueueProps{
        QueueName:               jsii.String("failed-messages-dlq"),
        RetentionPeriod:         awscdk.Duration_Days(jsii.Number(14)),
        VisibilityTimeout:       awscdk.Duration_Minutes(jsii.Number(5)),
        MessageRetentionSeconds: jsii.Number(1209600), // 14 days
    },
})
```

### Processing DLQ Messages

```go
// Create a separate processor for DLQ messages
dlqProcessor := constructs.NewSQSProcessor(stack, jsii.String("DLQProcessor"), &constructs.SQSProcessorProps{
    FunctionProps: &constructs.LiftFunctionProps{
        Runtime: awslambda.Runtime_PROVIDED_AL2023(),
        Handler: jsii.String("bootstrap"),
        Code:    awslambda.Code_FromAsset(jsii.String("./dist/dlq-handler"), nil),
    },
    ExistingQueue: processor.DLQ, // Use the SNS processor's DLQ
    EnableDLQ:     jsii.Bool(false), // Don't create another DLQ
})
```

## Common Patterns

### Fan-Out Pattern

```go
// Central topic for all orders
orderTopic := constructs.NewSNSProcessor(stack, jsii.String("OrderTopic"), &constructs.SNSProcessorProps{
    FunctionProps: &constructs.LiftFunctionProps{
        Runtime: awslambda.Runtime_PROVIDED_AL2023(),
        Handler: jsii.String("bootstrap"),
        Code:    awslambda.Code_FromAsset(jsii.String("./dist/order-logger"), nil),
    },
})

// Add additional subscribers
inventoryFunction := constructs.NewLiftFunction(stack, jsii.String("InventoryUpdater"), inventoryProps)
orderTopic.Topic.AddSubscription(
    awssnssubscriptions.NewLambdaSubscription(inventoryFunction.Function, &awssnssubscriptions.LambdaSubscriptionProps{
        FilterPolicy: &map[string]awssns.SubscriptionFilter{
            "eventType": awssns.SubscriptionFilter_StringFilter(&awssns.StringConditions{
                Allowlist: &[]*string{jsii.String("order-created")},
            }),
        },
    }),
)

// Email notifications for high-value orders
orderTopic.Topic.AddSubscription(
    awssnssubscriptions.NewEmailSubscription(jsii.String("alerts@example.com"), &awssnssubscriptions.EmailSubscriptionProps{
        FilterPolicy: &map[string]awssns.SubscriptionFilter{
            "orderValue": awssns.SubscriptionFilter_NumericFilter(&awssns.NumericConditions{
                GreaterThan: jsii.Number(5000),
            }),
        },
    }),
)
```

### Multi-Region Pub/Sub

```go
// Create topics in multiple regions
regions := []string{"us-east-1", "eu-west-1", "ap-southeast-1"}

for _, region := range regions {
    regionalStack := awscdk.NewStack(app, jsii.String(fmt.Sprintf("Stack-%s", region)), &awscdk.StackProps{
        Env: &awscdk.Environment{
            Region: jsii.String(region),
        },
    })
    
    processor := constructs.NewSNSProcessor(regionalStack, jsii.String("RegionalProcessor"), &constructs.SNSProcessorProps{
        FunctionProps: functionProps,
        TopicProps: &awssns.TopicProps{
            TopicName: jsii.String(fmt.Sprintf("orders-%s", region)),
        },
    })
}
```

### Event Broadcasting

```go
// Broadcast events to multiple systems
broadcaster := constructs.NewSNSProcessor(stack, jsii.String("EventBroadcaster"), &constructs.SNSProcessorProps{
    FunctionProps: &constructs.LiftFunctionProps{
        Runtime: awslambda.Runtime_PROVIDED_AL2023(),
        Handler: jsii.String("bootstrap"),
        Code:    awslambda.Code_FromAsset(jsii.String("./dist/event-router"), nil),
    },
    TopicProps: &awssns.TopicProps{
        TopicName:   jsii.String("system-events"),
        DisplayName: jsii.String("System Event Broadcaster"),
    },
})

// Add different subscribers for different event types
// Analytics system
broadcaster.Topic.AddSubscription(
    awssnssubscriptions.NewSqsSubscription(analyticsQueue, &awssnssubscriptions.SqsSubscriptionProps{
        RawMessageDelivery: jsii.Bool(true),
    }),
)

// Audit system
broadcaster.Topic.AddSubscription(
    awssnssubscriptions.NewLambdaSubscription(auditFunction.Function, nil),
)
```

## Performance Optimization

### Batch Processing

```go
// In your Lambda handler
type SNSEvent struct {
    Records []SNSRecord `json:"Records"`
}

type SNSRecord struct {
    Sns SNSMessage `json:"Sns"`
}

type SNSMessage struct {
    Message           string                 `json:"Message"`
    MessageAttributes map[string]interface{} `json:"MessageAttributes"`
    TopicArn          string                 `json:"TopicArn"`
    Subject           string                 `json:"Subject"`
}

func handler(ctx context.Context, event SNSEvent) error {
    // Process all records in batch
    var messages []OrderMessage
    for _, record := range event.Records {
        var msg OrderMessage
        if err := json.Unmarshal([]byte(record.Sns.Message), &msg); err != nil {
            log.Printf("Failed to parse message: %v", err)
            continue
        }
        messages = append(messages, msg)
    }
    
    // Batch process messages
    return processBatch(messages)
}
```

### Connection Pooling

```go
// Initialize SDK clients outside handler
var (
    dynamoClient *dynamodb.DynamoDB
    snsClient    *sns.SNS
)

func init() {
    sess := session.Must(session.NewSession())
    dynamoClient = dynamodb.New(sess)
    snsClient = sns.New(sess)
}

func handler(ctx context.Context, event SNSEvent) error {
    // Reuse clients across invocations
    // Process messages...
}
```

## Best Practices

### 1. Message Structure

```go
// Define clear message schemas
type OrderEvent struct {
    EventID     string    `json:"eventId"`
    EventType   string    `json:"eventType"`
    EventTime   time.Time `json:"eventTime"`
    OrderID     string    `json:"orderId"`
    CustomerID  string    `json:"customerId"`
    TenantID    string    `json:"tenantId,omitempty"`
    OrderValue  float64   `json:"orderValue"`
    Items       []Item    `json:"items"`
}
```

### 2. Idempotent Processing

```go
func processMessage(msg OrderEvent) error {
    // Check if already processed
    processed, err := checkProcessed(msg.EventID)
    if err != nil {
        return err
    }
    if processed {
        log.Printf("Message %s already processed", msg.EventID)
        return nil
    }
    
    // Process message
    if err := processOrder(msg); err != nil {
        return err
    }
    
    // Mark as processed
    return markProcessed(msg.EventID)
}
```

### 3. Error Handling

```go
func handler(ctx context.Context, event SNSEvent) error {
    var errors []error
    
    for _, record := range event.Records {
        if err := processRecord(record); err != nil {
            log.Printf("Failed to process record: %v", err)
            errors = append(errors, err)
            // Continue processing other records
        }
    }
    
    // Return error only if all records failed
    if len(errors) == len(event.Records) {
        return fmt.Errorf("all records failed to process")
    }
    
    return nil // Partial success
}
```

### 4. Monitoring and Alerting

```go
processor := constructs.NewSNSProcessor(stack, jsii.String("MonitoredProcessor"), &constructs.SNSProcessorProps{
    FunctionProps: &constructs.LiftFunctionProps{
        Runtime: awslambda.Runtime_PROVIDED_AL2023(),
        Handler: jsii.String("bootstrap"),
        Code:    awslambda.Code_FromAsset(jsii.String("./dist"), nil),
        TracingEnabled: jsii.Bool(true), // Enable X-Ray tracing
        Environment: &map[string]*string{
            "LOG_LEVEL": jsii.String("INFO"),
        },
    },
})

// Add CloudWatch alarms
awscloudwatch.NewAlarm(stack, jsii.String("ProcessorErrors"), &awscloudwatch.AlarmProps{
    Metric: processor.Function.Function.MetricErrors(&awscloudwatch.MetricOptions{
        Period: awscdk.Duration_Minutes(jsii.Number(5)),
    }),
    Threshold:          jsii.Number(10),
    EvaluationPeriods:  jsii.Number(2),
})
```

## Troubleshooting

### Common Issues

1. **Messages not being delivered**
   - Check IAM permissions for SNS to invoke Lambda
   - Verify subscription filter policies match message attributes
   - Check CloudWatch logs for Lambda execution errors

2. **Duplicate message processing**
   - Implement idempotency using message deduplication ID
   - For FIFO topics, ensure proper message group ID usage
   - Store processed message IDs in DynamoDB with TTL

3. **High latency**
   - Optimize Lambda memory allocation
   - Use connection pooling for AWS SDK clients
   - Consider using FIFO topics if ordering is required

4. **Messages in DLQ**
   - Check Lambda function logs for errors
   - Verify message format matches expected schema
   - Ensure Lambda timeout is sufficient for processing

### Debugging Tips

```go
// Enable detailed logging
func handler(ctx context.Context, event SNSEvent) error {
    log.Printf("Received %d records", len(event.Records))
    
    for i, record := range event.Records {
        log.Printf("Processing record %d: TopicArn=%s", i, record.Sns.TopicArn)
        log.Printf("Message: %s", record.Sns.Message)
        log.Printf("Attributes: %+v", record.Sns.MessageAttributes)
        
        // Process record...
    }
    
    return nil
}
```

### Migration from SQS

If migrating from SQS to SNS:

1. SNS provides pub/sub vs SQS point-to-point
2. SNS has no message visibility timeout
3. SNS doesn't support message delay
4. Use SNS+SQS combination for delayed processing:

```go
// Create SQS queue subscribed to SNS topic
queue := awssqs.NewQueue(stack, jsii.String("ProcessingQueue"), &awssqs.QueueProps{
    VisibilityTimeout: awscdk.Duration_Minutes(jsii.Number(5)),
    DeliveryDelay:     awscdk.Duration_Minutes(jsii.Number(2)),
})

topic.AddSubscription(
    awssnssubscriptions.NewSqsSubscription(queue, &awssnssubscriptions.SqsSubscriptionProps{
        RawMessageDelivery: jsii.Bool(true),
    }),
)
```

## Next Steps

- Explore [EventBridge patterns](./eventbridge-patterns.md) for event-driven architectures
- Learn about [SQS patterns](./sqs-patterns.md) for queue-based processing
- See [Multi-Event patterns](./multi-event-patterns.md) for complex workflows