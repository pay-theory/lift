# DynamORM Table Migration Guide

This guide helps you migrate from the old DynamORM-specific table constructs to the new standardized Lift tables.

## Overview of Changes

The main changes in the new architecture:
1. All tables now use `pk` and `sk` as primary keys
2. GSIs are defined in DynamORM models, not CDK
3. Removed DynamORM-specific constructs (DynamORMTable, DynamORMCache, etc.)
4. Standardized on composite key patterns for multi-tenancy

## Migration Steps

### 1. Update CDK Table Definitions

#### Before (Old DynamORM Tables)
```go
// Old way - DynamORM-specific construct
table := constructs.NewDynamORMTable(stack, jsii.String("UserTable"), &constructs.DynamORMTableProps{
    TableName:        jsii.String("users"),
    PartitionKey:     jsii.String("TenantID"),
    SortKey:          jsii.String("UserID"),
    GSI1PartitionKey: jsii.String("Email"),
    GSI1SortKey:      jsii.String("CreatedAt"),
    GSI2PartitionKey: jsii.String("Status"),
    EnableStreams:    jsii.Bool(true),
})
```

#### After (New Lift Tables)
```go
// New way - Standard Lift table
table := constructs.NewLiftTable(stack, jsii.String("UserTable"), &constructs.LiftTableProps{
    TableName:           jsii.String("users"),
    EnableStreams:       jsii.Bool(true),
    TimeToLiveAttribute: jsii.String("ttl"),
})
```

### 2. Update DynamORM Models

#### Before (Old Structure)
```go
type User struct {
    TenantID  string ``
    UserID    string ``
    Email     string ``
    CreatedAt string ``
    Status    string ``
    Name      string ``
}
```

#### After (New Structure)
```go
type User struct {
    // Composite keys - BOTH tags required!
    PK string `dynamorm:"pk" `  // tenant#{tenant_id}
    SK string `dynamorm:"sk" `  // user#{user_id}
    
    // GSI definitions via struct tags
    Email     string `dynamorm:"index:email-index,pk" `
    CreatedAt string `dynamorm:"index:email-index,sk" `
    Status    string `dynamorm:"index:status-index,pk" `
    UserID    string `dynamorm:"index:status-index,sk" `
    
    // Keep tenant_id for filtering
    TenantID string `dynamorm:"index:tenant-index,pk" `
    
    // Business fields
    Name string    `json:"name" `
    TTL  int64     `json:"ttl,omitempty" dynamorm:"ttl"`
}
```

**Important**: Only `dynamorm` tags are needed - DynamORM handles all marshaling internally

### 3. Update Data Access Code

#### Before (Direct Key Access)
```go
// Old way - direct attribute access
user, err := dynamorm.Get[User](ctx, db).
    WithTable("users").
    WithKey("TenantID", tenantID).
    WithKey("UserID", userID).
    Execute()

// Query by email
users, err := dynamorm.Query[User](ctx, db).
    WithTable("users").
    WithIndex("GSI1").
    WithKey("Email", email).
    Execute()
```

#### After (Composite Keys)
```go
// New way - composite keys
user, err := dynamorm.Get[User](ctx, db).
    WithTable("users").
    WithPK(fmt.Sprintf("tenant#%s", tenantID)).
    WithSK(fmt.Sprintf("user#%s", userID)).
    Execute()

// Query by email (GSI defined in struct)
users, err := dynamorm.Query[User](ctx, db).
    WithTable("users").
    WithIndex("email-index").
    WithPK(email).
    Execute()
```

### 4. Data Migration Script

Here's a sample script to migrate existing data:

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/pay-theory/dynamorm"
)

// OldUser represents the legacy structure
type OldUser struct {
    TenantID  string ``
    UserID    string ``
    Email     string ``
    Name      string ``
    CreatedAt string ``
    Status    string ``
}

// NewUser represents the new structure
type NewUser struct {
    PK        string `dynamorm:"pk" `
    SK        string `dynamorm:"sk" `
    Email     string `dynamorm:"index:email-index,pk" `
    CreatedAt string `dynamorm:"index:email-index,sk" `
    TenantID  string `dynamorm:"index:tenant-index,pk" `
    UserID    string `json:"user_id" `
    Name      string `json:"name" `
    Status    string `json:"status" `
}

