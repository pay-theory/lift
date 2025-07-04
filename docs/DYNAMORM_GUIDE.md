# DynamORM Integration Guide

**DynamORM is Pay Theory's Go library for DynamoDB operations, providing type-safe queries and single-table design patterns.**

## Overview

DynamORM is the recommended way to interact with DynamoDB in Lift applications. It provides:
- Type-safe query building
- Automatic marshaling/unmarshaling
- Single-table design support
- Built-in pagination
- Transaction support
- Global secondary index management

## Installation

```bash
go get github.com/pay-theory/dynamorm
```

## Basic Setup with Lift

### 1. Configure DynamORM in Your Lift App

```go
package main

import (
    "os"
    
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
)

var db *dynamorm.Client

func init() {
    // Create AWS session
    sess := session.Must(session.NewSession(&aws.Config{
        Region: aws.String(os.Getenv("AWS_REGION")),
    }))
    
    // Initialize DynamORM client
    db = dynamorm.NewClient(dynamorm.Config{
        Session:   sess,
        TableName: os.Getenv("DYNAMODB_TABLE"),
    })
}

func main() {
    app := lift.New()
    
    // Configure app
    config := &lift.Config{
        LogLevel:       "INFO",
        MetricsEnabled: true,
    }
    app.WithConfig(config)
    
    // Add middleware
    app.Use(
        middleware.RequestID(),
        middleware.Logger(),
        middleware.Recover(),
    )
    
    // Routes
    app.GET("/users/:id", GetUser)
    app.POST("/users", CreateUser)
    app.GET("/users", ListUsers)
    
    lambda.Start(app.HandleRequest)
}
```

### 2. Define Your Models

```go
// User model with DynamORM tags
type User struct {
    // Partition key
    ID string `dynamorm:"id,pk"`
    
    // Sort key (for single-table design)
    SK string `dynamorm:"sk"`
    
    // Attributes
    Email     string    `dynamorm:"email" validate:"required,email"`
    Name      string    `dynamorm:"name" validate:"required"`
    TenantID  string    `dynamorm:"tenant_id"`
    CreatedAt time.Time `dynamorm:"created_at"`
    UpdatedAt time.Time `dynamorm:"updated_at"`
    
    // GSI attributes
    EmailGSI string `dynamorm:"email_gsi,gsi:email-index:pk"`
}

// Constructor for consistent SK format
func NewUser(id, tenantID string) *User {
    return &User{
        ID:       id,
        SK:       fmt.Sprintf("USER#%s", id),
        TenantID: tenantID,
    }
}
```

### 3. Implement Handlers

```go
// Create user with DynamORM
func CreateUser(ctx *lift.Context) error {
    var req struct {
        Email string `json:"email" validate:"required,email"`
        Name  string `json:"name" validate:"required"`
    }
    
    if err := ctx.ParseRequest(&req); err != nil {
        return lift.ValidationError(err.Error())
    }
    
    // Create user with tenant context
    user := NewUser(generateID(), ctx.TenantID())
    user.Email = req.Email
    user.Name = req.Name
    user.EmailGSI = req.Email // For GSI queries
    user.CreatedAt = time.Now()
    user.UpdatedAt = time.Now()
    
    // Save to DynamoDB
    if err := db.Put(user).Execute(); err != nil {
        ctx.Logger.Error("Failed to create user", "error", err)
        return lift.NewLiftError("DATABASE_ERROR", "Failed to create user", 500)
    }
    
    ctx.Status(201)
    return ctx.JSON(user)
}

// Get user by ID
func GetUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    
    var user User
    err := db.Get(&user).
        Key("id", userID).
        Key("sk", fmt.Sprintf("USER#%s", userID)).
        Execute()
    
    if err == dynamorm.ErrNotFound {
        return lift.NotFound("user not found")
    }
    if err != nil {
        ctx.Logger.Error("Failed to get user", "error", err)
        return lift.NewLiftError("DATABASE_ERROR", "Failed to retrieve user", 500)
    }
    
    // Verify tenant access
    if user.TenantID != ctx.TenantID() {
        return lift.AuthorizationError("access denied")
    }
    
    return ctx.JSON(user)
}

// List users with pagination
func ListUsers(ctx *lift.Context) error {
    tenantID := ctx.TenantID()
    
    // Parse pagination params
    limit := 20
    if l := ctx.Query("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
            limit = parsed
        }
    }
    
    cursor := ctx.Query("cursor")
    
    // Query users by tenant
    query := db.Query(&User{}).
        KeyCondition("tenant_id = :tenant", dynamorm.AttributeValue{
            ":tenant": tenantID,
        }).
        Limit(limit)
    
    if cursor != "" {
        query = query.StartKey(cursor)
    }
    
    var users []User
    result, err := query.Execute(&users)
    if err != nil {
        ctx.Logger.Error("Failed to list users", "error", err)
        return lift.NewLiftError("DATABASE_ERROR", "Failed to list users", 500)
    }
    
    response := map[string]interface{}{
        "users": users,
        "count": len(users),
    }
    
    if result.LastEvaluatedKey != "" {
        response["next_cursor"] = result.LastEvaluatedKey
    }
    
    return ctx.JSON(response)
}
```

