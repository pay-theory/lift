# Middleware Test Compilation Fixes Needed

**Date**: 2025-06-12-20_09_06  
**Status**: 🔄 IN PROGRESS  
**Component**: `pkg/middleware/*_test.go`

## Issues Identified

### 1. Mock Interface Mismatches
**Files Affected**: 
- `pkg/middleware/integration_test.go`
- `pkg/middleware/servicemesh_test.go` 
- `pkg/middleware/ratelimit_test.go`

**Problems**:
- Mock logger methods have wrong signatures: `Debug(string, map[string]interface{})` should be `Debug(string, ...map[string]interface{})`
- Mock metrics methods missing variadic tags: `Counter(string)` should be `Counter(string, ...map[string]string)`
- Missing interface methods: `GetStats()`, `WithField()`, `WithFields()`, etc.

### 2. Request Struct Field Issues
**Problem**: Tests use non-existent fields like `Method`, `Path`, `Headers` on `lift.Request`
**Solution**: Use the correct fields from `adapters.Request`:
```go
// ❌ Wrong
Request: &lift.Request{
    Method: "GET",
    Path:   "/test",
    Headers: make(map[string]string),
}

// ✅ Correct  
Request: &lift.Request{
    Request: &adapters.Request{
        Method:      "GET",
        Path:        "/test", 
        Headers:     make(map[string]string),
        QueryParams: make(map[string]string),
    },
}
```

### 3. Missing Context Methods
**Problem**: Tests call `ctx.SetTenantID()` and `ctx.GetTenantID()` which don't exist
**Solution**: Use the context value system:
```go
// ❌ Wrong
ctx.SetTenantID("tenant1")
tenantID := ctx.GetTenantID()

// ✅ Correct
ctx.Set("tenant_id", "tenant1")
tenantID := ctx.TenantID() // Uses existing method
```

### 4. Undefined Types and Constants
**Missing Types**:
- `LoadSheddingStrategyRandom`
- `HealthConfig`, `HealthChecker`, `HealthMiddleware`
- Various middleware configuration fields

**Missing Methods**:
- `RateLimit`, `TenantRateLimit`, `UserRateLimit`, etc.
- `defaultKeyFunc`, `defaultErrorHandler`

## Recommended Solutions

### Phase 1: Fix Mock Implementations
Create proper mock implementations that satisfy all interfaces:

```go
// Correct mock logger
type mockServiceMeshLogger struct {
    logs []map[string]interface{}
}

func (m *mockServiceMeshLogger) Debug(msg string, fields ...map[string]interface{}) {
    entry := map[string]interface{}{"level": "debug", "message": msg}
    for _, fieldMap := range fields {
        for k, v := range fieldMap {
            entry[k] = v
        }
    }
    m.logs = append(m.logs, entry)
}

// Implement all required interface methods...
func (m *mockServiceMeshLogger) WithField(key string, value interface{}) lift.Logger { return m }
func (m *mockServiceMeshLogger) WithFields(fields map[string]interface{}) lift.Logger { return m }
func (m *mockServiceMeshLogger) WithRequestID(requestID string) observability.StructuredLogger { return m }
func (m *mockServiceMeshLogger) GetStats() observability.LoggerStats { return observability.LoggerStats{} }
// ... etc
```

### Phase 2: Fix Request Construction
Update all test request creation:

```go
func createTestContext() *lift.Context {
    return &lift.Context{
        Context: context.Background(),
        Request: &lift.Request{
            Request: &adapters.Request{
                Method:      "GET",
                Path:        "/test",
                Headers:     make(map[string]string),
                QueryParams: make(map[string]string),
                PathParams:  make(map[string]string),
                TriggerType: adapters.TriggerAPIGateway,
            },
        },
        Response: &lift.Response{
            StatusCode: 200,
            Headers:    make(map[string]string),
        },
    }
}
```

### Phase 3: Add Missing Types
Either implement missing middleware types or create stub implementations for testing:

```go
// Add to appropriate middleware files
type LoadSheddingStrategy string
const LoadSheddingStrategyRandom LoadSheddingStrategy = "random"

type HealthConfig struct {
    EnableDetailedHealth bool
    EnableReadiness      bool  
    EnableLiveness       bool
    HealthCheckInterval  time.Duration
    HealthChecks         map[string]HealthChecker
}

type HealthChecker interface {
    Check(ctx context.Context) error
}

func HealthMiddleware(config HealthConfig) lift.Middleware {
    // Implementation
}
```

### Phase 4: Fix Context Methods
Add missing methods to Context or update tests to use existing patterns:

```go
// Add to lift.Context if needed
func (c *Context) SetTenantID(tenantID string) {
    c.Set("tenant_id", tenantID)
}

func (c *Context) GetTenantID() string {
    return c.TenantID() // Use existing method
}
```

## Files Requiring Updates

1. **pkg/middleware/integration_test.go** - Mock interfaces, request construction
2. **pkg/middleware/servicemesh_test.go** - Mock interfaces, request construction  
3. **pkg/middleware/ratelimit_test.go** - Missing types, request construction
4. **pkg/middleware/loadshedding.go** - Add missing constants/types
5. **pkg/middleware/health.go** - Create if missing
6. **pkg/lift/context.go** - Add missing methods if needed

## Priority
**HIGH** - These compilation errors prevent the middleware package from building and testing.

## Next Steps
1. Complete mock interface implementations
2. Fix request struct usage across all test files
3. Implement or stub missing middleware types
4. Add missing context methods
5. Verify all tests compile and run

---
**Related**: Bulkhead middleware fixes completed in `2025-06-12-20_09_06-bulkhead-middleware-fixes.md` 