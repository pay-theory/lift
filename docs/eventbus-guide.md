# EventBus Guide

## Overview

The Lift EventBus provides a durable, serverless-safe event bus implementation backed by DynamoDB. Unlike in-memory event buses that lose events when Lambda containers scale down, the Lift EventBus persists all events to DynamoDB, making it suitable for production serverless environments.

## Table of Contents

- [Key Features](#key-features)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Usage Examples](#usage-examples)
- [CDK Deployment](#cdk-deployment)
- [Best Practices](#best-practices)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)

## Key Features

### Durable Storage
- **DynamoDB-backed**: All events are persisted to DynamoDB
- **Survives Lambda recycling**: Events remain available across container lifecycles
- **TTL support**: Automatic event expiration for compliance and cost optimization
- **Point-in-time recovery**: Automated backups for disaster recovery

### Event Processing
- **Time-ordered events**: ULID-based IDs ensure chronological ordering
- **Multi-tenant support**: Built-in tenant isolation
- **Efficient queries**: Optimized partition and sort keys for fast lookups
- **Batch operations**: Publish multiple events efficiently
- **DynamoDB Streams**: Optional stream processing for real-time event handling

### Operational Excellence
- **CloudWatch metrics**: Automatic metrics emission for observability
- **Retry logic**: Built-in retry with exponential backoff
- **Throttling protection**: Handles DynamoDB throttling gracefully
- **Error handling**: Comprehensive error handling and logging

### Developer Experience
- **Simple API**: Clean, intuitive interface
- **Type-safe**: Strongly typed event structures
- **Testing support**: In-memory mock for unit testing
- **CDK constructs**: Infrastructure-as-code deployment patterns

## Architecture

### Data Model

The EventBus uses a single-table design optimized for DynamoDB:

```
Partition Key (pk): {tenant_id}#{event_type}
Sort Key (sk):      {timestamp_nanos}#{event_id}

GSI 1: tenant-timestamp-index
  PK: tenant_id
  SK: published_at

GSI 2: event-id-index (optional)
  PK: id
```

### Key Benefits of This Design

1. **Efficient tenant queries**: Query all events for a tenant
2. **Type-filtered queries**: Query specific event types for a tenant
3. **Time-range queries**: Query events within a time window
4. **Ordered results**: Events are naturally ordered by time

### Event Structure

```go
type Event struct {
    ID            string          // ULID for time-ordered unique ID
    EventType     string          // e.g., "partner.created"
    TenantID      string          // Multi-tenant isolation
    SourceID      string          // Source entity ID
    Payload       json.RawMessage // Event data (any JSON)
    Metadata      map[string]string // Additional context
    Tags          []string        // For filtering
    PublishedAt   time.Time       // When published
    CreatedAt     time.Time       // When created
    ExpiresAt     time.Time       // For TTL
    CorrelationID string          // For tracing
    Version       int             // Schema version
    RetryCount    int             // For failed processing
}
```

## Quick Start

### 1. Install Dependencies

```bash
go get github.com/pay-theory/lift
go get github.com/pay-theory/dynamorm@v1.0.39

# Optional (only if you want EventBus CloudWatch metrics)
go get github.com/aws/aws-sdk-go-v2/service/cloudwatch
```

### 2. Create an EventBus

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/dynamorm/pkg/session"
    "github.com/pay-theory/lift/pkg/services"
)

func main() {
    ctx := context.Background()

    // Load AWS config (used here for region + CloudWatch client)
    awsCfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        log.Fatal(err)
    }

    // Initialize DynamORM (use this for all DynamoDB access)
    db, err := dynamorm.New(session.Config{
        Region: awsCfg.Region,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Optional: CloudWatch metrics
    cloudwatchClient := cloudwatch.NewFromConfig(awsCfg)

    // Create EventBus
    eventBus := services.NewDynamoDBEventBus(db, services.EventBusConfig{
        TableName:        "my-app-events-live",
        TTL:              30 * 24 * time.Hour, // 30 days
        EnableMetrics:    true,
        MetricsNamespace: "MyApp/EventBus",
    }).WithCloudWatch(cloudwatchClient)

    // Publish an event
    event, err := services.NewEvent(
        "partner.created",      // event type
        "tenant-123",           // tenant ID
        "partner-456",          // source ID
        map[string]interface{}{ // payload
            "name":  "Acme Corp",
            "email": "contact@acme.com",
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    eventID, err := eventBus.Publish(ctx, event)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Published event: %s", eventID)
}
```

### 3. Query Events

```go
// Query events for a tenant
events, err := eventBus.Query(context.Background(), &services.EventQuery{
    TenantID:  "tenant-123",
    EventType: "partner.created",
    Limit:     100,
})
if err != nil {
    log.Fatal(err)
}

for _, event := range events {
    log.Printf("Event: %s at %s", event.ID, event.PublishedAt)
}
```

## Usage Examples

### Publishing Events

#### Basic Event

```go
event, err := services.NewEvent(
    "user.registered",
    "tenant-abc",
    "user-123",
    map[string]interface{}{
        "username": "john.doe",
        "email":    "john@example.com",
    },
)
if err != nil {
    return err
}

eventID, err := eventBus.Publish(ctx, event)
```

#### Event with Metadata and Tags

```go
event, err := services.NewEvent("order.placed", "tenant-abc", "order-789", orderData)
if err != nil {
    return err
}

event.WithMetadata("user_id", "user-123").
    WithMetadata("ip_address", "192.168.1.1").
    WithTags("high-value", "priority").
    WithCorrelationID("req-xyz-123").
    WithTTL(7 * 24 * time.Hour) // Override default TTL

eventID, err := eventBus.Publish(ctx, event)
```

#### Batch Publishing

```go
events := []*services.Event{
    mustCreateEvent("event.type1", "tenant-1", "source-1", data1),
    mustCreateEvent("event.type2", "tenant-1", "source-2", data2),
    mustCreateEvent("event.type3", "tenant-1", "source-3", data3),
}

eventIDs, err := eventBus.(*services.DynamoDBEventBus).BatchPublish(ctx, events)
if err != nil {
    return err
}

log.Printf("Published %d events", len(eventIDs))
```

### Querying Events

#### Query by Event Type

```go
query := &services.EventQuery{
    TenantID:  "tenant-123",
    EventType: "partner.created",
    Limit:     50,
}

events, err := eventBus.Query(ctx, query)
```

#### Query with Time Range

```go
startTime := time.Now().Add(-24 * time.Hour)
endTime := time.Now()

query := &services.EventQuery{
    TenantID:  "tenant-123",
    EventType: "order.placed",
    StartTime: &startTime,
    EndTime:   &endTime,
    Limit:     100,
}

events, err := eventBus.Query(ctx, query)
```

#### Query with Tags

```go
query := &services.EventQuery{
    TenantID:  "tenant-123",
    EventType: "transaction.completed",
    Tags:      []string{"high-value", "verified"},
    Limit:     100,
}

events, err := eventBus.Query(ctx, query)
```

#### Paginated Queries

```go
var allEvents []*services.Event

query := &services.EventQuery{
    TenantID:  "tenant-123",
    EventType: "event.type",
    Limit:     100,
}

for {
    events, err := eventBus.Query(ctx, query)
    if err != nil {
        return err
    }

    allEvents = append(allEvents, events...)

    // Check if there are more results
    if query.NextKey == nil {
        break // No more pages
    }

    // Use the returned NextKey for the next query
    query.LastEvaluatedKey = query.NextKey
    query.NextKey = nil
}

log.Printf("Retrieved %d total events", len(allEvents))
```

### Processing Events

#### Unmarshal Event Payload

```go
type PartnerCreatedPayload struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Tier  string `json:"tier"`
}

event, err := eventBus.GetEvent(ctx, eventID)
if err != nil {
    return err
}

var payload PartnerCreatedPayload
if err := event.UnmarshalPayload(&payload); err != nil {
    return err
}

log.Printf("Partner: %s (%s)", payload.Name, payload.Email)
```

#### Stream Processing Handler

```go
func HandlePartnerCreated(ctx context.Context, event *services.Event) error {
    var payload PartnerCreatedPayload
    if err := event.UnmarshalPayload(&payload); err != nil {
        return fmt.Errorf("failed to unmarshal: %w", err)
    }

    // Process the event
    log.Printf("Processing partner.created: %s", payload.Name)

    // Send welcome email, provision resources, etc.
    if err := sendWelcomeEmail(payload.Email); err != nil {
        return fmt.Errorf("failed to send email: %w", err)
    }

    return nil
}
```

### Lambda Integration

#### Publisher Lambda

```go
package main

import (
    "context"
    "log"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/dynamorm/pkg/session"
    "github.com/pay-theory/lift/pkg/services"
)

var eventBus services.EventBus

func init() {
    ctx := context.Background()

    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        log.Fatal(err)
    }

    db, err := dynamorm.New(session.Config{
        Region: cfg.Region,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Table name is derived from APP_NAME/STAGE[/PARTNER] by default.
    eventBus = services.NewDynamoDBEventBus(db, services.EventBusConfig{})
}

type CreatePartnerRequest struct {
    TenantID string `json:"tenant_id"`
    Name     string `json:"name"`
    Email    string `json:"email"`
}

func handler(ctx context.Context, req CreatePartnerRequest) (string, error) {
    // Create partner in database
    partnerID, err := createPartnerInDB(req)
    if err != nil {
        return "", err
    }

    // Publish event
    event, err := services.NewEvent(
        "partner.created",
        req.TenantID,
        partnerID,
        req,
    )
    if err != nil {
        return "", err
    }

    event.WithMetadata("source", "api").
        WithTags("partner", "onboarding")

    eventID, err := eventBus.Publish(ctx, event)
    if err != nil {
        log.Printf("Failed to publish event: %v", err)
        // Don't fail the request if event publishing fails
    }

    return partnerID, nil
}

func main() {
    lambda.Start(handler)
}
```

#### Stream Processor Lambda

```go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/services"
)

func main() {
    app := lift.New()

    // Route by event_type; handlers receive a typed *services.Event (decoded via DynamORM).
    _ = app.EventBus("partner.created", func(ctx *lift.Context, event *services.Event) error {
        return handlePartnerCreated(ctx.Context, event)
    })

    _ = app.EventBus("partner.updated", func(ctx *lift.Context, event *services.Event) error {
        return handlePartnerUpdated(ctx.Context, event)
    })

    _ = app.EventBus("key.rotated", func(ctx *lift.Context, event *services.Event) error {
        return handleKeyRotated(ctx.Context, event)
    })

    // Handles partial batch failures correctly when the event source mapping enables ReportBatchItemFailures.
    lambda.Start(app.HandleRequest)
}
```

### Consumer Checkpoints (At-least-once Safety)

DynamoDB Streams are **at-least-once**. If your processor can see duplicate deliveries, use Lift’s checkpoint helpers (co-located in the EventBus table) to make your consumer safely idempotent:

- `services.EventBusIsProcessed(ctx, db, consumer, eventID)`
- `services.EventBusMarkProcessed(ctx, db, consumer, event, retention)` (conditional write via DynamORM `IfNotExists()`)

**CORRECT**: check at start, mark after success

```go
// requires: import "time"
consumer := "lesser-feed-indexer"

done, err := services.EventBusIsProcessed(ctx, db, consumer, event.ID)
if err != nil {
    return err
}
if done {
    return nil // already processed by this consumer
}

// ... do work ...

_, err = services.EventBusMarkProcessed(ctx, db, consumer, event, 30*24*time.Hour)
return err
```

### Delayed Publish (Workflow Primitive)

Lift provides EventBus scheduling helpers that keep everything in **EventBus + DynamORM** (no Step Functions required):

- `services.EventBusSchedule(...)` writes a delayed publish request into the EventBus table.
- `services.EventBusDrainDueScheduledWithOptions(...)` queries due items, publishes them, and deletes the schedule items with optional backoff/quarantine semantics.
- `services.EventBusDrainDueScheduled(...)` is the simple variant that stops on the first publish error.

#### Concurrency-Safe Draining (Leases) + Idempotent Publish

`EventBusDrainDueScheduledWithOptions` is safe to run with **multiple concurrent drainers**:

- Each due row is **claimed/leased** with a conditional update (`lease_until` must be missing or expired).
- Scheduled rows are deleted only if the lease still matches (`lease_id`).
- If a drainer crashes mid-publish, the lease expires and another drainer can reclaim.

Publishing is also safe:

- `DynamoDBEventBus.Publish` uses a conditional create and treats “already exists” as success (no overwrite / no re-trigger).
- The scheduler drain sets **stable keys** based on `(tenant_id, event_type, due_at, event_id)` so replays are safe.

#### Scheduler Lambda

```go
package main

import (
    "context"
    "time"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/lift/pkg/services"
)

var (
    db  *dynamorm.LambdaDB
    bus *services.DynamoDBEventBus
)

func init() {
    var err error
    db, err = dynamorm.NewLambdaOptimized()
    if err != nil {
        panic(err)
    }
    bus = services.NewDynamoDBEventBus(db, services.EventBusConfig{})
}

func handler(ctx context.Context, _ any) error {
    _, err := services.EventBusDrainDueScheduledWithOptions(ctx, db, bus, time.Now(), 100, services.EventBusScheduleDrainOptions{
        LeaseDuration:          2 * time.Minute, // default
        LeaseOwner:             "eventbus-scheduler",
        ContinueOnPublishError: true,         // backoff instead of failing the Lambda
        MaxPublishAttempts:     10,           // quarantine after N failed publishes
        RetryBaseDelay:         5 * time.Second,
        RetryMaxDelay:          5 * time.Minute,
        QuarantineRetention:    14 * 24 * time.Hour,
    })
    return err // nil unless an unexpected DB error occurs
}

func main() { lambda.Start(handler) }
```

When a scheduled item reaches `MaxPublishAttempts`, it is moved to a quarantine record (`services.EventBusQuarantinedScheduledEvent`) and removed from the schedule queue.
You can restore a quarantined item back into the queue with `services.EventBusReplayQuarantinedScheduled(...)`.

#### CDK Wiring (EventBridge → Scheduler Lambda)

```go
_ = liftconstructs.NewEventBusScheduler(stack, jsii.String("Scheduler"), &liftconstructs.EventBusSchedulerProps{
    Table:              table,
    AppName:            jsii.String(nameCtx.AppName),
    Stage:              jsii.String(nameCtx.Stage),
    Partner:            jsii.String(nameCtx.Tenant),
    ScheduleExpression: jsii.String("rate(1 minute)"),
    FunctionProps: awslambda.FunctionProps{
        Code:    awslambda.Code_FromAsset(jsii.String("./build/scheduler"), nil),
        Handler: jsii.String("bootstrap"),
        Runtime: awslambda.Runtime_PROVIDED_AL2023(),
    },
})
```

### Publish → Push to WebSockets (Streamer)

For the common “EventBus publish → push to sockets” path, use `services.FanoutEventBusEvent` with `pkg/streamer`:

```go
client, _ := streamer.NewClient(ctx, streamer.ClientConfig{Endpoint: os.Getenv("WEBSOCKET_ENDPOINT")})

_, _ = services.FanoutEventBusEvent(ctx, client, event, services.EventBusFanoutOptions{
    ResolveConnectionIDs: services.TenantConnectionsResolver(connectionStore),
})
```

You can also combine resolvers for subscription-aware fanout (topics, users, tenant broadcast) without scans:

Create `subscriptionStore` with `lift.NewDynamoDBSubscriptionStoreWithDB(db, lift.DynamoDBSubscriptionStoreConfig{})` (co-located in the websocket connections table) and pass it to `services.TopicConnectionsResolver`.

```go
resolver := services.UnionConnectionsResolver(
    services.TenantConnectionsResolver(connectionStore),
    services.UserConnectionsResolverFromMetadata(connectionStore, "user_id"),
    services.TopicConnectionsResolver(subscriptionStore, func(e *services.Event) []string {
        return []string{"home", "public"} // application-defined topics
    }),
)

_, _ = services.FanoutEventBusEvent(ctx, client, event, services.EventBusFanoutOptions{
    ResolveConnectionIDs: resolver,
})
```

## CDK Deployment

### Table Naming Requirements

**CRITICAL**: The `TableName` must be unique within your AWS account and region. Multiple applications using the default or same table name will cause deployment conflicts.

**Recommended naming convention**:
```
{app}-events-{stage}

# Or (multi-tenant)
{app}-{tenant}-events-{stage}
```

Examples:
- `autheory-events-live`
- `autheory-events-lab`
- `partner-hub-events-study`
- `autheory-acme-events-live`

You can generate the table name deterministically from deployment inputs using Lift's naming helpers:

```go
tableName, ok := naming.ResourceNameFromEnv("events") // uses APP_NAME, STAGE, optional PARTNER
if !ok {
    return fmt.Errorf("APP_NAME and STAGE are required for deterministic naming")
}
_ = tableName // use this name for your CDK/Terraform table definition
eventBus := services.NewDynamoDBEventBus(db, services.EventBusConfig{})
```

### Basic Table Deployment

```go
package main

	import (
	    "github.com/aws/aws-cdk-go/awscdk/v2"
	    "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	    "github.com/aws/constructs-go/constructs/v10"
	    "github.com/aws/jsii-runtime-go"
	    liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
	    "github.com/pay-theory/lift/pkg/naming"
	)

type MyStackProps struct {
    awscdk.StackProps
}

func NewMyStack(scope constructs.Construct, id string, props *MyStackProps) awscdk.Stack {
    stack := awscdk.NewStack(scope, &id, &props.StackProps)

    // Deterministic table name from deployment inputs (APP_NAME/STAGE[/PARTNER]).
    nameCtx := naming.FromEnv().Normalize()
    if !nameCtx.IsComplete() {
        panic("APP_NAME and STAGE are required for deterministic naming")
    }

    eventBusTable := liftconstructs.NewEventBusTable(stack, jsii.String("EventBus"), &liftconstructs.EventBusTableProps{
        TableName:                 jsii.String(nameCtx.ResourceName("events")),
        EnablePointInTimeRecovery: jsii.Bool(true),
        EnableStream:              jsii.Bool(true),
        EnableEventIDIndex:        jsii.Bool(true),
    })

    // Create a Lambda function that publishes events
    publisherEnv := map[string]*string{
        "APP_NAME": jsii.String(nameCtx.AppName),
        "STAGE":    jsii.String(nameCtx.Stage),
    }
    if nameCtx.Tenant != "" {
        publisherEnv["PARTNER"] = jsii.String(nameCtx.Tenant)
    }

    publisherFunction := liftconstructs.NewLiftFunction(stack, jsii.String("Publisher"), &liftconstructs.LiftFunctionProps{
        FunctionProps: awslambda.FunctionProps{
            FunctionName: jsii.String(nameCtx.ResourceName("event-publisher")),
            Code:         awslambda.Code_FromAsset(jsii.String("./build/publisher"), nil),
            Handler:      jsii.String("bootstrap"),
            Environment:  &publisherEnv,
        },
    })

    // Grant permissions
    eventBusTable.GrantWrite(publisherFunction.Function)

    return stack
}
```

### Stream Processor Wiring (Recommended)

Use `constructs.NewEventBusProcessor` to connect an EventBus table stream to a Lambda, with optional `event_type` filters and a DLQ by default.

```go
package main

import (
    "github.com/aws/aws-cdk-go/awscdk/v2"
    "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
    "github.com/aws/constructs-go/constructs/v10"
    "github.com/aws/jsii-runtime-go"

    liftconstructs "github.com/pay-theory/lift/pkg/cdk/constructs"
    "github.com/pay-theory/lift/pkg/naming"
)

func NewMyStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
    stack := awscdk.NewStack(scope, &id, props)

    nameCtx := naming.FromEnv().Normalize()
    if !nameCtx.IsComplete() {
        panic("APP_NAME and STAGE are required for deterministic naming")
    }

    table := liftconstructs.NewEventBusTable(stack, jsii.String("EventBus"), &liftconstructs.EventBusTableProps{
        TableName:          jsii.String(nameCtx.ResourceName("events")),
        EnableStream:       jsii.Bool(true),
        EnableEventIDIndex: jsii.Bool(true),
    })

    processorEnv := map[string]*string{
        "APP_NAME": jsii.String(nameCtx.AppName),
        "STAGE":    jsii.String(nameCtx.Stage),
    }
    if nameCtx.Tenant != "" {
        processorEnv["PARTNER"] = jsii.String(nameCtx.Tenant)
    }

    _ = liftconstructs.NewEventBusProcessor(stack, jsii.String("Processor"), &liftconstructs.EventBusProcessorProps{
        Table:      table,
        EventTypes: []string{"partner.created", "partner.updated"}, // optional
        FunctionProps: awslambda.FunctionProps{
            FunctionName: jsii.String(nameCtx.ResourceName("eventbus-processor")),
            Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
            Code:         awslambda.Code_FromAsset(jsii.String("./build/processor"), nil),
            Handler:      jsii.String("bootstrap"),
            Environment:  &processorEnv,
        },
    })

    // Publisher Lambda (write-only)
    publisherEnv := map[string]*string{
        "APP_NAME": jsii.String(nameCtx.AppName),
        "STAGE":    jsii.String(nameCtx.Stage),
    }
    if nameCtx.Tenant != "" {
        publisherEnv["PARTNER"] = jsii.String(nameCtx.Tenant)
    }

    apiFunction := liftconstructs.NewLiftFunction(stack, jsii.String("API"), &liftconstructs.LiftFunctionProps{
        FunctionProps: awslambda.FunctionProps{
            FunctionName: jsii.String(nameCtx.ResourceName("api")),
            Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
            Code:         awslambda.Code_FromAsset(jsii.String("./build/api"), nil),
            Handler:      jsii.String("bootstrap"),
            Environment:  &publisherEnv,
        },
    })
    table.GrantWrite(apiFunction.Function)

    return stack
}
```

### Multi-Environment Deployment

```go
// Example: Deploy separate EventBus for each stage
// Stages: lab, study, live

type DeploymentEnvironment struct {
    Name      string
    IsProd    bool
}

func deployEventBusForEnvironment(stack awscdk.Stack, env DeploymentEnvironment) *liftpatterns.EventBusPattern {
    return liftpatterns.NewEventBusPattern(stack, jsii.String(env.Name + "EventBus"), &liftpatterns.EventBusPatternProps{
        AppName:           "my-app",
        Stage:             env.Name, // lab / study / live
        EnableStream:      jsii.Bool(true),
        ProcessorCodePath: jsii.String("./build/processor"),
        Tags: &map[string]*string{
            "Stage": jsii.String(env.Name),
        },
    })
}

// Usage:
liveEventBus := deployEventBusForEnvironment(stack, DeploymentEnvironment{Name: "live", IsProd: true})
labEventBus := deployEventBusForEnvironment(stack, DeploymentEnvironment{Name: "lab", IsProd: false})
studyEventBus := deployEventBusForEnvironment(stack, DeploymentEnvironment{Name: "study", IsProd: false})
```

### Multi-Tier Deployment

```go
// Create separate EventBus for each tenant tier with unique table names
premiumEventBus := liftpatterns.NewEventBusPattern(stack, jsii.String("PremiumEventBus"), &liftpatterns.EventBusPatternProps{
    AppName:                   "my-app",
    Stage:                     "live",
    Partner:                   "premium",
    BillingMode:               awsdynamodb.BillingMode_PROVISIONED,
    EnablePointInTimeRecovery: jsii.Bool(true),
    ProcessorCodePath:         jsii.String("./build/premium-processor"),
    ProcessorMemory:           jsii.Number(512),
    Tags: &map[string]*string{
        "Tier": jsii.String("Premium"),
    },
})

standardEventBus := liftpatterns.NewEventBusPattern(stack, jsii.String("StandardEventBus"), &liftpatterns.EventBusPatternProps{
    AppName:           "my-app",
    Stage:             "live",
    Partner:           "standard",
    ProcessorCodePath: jsii.String("./build/standard-processor"),
    Tags: &map[string]*string{
        "Tier": jsii.String("Standard"),
    },
})
```

### Avoiding Naming Conflicts

```go
// ❌ BAD: Will conflict if multiple apps deployed to same account
eventBus1 := liftpatterns.NewEventBusPattern(stack, jsii.String("EventBus"), &liftpatterns.EventBusPatternProps{
    AppName: "my-app",
    Stage:   "live",
})

eventBus2 := liftpatterns.NewEventBusPattern(stack, jsii.String("EventBus"), &liftpatterns.EventBusPatternProps{
    AppName: "my-app",
    Stage:   "live",
})

// ✅ GOOD: Stable names prevent conflicts
eventBus1 := liftpatterns.NewEventBusPattern(stack, jsii.String("EventBus"), &liftpatterns.EventBusPatternProps{
    AppName: "autheory",
    Stage:   "live",
})

eventBus2 := liftpatterns.NewEventBusPattern(stack, jsii.String("EventBus"), &liftpatterns.EventBusPatternProps{
    AppName: "payment-service",
    Stage:   "live",
})

// ✅ GOOD: Explicit table names (most control)
eventBus := liftpatterns.NewEventBusPattern(stack, jsii.String("EventBus"), &liftpatterns.EventBusPatternProps{
    AppName:   "autheory",
    Stage:     "live",
    TableName: jsii.String("autheory-events-live-v2"),  // Full control over name
})
```

## Best Practices

### Event Design

#### 1. Use Clear Event Types

```go
// Good
"partner.created"
"partner.updated"
"partner.deleted"
"key.rotated"
"staff.granted"

// Bad
"partner"
"update"
"event1"
```

#### 2. Include Sufficient Context

```go
event, _ := services.NewEvent("order.placed", tenantID, orderID, orderData)
event.WithMetadata("user_id", userID).
    WithMetadata("session_id", sessionID).
    WithMetadata("source", "web").
    WithCorrelationID(requestID)
```

#### 3. Version Your Events

```go
type OrderPlacedV1 struct {
    OrderID   string  `json:"order_id"`
    Amount    float64 `json:"amount"`
}

type OrderPlacedV2 struct {
    OrderID   string  `json:"order_id"`
    Amount    float64 `json:"amount"`
    Currency  string  `json:"currency"`  // New field
    TaxAmount float64 `json:"tax_amount"` // New field
}

// Set version in event
event, _ := services.NewEvent("order.placed", tenantID, orderID, payloadV2)
event.Version = 2
```

### Performance Optimization

#### 1. Use Batch Publishing

```go
// Instead of:
for _, order := range orders {
    event, _ := services.NewEvent("order.placed", tenantID, order.ID, order)
    eventBus.Publish(ctx, event)
}

// Do this:
var events []*services.Event
for _, order := range orders {
    event, _ := services.NewEvent("order.placed", tenantID, order.ID, order)
    events = append(events, event)
}
eventBus.(*services.DynamoDBEventBus).BatchPublish(ctx, events)
```

#### 2. Set Appropriate TTL

```go
// Short-lived events (notifications, logs)
event.WithTTL(7 * 24 * time.Hour)

// Long-lived events (audit trail, compliance)
event.WithTTL(7 * 365 * 24 * time.Hour)

// Permanent events (never expire)
// Don't set TTL
```

#### 3. Use Tags for Filtering

```go
// Add tags for common queries
event.WithTags("priority", "verified", "us-west-2")

// Query by tags
query := &services.EventQuery{
    TenantID:  tenantID,
    Tags:      []string{"priority", "verified"},
}
```

### Error Handling

#### 1. Don't Fail Requests on Event Publishing

```go
func createPartner(ctx context.Context, req CreatePartnerRequest) error {
    // Create partner
    partnerID, err := db.CreatePartner(req)
    if err != nil {
        return err // Fail the request
    }

    // Publish event (best effort)
    event, _ := services.NewEvent("partner.created", req.TenantID, partnerID, req)
    if _, err := eventBus.Publish(ctx, event); err != nil {
        log.Printf("Failed to publish event: %v", err)
        // Don't return the error - log it instead
    }

    return nil
}
```

#### 2. Implement Retry in Stream Processors

```go
func processEvent(ctx context.Context, event *services.Event) error {
    maxRetries := 3
    event.RetryCount++

    err := handleEvent(ctx, event)
    if err != nil && event.RetryCount < maxRetries {
        // Update retry count and republish
        eventBus.Publish(ctx, event)
        return nil // Don't fail - we're retrying
    }

    return err
}
```

### Security

#### 1. Use IAM Roles with Least Privilege

```go
// CDK: Grant only necessary permissions
eventBus.GrantPublish(apiFunction) // Write-only for API
eventBus.GrantQuery(reportFunction) // Read-only for reports
```

#### 2. Encrypt Sensitive Data

```go
// Encrypt payload before publishing
encryptedPayload, err := encryptData(sensitiveData)
if err != nil {
    return err
}

event, _ := services.NewEvent("sensitive.event", tenantID, sourceID, encryptedPayload)
event.WithMetadata("encrypted", "true")
```

#### 3. Implement Tenant Isolation

```go
// Always include tenant ID
event, _ := services.NewEvent(eventType, tenantID, sourceID, payload)

// Always filter by tenant in queries
query := &services.EventQuery{
    TenantID: tenantID, // Required
    EventType: eventType,
}
```

## Testing

### Unit Testing with Mock EventBus

```go
func TestPublishEvent(t *testing.T) {
    // Create in-memory event bus for testing
    eventBus := services.NewMemoryEventBus()

    // Create and publish event
    event, err := services.NewEvent("test.event", "tenant-1", "source-1", map[string]string{
        "key": "value",
    })
    require.NoError(t, err)

    eventID, err := eventBus.Publish(context.Background(), event)
    require.NoError(t, err)
    require.NotEmpty(t, eventID)

    // Query events
    events, err := eventBus.Query(context.Background(), &services.EventQuery{
        TenantID: "tenant-1",
        EventType: "test.event",
    })
    require.NoError(t, err)
    require.Len(t, events, 1)
    assert.Equal(t, eventID, events[0].ID)
}
```

### Integration Testing

```go
func TestEventBusIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Create real DynamORM DB (requires an actual DynamoDB table)
    cfg, err := config.LoadDefaultConfig(context.Background())
    require.NoError(t, err)

    db, err := dynamorm.New(session.Config{Region: cfg.Region})
    require.NoError(t, err)

    // Create EventBus with test table
    eventBus := services.NewDynamoDBEventBus(db, services.EventBusConfig{
        TableName: "test-events-" + uuid.New().String(),
    })

    // Run tests...
    // (Remember to clean up the table after tests)
}
```

### Testing Stream Processors

```go
func TestStreamProcessor(t *testing.T) {
    var processedEvents []*services.Event

    // Create event bus with test handler
    eventBus := services.NewMemoryEventBus()
    err := eventBus.Subscribe(context.Background(), "test.event", func(ctx context.Context, event *services.Event) error {
        processedEvents = append(processedEvents, event)
        return nil
    })
    require.NoError(t, err)

    // Publish event
    event, _ := services.NewEvent("test.event", "tenant-1", "source-1", nil)
    _, err = eventBus.Publish(context.Background(), event)
    require.NoError(t, err)

    // Give handler time to process (async)
    time.Sleep(100 * time.Millisecond)

    // Verify handler was called
    require.Len(t, processedEvents, 1)
}
```

## Troubleshooting

### Events Not Appearing

**Problem**: Published events don't appear in queries

**Solutions**:
1. Check table name configuration
2. Verify IAM permissions (both read and write)
3. Check tenant ID matches between publish and query
4. Verify event hasn't expired (TTL)

```go
// Enable debug logging
log.Printf("Publishing to table: %s", config.TableName)
log.Printf("Event: tenant=%s type=%s", event.TenantID, event.EventType)
```

### Throttling Errors

**Problem**: `ProvisionedThroughputExceededException`

**Solutions**:
1. Switch to on-demand billing mode
2. Increase provisioned capacity
3. Use batch publishing
4. Implement exponential backoff (built-in)

```go
// Use on-demand billing in CDK
BillingMode: awsdynamodb.BillingMode_PAY_PER_REQUEST

// Or increase provisioned capacity
ReadCapacity:  jsii.Number(100),
WriteCapacity: jsii.Number(100),
```

### High Costs

**Problem**: DynamoDB costs are high

**Solutions**:
1. Reduce TTL to delete old events faster
2. Use appropriate billing mode for your traffic
3. Query efficiently (use partition keys)
4. Archive old events to S3

```go
// Reduce TTL
event.WithTTL(7 * 24 * time.Hour) // 7 days instead of 30

// Query efficiently (always use tenant ID)
query := &services.EventQuery{
    TenantID: tenantID,  // Required for efficient queries
    EventType: eventType,
}
```

### Stream Processing Delays

**Problem**: Stream processor is slow or backing up

**Solutions**:
1. Increase Lambda memory/timeout
2. Increase batch size
3. Enable parallel processing
4. Optimize handler code

```go
// In CDK (EventBusProcessor)
_ = liftconstructs.NewEventBusProcessor(stack, jsii.String("Processor"), &liftconstructs.EventBusProcessorProps{
    Table:     table,
    BatchSize: jsii.Number(100),
    FunctionProps: awslambda.FunctionProps{
        MemorySize: jsii.Number(512),
        Timeout:    awscdk.Duration_Minutes(jsii.Number(1)),
    },
})
```

### Memory EventBus Losing Events

**Problem**: Events disappear in Lambda

**Solution**: Switch to DynamoDB EventBus

```go
// Don't use this in production Lambda:
eventBus := services.NewMemoryEventBus()

// Use this instead:
eventBus := services.NewDynamoDBEventBus(db, config)
```

## Migration from In-Memory EventBus

### Step 1: Deploy Infrastructure

```go
// Add EventBus table to your CDK stack
nameCtx := naming.FromEnv().Normalize()
if !nameCtx.IsComplete() {
    panic("APP_NAME and STAGE are required for deterministic naming")
}

eventBusTable := liftconstructs.NewEventBusTable(stack, jsii.String("EventBus"), &liftconstructs.EventBusTableProps{
    TableName:          jsii.String(nameCtx.ResourceName("events")),
    EnableStream:       jsii.Bool(true),  // optional (required for stream processors)
    EnableEventIDIndex: jsii.Bool(true),
})
_ = eventBusTable
```

### Step 2: Update Lambda Code

```go
// Old code
// eventBus := hubSvc.NewInMemoryEventBus()

// New code
cfg, _ := config.LoadDefaultConfig(context.Background())
db, _ := dynamorm.New(session.Config{Region: cfg.Region})
eventBus := services.NewDynamoDBEventBus(db, services.EventBusConfig{})
```

### Step 3: Deploy and Test

1. Deploy with blue/green or canary
2. Monitor CloudWatch metrics
3. Verify events are persisting
4. Check DynamoDB console for events

### Step 4: Add Stream Processing (Optional)

```go
// Deploy stream processor (DynamoDB Streams → Lambda)
processorEnv := map[string]*string{
    "APP_NAME": jsii.String(nameCtx.AppName),
    "STAGE":    jsii.String(nameCtx.Stage),
}
if nameCtx.Tenant != "" {
    processorEnv["PARTNER"] = jsii.String(nameCtx.Tenant)
}

_ = liftconstructs.NewEventBusProcessor(stack, jsii.String("Processor"), &liftconstructs.EventBusProcessorProps{
    Table:      eventBusTable,
    EventTypes: []string{"partner.created"}, // optional
    FunctionProps: awslambda.FunctionProps{
        FunctionName: jsii.String(nameCtx.ResourceName("eventbus-processor")),
        Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
        Code:         awslambda.Code_FromAsset(jsii.String("./build/processor"), nil),
        Handler:      jsii.String("bootstrap"),
        Environment:  &processorEnv,
    },
})
```

## Related Documentation

- [API Reference](./api-reference.md)
- [CDK Guide](./cdk.md)
- [DynamoDB Streams Patterns](./cdk-dynamo-streams-patterns.md)
- [Testing Guide](./testing-guide.md)
- [Troubleshooting](./troubleshooting.md)

## Support

For issues or questions:
- Open an issue on GitHub
- Check existing documentation
- Review CloudWatch logs and metrics
- Enable debug logging