## Advanced Patterns

### Single-Table Design

DynamORM excels at single-table design patterns:

```go
// Base entity for single-table design
type Entity struct {
    PK string `dynamorm:"pk,pk"`
    SK string `dynamorm:"sk,sk"`
}

// Order model
type Order struct {
    Entity
    OrderID     string    `dynamorm:"order_id"`
    UserID      string    `dynamorm:"user_id"`
    TenantID    string    `dynamorm:"tenant_id"`
    Total       float64   `dynamorm:"total"`
    Status      string    `dynamorm:"status"`
    CreatedAt   time.Time `dynamorm:"created_at"`
}

// Access patterns:
// 1. Get order by ID: PK=ORDER#{orderID}, SK=ORDER#{orderID}
// 2. Get user orders: PK=USER#{userID}, SK=ORDER#{orderID}
// 3. Get tenant orders: GSI with PK=TENANT#{tenantID}, SK=ORDER#{timestamp}

func NewOrder(orderID, userID, tenantID string) *Order {
    return &Order{
        Entity: Entity{
            PK: fmt.Sprintf("ORDER#%s", orderID),
            SK: fmt.Sprintf("ORDER#%s", orderID),
        },
        OrderID:   orderID,
        UserID:    userID,
        TenantID:  tenantID,
        CreatedAt: time.Now(),
    }
}

// Also create user-order relationship
func CreateOrderWithRelationships(order *Order) error {
    // Use transaction for consistency
    tx := db.Transaction()
    
    // Main order record
    tx.Put(order)
    
    // User-order relationship
    tx.Put(&Entity{
        PK: fmt.Sprintf("USER#%s", order.UserID),
        SK: fmt.Sprintf("ORDER#%s#%s", order.CreatedAt.Format(time.RFC3339), order.OrderID),
    })
    
    // Execute transaction
    return tx.Execute()
}
```

### Batch Operations

```go
func BatchCreateUsers(ctx *lift.Context) error {
    var req struct {
        Users []struct {
            Email string `json:"email" validate:"required,email"`
            Name  string `json:"name" validate:"required"`
        } `json:"users" validate:"required,dive"`
    }
    
    if err := ctx.ParseRequest(&req); err != nil {
        return lift.ValidationError(err.Error())
    }
    
    // Prepare batch write
    batch := db.BatchWrite()
    
    users := make([]*User, 0, len(req.Users))
    for _, u := range req.Users {
        user := NewUser(generateID(), ctx.TenantID())
        user.Email = u.Email
        user.Name = u.Name
        user.CreatedAt = time.Now()
        
        batch.Put(user)
        users = append(users, user)
    }
    
    // Execute batch (automatically handles 25-item limit)
    if err := batch.Execute(); err != nil {
        ctx.Logger.Error("Batch create failed", "error", err)
        return lift.NewLiftError("DATABASE_ERROR", "Failed to create users", 500)
    }
    
    return ctx.JSON(map[string]interface{}{
        "created": len(users),
        "users":   users,
    })
}
```

### Complex Queries with GSI

```go
// Find user by email using GSI
func GetUserByEmail(ctx *lift.Context) error {
    email := ctx.Query("email")
    if email == "" {
        return lift.ValidationError("email parameter required")
    }
    
    var users []User
    err := db.Query(&User{}).
        Index("email-index").
        KeyCondition("email_gsi = :email", dynamorm.AttributeValue{
            ":email": email,
        }).
        Execute(&users)
    
    if err != nil {
        ctx.Logger.Error("Failed to query by email", "error", err)
        return lift.NewLiftError("DATABASE_ERROR", "Query failed", 500)
    }
    
    if len(users) == 0 {
        return lift.NotFound("user not found")
    }
    
    // Verify tenant access
    user := users[0]
    if user.TenantID != ctx.TenantID() {
        return lift.AuthorizationError("access denied")
    }
    
    return ctx.JSON(user)
}
```

### Transactions

```go
func TransferCredits(ctx *lift.Context) error {
    var req struct {
        FromUserID string  `json:"from_user_id" validate:"required"`
        ToUserID   string  `json:"to_user_id" validate:"required"`
        Amount     float64 `json:"amount" validate:"required,gt=0"`
    }
    
    if err := ctx.ParseRequest(&req); err != nil {
        return lift.ValidationError(err.Error())
    }
    
    // Use transaction for atomic transfer
    err := db.TransactWrite().
        Update(&User{ID: req.FromUserID}).
            Set("credits", ":newCredits").
            Condition("credits >= :amount").
            Values(dynamorm.AttributeValue{
                ":newCredits": dynamorm.Subtract("credits", req.Amount),
                ":amount":     req.Amount,
            }).
        Update(&User{ID: req.ToUserID}).
            Set("credits", ":newCredits").
            Values(dynamorm.AttributeValue{
                ":newCredits": dynamorm.Add("credits", req.Amount),
            }).
        Execute()
    
    if err != nil {
        if dynamorm.IsConditionFailed(err) {
            return lift.ValidationError("insufficient credits")
        }
        return lift.NewLiftError("TRANSFER_ERROR", "Transfer failed", 500)
    }
    
    return ctx.JSON(map[string]string{
        "status": "success",
        "message": fmt.Sprintf("Transferred %.2f credits", req.Amount),
    })
}
```

