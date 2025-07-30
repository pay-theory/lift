# DynamORM Integration with Lift

This guide explains how DynamORM integrates with Lift for DynamoDB operations, including table design patterns, multi-tenant support, and CDK infrastructure.

## Overview

Lift uses a standardized approach for all DynamoDB tables, making them compatible with DynamORM. All tables follow a consistent single-table design pattern with:
- Primary key: `pk` (partition key)
- Sort key: `sk` (sort key)
- Global Secondary Indexes (GSIs) must be created in your infrastructure code
- DynamORM struct tags map model fields to existing GSIs
- Time-to-live (TTL) attributes for automatic data expiration

## DynamoDB Connection Initialization

### Basic Initialization

To initialize a DynamoDB connection in your Lift application:

```go
import (
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/dynamorm/pkg/core"
    "github.com/pay-theory/dynamorm/pkg/session"
)

// Initialize DynamoDB connection
var db core.DB
db, err := dynamorm.NewBasic(session.Config{
    Region: "us-east-1",
    // For local testing with DynamoDB Local:
    // Endpoint: "http://localhost:8000",
})
if err != nil {
    log.Fatal("Failed to initialize DynamoDB:", err)
}
```

### Extended Functionality

If you need extended DynamORM features, use `dynamorm.New()`:

```go
// Returns core.ExtendedDB with additional features
var db core.ExtendedDB
db, err := dynamorm.New(session.Config{
    Region: "us-east-1",
})
if err != nil {
    log.Fatal("Failed to initialize DynamoDB:", err)
}
```

### Using the Connection

The initialized `db` (type `core.DB` or `core.ExtendedDB`) can be used throughout your application:

```go
// Query example
users, err := dynamorm.Query[User](ctx, db).
    WithTable("my-app-table").
    WithPK("user#123").
    Execute()

// With rate limiting
rateLimiter := limited.NewDynamoRateLimiter(
    db,  // Pass the core.DB
    nil, // Use default config
    strategy,
    logger,
)
```

## Table Structure

### Standard Table Design

All Lift tables use the same basic structure. **Important**: If your table needs GSIs, you must define them in your infrastructure code:

```go
// CDK Table Creation
liftTable := constructs.NewLiftTable(stack, jsii.String("MyTable"), &constructs.LiftTableProps{
    TableName:           jsii.String("my-app-table"),
    TimeToLiveAttribute: jsii.String("ttl"),
    EnableStreams:       jsii.Bool(true),
})

// Add GSIs using the underlying DynamoDB table
liftTable.Table.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
    IndexName: jsii.String("email-index"),
    PartitionKey: &awsdynamodb.Attribute{
        Name: jsii.String("Email"),
        Type: awsdynamodb.AttributeType_STRING,
    },
})

liftTable.Table.AddGlobalSecondaryIndex(&awsdynamodb.GlobalSecondaryIndexProps{
    IndexName: jsii.String("tenant-index"),
    PartitionKey: &awsdynamodb.Attribute{
        Name: jsii.String("TenantID"),
        Type: awsdynamodb.AttributeType_STRING,
    },
    SortKey: &awsdynamodb.Attribute{
        Name: jsii.String("CreatedAt"),
        Type: awsdynamodb.AttributeType_STRING,
    },
})
```

This creates a table with:
- Partition key: `pk` (String)
- Sort key: `sk` (String)
- Global Secondary Indexes as defined
- Pay-per-request billing
- Optional TTL attribute
- Optional DynamoDB Streams

**Note**: GSIs must be created during table creation. DynamORM cannot add GSIs to existing tables.

### DynamORM Model Definition

Define your data models using DynamORM struct tags:

```go
type User struct {
    // Keys - DynamORM uses field names as DynamoDB attribute names
    PK string `dynamorm:"pk"`  // user#{user_id}
    SK string `dynamorm:"sk"`  // user#{user_id}
    
    // GSI attributes
    Email     string `dynamorm:"index:email-index,pk"`      // For email lookups
    TenantID  string `dynamorm:"index:tenant-index,pk"`     // For tenant queries
    CreatedAt string `dynamorm:"index:tenant-index,sk"`     // For sorting
    
    // Data attributes
    UserID    string    `json:"user_id"`
    Name      string    `json:"name"`
    Status    string    `json:"status"`
    UpdatedAt time.Time `json:"updated_at"`
    TTL       int64     `json:"ttl,omitempty" dynamorm:"ttl"`
}
```

**Important**: 
- DynamORM uses the Go field names as DynamoDB attribute names
- The table must be created with matching attribute names (e.g., "PK" and "SK" for the keys)
- Only `dynamorm` tags are needed - DynamORM handles the marshaling internally

## Multi-Tenant Patterns

Lift supports multi-tenant architectures using composite keys:

### Tenant-Isolated Data

