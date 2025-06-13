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

**Two separate issues were identified:**

### Issue 1: Type Conversion in `extractStringMapField`

The `extractStringMapField` function only handled `map[string]interface{}` but not `map[string]string`. When the WebSocket event is converted to generic format, the `QueryStringParameters` field remains as `map[string]string`, but the function expected `map[string]interface{}`.

### Issue 2: Request Wrapper Creation in `parseEvent`

The `parseEvent` method in `pkg/lift/app.go` was incorrectly creating the request wrapper:

```go
// WRONG: Only sets embedded field, doesn't copy QueryParams
return &Request{Request: adapterRequest}, nil

// CORRECT: Uses NewRequest to properly copy all fields
return NewRequest(adapterRequest), nil
```

## Solution

### Fix 1: Updated `extractStringMapField` Function

Updated the function in `pkg/lift/adapters/adapter.go` to handle both input types:

```go
// extractStringMapField safely extracts a string map field from a map
// Handles both map[string]string and map[string]interface{} input types
func extractStringMapField(data map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	if value, exists := data[key]; exists {
		// Handle map[string]string directly
		if stringMap, ok := value.(map[string]string); ok {
			for k, v := range stringMap {
				result[k] = v
			}
		} else if mapValue, ok := value.(map[string]interface{}); ok {
			// Handle map[string]interface{} by converting values to strings
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

### Fix 2: Corrected Request Creation in `parseEvent`

Updated the `parseEvent` method in `pkg/lift/app.go`:

```go
// parseEvent converts a Lambda event to our Request structure
func (a *App) parseEvent(event interface{}) (*Request, error) {
	// Use the adapter registry to automatically detect and parse the event
	adapterRequest, err := a.adapterRegistry.DetectAndAdapt(event)
	if err != nil {
		return nil, err
	}

	// Properly wrap the adapter request using NewRequest to copy all fields
	return NewRequest(adapterRequest), nil
}
```

## Testing

1. **Added comprehensive test coverage**:
   - `TestWebSocketQueryParameterBugFix`: Specifically tests the bug scenario
   - `TestExtractStringMapFieldBothTypes`: Tests the function with both input types

2. **Verified fix works**:
   - All existing tests continue to pass
   - New tests pass
   - Full integration test confirms `ctx.Query("Authorization")` returns the expected value
   - WebSocket JWT authentication patterns now work correctly

## Files Modified

1. `pkg/lift/adapters/adapter.go`: Fixed `extractStringMapField` function
2. `pkg/lift/app.go`: Fixed `parseEvent` method to use `NewRequest`
3. `pkg/lift/adapters/adapter_test.go`: Updated trigger count for WebSocket support
4. `pkg/lift/adapters/websocket_test.go`: Added regression tests

## Impact

- ✅ **Fixed**: WebSocket query parameters are now properly accessible via `ctx.Query()`
- ✅ **Backward Compatible**: Existing functionality remains unchanged
- ✅ **Well Tested**: Added comprehensive test coverage to prevent regression
- ✅ **Standard JWT Authentication**: WebSocket JWT authentication patterns now work correctly
- ✅ **Full Integration**: End-to-end flow from AWS Lambda event to handler context works correctly

## Resolution Status

**RESOLVED** - The WebSocket query parameter bug has been completely fixed and thoroughly tested. The issue involved two separate problems:

1. **Type handling** in the adapter's `extractStringMapField` function
2. **Request creation** in the app's `parseEvent` method

Both issues have been resolved, and users can now access query parameters in WebSocket handlers using `ctx.Query("paramName")` as expected. This enables standard WebSocket JWT authentication patterns where tokens are passed via query parameters during the WebSocket handshake. 