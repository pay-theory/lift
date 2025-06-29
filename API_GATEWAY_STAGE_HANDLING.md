# API Gateway Stage Handling in Lift

## Overview

When using AWS API Gateway HTTP API v2 with custom domains and base path mappings, the stage name may be included in the request path sent to your Lambda function. As of this update, Lift now automatically handles this scenario by detecting and stripping the stage prefix from the path.

## The Issue

### Before the Fix
When using custom domains with base path mapping:
- **API Gateway Route**: `ANY /v1/customers`
- **Path Sent to Lambda**: `/paytheorystudy/v1/customers` (includes stage)
- **Lift Route Required**: `/paytheorystudy/v1/customers` (had to match exactly)

This forced developers to include stage prefixes in their route definitions:
```go
// Had to do this (ugly and breaks local development)
stage := os.Getenv("STAGE")
app.POST("/"+stage+"/v1/customers", handler)
```

### After the Fix
Lift now automatically strips the stage prefix:
- **API Gateway Route**: `ANY /v1/customers`
- **Path Sent to Lambda**: `/paytheorystudy/v1/customers` (includes stage)
- **Path After Processing**: `/v1/customers` (stage stripped)
- **Lift Route Required**: `/v1/customers` (clean and simple)

Now you can write clean routes:
```go
// This now works correctly
app.POST("/v1/customers", handler)
```

## How It Works

The API Gateway v2 adapter now:
1. Extracts the stage from `requestContext.stage`
2. Checks if the path starts with `/{stage}`
3. Strips the stage prefix if present
4. Handles special cases like `$default` stage

## When This Applies

Stage prefix stripping occurs when:
- Using API Gateway HTTP API v2
- Using custom domains with base path mapping
- API Gateway includes the stage in the path

Stage prefix stripping does NOT occur when:
- The stage is `$default`
- The path doesn't start with the stage prefix
- The stage is empty or missing

## Examples

### Custom Domain with Stage
```json
{
  "requestContext": {
    "stage": "paytheorystudy",
    "http": {
      "path": "/paytheorystudy/v1/customers"
    }
  }
}
```
**Result**: Path becomes `/v1/customers`

### Direct API Gateway URL
```json
{
  "requestContext": {
    "stage": "prod",
    "http": {
      "path": "/users"
    }
  }
}
```
**Result**: Path remains `/users` (no change needed)

### $default Stage
```json
{
  "requestContext": {
    "stage": "$default",
    "http": {
      "path": "/api/data"
    }
  }
}
```
**Result**: Path remains `/api/data` (no stripping for $default)

## Testing

Your routes work the same way across all environments:
```bash
# Production (with custom domain and stage)
curl https://api.mockery.cloud/v1/customers

# Direct API Gateway URL
curl https://xyz.execute-api.us-east-1.amazonaws.com/paytheorystudy/v1/customers

# Local development
curl http://localhost:8080/v1/customers
```

All three requests route to the same handler defined as:
```go
app.POST("/v1/customers", createCustomerHandler)
```

## Migration

If you were working around this issue by including stage prefixes in your routes:

### Old Code (Remove)
```go
stage := os.Getenv("STAGE")
app.POST("/"+stage+"/v1/customers", handler)
app.GET("/"+stage+"/v1/customers/:id", handler)
```

### New Code (Use)
```go
app.POST("/v1/customers", handler)
app.GET("/v1/customers/:id", handler)
```

## API Gateway v1 vs v2

- **API Gateway v1 (REST API)**: Never includes stage in path
- **API Gateway v2 (HTTP API)**: May include stage when using custom domains

This fix ensures consistent behavior regardless of API Gateway version or configuration.