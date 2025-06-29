# Lift Framework Issue: LiftError Status Codes Not Properly Set in HTTP Responses

## ✅ RESOLVED

This issue has been **FIXED** in the latest version. The `handleError` method in `pkg/lift/app.go` now properly handles `LiftError` objects and maps their status codes to HTTP responses.

## Summary

After successfully upgrading to lift v1.0.24 and resolving the middleware execution issue, we discovered that `LiftError` objects with specific status codes were being returned as HTTP 500 instead of their intended status codes (e.g., 401, 400, 404).

## Environment

- **lift version**: v1.0.24
- **Go version**: 1.23.10
- **AWS Lambda Runtime**: provided.al2
- **Deployment**: AWS Lambda + API Gateway (REST API)

## Issue Description

When middleware or handlers return `LiftError` objects with specific status codes, the HTTP response always returns status 500 instead of the intended status code. However, the error logging and middleware execution work correctly.

## Expected Behavior

When returning a `LiftError` with a specific status code, the HTTP response should use that status code:

```go
// This should result in HTTP 401
return lift.NewLiftError("UNAUTHORIZED", "No API key provided", 401)

// This should result in HTTP 400  
return lift.NewLiftError("BAD_REQUEST", "Invalid input", 400)

// This should result in HTTP 404
return lift.NewLiftError("NOT_FOUND", "Resource not found", 404)
```

## Actual Behavior

All `LiftError` returns result in HTTP 500 responses, regardless of the specified status code.

## Evidence

### CloudWatch Logs (Correct)
The logs show the errors are being processed correctly with proper error codes:
```
2025/06/29 01:24:15 [REQUEST] GET /customers
2025/06/29 01:24:15 [ERROR] GET /customers - [UNAUTHORIZED] No API key provided (took 19.453µs)
```

### HTTP Response (Incorrect)
```bash
$ curl -v https://api.example.com/v1/customers
< HTTP/2 500 
< content-type: application/json
< content-length: 33
< 
{"error":"Internal server error"}
```

### Expected HTTP Response
```bash
$ curl -v https://api.example.com/v1/customers
< HTTP/2 401 
< content-type: application/json
< 
{"code":"UNAUTHORIZED","message":"No API key provided"}
```

## Code Examples

### Middleware Implementation
```go
func APIKeyAuth() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            authHeader := ctx.Header("Authorization")
            if authHeader == "" {
                // This should return HTTP 401, but returns HTTP 500
                return lift.NewLiftError("UNAUTHORIZED", "No API key provided", 401)
            }
            
            if !isValidAPIKey(authHeader) {
                // This should return HTTP 401, but returns HTTP 500
                return lift.NewLiftError("UNAUTHORIZED", "Invalid API key format", 401)
            }
            
            return next.Handle(ctx)
        })
    }
}
```

### Handler Implementation
```go
func GetUser(ctx *lift.Context) error {
    userID := ctx.Param("id")
    if userID == "" {
        // This should return HTTP 400, but returns HTTP 500
        return lift.NewLiftError("BAD_REQUEST", "User ID is required", 400)
    }
    
    user, err := getUserFromDB(userID)
    if err == sql.ErrNoRows {
        // This should return HTTP 404, but returns HTTP 500
        return lift.NewLiftError("NOT_FOUND", "User not found", 404)
    }
    
    return ctx.JSON(user)
}
```

## Verification Steps

1. **Middleware is executing correctly**: CloudWatch logs show proper error codes and messages
2. **CORS headers are set**: Response includes proper CORS headers from middleware
3. **Request ID headers are set**: Response includes `X-Request-ID` from middleware
4. **Error messages are processed**: Logs show the exact error codes and messages from `LiftError`

This confirms that:
- ✅ Middleware execution is working (fixed in v1.0.24)
- ✅ Error creation and logging is working
- ❌ HTTP status code mapping is not working

## Impact

