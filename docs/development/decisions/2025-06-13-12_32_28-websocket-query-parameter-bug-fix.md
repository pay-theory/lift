# Decision: WebSocket Query Parameter Bug Fix

## Date: 2025-06-13-12_32_28

## Context

A critical bug was reported in Lift's WebSocket adapter where query parameters from `APIGatewayWebsocketProxyRequest.QueryStringParameters` were not being properly mapped to the Lift context's `ctx.Query()` method. This was blocking WebSocket JWT authentication patterns that rely on passing tokens via query parameters during the WebSocket handshake.

## Problem

The `extractStringMapField` function in `pkg/lift/adapters/adapter.go` only handled `map[string]interface{}` input but not `map[string]string`. When WebSocket events are converted to generic format, the `QueryStringParameters` field remains as `map[string]string`, causing the extraction to fail silently and return an empty map.

## Decision

**We decided to fix the `extractStringMapField` function to handle both `map[string]string` and `map[string]interface{}` input types.**

### Rationale

1. **Backward Compatibility**: The fix maintains compatibility with existing code that uses `map[string]interface{}`
2. **Type Safety**: The solution properly handles type conversion without breaking existing functionality
3. **Minimal Impact**: The change is localized to a single utility function
4. **Standard Patterns**: This enables standard WebSocket authentication patterns using JWT tokens in query parameters

### Implementation

Modified the `extractStringMapField` function to:
1. First check for `map[string]string` and handle it directly
2. Fall back to the existing `map[string]interface{}` handling
3. Maintain the same return type and behavior

## Alternatives Considered

1. **Change event conversion**: Modify `convertWebSocketEventToGeneric` to convert `map[string]string` to `map[string]interface{}`
   - **Rejected**: Would require changes in multiple places and could introduce other issues

2. **Create separate function**: Add a new function specifically for WebSocket query parameters
   - **Rejected**: Would create code duplication and inconsistency

3. **Type assertion in WebSocket adapter**: Handle the type conversion only in the WebSocket adapter
   - **Rejected**: The issue could affect other adapters using the same utility function

## Testing Strategy

1. **Regression Tests**: Added `TestWebSocketQueryParameterBugFix` to prevent regression
2. **Comprehensive Coverage**: Added `TestExtractStringMapFieldBothTypes` to test all input scenarios
3. **Existing Tests**: Verified all existing tests continue to pass
4. **Real-world Scenario**: Tested with actual WebSocket JWT authentication use case

## Impact Assessment

### Positive Impact
- ✅ Fixes critical WebSocket authentication bug
- ✅ Enables standard JWT authentication patterns
- ✅ Maintains backward compatibility
- ✅ Improves robustness of type handling

### Risk Assessment
- ⚠️ **Low Risk**: Change is localized and well-tested
- ⚠️ **Minimal Surface Area**: Only affects query parameter extraction
- ⚠️ **Backward Compatible**: No breaking changes

## Implementation Details

### Files Modified
- `pkg/lift/adapters/adapter.go`: Fixed `extractStringMapField` function
- `pkg/lift/adapters/adapter_test.go`: Updated trigger count for WebSocket support
- `pkg/lift/adapters/websocket_test.go`: Added comprehensive test coverage

### Code Changes
```go
// Before: Only handled map[string]interface{}
if mapValue, ok := value.(map[string]interface{}); ok {
    // ...
}

// After: Handles both map[string]string and map[string]interface{}
if stringMap, ok := value.(map[string]string); ok {
    for k, v := range stringMap {
        result[k] = v
    }
} else if mapValue, ok := value.(map[string]interface{}); ok {
    // ... existing logic
}
```

## Monitoring and Validation

1. **Test Coverage**: All tests pass including new regression tests
2. **Manual Validation**: Confirmed `ctx.Query("Authorization")` works in WebSocket handlers
3. **Integration Testing**: Verified with actual WebSocket JWT authentication flow

## Future Considerations

1. **Type Safety**: Consider using generics in future Go versions for better type safety
2. **Documentation**: Update WebSocket documentation to highlight query parameter support
3. **Examples**: Add WebSocket JWT authentication examples to the documentation

## Approval

This decision was implemented as a critical bug fix to unblock WebSocket authentication functionality. The fix is minimal, well-tested, and maintains backward compatibility.

**Status**: ✅ **IMPLEMENTED AND TESTED** 