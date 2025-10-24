# Rate Limiting with the Limited Library

Lift ships with first-class support for the [`limited`](https://github.com/pay-theory/limited) rate limiting library. This guide walks through everything you need to build a production-ready limiter backed by DynamoDB using Lift’s middleware.

## When to Use the Limited Integration

Use the Limited-backed middleware when you need:
- **Persistent, distributed quotas** that survive cold starts and scale across Lambda instances.
- **Automatic scoping** that prefers per-user limits, then tenant, and finally caller IP.
- **Standard HTTP feedback** (the middleware sets `X-RateLimit-*` and `Retry-After` headers).
- **Graceful degradation**—Lift fails open if the limiter cannot record usage (by design, requests are allowed and errors are logged).

Stick with simpler in-memory guards only for purely internal traffic or smoke tests.

## Prerequisites

1. **AWS credentials and region** – the middleware reads `AWS_REGION` or `AWS_DEFAULT_REGION`.
2. **DynamoDB table** the limiter can write to (pay-per-request billing is recommended).
3. **IAM permissions** giving the Lambda function `dynamodb:GetItem`, `PutItem`, `UpdateItem`, and `DeleteItem` on the table.
4. **Dependencies** in your Go module:
   ```bash
   go get github.com/pay-theory/limited@latest
   go get github.com/pay-theory/dynamorm@latest
   go get go.uber.org/zap@latest
   ```

> **Tip:** If you deploy with the Lift CDK constructs, the `RateLimitedFunction` pattern creates the table, injects the environment variables, and grants IAM permissions automatically.

## Step 1 – Create the DynamoDB Table

Provision a schema compatible with Limited. The simplest option uses `PK`/`SK` keys and TTL for cleanup:

```bash
aws dynamodb create-table \
  --table-name rate-limits \
  --attribute-definitions \
    AttributeName=PK,AttributeType=S \
    AttributeName=SK,AttributeType=S \
  --key-schema \
    AttributeName=PK,KeyType=HASH \
    AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --table-class STANDARD
```

Set the table name on the function with `RATE_LIMIT_TABLE_NAME` if you want to override Limited’s default (`rate-limits`). TTL should point at the item attribute you configure inside Limited (`expires_at` is the default).

## Step 2 – Configure the Environment

Ensure your Lambda (or local process) exposes:

| Variable | Purpose |
| --- | --- |
| `AWS_REGION` (or `AWS_DEFAULT_REGION`) | Region used when opening DynamoDB sessions |
| `RATE_LIMIT_TABLE_NAME` (optional) | Overrides the table name if you do not use the default `rate-limits` |

If you run against DynamoDB Local, supply `LIMITED_ENDPOINT` (or set `LimitedConfig.Endpoint`) so the middleware connects to the local emulator.

## Step 3 – Initialize the Limited Middleware

The core entry point is `middleware.LimitedRateLimit`, which returns a standard Lift middleware. It accepts a `LimitedConfig` struct:

```go
import (
    "time"

    "github.com/pay-theory/lift/pkg/middleware"
    "go.uber.org/zap"
)

func initLimiter() (lift.Middleware, error) {
    logger, _ := zap.NewProduction()

    return middleware.LimitedRateLimit(middleware.LimitedConfig{
        Logger:    logger,         // falls back to zap.NewNop() if nil
        Region:    "us-east-1",    // REQUIRED unless AWS_REGION is set
        TableName: "rate-limits",  // default if omitted
        Strategy:  "sliding",      // "sliding" or any other value for fixed window
        Window:    time.Minute,    // default: 1 hour
        Limit:     500,            // default: 1000
    })
}
```

Attach the middleware when you set up the Lift app:

```go
app := lift.New()

rateLimiter, err := initLimiter()
if err != nil {
    panic(err)
}

app.Use(rateLimiter) // global safeguard
```

The middleware will:
1. Connect to DynamoDB through the lightweight `dynamorm.NewBasic` client.
2. Create a Limited strategy (`fixed` window by default, `sliding` when requested).
3. Generate a composite key (`user:{id}`, `tenant:{id}`, or `ip:{address}`) based on the current Lift context.
4. Call `CheckAndIncrement`, emit standard headers, and return a JSON 429 when the quota is exhausted.

## Step 4 – Target Specific Routes

You can mount the limiter on selected route groups instead of globally:

```go
public := app.Group("/public")
if limiter, err := middleware.IPRateLimitWithLimited(25, time.Minute); err == nil {
    public.Use(limiter)
}
public.POST("/login", handleLogin)
public.POST("/signup", handleSignup)

api := app.Group("/api")
if limiter, err := middleware.UserRateLimitWithLimited(1_000, time.Hour); err == nil {
    api.Use(limiter)
}
api.GET("/orders", listOrders)
```

The helper constructors (`IPRateLimitWithLimited`, `UserRateLimitWithLimited`, `TenantRateLimitWithLimited`) look up `AWS_REGION`, keep sensible defaults, and call `LimitedRateLimit` under the hood.

## How Key Generation Works

The middleware never asks you to define a key function. It enforces the following priority:
1. If `ctx.UserID()` is non-empty → `user:{id}`.
2. Else, if `ctx.TenantID()` is non-empty → `tenant:{id}`.
3. Else it falls back to the caller IP (using `X-Forwarded-For`, then `X-Real-IP`, then `unknown`).

Each decision also records metadata (`user_id`, `tenant_id`, or `ip`) so the DynamoDB record retains context.

If you need a fully custom key (e.g., API key + endpoint), wrap the middleware:

```go
rateLimiter, _ := middleware.LimitedRateLimit(middleware.LimitedConfig{Region: "us-east-1"})

app.Use(func(next lift.Handler) lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        // Stash custom identifiers before Limited runs
        ctx.Set("custom_rate_key", fmt.Sprintf("apikey:%s", ctx.Header("X-API-Key")))
        return next.Handle(ctx)
    })
})

app.Use(rateLimiter)
```

In this pattern, you intercept the `generateKey` function by populating `ctx.UserID()`/`TenantID()` or by modifying headers before the limiter executes.

## Choosing a Strategy

| Strategy | Config Value | Behavior |
| --- | --- | --- |
| Fixed window | any value other than `"sliding"` | Resets counters every `Window` duration. Cheaper but allows bursts at window edges. |
| Sliding window | `"sliding"` | More precise: counts rolling requests using Limited’s sliding-window implementation. Set `Window/10` granularity automatically. |

Tune `Window` and `Limit` per route:

```go
highPriority, _ := middleware.LimitedRateLimit(middleware.LimitedConfig{
    Region:   os.Getenv("AWS_REGION"),
    Strategy: "sliding",
    Window:   5 * time.Minute,
    Limit:    250,
})
```

## Deployment with the Lift CDK

Use the `RateLimitedFunction` construct to apply rate limiting without hand wiring:

```ts
import { RateLimitedFunction, RateLimitType } from "@pay-theory/lift/cdk";

new RateLimitedFunction(this, "OrdersFn", {
  RateLimitType: RateLimitType.USER,
  WindowSeconds: 900,
  Limit: 500,
  LiftFunctionProps: {
    FunctionProps: { /* standard Lambda props */ },
  },
});
```

The construct:
- Creates or reuses a DynamoDB table.
- Injects `RATE_LIMIT_TABLE_NAME`, `AWS_REGION`, and Limited configuration into the Lambda environment.
- Grants read/write permissions on the table.
- Optionally sets up CloudWatch alarms for throttling metrics.

## Local & Integration Testing

1. **DynamoDB Local** – run `docker run -p 8000:8000 amazon/dynamodb-local` and configure the limiter with an endpoint:
   ```go
   limiter, _ := middleware.LimitedRateLimit(middleware.LimitedConfig{
       Region:   "us-east-1",
       Endpoint: "http://localhost:8000",
   })
   ```
2. **Integration tests** – spin up the middleware and issue more requests than the limit; inspect the response status and headers:
   ```go
   ctx := lifttesting.NewTestContext("GET", "/api/orders")
   limiter, _ := middleware.IPRateLimitWithLimited(2, time.Second)
   ```
3. **Reset state** – delete items from the table or use the forthcoming `Reset` APIs in Limited when testing quotas.

## Observability & Operations

- **Logging** – Limited errors are logged via Zap (`Rate limit check failed`). Ensure your logging pipeline captures them.
- **Metrics** – Add your own CloudWatch counters around 429 responses or adopt the CDK alarm helpers (`AddRateLimitAlarm`).
- **Fail-open behavior** – If DynamoDB is throttled or unreachable, Lift logs the error and continues handling the request. Consider combining rate limiting with AWS WAF IP throttling for defense-in-depth.
- **Cleanup** – DynamoDB TTL removes expired entries automatically. Keep the TTL attribute aligned with the table definition.

## Troubleshooting

| Symptom | Likely Cause | Fix |
| --- | --- | --- |
| Middleware returns an error during startup | Missing `AWS_REGION` or table misconfiguration | Set the environment variable or pass `Region` explicitly; confirm table exists |
| Requests never get limited | Table name mismatch or IAM denies writes | Set `RATE_LIMIT_TABLE_NAME` to the deployed table and verify the execution role policies |
| All requests reported as coming from `ip:unknown` | Missing proxy headers | Ensure API Gateway forwards `X-Forwarded-For`/`X-Real-IP`, or enrich the Lift context before the limiter |
| Sudden surge of 429s | Legitimate traffic spike or bugged client | Check CloudWatch metrics, confirm limit values, and communicate limits to clients |

## Next Steps

- Explore `examples/rate-limiting` for multi-tenant end-to-end samples.
- Combine Limited with Lift’s authentication middleware so the limiter consistently sees `ctx.UserID()` and `ctx.TenantID()`.
- Layer AWS WAF or API Gateway throttling for emergency “kill switches” in front of Lift.

With these pieces in place, you have a robust, DynamoDB-backed rate limiter that plays nicely with Lift’s middleware pipeline and deployment tooling.
