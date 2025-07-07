# Event Orchestrator Pattern

The EventOrchestrator pattern provides a unified approach to handling multiple AWS event sources with built-in correlation, routing, and saga pattern support. This pattern is ideal for complex event-driven architectures where events from different sources need to be processed, correlated, and orchestrated.

## Overview

The EventOrchestrator combines multiple event sources into a single processing pipeline with:
- Unified event processing across SQS, EventBridge, S3, DynamoDB Streams, SNS, and Kinesis
- Event correlation and routing capabilities
- Saga pattern support for distributed transactions
- Dead letter queue handling for all event sources
- Centralized monitoring and observability

## Basic Usage

```go
import (
    "github.com/pay-theory/lift/pkg/cdk/patterns"
)

orchestrator := patterns.NewEventOrchestrator(stack, jsii.String("MyOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("order-processor"),
    ProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableSQS:              jsii.Bool(true),
    EnableEventBridge:      jsii.Bool(true),
    EnableS3:               jsii.Bool(true),
})
```

## Configuration Options

### Core Properties

| Property | Type | Description | Default |
|----------|------|-------------|---------|
| OrchestratorName | *string | Name of the orchestrator (used for resource naming) | Required |
| ProcessorCodeAssetPath | *string | Path to the Lambda processor code | Required |
| EnableEventCorrelation | *bool | Enable event correlation with routing table | false |
| EnableSagaPattern | *bool | Enable saga pattern support | false |
| Environment | *map[string]*string | Additional environment variables | nil |

### Event Source Flags

| Property | Type | Description |
|----------|------|-------------|
| EnableSQS | *bool | Enable SQS event source |
| EnableEventBridge | *bool | Enable EventBridge event source |
| EnableS3 | *bool | Enable S3 event source |
| EnableDynamoStreams | *bool | Enable DynamoDB Streams event source |
| EnableSNS | *bool | Enable SNS event source |
| EnableKinesis | *bool | Enable Kinesis event source |

### Custom Event Source Configuration

Each event source can be customized with specific properties:

```go
orchestrator := patterns.NewEventOrchestrator(stack, jsii.String("MyOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("order-processor"),
    ProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableSQS:              jsii.Bool(true),
    EnableEventBridge:      jsii.Bool(true),
    
    // Custom SQS configuration
    SQSProcessorProps: &constructs.SQSProcessorProps{
        QueueName:        jsii.String("orders-queue"),
        EnableFIFO:       jsii.Bool(true),
        MaxReceiveCount:  jsii.Number(3),
    },
    
    // Custom EventBridge configuration
    EventBridgeHandlerProps: &constructs.EventBridgeHandlerProps{
        EventPattern: &awseventbridge.EventPattern{
            Source:     &[]*string{jsii.String("order.service")},
            DetailType: &[]*string{jsii.String("Order Created"), jsii.String("Order Updated")},
        },
    },
})
```

## Event Correlation

Enable event correlation to track related events across different sources:

```go
orchestrator := patterns.NewEventOrchestrator(stack, jsii.String("MyOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("order-processor"),
    ProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableEventCorrelation: jsii.Bool(true),
    EnableSQS:              jsii.Bool(true),
    EnableEventBridge:      jsii.Bool(true),
})

// The orchestrator creates a routing table with:
// - Partition key: correlationId
// - Sort key: eventId
// - GSIs for querying by event source and saga ID
```

### Routing Table Schema

The event routing table includes:
- **Primary Key**: correlationId (HASH), eventId (RANGE)
- **GSI 1**: eventSource-timestamp for querying events by source
- **GSI 2**: sagaId-status for saga pattern support
- **Attributes**: timestamp, eventSource, sagaId, status, payload

## Saga Pattern Support

Enable distributed transaction management with the saga pattern:

```go
orchestrator := patterns.NewEventOrchestrator(stack, jsii.String("MyOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("order-saga"),
    ProcessorCodeAssetPath: jsii.String("./dist/saga-processor"),
    EnableSagaPattern:      jsii.Bool(true),
    EnableEventCorrelation: jsii.Bool(true),
    EnableSQS:              jsii.Bool(true),
    EnableEventBridge:      jsii.Bool(true),
})

// Environment variables set:
// - SAGA_PATTERN_ENABLED=true
// - EVENT_CORRELATION_ENABLED=true
```

## Using Existing Resources

Integrate with existing AWS resources:

```go
existingTable := awsdynamodb.Table_FromTableName(stack, jsii.String("ExistingTable"), jsii.String("my-routing-table"))
existingBus := awseventbridge.EventBus_FromEventBusName(stack, jsii.String("ExistingBus"), jsii.String("my-event-bus"))

orchestrator := patterns.NewEventOrchestrator(stack, jsii.String("MyOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("order-processor"),
    ProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EventRoutingTable:      existingTable,
    EventBridgeCustomBus:   existingBus,
    EnableEventCorrelation: jsii.Bool(true),
})
```

