# DynamORM Integration Changes in Lift

## What Changed?

We've standardized how DynamoDB tables work with DynamORM in Lift. The key changes are:

1. **All tables now use `pk` and `sk`** as attribute names
2. **GSIs are defined in Go structs**, not CDK
3. **Removed DynamORM-specific CDK constructs** 
4. **Standardized composite key patterns** for better organization

## Why These Changes?

- **Consistency**: All Lift applications now follow the same patterns
- **Simplicity**: Less CDK code, more declarative models
- **Flexibility**: GSIs can be added/changed without infrastructure updates
- **DynamORM Alignment**: Better integration with DynamORM's features

## Quick Migration Example

### Before (Old Way)
```go
// CDK - Complex table definition
table := constructs.NewDynamORMTable(stack, id, &constructs.DynamORMTableProps{
    PartitionKey: "UserID",
    SortKey: "TenantID",
    GSI1PartitionKey: "Email",
})

// Go Model - Simple
type User struct {
    UserID   string `dynamodbav:"UserID"`
    TenantID string `dynamodbav:"TenantID"`
    Email    string `dynamodbav:"Email"`
}
```

### After (New Way)
```go
// CDK - Simple table definition
table := constructs.NewLiftTable(stack, id, &constructs.LiftTableProps{
    TableName: jsii.String("users"),
})

// Go Model - GSIs defined here
type User struct {
    PK    string `dynamorm:"pk"`                    // user#{user_id}
    SK    string `dynamorm:"sk"`                    // tenant#{tenant_id}
    Email string `dynamorm:"index:email-index,pk"` // GSI defined in tag
}
```

## Key Concepts

### 1. Composite Keys
Always use descriptive composite keys:
```go
PK: "user#123"        // Not just "123"
PK: "tenant#abc"      // Not just "abc"  
PK: "order#2024-001"  // Not just "2024-001"
```

### 2. GSI Definition
GSIs are now defined in struct tags:
```go
type Product struct {
    PK       string `dynamorm:"pk"`                     // product#{id}
    SK       string `dynamorm:"sk"`                     // product#{id}
    Category string `dynamorm:"index:category-index,pk"` // GSI for category queries
    Price    int    `dynamorm:"index:price-index,pk"`    // GSI for price ranges
}
```

### 3. Multi-Tenant Patterns
```go
// All tenant data uses tenant ID as partition key
PK: "tenant#123", SK: "user#456"     // User in tenant
PK: "tenant#123", SK: "project#789"  // Project in tenant
PK: "tenant#123", SK: "config#main"  // Tenant config
```

## Common Patterns

### Rate Limiting
```go
type RateLimit struct {
    PK        string `dynamorm:"pk"` // ratelimit#{key}#{window}
    SK        string `dynamorm:"sk"` // ratelimit#{key}#{window}
    Count     int    `json:"count"`
    ExpiresAt int64  `json:"expires_at" dynamorm:"ttl"`
}
```

### WebSocket Connections
```go
type Connection struct {
    PK           string `dynamorm:"pk"`                      // connection#{id}
    SK           string `dynamorm:"sk"`                      // connection#{id}
    UserID       string `dynamorm:"index:user-index,pk"`     // For user lookups
    ConnectionID string `json:"connection_id"`
    TTL          int64  `json:"ttl" dynamorm:"ttl"`
}
```

### Event Store
```go
type Event struct {
    PK        string `dynamorm:"pk"`                    // stream#{stream_id}
    SK        string `dynamorm:"sk"`                    // event#{timestamp}#{id}
    EventType string `dynamorm:"index:type-index,pk"`   // For type queries
    Timestamp string `dynamorm:"index:type-index,sk"`
}
```

## FAQ

**Q: Do I need to migrate existing data?**
A: Yes, if you're using the old attribute names. See the [migration guide](dynamorm-migration-guide.md).

**Q: Can I still create GSIs in CDK?**
A: No, GSIs should be defined in your DynamORM models using struct tags.

**Q: What about existing applications?**
A: They'll continue to work, but we recommend migrating to the new structure.

**Q: Is this backwards compatible?**
A: No, you'll need to update your models and queries to use the new structure.

## Getting Help

- See full [DynamORM Integration Guide](dynamorm-integration.md)
- Check the [Migration Guide](dynamorm-migration-guide.md) 
- Review examples in `examples/dynamorm-multi-tenant/`
- Look at test files for patterns