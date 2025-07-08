# DynamORM Integration with Lift

This guide explains how DynamORM integrates with Lift for DynamoDB operations, including table design patterns, multi-tenant support, and CDK infrastructure.

## Overview

Lift uses a standardized approach for all DynamoDB tables, making them compatible with DynamORM. All tables follow a consistent single-table design pattern with:
- Primary key: `pk` (partition key)
- Sort key: `sk` (sort key)
- Global Secondary Indexes (GSIs) defined through DynamORM struct tags
- Time-to-live (TTL) attributes for automatic data expiration

## Table Structure

### Standard Table Design

All Lift tables use the same basic structure:

```go
// CDK Table Creation
table := constructs.NewLiftTable(stack, jsii.String("MyTable"), &constructs.LiftTableProps{
    TableName:           jsii.String("my-app-table"),
    TimeToLiveAttribute: jsii.String("ttl"),
    EnableStreams:       jsii.Bool(true),
})
```

This creates a table with:
- Partition key: `pk` (String)
- Sort key: `sk` (String)
- Pay-per-request billing
- Optional TTL attribute
- Optional DynamoDB Streams

### DynamORM Model Definition

Define your data models using BOTH DynamORM struct tags AND DynamoDB marshaling tags:

```go
type User struct {
    // Keys - MUST have both dynamorm AND dynamodbav tags
    PK string `dynamorm:"pk" dynamodbav:"pk"`  // user#{user_id}
    SK string `dynamorm:"sk" dynamodbav:"sk"`  // user#{user_id}
    
    // GSI attributes - need dynamodbav for marshaling
    Email     string `dynamorm:"index:email-index,pk" dynamodbav:"email"`      // For email lookups
    TenantID  string `dynamorm:"index:tenant-index,pk" dynamodbav:"tenant_id"` // For tenant queries
    CreatedAt string `dynamorm:"index:tenant-index,sk" dynamodbav:"created_at"` // For sorting
    
    // Data attributes
    UserID    string    `json:"user_id" dynamodbav:"user_id"`
    Name      string    `json:"name" dynamodbav:"name"`
    Status    string    `json:"status" dynamodbav:"status"`
    UpdatedAt time.Time `json:"updated_at" dynamodbav:"updated_at"`
    TTL       int64     `json:"ttl,omitempty" dynamodbav:"ttl,omitempty" dynamorm:"ttl"`
}
```

**Important**: 
- `dynamorm` tags tell DynamORM which fields are keys and indexes
- `dynamodbav` tags are required for actual DynamoDB marshaling
- Both tags are necessary for proper operation
```

## Multi-Tenant Patterns

Lift supports multi-tenant architectures using composite keys:

### Tenant-Isolated Data

```go
type TenantUser struct {
    // Composite keys for tenant isolation - BOTH tags required
    PK string `dynamorm:"pk" dynamodbav:"pk"`  // tenant#{tenant_id}
    SK string `dynamorm:"sk" dynamodbav:"sk"`  // user#{user_id}
    
    // Tenant queries
    TenantID   string `dynamorm:"index:tenant-index,pk" dynamodbav:"tenant_id"`
    EntityType string `dynamorm:"index:tenant-index,sk" dynamodbav:"entity_type"`  // "user"
    
    // User data
    UserID   string `json:"user_id" dynamodbav:"user_id"`
    Email    string `json:"email" dynamodbav:"email"`
    Name     string `json:"name" dynamodbav:"name"`
}

// Query all users for a tenant
users, err := dynamorm.Query[TenantUser](ctx, db).
    WithPK(fmt.Sprintf("tenant#%s", tenantID)).
    WithSKPrefix("user#").
    Execute()
