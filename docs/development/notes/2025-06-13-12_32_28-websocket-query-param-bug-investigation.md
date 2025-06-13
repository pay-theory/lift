# WebSocket Query Parameter Bug Investigation

## Date: 2025-06-13-12_32_28

## Issue Summary
Lift's WebSocket adapter is not properly mapping query parameters from `APIGatewayWebsocketProxyRequest.QueryStringParameters` to the Lift context's `ctx.Query()` method.

## Investigation Findings

### Code Analysis

1. **WebSocket Adapter (`pkg/lift/adapters/websocket.go`)**:
   - Line 102: `queryParams := extractStringMapField(eventMap, "queryStringParameters")`
   - The adapter correctly extracts query parameters using `extractStringMapField`
   - Query parameters are properly assigned to `req.QueryParams`

2. **Context Query Method (`pkg/lift/context.go`)**:
   - Line 52-57: The `Query()` method correctly accesses `c.Request.QueryParams[key]`
   - No issues found in the context implementation

3. **Event Conversion (`pkg/lift/app_websocket.go`)**:
   - Line 226: `"queryStringParameters": event.QueryStringParameters,`
   - The conversion correctly maps the query parameters from the AWS event

4. **Test Coverage (`pkg/lift/adapters/websocket_test.go`)**:
   - Line 101: Test includes query parameters in the event
   - Line 109: Test validates that query parameters are correctly extracted
   - **The test passes**, indicating the adapter works correctly

## Root Cause Analysis

The issue appears to be in the **type conversion** within the `extractStringMapField` function. Let me examine this function more closely.

### `extractStringMapField` Function Analysis

```go
func extractStringMapField(data map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	if value, exists := data[key]; exists {
		if mapValue, ok := value.(map[string]interface{}); ok {
			for k, v := range mapValue {
				if str, ok := v.(string); ok {
					result[k] = str
				}
			}
		}
	}
	return result
}
```

**Potential Issue**: The function expects `map[string]interface{}` but the AWS Lambda event might be providing `map[string]string` directly.

## Next Steps

1. Create a comprehensive test that reproduces the exact issue
2. Debug the type conversion in `extractStringMapField`
3. Fix the type handling to support both `map[string]string` and `map[string]interface{}`
4. Add additional test coverage for edge cases

## Test Plan

Create a test that:
1. Uses the exact same event structure as reported in the bug
2. Tests both the adapter directly and through the full app flow
3. Validates that `ctx.Query("Authorization")` returns the expected value 