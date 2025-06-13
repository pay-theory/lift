# Rate Limiting Example

This example demonstrates how to implement comprehensive rate limiting in a Lift application using the Pay Theory Limited library with DynamoDB backend.

## Features

- **Multiple Rate Limiting Strategies**
  - Fixed window
  - Sliding window
  - Multi-window

- **Various Rate Limiting Scopes**
  - Global/endpoint rate limiting
  - Tenant-based rate limiting
  - User-based rate limiting
  - IP-based rate limiting
  - Custom composite keys

- **Multi-tenant Support**
  - Different limits for different tenant tiers
  - Tenant isolation
  - Automatic tenant context

## Setup

### 1. DynamoDB Table

The rate limiter requires a DynamoDB table. You can create it using AWS CLI:

```bash
aws dynamodb create-table \
  --table-name rate_limits \
  --attribute-definitions \
    AttributeName=PK,AttributeType=S \
    AttributeName=SK,AttributeType=S \
  --key-schema \
    AttributeName=PK,KeyType=HASH \
    AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST
```

### 2. Environment Variables

```bash
export AWS_REGION=us-east-1
export RATE_LIMIT_TABLE=rate_limits
```

## Usage Examples

### Basic Rate Limiting

```go
package main

import (
    "time"
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/lift/pkg/dynamorm"
)

func main() {
    app := lift.New()

    // Set up DynamORM (required for rate limiting)
    app.Use(dynamorm.WithDynamORM(&dynamorm.DynamORMConfig{
        TableName: "rate_limits",
        Region:    "us-east-1",
    }))

    // Global rate limit: 1000 requests per hour
    app.Use(middleware.EndpointRateLimit(1000, time.Hour))

    // API routes
    app.GET("/api/users", handleListUsers)
    app.POST("/api/users", handleCreateUser)

    app.Start()
}
```

### Tenant-Based Rate Limiting

```go
// Different limits for different tenant tiers
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    Strategy:     "sliding_window",
    Window:       time.Hour,
    DefaultLimit: 100,  // Free tier
    TenantLimits: map[string]int{
        "basic":      500,   // Basic tier
        "premium":    2000,  // Premium tier
        "enterprise": 10000, // Enterprise tier
    },
}))
```

### IP-Based Rate Limiting for Public Endpoints

```go
// Protect public endpoints from abuse
publicAPI := app.Group("/public")
publicAPI.Use(middleware.IPRateLimit(10, time.Minute))

// These endpoints are rate limited by IP
publicAPI.POST("/signup", handleSignup)
publicAPI.POST("/login", handleLogin)
publicAPI.POST("/forgot-password", handleForgotPassword)
```

### Custom Rate Limiting

```go
// Rate limit by custom key (e.g., API key + endpoint)
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    Strategy:     "sliding_window",
    Window:       time.Minute,
    DefaultLimit: 100,
    KeyFunc: func(ctx *lift.Context) limited.RateLimitKey {
        apiKey := ctx.Header("X-API-Key")
        return limited.RateLimitKey{
            Identifier: apiKey,
            Resource:   ctx.Request.Path,
            Operation:  ctx.Request.Method,
            Metadata: map[string]string{
                "api_key":  apiKey,
                "endpoint": fmt.Sprintf("%s %s", ctx.Request.Method, ctx.Request.Path),
            },
        }
    },
}))
```

### Per-Endpoint Rate Limiting

```go
// Different limits for different endpoints
apiV1 := app.Group("/api/v1")

// Expensive operation - heavily limited
apiV1.POST("/expensive-operation", 
    middleware.RateLimit(middleware.RateLimitConfig{
        Strategy:     "fixed_window",
        Window:       time.Hour,
        DefaultLimit: 10, // Only 10 per hour
    }),
    handleExpensiveOperation,
)

// Normal operation - standard limits
apiV1.GET("/users", 
    middleware.UserRateLimit(100, time.Minute),
    handleListUsers,
)
```

## Rate Limit Headers

The middleware automatically adds rate limit headers to responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1640995200
Retry-After: 3600 (only on 429 responses)
```

## Custom Error Responses

```go
app.Use(middleware.RateLimit(middleware.RateLimitConfig{
    // ... other config ...
    ErrorHandler: func(ctx *lift.Context, decision *limited.LimitDecision) error {
        return ctx.Status(429).JSON(map[string]interface{}{
            "error": "Rate limit exceeded",
            "message": fmt.Sprintf(
                "You have exceeded the rate limit of %d requests. Please try again after %s.",
                decision.Limit,
                decision.ResetsAt.Format(time.RFC3339),
            ),
            "retry_after": decision.RetryAfter.Seconds(),
            "upgrade_url": "https://example.com/pricing",
        })
    },
}))
```

## Performance Considerations

1. **DynamoDB Capacity**: The rate limiter uses DynamoDB for storage. Ensure your table has sufficient read/write capacity.

2. **Caching**: Consider implementing a local cache for frequently accessed rate limit data to reduce DynamoDB calls.

3. **Fail Open**: By default, the rate limiter fails open (allows requests) if there's an error checking limits. This prevents service disruption but may allow some requests through during failures.

## Testing Rate Limits

```bash
# Test rate limiting with curl
for i in {1..15}; do
  curl -X GET https://api.example.com/users \
    -H "Authorization: Bearer token" \
    -H "X-Tenant-ID: test-tenant" \
    -w "\nStatus: %{http_code}, Remaining: %{header.x-ratelimit-remaining}\n"
  sleep 1
done
```

## Monitoring

Monitor rate limiting effectiveness using CloudWatch metrics:

- Rate limit hits (429 responses)
- Rate limit checks (total requests)
- DynamoDB read/write capacity
- Lambda duration with rate limiting overhead

## Best Practices

1. **Choose Appropriate Windows**: Use shorter windows (minutes) for abuse prevention and longer windows (hours/days) for quota management.

2. **Set Reasonable Limits**: Start with conservative limits and adjust based on actual usage patterns.

3. **Use Multiple Strategies**: Combine different rate limiting strategies for comprehensive protection:
   - IP-based for public endpoints
   - User-based for authenticated endpoints
   - Tenant-based for multi-tenant applications

4. **Provide Clear Error Messages**: Help users understand why they're rate limited and when they can retry.

5. **Consider Business Logic**: Different operations may need different limits:
   - Read operations: Higher limits
   - Write operations: Moderate limits
   - Expensive operations: Lower limits

## Troubleshooting

### Rate Limits Not Working

1. Ensure DynamORM middleware is added before rate limiting middleware
2. Check that the DynamoDB table exists and is accessible
3. Verify IAM permissions for DynamoDB access

### Performance Issues

1. Check DynamoDB metrics for throttling
2. Consider implementing caching for frequently accessed limits
3. Use appropriate granularity for sliding window strategy

### Incorrect Counts

1. Verify time synchronization across Lambda instances
2. Check for clock drift issues
3. Ensure consistent key generation 