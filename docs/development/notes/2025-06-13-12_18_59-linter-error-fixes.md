# Linter Error Fixes - 2025-06-13-12_18_59

## Issues Found

### 1. WebSocket SendMessage Method Signature Errors

**Files:** `examples/websocket-enhanced/main.go`
**Lines:** 186, 206
**Error:** `too many arguments in call to ws.SendMessage`

The WebSocket `SendMessage` method signature is:
```go
func (wc *WebSocketContext) SendMessage(data []byte) error
```

But the code is calling it with two arguments:
```go
ws.SendMessage(ws.ConnectionID(), []byte(`{"error":"Invalid broadcast format"}`))
```

**Fix:** Remove the connection ID parameter since `SendMessage` sends to the current connection.

### 2. Go Module Dependency Issues

**File:** `go.mod`
**Lines:** 29, 34, 35
**Error:** Dependencies should be direct

The following dependencies are marked as indirect but should be direct:
- `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`
- `github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi`
- `github.com/aws/aws-sdk-go-v2/service/dynamodb`

**Fix:** Move these to the require section.

### 3. Copylock Issues in Chaos Testing

**File:** `pkg/testing/enterprise/chaos.go`
**Multiple lines:** 173, 178, 236, 253, 270, 351, 446
**Error:** Passing lock by value

The `ChaosMetrics` struct contains a `sync.RWMutex` and is being passed by value instead of by pointer.

**Fix:** Change method signatures to accept `*ChaosMetrics` instead of `ChaosMetrics`.

### 4. Unused Parameter Warnings (NEW)

**Files:** Multiple security and CLI files
**Error:** `unused parameter` warnings

Multiple functions have unused parameters that should either be used or marked as unused with underscore.

**Fix:** Either use the parameters or rename them to `_` to indicate they're intentionally unused.

### 5. Unused Write Warnings (NEW)

**File:** `pkg/security/soc2_continuous_monitoring_test.go`
**Error:** `unused write to field` warnings

Test code is setting struct fields that are never read.

**Fix:** Either use the fields in assertions or remove the unused assignments.

## Resolution Plan

1. Fix WebSocket SendMessage calls
2. Update go.mod dependencies
3. Fix copylock issues in chaos testing
4. Fix unused parameter warnings
5. Fix unused write warnings in tests
6. Run go mod tidy to clean up

## Resolution Summary

### ✅ Fixed WebSocket SendMessage Calls

**File:** `examples/websocket-enhanced/main.go`
- Line 186: Removed `ws.ConnectionID()` parameter from `ws.SendMessage()` call
- Line 206: Removed `ws.ConnectionID()` parameter from `ws.SendMessage()` call

The `SendMessage` method sends to the current connection automatically, so the connection ID parameter was incorrect.

### ✅ Fixed Go Module Dependencies

**File:** `go.mod`
- Moved `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue` from indirect to direct
- Moved `github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi` from indirect to direct  
- Moved `github.com/aws/aws-sdk-go-v2/service/dynamodb` from indirect to direct
- Ran `go mod tidy` to clean up

### ✅ Fixed Copylock Issues

**File:** `pkg/testing/enterprise/chaos.go`
- Changed `ChaosScenario.ValidateRecovery()` interface to accept `*ChaosMetrics` instead of `ChaosMetrics`
- Updated `RecoveryValidator.ValidateRecovery()` method signature to accept `*ChaosMetrics`
- Updated `RecoveryValidator.validateMetrics()` method signature to accept `*ChaosMetrics`
- Updated concrete implementations:
  - `NetworkLatencyScenario.ValidateRecovery()`
  - `ServiceUnavailableScenario.ValidateRecovery()`
- Updated method calls to pass pointers instead of values

### ✅ Fixed Unused Parameter Warnings

**Files:** Multiple security and CLI files
- `pkg/cli/commands.go`: Fixed unused `template` parameter in `generateMainFile`
- `pkg/security/compliance_dashboard.go`: Fixed unused `metrics` parameters in export functions
- `pkg/security/enhanced_compliance.go`: Fixed multiple unused `ctx` parameters in helper methods
- `pkg/security/risk_scoring.go`: Fixed unused `ctx`, `event`, `ipAddress`, and `userAgent` parameters

**Fix:** Renamed unused parameters to `_` to indicate they are intentionally unused.

### ✅ Fixed Unused Write Warnings

**File:** `pkg/security/soc2_continuous_monitoring_test.go`
- Removed unused struct field assignments in test functions
- Kept only the fields that are actually used in assertions
- Cleaned up test data structures to remove unnecessary field assignments

**Fix:** Removed unused field assignments from test structs, keeping only fields that are verified in assertions.

### ✅ Verification

- All files compile successfully with `go build ./...`
- WebSocket enhanced example builds correctly
- Security tests pass with cleaned up test data
- No more linter errors reported

The copylock issues were resolved by ensuring that structs containing `sync.RWMutex` are always passed by pointer rather than by value, which prevents copying the mutex and potential deadlocks.

The unused parameter warnings were resolved by marking intentionally unused parameters with underscore (`_`), which is the Go convention for indicating that a parameter is required by an interface but not used in the implementation.

The unused write warnings in tests were resolved by removing unnecessary field assignments that weren't being verified, making the tests cleaner and more focused. 