## Testing with DynamORM

Use DynamoDB Local for testing:

```go
func TestCreateUser(t *testing.T) {
    // Set up test database
    testDB := setupTestDB(t)
    defer teardownTestDB(t)
    
    // Create test context
    ctx := lifttesting.NewTestContext(
        lifttesting.WithMethod("POST"),
        lifttesting.WithPath("/users"),
        lifttesting.WithBody(`{"email":"test@example.com","name":"Test User"}`),
        lifttesting.WithTenantID("test-tenant"),
    )
    
    // Override global db for test
    originalDB := db
    db = testDB
    defer func() { db = originalDB }()
    
    // Execute handler
    err := CreateUser(ctx)
    assert.NoError(t, err)
    assert.Equal(t, 201, ctx.Response.StatusCode)
    
    // Verify in database
    var user User
    err = testDB.Get(&user).
        Key("email_gsi", "test@example.com").
        Index("email-index").
        Execute()
    
    assert.NoError(t, err)
    assert.Equal(t, "Test User", user.Name)
    assert.Equal(t, "test-tenant", user.TenantID)
}
```

## Best Practices

### 1. Use Middleware for Database Injection

```go
// DynamORM middleware for dependency injection
func DynamORMMiddleware(client *dynamorm.Client) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            ctx.Set("db", client)
            return next.Handle(ctx)
        })
    }
}

// In handler
func GetUser(ctx *lift.Context) error {
    db := ctx.Get("db").(*dynamorm.Client)
    // Use db...
}
```

### 2. Consistent Error Handling

```go
func handleDynamoError(err error, operation string) error {
    switch {
    case err == nil:
        return nil
    case err == dynamorm.ErrNotFound:
        return lift.NotFound("resource not found")
    case dynamorm.IsConditionFailed(err):
        return lift.ValidationError("condition check failed")
    case dynamorm.IsThrottled(err):
        return lift.NewLiftError("RATE_LIMITED", "Too many requests", 429)
    default:
        return lift.NewLiftError("DATABASE_ERROR", 
            fmt.Sprintf("%s failed", operation), 500)
    }
}
```

### 3. Optimize for Performance

```go
// Use projection to reduce data transfer
func ListUserSummaries(ctx *lift.Context) error {
    var users []User
    err := db.Query(&User{}).
        KeyCondition("tenant_id = :tenant", dynamorm.AttributeValue{
            ":tenant": ctx.TenantID(),
        }).
        Project("id", "name", "email"). // Only fetch needed fields
        Execute(&users)
    
    if err != nil {
        return handleDynamoError(err, "list users")
    }
    
    return ctx.JSON(users)
}

// Use consistent read only when necessary
func GetUserForUpdate(ctx *lift.Context) error {
    var user User
    err := db.Get(&user).
        Key("id", ctx.Param("id")).
        ConsistentRead(true). // Ensures latest data
        Execute()
    
    // ... rest of handler
}
```

### 4. Implement Caching

```go
// Cache frequently accessed data
func GetUserWithCache(ctx *lift.Context) error {
    userID := ctx.Param("id")
    cacheKey := fmt.Sprintf("user:%s", userID)
    
    // Check cache first
    if cached := cache.Get(cacheKey); cached != nil {
        return ctx.JSON(cached)
    }
    
    // Fetch from DynamoDB
    var user User
    err := db.Get(&user).
        Key("id", userID).
        Execute()
    
    if err != nil {
        return handleDynamoError(err, "get user")
    }
    
    // Cache for 5 minutes
    cache.Set(cacheKey, user, 5*time.Minute)
    
    return ctx.JSON(user)
}
```

## Migration from Raw DynamoDB SDK

If you're migrating from the raw AWS SDK:

```go
// Before (AWS SDK)
input := &dynamodb.GetItemInput{
    TableName: aws.String("users"),
    Key: map[string]*dynamodb.AttributeValue{
        "id": {S: aws.String(userID)},
    },
}
result, err := svc.GetItem(input)

// After (DynamORM)
var user User
err := db.Get(&user).Key("id", userID).Execute()
```

## Resources

- [DynamORM Documentation](https://github.com/pay-theory/dynamorm)
- [DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [Single-Table Design](https://www.alexdebrie.com/posts/dynamodb-single-table/)

---

DynamORM provides the perfect complement to Lift for building scalable serverless applications with DynamoDB. Its type-safe queries and single-table design support make it ideal for multi-tenant SaaS applications.