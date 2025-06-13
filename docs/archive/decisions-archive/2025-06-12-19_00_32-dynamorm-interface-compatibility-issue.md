# Follow-up: Mock Interface Compatibility Issue

**Repository**: `github.com/pay-theory/dynamorm`  
**Issue Type**: Bug/Enhancement  
**Related To**: Previous testing guidance issue

---

## Title: MockDB Does Not Implement Full ExtendedDB Interface

### Summary

Thank you for the excellent guidance on the factory pattern! We've successfully implemented it, but encountered an interface compatibility issue: `mocks.MockDB` doesn't implement all methods of `core.ExtendedDB`, preventing direct usage in the factory pattern.

### The Issue

When trying to use the factory pattern as recommended:

```go
factory := &dynamorm.MockDBFactory{MockDB: mockDB}
```

We get compilation error:
```
cannot use mockDB (variable of type *mocks.MockDB) as core.ExtendedDB value: 
*mocks.MockDB does not implement core.ExtendedDB (missing methods: 
AutoMigrateWithOptions, CreateTable, DropTable, etc.)
```

### Missing Methods

The `core.ExtendedDB` interface includes methods not present in `MockDB`:
- `AutoMigrateWithOptions(models any, options ...any) error`
- `CreateTable(model any) error`
- `DropTable(model any) error`
- `TableExists(model any) (bool, error)`
- Several others...

### Current Workaround

We're using `interface{}` in the factory to bypass type checking:

```go
type DBFactory interface {
    CreateDB(config session.Config) (interface{}, error)
}
```

This works but loses type safety.

### Proposed Solutions

#### Option 1: Complete MockExtendedDB
Provide a `MockExtendedDB` that implements the full interface:

```go
// In mocks package
type MockExtendedDB struct {
    *MockDB
}

func (m *MockExtendedDB) AutoMigrateWithOptions(models any, options ...any) error {
    args := m.Called(models, options)
    return args.Error(0)
}

func (m *MockExtendedDB) CreateTable(model any) error {
    args := m.Called(model)
    return args.Error(0)
}

// ... other missing methods
```

#### Option 2: Interface Segregation
Define a smaller interface for common operations:

```go
// CoreDB represents the subset of ExtendedDB used in most applications
type CoreDB interface {
    Model(model any) Query
    WithContext(ctx context.Context) CoreDB
    Transaction(fn func(*Tx) error) error
    // Only the commonly used methods
}
```

#### Option 3: Mock Generator
Provide a helper that generates the full mock:

```go
func NewMockExtendedDB() *MockExtendedDB {
    mock := &MockExtendedDB{
        MockDB: new(MockDB),
    }
    // Setup default expectations for rarely-used methods
    mock.On("AutoMigrateWithOptions", mock.Anything, mock.Anything).Return(nil).Maybe()
    mock.On("CreateTable", mock.Anything).Return(nil).Maybe()
    // etc...
    return mock
}
```

### Impact

This affects anyone trying to use the factory pattern with DynamORM mocks for:
- Unit testing with dependency injection
- Integration testing with clean architecture
- Following the recommended testing patterns

### Questions

1. Is `ExtendedDB` the right interface to mock, or should we use a subset?
2. Are all ExtendedDB methods necessary for typical usage?
3. Would you accept a PR adding `MockExtendedDB`?

### Our Use Case

In Lift, we primarily use:
- `Model()`, `WithContext()` for queries
- `Transaction()` for atomic operations
- Basic CRUD operations

We don't typically use schema management methods (`CreateTable`, `AutoMigrate`) in application code.

### Temporary Solution

For now, we've created a wrapper, but would prefer an official solution:

```go
type TestDB struct {
    mock *mocks.MockDB
}

func (t *TestDB) Model(m any) core.Query {
    return t.mock.Model(m)
}

// Implement other needed methods...
```

### Request

Could you provide guidance on:
1. The intended way to mock `ExtendedDB` for testing
2. Whether a `MockExtendedDB` would be a welcome addition
3. If interface segregation would be a better approach

Thank you for your continued support! The factory pattern is working great otherwise, and we're excited to contribute back once we resolve this interface compatibility issue.

---

**Environment**:
- DynamORM Version: v1.0.9
- Go Version: 1.23
- Issue discovered while implementing factory pattern from your guidance 