```

### Cross-Tenant Queries

```go
type GlobalUser struct {
    PK string `dynamorm:"pk" dynamodbav:"pk"`  // user#{user_id}
    SK string `dynamorm:"sk" dynamodbav:"sk"`  // user#{user_id}
    
    // Global email index
    Email    string `dynamorm:"index:email-index,pk" dynamodbav:"email"`
    TenantID string `dynamorm:"index:email-index,sk" dynamodbav:"tenant_id"`
    
    // Cross-tenant tenant lookup (same field can be in multiple indexes)
    UserID string `dynamorm:"index:tenant-users,sk" dynamodbav:"user_id"`
}
```

## Common Table Patterns

### 1. Connection Table (WebSocket)

```go
type Connection struct {
    PK string `dynamorm:"pk" dynamodbav:"pk"`  // connection#{connection_id}
    SK string `dynamorm:"sk" dynamodbav:"sk"`  // connection#{connection_id}
    
    // Indexes
    UserID    string `dynamorm:"index:user-connections,pk" dynamodbav:"user_id"`
    Timestamp string `dynamorm:"index:user-connections,sk" dynamodbav:"timestamp"`
    
    // Connection data
    ConnectionID string    `json:"connection_id" dynamodbav:"connection_id"`
    Endpoint     string    `json:"endpoint" dynamodbav:"endpoint"`
    ConnectedAt  time.Time `json:"connected_at" dynamodbav:"connected_at"`
    TTL          int64     `json:"ttl" dynamodbav:"ttl,omitempty"`
}
```

### 2. Rate Limiting Table

```go
type RateLimit struct {
    PK string `dynamorm:"pk" dynamodbav:"pk"`  // ratelimit#{identifier}#{window}
    SK string `dynamorm:"sk" dynamodbav:"sk"`  // ratelimit#{identifier}#{window}
    
    // Rate limit indexes
    IPAddress  string `dynamorm:"index:ip-index,pk" dynamodbav:"ip_address,omitempty"`
    UserID     string `dynamorm:"index:user-index,pk" dynamodbav:"user_id,omitempty"`
    TenantID   string `dynamorm:"index:tenant-index,pk" dynamodbav:"tenant_id,omitempty"`
    
    // Rate limit data
    Identifier string `json:"identifier" dynamodbav:"identifier"`
    WindowTime string `json:"window_time" dynamodbav:"window_time"`
    Count      int    `json:"count" dynamodbav:"count"`
    ExpiresAt  int64  `json:"expires_at" dynamodbav:"expires_at" dynamorm:"ttl"`
}
```

### 3. Idempotency Table

```go
type IdempotencyRecord struct {
    PK string `dynamorm:"pk" dynamodbav:"pk"`  // idempotency#{key}
    SK string `dynamorm:"sk" dynamodbav:"sk"`  // idempotency#{key}
    
    // Lookup indexes
    FunctionName string `dynamorm:"index:function-index,pk" dynamodbav:"function_name"`
    Status       string `dynamorm:"index:status-index,pk" dynamodbav:"status"`
    Timestamp    string `dynamorm:"index:status-index,sk" dynamodbav:"timestamp"`
    
    // Idempotency data
    IdempotencyKey string          `json:"idempotency_key" dynamodbav:"idempotency_key"`
    Response       json.RawMessage `json:"response" dynamodbav:"response"`
    Status         string          `json:"status" dynamodbav:"status"`
    ExpiresAt      int64           `json:"expires_at" dynamodbav:"expires_at" dynamorm:"ttl"`
}
```

### 4. Event Store

```go
type Event struct {
    PK string `dynamorm:"pk" dynamodbav:"pk"`  // stream#{stream_id}
    SK string `dynamorm:"sk" dynamodbav:"sk"`  // event#{timestamp}#{event_id}
    
    // Event indexes
    EventType  string `dynamorm:"index:type-index,pk" dynamodbav:"event_type"`
    Timestamp  string `dynamorm:"index:type-index,sk" dynamodbav:"timestamp"`
    AggregateID string `dynamorm:"index:aggregate-index,pk" dynamodbav:"aggregate_id"`
    Version     int    `dynamorm:"index:aggregate-index,sk" dynamodbav:"version"`
    
    // Event data
    EventID   string          `json:"event_id" dynamodbav:"event_id"`
    Data      json.RawMessage `json:"data" dynamodbav:"data"`
    Metadata  map[string]any  `json:"metadata" dynamodbav:"metadata,omitempty"`
}
```

## CDK Integration

### Creating Tables with CDK

```go
// Basic table
table := constructs.NewLiftTable(stack, jsii.String("AppTable"), &constructs.LiftTableProps{
    TableName: jsii.String("my-app-table"),
})

// With specific TTL attribute
rateLimitTable := constructs.NewRateLimitTable(stack, jsii.String("RateLimits"), &constructs.RateLimitTableProps{
    TableName:           jsii.String("rate-limits"),
    TimeToLiveAttribute: jsii.String("expires_at"),
})

// Connection table for WebSocket
connectionTable := constructs.NewConnectionTable(stack, jsii.String("Connections"), &constructs.ConnectionTableProps{
    TableName: jsii.String("websocket-connections"),
})
```

### Complete Application Pattern

```go
app := patterns.NewLiftApp(stack, jsii.String("MyApp"), &patterns.LiftAppProps{
    AppName:           jsii.String("my-app"),
    EnableDatabase:    jsii.Bool(true),
    EnableMultiTenant: jsii.Bool(true),
    Environment: &map[string]*string{
        "DYNAMODB_TABLE": jsii.String("my-app-table"),
    },
})

// The app automatically:
// - Creates a DynamoDB table with pk/sk structure
// - Grants Lambda read/write permissions
// - Sets the table name in Lambda environment variables
```

## Using DynamORM in Lambda Functions

### Basic Setup

```go
package main

