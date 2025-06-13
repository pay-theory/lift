# Rate Limiting Solution - Using Pay Theory Limited

**Date**: 2025-06-12-15_32_34  
**Project Manager**: AI Assistant  
**Decision**: Use Pay Theory's Limited library for rate limiting

## Solution Overview

The [Pay Theory Limited library](https://github.com/pay-theory/limited) provides a DynamoDB-based rate limiting solution that perfectly aligns with Lift's requirements.

## Key Features

### 1. DynamORM Integration
- Built on top of Pay Theory's DynamORM
- Uses DynamoDB for distributed rate limiting
- Atomic check-and-increment operations

### 2. Multiple Strategies
- **Fixed Window**: Traditional time-based windows
- **Sliding Window**: More accurate with sub-window checks
- **Multi-Window**: Enforce multiple limits simultaneously (e.g., 100/minute AND 1000/hour)

### 3. Multi-Tenant Support
- Per-identifier limits (perfect for tenant/user isolation)
- Per-resource limits (different limits for different endpoints)
- Customizable identifier extraction

### 4. Production Features
- Fail-open option for resilience
- TTL-based cleanup
- Usage statistics tracking
- Ready-to-use HTTP middleware

## Integration Plan for Lift

### 1. Add Dependency
```go
// go.mod
require github.com/pay-theory/limited v1.0.0
```

### 2. Create Rate Limiting Middleware
```go
// pkg/middleware/ratelimit.go
package middleware

import (
    "time"
    "github.com/pay-theory/limited"
    "github.com/pay-theory/limited/middleware"
    "github.com/pay-theory/lift/pkg/lift"
)

// RateLimitConfig configures rate limiting behavior
type RateLimitConfig struct {
    // Strategy configuration
    WindowSize    time.Duration
    MaxRequests   int64
    
    // Multi-tenant configuration
    PerTenantLimits map[string]limited.Limit
    PerUserLimits   map[string]limited.Limit
    
    // Resource-based limits
    ResourceLimits  map[string]limited.Limit
    
    // Options
    FailOpen        bool
    SkipPaths       []string
}

// RateLimit creates a rate limiting middleware using Limited
func RateLimit(config RateLimitConfig, limiter *limited.DynamoRateLimiter) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Skip rate limiting for certain paths
            for _, path := range config.SkipPaths {
                if ctx.Request.Path == path {
                    return next.Handle(ctx)
                }
            }
            
            // Extract identifier (tenant + user)
            identifier := extractIdentifier(ctx)
            resource := ctx.Request.Path
            
            // Check rate limit
            decision, err := limiter.CheckAndIncrement(ctx.Context, limited.RateLimitKey{
                Identifier: identifier,
                Resource:   resource,
                Operation:  ctx.Request.Method,
            })
            
            if err != nil && !config.FailOpen {
                return lift.InternalError("Rate limiter unavailable")
            }
            
            if !decision.Allowed {
                // Add rate limit headers
                ctx.Response.Header("X-RateLimit-Limit", fmt.Sprintf("%d", decision.Limit))
                ctx.Response.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", decision.Remaining))
                ctx.Response.Header("X-RateLimit-Reset", fmt.Sprintf("%d", decision.ResetAt.Unix()))
                ctx.Response.Header("Retry-After", fmt.Sprintf("%d", int(decision.RetryAfter.Seconds())))
                
                return lift.TooManyRequests("Rate limit exceeded")
            }
            
            // Add rate limit info to response headers
            ctx.Response.Header("X-RateLimit-Limit", fmt.Sprintf("%d", decision.Limit))
            ctx.Response.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", decision.Remaining))
            
            return next.Handle(ctx)
        })
    }
}

// extractIdentifier creates a unique identifier for rate limiting
func extractIdentifier(ctx *lift.Context) string {
    tenantID := ctx.TenantID()
    userID := ctx.UserID()
    
    if tenantID != "" && userID != "" {
        return fmt.Sprintf("tenant:%s:user:%s", tenantID, userID)
    } else if tenantID != "" {
        return fmt.Sprintf("tenant:%s", tenantID)
    } else if userID != "" {
        return fmt.Sprintf("user:%s", userID)
    }
    
    // Fallback to IP address
    return fmt.Sprintf("ip:%s", ctx.Header("X-Real-IP"))
}
```

### 3. Usage in Lift Applications
```go
// In your main.go
func main() {
    app := lift.New()
    
    // Initialize DynamoDB connection (shared with DynamORM)
    db, err := dynamorm.New(dynamorm.Config{
        Region:    "us-east-1",
        TableName: "rate-limits",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Create rate limiter with multi-window strategy
    strategy := limited.NewMultiWindowStrategy([]limited.WindowConfig{
        {Duration: time.Minute, MaxRequests: 100},   // 100/minute
        {Duration: time.Hour, MaxRequests: 1000},    // 1000/hour
        {Duration: 24 * time.Hour, MaxRequests: 10000}, // 10k/day
    })
    
    limiter := limited.NewDynamoRateLimiter(
        db,
        &limited.Config{
            FailOpen: true,
            TTLHours: 24,
            // Per-tenant limits
            IdentifierLimits: map[string]limited.Limit{
                "tenant:premium": {
                    RequestsPerMinute: 1000,
                    RequestsPerHour:   10000,
                },
                "tenant:enterprise": {
                    RequestsPerMinute: 10000,
                    RequestsPerHour:   100000,
                },
            },
            // Per-resource limits
            ResourceLimits: map[string]limited.Limit{
                "/api/expensive-operation": {
                    RequestsPerMinute: 10,
                    RequestsPerHour:   100,
                },
            },
        },
        strategy,
        app.Logger,
    )
    
    // Add rate limiting middleware
    app.Use(middleware.RateLimit(middleware.RateLimitConfig{
        WindowSize:  time.Minute,
        MaxRequests: 100,
        FailOpen:    true,
        SkipPaths:   []string{"/health", "/metrics"},
    }, limiter))
    
    // Your routes...
    app.GET("/api/users", getUsersHandler)
    
    app.Start()
}
```

## Benefits of Using Limited

### 1. Immediate Solution
- No need to build rate limiting from scratch
- Already production-tested at Pay Theory
- Saves Sprint 2 development time

### 2. DynamORM Integration
- Uses the same DynamORM library Lift is already integrating
- Shared database connection
- Consistent data patterns

### 3. Multi-Tenant Ready
- Per-tenant rate limits out of the box
- Supports complex identifier strategies
- Resource-based limiting

### 4. Production Features
- Distributed rate limiting across Lambda instances
- Atomic operations prevent race conditions
- TTL-based cleanup prevents table bloat
- Fail-open option for resilience

## Updated Action Items

### Remove from TODO List
- ❌ ~~Rate Limiting Middleware implementation~~

### Add to Sprint 2
- ✅ Integrate Limited library
- ✅ Create Lift-specific middleware wrapper
- ✅ Add configuration for multi-tenant limits
- ✅ Create examples showing rate limiting usage

### Dependencies
1. Complete DynamORM integration first (Limited depends on it)
2. Ensure shared DynamoDB table strategy
3. Test with multi-tenant scenarios

## Table Setup Required

The Limited library requires a DynamoDB table:

```bash
# Create rate-limits table
aws dynamodb create-table \
    --table-name lift-rate-limits \
    --attribute-definitions \
        AttributeName=PK,AttributeType=S \
        AttributeName=SK,AttributeType=S \
    --key-schema \
        AttributeName=PK,KeyType=HASH \
        AttributeName=SK,KeyType=RANGE \
    --billing-mode PAY_PER_REQUEST

# Enable TTL
aws dynamodb update-time-to-live \
    --table-name lift-rate-limits \
    --time-to-live-specification \
        Enabled=true,AttributeName=TTL
```

## Conclusion

Using the Pay Theory Limited library is the optimal solution for rate limiting in Lift. It provides all required features, integrates seamlessly with DynamORM, and is already production-tested. This significantly reduces our development effort while providing a more robust solution than we could build in the allocated time. 