## Environment Variables

The orchestrator automatically sets environment variables for the processor:

### Core Variables
- `ORCHESTRATOR_NAME`: Name of the orchestrator
- `ORCHESTRATION_BUS_NAME`: EventBridge bus name for orchestration events
- `ORCHESTRATION_BUS_ARN`: EventBridge bus ARN

### Event Source Indicators
- `EVENT_SOURCE_SQS`: "enabled" if SQS is enabled
- `EVENT_SOURCE_EVENTBRIDGE`: "enabled" if EventBridge is enabled
- `EVENT_SOURCE_S3`: "enabled" if S3 is enabled
- `EVENT_SOURCE_DYNAMO`: "enabled" if DynamoDB Streams is enabled
- `EVENT_SOURCE_SNS`: "enabled" if SNS is enabled
- `EVENT_SOURCE_KINESIS`: "enabled" if Kinesis is enabled

### Feature Flags
- `EVENT_CORRELATION_ENABLED`: "true" if correlation is enabled
- `SAGA_PATTERN_ENABLED`: "true" if saga pattern is enabled

### Routing Table Variables (if correlation enabled)
- `EVENT_ROUTING_TABLE_NAME`: DynamoDB table name
- `EVENT_ROUTING_TABLE_ARN`: DynamoDB table ARN

## Helper Methods

The orchestrator provides helper methods for common operations:

```go
// Grant permissions to external resources
orchestrator.GrantReadWriteToRoutingTable(myFunction)
orchestrator.GrantPutEventsToOrchestrationBus(myFunction)

// Get references to created resources
queue := orchestrator.GetQueue()        // SQS Queue
topic := orchestrator.GetTopic()        // SNS Topic
bucket := orchestrator.GetBucket()      // S3 Bucket
stream := orchestrator.GetStream()      // Kinesis Stream
table := orchestrator.GetTable()        // DynamoDB Table
```

## Advanced Patterns

### Multi-Region Event Processing

```go
orchestrator := patterns.NewEventOrchestrator(stack, jsii.String("GlobalOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("global-processor"),
    ProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableEventBridge:      jsii.Bool(true),
    EnableKinesis:          jsii.Bool(true),
    
    EventBridgeHandlerProps: &constructs.EventBridgeHandlerProps{
        CrossAccountEventBusArn: jsii.String("arn:aws:events:us-west-2:123456789012:event-bus/central-bus"),
    },
    
    KinesisProcessorProps: &constructs.KinesisProcessorProps{
        EnableEnhancedFanOut: jsii.Bool(true),
        StreamMode:           awskinesis.StreamMode_ON_DEMAND,
    },
})
```

### Event-Driven Microservices

```go
// Order Service
orderOrchestrator := patterns.NewEventOrchestrator(stack, jsii.String("OrderOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("order-service"),
    ProcessorCodeAssetPath: jsii.String("./dist/order-processor"),
    EnableSQS:              jsii.Bool(true),
    EnableEventBridge:      jsii.Bool(true),
    EnableEventCorrelation: jsii.Bool(true),
    
    EventBridgeHandlerProps: &constructs.EventBridgeHandlerProps{
        EventPattern: &awseventbridge.EventPattern{
            Source: &[]*string{jsii.String("payment.service"), jsii.String("inventory.service")},
        },
    },
})

// Payment Service
paymentOrchestrator := patterns.NewEventOrchestrator(stack, jsii.String("PaymentOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("payment-service"),
    ProcessorCodeAssetPath: jsii.String("./dist/payment-processor"),
    EnableSNS:              jsii.Bool(true),
    EnableEventBridge:      jsii.Bool(true),
    
    EventBridgeHandlerProps: &constructs.EventBridgeHandlerProps{
        CustomEventBus: orderOrchestrator.OrchestrationBus, // Share event bus
    },
})
```

### Data Pipeline with Multiple Sources

```go
pipelineOrchestrator := patterns.NewEventOrchestrator(stack, jsii.String("DataPipeline"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("data-pipeline"),
    ProcessorCodeAssetPath: jsii.String("./dist/pipeline-processor"),
    EnableS3:               jsii.Bool(true),
    EnableKinesis:          jsii.Bool(true),
    EnableDynamoStreams:    jsii.Bool(true),
    EnableEventCorrelation: jsii.Bool(true),
    
    S3ProcessorProps: &constructs.S3ProcessorProps{
        EventTypes: &[]awss3.EventType{
            awss3.EventType_OBJECT_CREATED,
        },
        Filters: &[]*awss3.NotificationKeyFilter{
            {Prefix: jsii.String("raw-data/")},
        },
    },
    
    KinesisProcessorProps: &constructs.KinesisProcessorProps{
        EnableTumblingWindow: jsii.Bool(true),
        TumblingWindowSeconds: jsii.Number(60),
    },
})
```