```go
type TenantUser struct {
    // Composite keys for tenant isolation
    PK string `dynamorm:"pk"`  // tenant#{tenant_id}
    SK string `dynamorm:"sk"`  // user#{user_id}
    
    // Tenant queries
    TenantID   string `dynamorm:"index:tenant-index,pk"`
    EntityType string `dynamorm:"index:tenant-index,sk"`  // "user"
    
    // User data
    UserID   string `json:"user_id"`
    Email    string `json:"email"`
    Name     string `json:"name"`
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
    PK string `dynamorm:"pk"`  // user#{user_id}
    SK string `dynamorm:"sk"`  // user#{user_id}
    
    // Global email index
    Email    string `dynamorm:"index:email-index,pk"`
    TenantID string `dynamorm:"index:email-index,sk"`
    
    // Cross-tenant tenant lookup (same field can be in multiple indexes)
    UserID string `dynamorm:"index:tenant-users,sk"`
}
```

## Common Table Patterns

### 1. Connection Table (WebSocket)

```go
type Connection struct {
    PK string `dynamorm:"pk"`  // connection#{connection_id}
    SK string `dynamorm:"sk"`  // connection#{connection_id}
    
    // Indexes
    UserID    string `dynamorm:"index:user-connections,pk"`
    Timestamp string `dynamorm:"index:user-connections,sk"`
    
    // Connection data
    ConnectionID string    `json:"connection_id"`
    Endpoint     string    `json:"endpoint"`
    ConnectedAt  time.Time `json:"connected_at"`
    TTL          int64     `json:"ttl"`
}
```

### 2. Rate Limiting Table

```go
type RateLimit struct {
    PK string `dynamorm:"pk"`  // ratelimit#{identifier}#{window}
    SK string `dynamorm:"sk"`  // ratelimit#{identifier}#{window}
    
    // Rate limit indexes
    IPAddress  string `dynamorm:"index:ip-index,pk"`
    UserID     string `dynamorm:"index:user-index,pk"`
    TenantID   string `dynamorm:"index:tenant-index,pk"`
    
    // Rate limit data
    Identifier string `json:"identifier"`
    WindowTime string `json:"window_time"`
    Count      int    `json:"count"`
    ExpiresAt  int64  `json:"expires_at" dynamorm:"ttl"`
}
```

### 3. Idempotency Table

```go
type IdempotencyRecord struct {
    PK string `dynamorm:"pk"`  // idempotency#{key}
    SK string `dynamorm:"sk"`  // idempotency#{key}
    
    // Lookup indexes
    FunctionName string `dynamorm:"index:function-index,pk"`
    Status       string `dynamorm:"index:status-index,pk"`
    Timestamp    string `dynamorm:"index:status-index,sk"`
    
    // Idempotency data
    IdempotencyKey string          `json:"idempotency_key"`
    Response       json.RawMessage `json:"response"`
    Status         string          `json:"status"`
    ExpiresAt      int64           `json:"expires_at" dynamorm:"ttl"`
}
```

### 4. Event Store

```go
type Event struct {
    PK string `dynamorm:"pk"`  // stream#{stream_id}
    SK string `dynamorm:"sk"`  // event#{timestamp}#{event_id}
    
    // Event indexes
    EventType  string `dynamorm:"index:type-index,pk"`
    Timestamp  string `dynamorm:"index:type-index,sk"`
    AggregateID string `dynamorm:"index:aggregate-index,pk"`
    Version     int    `dynamorm:"index:aggregate-index,sk"`
    
    // Event data
    EventID   string          `json:"event_id"`
    Data      json.RawMessage `json:"data"`
    Metadata  map[string]any  `json:"metadata"`
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
    "github.com/pay-theory/dynamorm/pkg/core"
    "github.com/pay-theory/dynamorm/pkg/session"
    "github.com/pay-theory/lift/pkg/lift"
)

var db core.DB

func init() {
    // Initialize DynamoDB connection
    var err error
    db, err = dynamorm.NewBasic(session.Config{
        Region: os.Getenv("AWS_REGION"),
    })
    if err != nil {
        panic(err)
    }
}

func handler(ctx *lift.Context) error {
    tableName := os.Getenv("DYNAMODB_TABLE")
    
    // Create a new user
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
    PK       string `dynamorm:"pk"`        // tenant#{tenant_id}
    SK       string `dynamorm:"sk"`        // user#{user_id}
    Email    string `dynamorm:"index:email-index,pk"`
    TenantID string `json:"tenant_id"`
    UserID   string `json:"user_id"`
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

1. **GSI not created**: GSIs must be created in your infrastructure (CDK, CloudFormation, or AWS Console). DynamORM struct tags only tell DynamORM how to use existing GSIs - they don't create them. Ensure both:
   - Your infrastructure creates the GSIs with matching names
   - Your struct has proper index tags that match the GSI names

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