func migrateUsers(ctx context.Context, oldTable, newTable string) error {
    // Scan old table
    oldUsers, err := dynamorm.Scan[OldUser](ctx, db).
        WithTable(oldTable).
        Execute()
    if err != nil {
        return err
    }
    
    // Transform and write to new table
    for _, old := range oldUsers {
        new := NewUser{
            PK:        fmt.Sprintf("tenant#%s", old.TenantID),
            SK:        fmt.Sprintf("user#%s", old.UserID),
            Email:     old.Email,
            CreatedAt: old.CreatedAt,
            TenantID:  old.TenantID,
            UserID:    old.UserID,
            Name:      old.Name,
            Status:    old.Status,
        }
        
        err := dynamorm.Put(ctx, db, newTable, new).Execute()
        if err != nil {
            log.Printf("Failed to migrate user %s: %v", old.UserID, err)
            continue
        }
    }
    
    return nil
}
```

### 5. Update CDK Patterns

#### Multi-Tenant API Pattern

**Before:**
```go
patterns.NewMultiTenantAPI(stack, jsii.String("API"), &patterns.MultiTenantAPIProps{
    ApiName:     jsii.String("tenant-api"),
    TableConfig: patterns.MultiTenantTableConfig_SEPARATE_TABLES,
})
```

**After:**
```go
patterns.NewLiftApp(stack, jsii.String("API"), &patterns.LiftAppProps{
    AppName:           jsii.String("tenant-api"),
    EnableMultiTenant: jsii.Bool(true),
    EnableDatabase:    jsii.Bool(true),
})
```

## Common Migration Patterns

### 1. Rate Limiting Tables

**Old Structure:**
```go
type RateLimit struct {
    Identifier string
    Window     string
    Count      int
    ExpiresAt  int64
}
```

**New Structure:**
```go
type RateLimit struct {
    PK        string `dynamorm:"pk" `  // ratelimit#{identifier}#{window}
    SK        string `dynamorm:"sk" `  // ratelimit#{identifier}#{window}
    IPAddress string `dynamorm:"index:ip-index,pk" `
    UserID    string `dynamorm:"index:user-index,pk" `
    Count     int    `json:"count" `
    ExpiresAt int64  `json:"expires_at" dynamorm:"ttl"`
}
```

### 2. Connection Tables (WebSocket)

**Old Structure:**
```go
type Connection struct {
    ConnectionID string
    UserID       string
    ConnectedAt  string
}
```

**New Structure:**
```go
type Connection struct {
    PK           string `dynamorm:"pk" `  // connection#{connection_id}
    SK           string `dynamorm:"sk" `  // connection#{connection_id}
    UserID       string `dynamorm:"index:user-connections,pk" `
    ConnectedAt  string `dynamorm:"index:user-connections,sk" `
    ConnectionID string `json:"connection_id" `
    TTL          int64  `json:"ttl" dynamorm:"ttl"`
}
```

## Testing Your Migration

1. **Test queries with new key structure:**
```go
func TestUserQuery(t *testing.T) {
    // Test single item get
    user, err := dynamorm.Get[User](ctx, db).
        WithTable("users").
        WithPK("tenant#123").
        WithSK("user#456").
        Execute()
    assert.NoError(t, err)
    assert.Equal(t, "123", user.TenantID)
    
    // Test query by tenant
    users, err := dynamorm.Query[User](ctx, db).
        WithTable("users").
        WithPK("tenant#123").
        WithSKPrefix("user#").
        Execute()
    assert.NoError(t, err)
    assert.Greater(t, len(users), 0)
}
```

2. **Verify GSI queries:**
```go
func TestGSIQuery(t *testing.T) {
    // Query by email
    users, err := dynamorm.Query[User](ctx, db).
        WithTable("users").
        WithIndex("email-index").
        WithPK("user@example.com").
        Execute()
    assert.NoError(t, err)
}
```

## Rollback Plan

If you need to rollback:

1. Keep old table intact during migration
2. Use dual-write pattern during transition:
```go
// Write to both old and new format
func SaveUser(user User) error {
    // Write to new table
    newUser := transformToNew(user)
    err := dynamorm.Put(ctx, db, newTable, newUser).Execute()
    
    // Also write to old table
    oldUser := transformToOld(user)
    err = dynamorm.Put(ctx, db, oldTable, oldUser).Execute()
    
    return err
}
```

3. Switch reads gradually:
```go
func GetUser(tenantID, userID string) (*User, error) {
    if featureFlag.UseNewTable() {
        return getFromNewTable(tenantID, userID)
    }
    return getFromOldTable(tenantID, userID)
}
```

## Benefits After Migration

1. **Consistent table structure** across all Lift applications
2. **Better DynamORM integration** with standard patterns
3. **Simplified CDK code** with fewer custom constructs
4. **Clear multi-tenant patterns** using composite keys
5. **Future-proof** for DynamORM enhancements

## Getting Help

- Check examples in `examples/dynamorm-multi-tenant/`
- Review test files for migration patterns
- See `docs/dynamorm-integration.md` for detailed patterns

## Checklist

- [ ] Update all CDK table definitions to use `NewLiftTable`
- [ ] Update DynamORM models with new key structure
- [ ] Add GSI definitions to struct tags
- [ ] Update all queries to use composite keys
- [ ] Test all access patterns
- [ ] Migrate existing data
- [ ] Update Lambda environment variables if needed
- [ ] Remove old DynamORM-specific constructs from imports