## Best Practices

### 1. Event Source Selection
- Use **SQS** for reliable, ordered processing with retry capabilities
- Use **EventBridge** for event routing and fan-out patterns
- Use **S3** for file-based event processing
- Use **Kinesis** for high-throughput streaming data
- Use **SNS** for pub/sub and fan-out messaging
- Use **DynamoDB Streams** for change data capture

### 2. Performance Optimization
- Enable batching for SQS and Kinesis to reduce Lambda invocations
- Use FIFO queues/topics when ordering is critical
- Configure appropriate visibility timeouts and retry policies
- Use enhanced fan-out for Kinesis when multiple consumers exist

### 3. Error Handling
- All event sources include DLQ configuration by default
- Implement idempotent processing for at-least-once delivery
- Use correlation IDs for distributed tracing
- Monitor DLQ depth and set up alerts

### 4. Cost Optimization
- Use on-demand scaling for variable workloads
- Configure appropriate retention periods
- Enable compression where supported
- Monitor and optimize batch sizes

## Monitoring and Observability

The orchestrator integrates with CloudWatch and X-Ray:

```go
orchestrator := patterns.NewEventOrchestrator(stack, jsii.String("MonitoredOrchestrator"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("monitored-processor"),
    ProcessorCodeAssetPath: jsii.String("./dist/processor"),
    EnableEventCorrelation: jsii.Bool(true),
    
    FunctionProps: &constructs.LiftFunctionProps{
        EnableTracing: jsii.Bool(true),
        Environment: &map[string]*string{
            "LOG_LEVEL": jsii.String("DEBUG"),
        },
    },
})

// Access metrics
orchestrator.Processor.MetricErrors()
orchestrator.Processor.MetricThrottles()
orchestrator.Processor.MetricDuration()
```

## Example: Order Processing System

```go
// Complete order processing system with multiple event sources
orderSystem := patterns.NewEventOrchestrator(stack, jsii.String("OrderSystem"), &patterns.EventOrchestratorProps{
    OrchestratorName:       jsii.String("order-system"),
    ProcessorCodeAssetPath: jsii.String("./dist/order-processor"),
    EnableSQS:              jsii.Bool(true),      // Order queue
    EnableEventBridge:      jsii.Bool(true),      // Order events
    EnableS3:               jsii.Bool(true),       // Order documents
    EnableDynamoStreams:    jsii.Bool(true),      // Order updates
    EnableSNS:              jsii.Bool(true),       // Order notifications
    EnableEventCorrelation: jsii.Bool(true),
    EnableSagaPattern:      jsii.Bool(true),
    
    SQSProcessorProps: &constructs.SQSProcessorProps{
        QueueName:  jsii.String("order-queue"),
        EnableFIFO: jsii.Bool(true),
    },
    
    EventBridgeHandlerProps: &constructs.EventBridgeHandlerProps{
        EventPattern: &awseventbridge.EventPattern{
            Source:     &[]*string{jsii.String("order.api"), jsii.String("order.admin")},
            DetailType: &[]*string{jsii.String("Order Created"), jsii.String("Order Updated"), jsii.String("Order Cancelled")},
        },
    },
    
    S3ProcessorProps: &constructs.S3ProcessorProps{
        BucketName: jsii.String("order-documents"),
        EventTypes: &[]awss3.EventType{awss3.EventType_OBJECT_CREATED},
    },
    
    DynamoStreamProcessorProps: &constructs.DynamoStreamProcessorProps{
        TableName:      jsii.String("orders"),
        StreamViewType: awsdynamodb.StreamViewType_NEW_AND_OLD_IMAGES,
    },
    
    SNSProcessorProps: &constructs.SNSProcessorProps{
        TopicName: jsii.String("order-notifications"),
    },
    
    Environment: &map[string]*string{
        "ORDER_API_URL":     jsii.String("https://api.example.com/orders"),
        "NOTIFICATION_TYPE": jsii.String("EMAIL"),
    },
})
```

## Troubleshooting

### Events Not Processing
1. Check Lambda function logs for errors
2. Verify event source mappings are active
3. Check DLQ for failed messages
4. Ensure IAM permissions are correct

### Correlation Not Working
1. Verify EVENT_CORRELATION_ENABLED is set
2. Check routing table has proper indexes
3. Ensure correlationId is included in events
4. Monitor routing table for throttling

### Performance Issues
1. Increase Lambda memory/timeout
2. Adjust batch sizes for event sources
3. Enable concurrent executions
4. Consider splitting into multiple orchestrators