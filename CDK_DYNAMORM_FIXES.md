# CDK DynamORM Fixes - Systematic Replacement Guide

## STATUS: COMPLETED ✅

All CDK constructs have been updated to work with DynamORM. Tables now only create `pk`/`sk` attributes and GSIs are defined through DynamORM model struct tags.

## Summary
All CDK constructs that create DynamoDB tables are fundamentally broken. They create infrastructure that doesn't match DynamORM's requirements and try to do DynamORM's job instead of just creating basic tables.

## Core Problem
1. DynamORM defines table structure through struct tags (`dynamorm:"pk"`, `dynamorm:"index:name,pk"`)
2. CDK constructs are creating GSIs and table structures that don't match
3. Tables should ONLY have `pk`/`sk` - DynamORM handles the rest

## Changes Made

### 1. LiftTable (Priority 1) ✅
- Removed hardcoded GSI creation
- Removed `EnableMultiTenant` field
- Tables now only create `pk`/`sk` attributes
- Added `GetTableName()`, `GetTableArn()`, `GetStreamArn()`, and `GetResourceName()` methods

### 2. DynamORMTable ✅
- Completely removed (deleted file)
- All references replaced with LiftTable

### 3. Specialized Table Constructs ✅
All updated to use LiftTable internally:
- ConnectionTable
- IdempotencyTable  
- RateLimitTable
- RequestTrackingTable
- EventRoutingTable
- StreamingTable
- DynamORMEventStore (uses LiftTable for event/snapshot tables)

### 4. StreamProcessor ✅
- Created new StreamProcessor to replace DynamORMStreamProcessor
- Uses StreamingTable instead of DynamORMTable
- Fixed DeadLetterQueue configuration

### 5. Unused Constructs Deleted ✅
- DynamORMCRUDAPI
- DynamORMCache
- MultiTenantAPI
- DynamORMStreamProcessor

### 6. Examples Updated ✅
- dynamorm-multi-tenant example updated
- dynamorm-stream-processing example updated
- Removed calls to undefined methods

### 7. Method Updates ✅
All permission methods updated to use standard DynamoDB grant methods:
- `AddDynamORMPermissions()` → `GrantReadWrite()`
- `GrantRead()` → `Table.GrantReadData()`
- `GrantWrite()` → `Table.GrantWriteData()`

## Constructs to Fix/Replace

### Priority 1: Core Table Constructs (Break Everything Else)

#### 1. `/pkg/cdk/constructs/dynamodb.go` - `LiftTable`
**Current Problem:**
- Creates generic `gsi1` with `gsi1pk`/`gsi1sk` that nobody uses
- `EnableMultiTenant` flag does nothing

**Required Fix:**
- Remove ALL GSI creation
- Just create table with `pk` and `sk` attributes
- Remove multi-tenant logic

**New Structure:**
```go
// Simple table with pk/sk only
table := awsdynamodb.NewTable(scope, id, &awsdynamodb.TableProps{
    TableName: props.TableName,
    PartitionKey: &awsdynamodb.Attribute{Name: jsii.String("pk"), Type: awsdynamodb.AttributeType_STRING},
    SortKey: &awsdynamodb.Attribute{Name: jsii.String("sk"), Type: awsdynamodb.AttributeType_STRING},
    BillingMode: props.BillingMode,
    // Basic settings only
})
```

#### 2. `/pkg/cdk/constructs/dynamorm_table.go` - `DynamORMTable`
**Current Problem:**
- 1369 lines trying to be smart about table structure
- Creates wrong GSIs (tenant_id, entity_type, created_at, status)
- Doesn't read DynamORM struct tags
- ConfigureMultiTenant() creates hardcoded GSIs

**Required Fix:**
- DELETE ENTIRELY or
- Strip down to just basic table creation like LiftTable
- NO GSI creation
- NO multi-tenant "magic"

### Priority 2: Constructs Using Wrong Tables

#### 3. `/pkg/cdk/constructs/dynamorm_crud_api.go`
**Uses:** `DynamORMTable`
**Problem:** Assumes wrong table structure for CRUD
**Fix:** Use simple table creation

#### 4. `/pkg/cdk/constructs/dynamorm_stream_processor.go`
**Uses:** `DynamORMTable`
**Problem:** Assumes wrong table/GSI structure
**Fix:** Use simple table creation

