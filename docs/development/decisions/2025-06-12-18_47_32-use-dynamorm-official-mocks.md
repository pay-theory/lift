# Decision: Use DynamORM Official Mocks Instead of Custom Mocks

**Date**: 2025-06-12-18_47_32  
**Status**: ✅ APPROVED  
**Impact**: Testing Framework, DynamORM Integration

## Context

During Sprint 4 development, we were creating custom mocks for DynamORM in `pkg/testing/mocks.go`. However, DynamORM already provides comprehensive official mocks using testify/mock.

## Problem

1. **Duplicate Effort**: Creating custom mocks when official ones exist
2. **Maintenance Burden**: Our custom mocks need to be kept in sync with DynamORM API changes
3. **Interface Mismatch**: Our custom mocks had different interfaces than the actual DynamORM wrappers
4. **Missing Features**: Our mocks didn't cover all 26+ Query interface methods

## Decision

**We will use DynamORM's official mocks from `github.com/pay-theory/dynamorm/pkg/mocks`**

## Rationale

### ✅ Benefits of Official Mocks

1. **Complete Coverage**: All 26+ Query methods, DB methods, and UpdateBuilder methods
2. **Type Safety**: Implements actual DynamORM interfaces
3. **Testify Integration**: Seamless integration with testify/mock
4. **Maintenance**: Maintained by DynamORM team, always up-to-date
5. **Documentation**: Extensive examples and godoc comments
6. **Chainable Methods**: Proper support for method chaining
7. **Flexible Matching**: Support for `mock.Anything`, `mock.MatchedBy`, etc.

### ❌ Problems with Custom Mocks

1. **Interface Mismatch**: Our `MockDynamORM` had `(ctx, table, key, item)` signature vs DynamORM's `(ctx, item)` 
2. **Incomplete**: Missing many DynamORM features like query builders, transactions, etc.
3. **Maintenance**: We'd need to update them every time DynamORM changes
4. **Testing Complexity**: Required wrapper classes to match interfaces

## Implementation Plan

### 1. Update Test Dependencies
```bash
# Ensure we have the latest DynamORM with mocks
go get github.com/pay-theory/dynamorm@latest
```

### 2. Replace Custom Mocks in Tests
```go
// OLD: Custom mock
mockDB := lifttesting.NewMockDynamORM()

// NEW: Official DynamORM mocks
mockDB := new(mocks.MockDB)
mockQuery := new(mocks.MockQuery)
```

### 3. Update Test Patterns
```go
func TestCreateUser(t *testing.T) {
    // Setup mocks
    mockDB := new(mocks.MockDB)
    mockQuery := new(mocks.MockQuery)
    
    // Setup expectations
    mockDB.On("Model", mock.AnythingOfType("*main.User")).Return(mockQuery)
    mockQuery.On("Create").Return(nil)
    
    // Test execution
    app := setupTestApp(mockDB)
    response := app.POST("/users", userRequest)
    
    // Assertions
    assert.Equal(t, 201, response.StatusCode)
    mockDB.AssertExpectations(t)
    mockQuery.AssertExpectations(t)
}
```

### 4. Remove Custom Mock Code
- Delete `MockDynamORM` and related code from `pkg/testing/mocks.go`
- Keep other useful mocks (AWS services, HTTP clients, etc.)

## Migration Strategy

1. **Phase 1**: Update basic-crud-api example to use official mocks
2. **Phase 2**: Update DynamORM integration tests
3. **Phase 3**: Remove custom mock code
4. **Phase 4**: Document new testing patterns

## Expected Outcomes

1. **Reduced Maintenance**: No need to maintain custom DynamORM mocks
2. **Better Test Coverage**: Access to all DynamORM features in tests
3. **Improved Reliability**: Tests use the same interfaces as production code
4. **Easier Onboarding**: Developers familiar with DynamORM can use familiar mocking patterns

## References

- DynamORM Mocks Documentation: `github.com/pay-theory/dynamorm/pkg/mocks`
- Testify Mock Library: `github.com/stretchr/testify/mock`
- Sprint 4 Testing Requirements: `docs/development/prompts/lift-integration-testing-assistant.md`

## Next Steps

1. Update `examples/basic-crud-api/main_test.go` to use official mocks
2. Create example test patterns for common DynamORM operations
3. Update testing documentation with new patterns
4. Remove custom mock code once migration is complete 