import (
    "context"
    "os"
    
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/dynamorm/pkg/session"
    "github.com/pay-theory/lift/pkg/lift"
)

var db *dynamorm.Client

func init() {
    // Initialize DynamORM client
    client, err := dynamorm.NewClient(session.Config{
        Region: os.Getenv("AWS_REGION"),
    })
    if err != nil {
        panic(err)
    }
    db = client
}

func handler(ctx *lift.Context) error {
    tableName := os.Getenv("DYNAMODB_TABLE")
    
    // Create a new user - make sure to set all fields with dynamodbav tags
    user := &User{
        PK:        fmt.Sprintf("user#%s", userID),
        SK:        fmt.Sprintf("user#%s", userID),
        UserID:    userID,
        Email:     email,
        TenantID:  ctx.TenantID(),
        Name:      name,
        Status:    "active",
        CreatedAt: time.Now().Format(time.RFC3339),
        UpdatedAt: time.Now(),
    }
    
    err := dynamorm.Put(ctx.Context, db, tableName, user).Execute()
    if err != nil {
        return lift.NewError(500, "Failed to create user", nil)
    }
    
    return ctx.JSON(user)
}
```

### Multi-Tenant Queries

```go
func listTenantUsers(ctx *lift.Context) error {
    tableName := os.Getenv("DYNAMODB_TABLE")
    tenantID := ctx.TenantID()
    
    if tenantID == "" {
        return lift.NewError(400, "Tenant ID required", nil)
    }
    
    // Query all users for the tenant
    users, err := dynamorm.Query[TenantUser](ctx.Context, db).
        WithTable(tableName).
        WithPK(fmt.Sprintf("tenant#%s", tenantID)).
        WithSKPrefix("user#").
        Execute()
    
    if err != nil {
        return lift.NewError(500, "Failed to query users", nil)
    }
    
    return ctx.JSON(users)
}
```

## Migration from Old DynamORM Tables

If you have existing tables using the old DynamORM-specific constructs, migrate them to the standard format:

### Old Structure
```go
// Before - DynamORM-specific table
table := constructs.NewDynamORMTable(stack, id, &constructs.DynamORMTableProps{
    PartitionKey: "TenantID",
    SortKey:      "UserID",
    GSI1PartitionKey: "Email",
    // ...
})
```

### New Structure
```go
// After - Standard Lift table
table := constructs.NewLiftTable(stack, id, &constructs.LiftTableProps{
    TableName: jsii.String("my-table"),
})

// Define structure in DynamORM model
type User struct {
    PK       string `dynamorm:"pk" dynamodbav:"pk"`        // tenant#{tenant_id}
    SK       string `dynamorm:"sk" dynamodbav:"sk"`        // user#{user_id}
    Email    string `dynamorm:"index:email-index,pk" dynamodbav:"email"`
    TenantID string `json:"tenant_id" dynamodbav:"tenant_id"`
    UserID   string `json:"user_id" dynamodbav:"user_id"`
}
```

## Best Practices

1. **Always use composite keys** for clear entity identification:
   - `pk`: `entity_type#{id}` (e.g., `user#123`, `tenant#abc`)
   - `sk`: Depends on access pattern (could be same as pk or hierarchical)

2. **Design indexes based on access patterns**:
   - Use GSIs for alternative query patterns
   - Keep GSI keys meaningful and efficient

3. **Implement TTL for temporary data**:
   - Rate limits, idempotency records, temporary tokens
   - Set appropriate expiration times

4. **Use batch operations** for efficiency:
   ```go
   items := []interface{}{user1, user2, user3}
   err := dynamorm.BatchPut(ctx, db, tableName, items).Execute()
   ```

5. **Handle multi-tenancy at the data layer**:
   - Include tenant ID in composite keys
   - Use tenant-specific GSIs for efficient queries
   - Validate tenant access in Lambda handlers

## Troubleshooting

### Common Issues

1. **GSI not created**: GSIs are defined in DynamORM models, not CDK. Ensure your struct has proper index tags.

2. **Query returns no results**: Check your key construction. Use composite keys correctly.

3. **TTL not working**: Ensure the TTL attribute name matches between CDK and your model tags.

4. **Access denied**: Verify Lambda has proper IAM permissions for the table and indexes.

### Debug Tips

Enable DynamORM debugging:
```go
os.Setenv("DYNAMORM_DEBUG", "true")
```

Log table operations:
```go
app := lift.New(lift.WithDebug(true))
```

## Examples

See the following examples in the repository:
- `examples/dynamorm-multi-tenant/` - Multi-tenant SaaS application
- `examples/websocket-enhanced/` - WebSocket with connection management
- `examples/production-api/` - Production-ready API with all features
- `examples/rate-limiting/` - Rate limiting implementation