#### 5. `/pkg/cdk/constructs/dynamorm_event_store.go`
**Uses:** `DynamORMTable` for event and snapshot tables
**Problem:** Wrong key structure
**Fix:** Create simple tables

#### 6. `/pkg/cdk/constructs/dynamorm_cache.go`
**Uses:** DynamoDB for caching
**Problem:** Wrong key patterns
**Fix:** Simple pk/sk table

#### 7. `/pkg/cdk/constructs/connection_table.go`
**Purpose:** WebSocket connections
**Problem:** Complex key structure that doesn't match DynamORM
**Fix:** Simplify to pk/sk

#### 8. `/pkg/cdk/constructs/idempotency_table.go`
**Purpose:** Idempotency tracking
**Problem:** Wrong structure
**Fix:** Simple pk/sk table

#### 9. `/pkg/cdk/constructs/ratelimited.go` & `ratelimited_integration.go`
**Purpose:** Rate limiting
**Problem:** Custom table structure
**Fix:** Use DynamORM-compatible structure

### Priority 3: Patterns Using Broken Constructs

#### 10. `/pkg/cdk/patterns/lift_app.go`
**Uses:** `LiftTable` with `EnableMultiTenant`
**Problem:** Creates tables that don't work with DynamORM
**Fix:** Use corrected LiftTable

#### 11. `/pkg/cdk/patterns/event_driven_api.go`
**Uses:** Multiple broken table constructs
**Fix:** Update to use simple tables

#### 12. `/pkg/cdk/patterns/event_orchestrator.go`
**Uses:** Event tables with wrong structure
**Fix:** Simple tables

#### 13. `/pkg/cdk/patterns/secure_api.go`
**Uses:** Various table constructs
**Fix:** Update to simple tables

#### 14. `/pkg/cdk/patterns/tenant_management.go`
**Purpose:** Multi-tenant patterns
**Problem:** Doesn't understand DynamORM multi-tenancy
**Fix:** Remove or simplify

### Priority 4: Stacks

#### 15. `/pkg/cdk/stacks/multi_tenant_saas.go`
**Uses:** Broken multi-tenant patterns
**Fix:** Update to use DynamORM patterns

#### 16. `/pkg/cdk/stacks/event_driven.go`
**Uses:** Event table constructs
**Fix:** Simple tables

#### 17. `/pkg/cdk/stacks/microservice.go`
**Uses:** Various patterns
**Fix:** Update patterns

### Priority 5: Tests
All `*_test.go` files need updating after fixing constructs

## Correct DynamORM Table Pattern

### What CDK Should Create:
```go
// ONLY THIS - nothing more
table := awsdynamodb.NewTable(scope, jsii.String("Table"), &awsdynamodb.TableProps{
    TableName:    jsii.String("my-table"),
    PartitionKey: &awsdynamodb.Attribute{
        Name: jsii.String("pk"),
        Type: awsdynamodb.AttributeType_STRING,
    },
    SortKey: &awsdynamodb.Attribute{
        Name: jsii.String("sk"),
        Type: awsdynamodb.AttributeType_STRING,
    },
    BillingMode: awsdynamodb.BillingMode_PAY_PER_REQUEST,
    // Optional: streams, TTL, etc.
})
```

### What DynamORM Models Define:
```go
type User struct {
    // Keys
    PK string `dynamorm:"pk" json:"-"`
    SK string `dynamorm:"sk" json:"-"`
    
    // GSI definitions in the model
    TenantID   string `dynamorm:"index:tenant-index,pk" json:"tenant_id"`
    EntityType string `dynamorm:"index:tenant-index,sk" json:"entity_type"`
    CreatedAt  string `dynamorm:"index:created-index,pk" json:"created_at"`
    
    // Fields
    UserID    string `json:"user_id"`
    AccountID string `json:"account_id"`
}
```

## Implementation Order

1. **Fix LiftTable** - Remove GSI creation
2. **Delete/Gut DynamORMTable** - Too complex to fix
3. **Create SimpleDynamORMTable** - Just pk/sk creation
4. **Update all constructs** - Use simple table
5. **Update patterns** - Remove "smart" logic
6. **Update stacks** - Use fixed patterns
7. **Fix tests** - Test correct behavior

## Success Criteria
- Tables only have `pk` and `sk` attributes
- NO hardcoded GSIs in CDK
- DynamORM models define all structure through struct tags
- Multi-tenant handled by data patterns, not infrastructure