- **API consumers receive misleading status codes**: All errors appear as server errors (500) instead of client errors (4xx)
- **Error handling complexity**: Clients cannot distinguish between different error types
- **Monitoring issues**: 500 errors trigger alerts for server issues when they're actually client errors
- **HTTP standard compliance**: Violates HTTP status code semantics

## Reproduction Steps

1. Create a simple Lambda with lift v1.0.24:
```go
func main() {
    app := lift.New()
    
    app.Use(func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            return lift.NewLiftError("UNAUTHORIZED", "Test error", 401)
        })
    })
    
    app.GET("/test", func(ctx *lift.Context) error {
        return ctx.JSON(map[string]string{"status": "ok"})
    })
    
    lambda.Start(app.HandleRequest)
}
```

2. Deploy to AWS Lambda + API Gateway
3. Make request: `curl -v https://api-url/test`
4. Observe: HTTP 500 instead of HTTP 401

## Additional Context

- This issue appears to be introduced in lift v1.0.24 or is related to the new error handling system
- The `LiftError` struct correctly contains the status code in the `StatusCode` field
- Error logging middleware correctly extracts and logs the status code
- Only the final HTTP response status code mapping is incorrect

## Suggested Investigation Areas

1. **Response building**: Check how `LiftError.StatusCode` is mapped to HTTP response status
2. **Error middleware**: Verify if built-in error handling middleware is overriding status codes
3. **API Gateway integration**: Ensure Lambda response format includes proper `statusCode` field
4. **Context error handling**: Check if `ctx.Status()` is being called with the `LiftError.StatusCode`

## Related Working Features

These features work correctly and confirm the framework is functioning:
- Middleware execution order
- Error logging with correct codes
- CORS header setting
- Request ID generation
- Error message formatting

## ✅ Resolution Details

### Root Cause
The issue was in the `handleError` method in `pkg/lift/app.go`. The method was always returning HTTP 500 status code regardless of the `LiftError.StatusCode` value because it was not checking if the error was a `LiftError` type.

### Fix Applied
Updated the `handleError` method to:
1. Check if the error is a `*LiftError` using type assertion
2. Extract the status code from `LiftError.StatusCode` 
3. Build a proper error response with code, message, and details
4. Set the HTTP status code correctly

### Code Changes
```go
// handleError processes errors and returns appropriate responses
func (a *App) handleError(ctx *Context, err error) (any, error) {
	// Handle Lift errors properly by setting appropriate status codes
	if liftErr, ok := err.(*LiftError); ok {
		resp := map[string]any{
			"code":    liftErr.Code,
			"message": liftErr.Message,
		}
		
		// Include details if present
		if len(liftErr.Details) > 0 {
			resp["details"] = liftErr.Details
		}
		
		ctx.Status(liftErr.StatusCode).JSON(resp)
		return ctx.Response, nil
	}

	// For non-Lift errors, set 500 status
	ctx.Status(500).JSON(map[string]string{
		"error": "Internal server error",
	})

	return ctx.Response, nil
}
```

### Test Coverage
Added comprehensive tests in `pkg/lift/app_error_handling_test.go` to verify:
- ✅ 400, 401, 404, 409 status codes are properly set
- ✅ Error codes and messages are included in response
- ✅ Details are included when present
- ✅ Non-LiftError objects still return 500
- ✅ Middleware errors are handled correctly

### Verification
All test cases now pass:
- `lift.NewLiftError("UNAUTHORIZED", "No API key provided", 401)` → HTTP 401
- `lift.NewLiftError("BAD_REQUEST", "Invalid input", 400)` → HTTP 400  
- `lift.NewLiftError("NOT_FOUND", "Resource not found", 404)` → HTTP 404
- `lift.NewLiftError("CONFLICT", "Resource exists", 409)` → HTTP 409

### Impact
This fix ensures that:
- ✅ LiftError.StatusCode is properly mapped to HTTP response status
- ✅ Error details are included in the response body
- ✅ Both test and production paths work consistently
- ✅ API consumers receive correct HTTP status codes
- ✅ Monitoring systems get accurate error classifications