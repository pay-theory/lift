# Rate Limiting with Limited Library

This example demonstrates how to properly use the `pay-theory/limited` library with Lift for distributed rate limiting using DynamoDB.

## Why This Approach?

The built-in Lift rate limiting middleware (`middleware.RateLimitMiddleware`) requires a `DynamORMWrapper` that has no public constructor. This example shows how to use the `limited` library directly, which properly implements DynamORM integration.

## Key Features

- Uses `limited` library with proper DynamORM integration
- Multiple rate limiting strategies for different endpoints
- IP-based limiting for public endpoints
- User-based limiting for authenticated endpoints
- Proper rate limit headers (X-RateLimit-*)
- Graceful fallback on DynamoDB errors

## Setup

1. Add the limited dependency to your go.mod:
```bash
go get github.com/pay-theory/limited
```

2. Create a DynamoDB table for rate limits:
```bash
aws dynamodb create-table \
  --table-name rate-limits \
  --attribute-definitions \
    AttributeName=pk,AttributeType=S \
    AttributeName=sk,AttributeType=S \
  --key-schema \
    AttributeName=pk,KeyType=HASH \
    AttributeName=sk,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST
```

3. Configure AWS credentials and region

## Usage

The example shows three different rate limiting strategies:

### Public Endpoints (IP-based)
- 1000 requests per hour
- Rate limited by IP address
- Applied to `/public/*` endpoints

### API Endpoints (User-based)
- 100 requests per 15 minutes
- Rate limited by authenticated user ID
- Applied to `/api/*` endpoints

### Expensive Operations
- 10 requests per hour
- Stricter limits for resource-intensive operations
- Applied to specific endpoints like `/api/expensive-operation`

## Implementation Details

1. **DynamoDB Connection**: Uses `core.NewDB()` from DynamORM directly
2. **Rate Limit Keys**: Generated based on context (IP, User ID, Tenant ID)
3. **Headers**: Automatically sets standard rate limit headers
4. **Error Handling**: Allows requests through on DynamoDB errors (fail open)

## Example Response Headers

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 99
X-RateLimit-Reset: 1703001600
```

On rate limit exceeded:
```
HTTP/1.1 429 Too Many Requests
Retry-After: 60
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1703001600

{
  "error": "Rate limit exceeded",
  "limit": 100,
  "remaining": 0,
  "reset_at": 1703001600,
  "retry_after": 60
}
```

## Local Testing

For local testing with DynamoDB Local:

```go
db, err := core.NewDB(core.Config{
    Region:    "us-east-1",
    TableName: "rate-limits",
    Endpoint:  "http://localhost:8000",
})
```

## Production Considerations

1. **Table Design**: The `limited` library handles the table schema
2. **TTL**: Configure DynamoDB TTL on the appropriate attribute
3. **Monitoring**: Use CloudWatch metrics to monitor rate limit hits
4. **Costs**: Consider DynamoDB costs for high-traffic applications

## Alternative Strategies

The `limited` library supports multiple strategies:

- `NewFixedWindowStrategy`: Fixed time windows (shown in example)
- `NewSlidingWindowStrategy`: Sliding time windows
- `NewTokenBucketStrategy`: Token bucket algorithm
- `NewLeakyBucketStrategy`: Leaky bucket algorithm

Choose based on your specific requirements.