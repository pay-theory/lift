# GitHub Issue for DynamORM Team

**Repository**: `github.com/pay-theory/dynamorm`  
**Issue Type**: Question/Enhancement  
**Labels**: testing, mocks, documentation

---

## Title: Testing Best Practices and Mock Integration Guidance

### Summary

We're building a serverless framework (Lift) that integrates with DynamORM and want to ensure we're following best practices for testing. We've discovered the excellent mock implementations in `pkg/mocks/` but need guidance on integration patterns, especially for middleware-based architectures.

### Background

**Our Use Case:**
- Building a serverless framework that wraps DynamORM with middleware
- Need comprehensive testing for CRUD operations, transactions, and multi-tenant scenarios
- Want to use official DynamORM mocks instead of custom implementations

**Current Architecture:**
```go
// Middleware that provides DynamORM to handlers
func WithDynamORM(config *Config) Middleware {
    return func(next Handler) Handler {
        return func(ctx *Context) error {
            db, err := dynamorm.New(sessionConfig)
            if err != nil {
                return err
            }
            
            // Store in context for handlers
            ctx.Set("dynamorm", db)
            return next.Handle(ctx)
        }
    }
}

// Handler that uses DynamORM
func CreateUser(ctx *Context) error {
    db := ctx.Get("dynamorm").(dynamorm.DB)
    return db.Model(&User{}).Create()
}
```

### Questions

#### 1. **Mock Integration Patterns**
What's the recommended way to integrate the official mocks (`mocks.MockDB`, `mocks.MockQuery`) in middleware-based architectures?

**Options we're considering:**
- A) Override context values after middleware runs
- B) Dependency injection in middleware
- C) Test-specific middleware variants
- D) Factory pattern for DB creation

#### 2. **Interface Compatibility**
The mocks implement the core DynamORM interfaces, but our middleware creates wrapper types. Should we:
- Use the mocks directly and adapt our wrappers?
- Create adapter patterns between mocks and wrappers?
- Modify our architecture to be more mock-friendly?

#### 3. **Testing Patterns Documentation**
Would the DynamORM team be open to expanding testing documentation with:
- Middleware integration examples
- Multi-tenant testing patterns
- Transaction testing with mocks
- Performance testing approaches

### Current Implementation

**What Works:**
```go
func TestUserOperations(t *testing.T) {
    mockDB := new(mocks.MockDB)
    mockQuery := new(mocks.MockQuery)
    
    // Setup expectations
    mockDB.On("Model", mock.AnythingOfType("*User")).Return(mockQuery)
    mockQuery.On("Create").Return(nil)
    
    // Direct usage works great
    var user User
    err := mockDB.Model(&user).Create()
    assert.NoError(t, err)
    
    mockDB.AssertExpectations(t)
}
```

**What's Challenging:**
```go
func TestWithMiddleware(t *testing.T) {
    // How to inject mocks into middleware-based architecture?
    app := lift.New()
    app.Use(WithDynamORM(config)) // This creates real DB connection
    
    // Need pattern to override with mocks for testing
}
```

### Proposed Enhancements

#### 1. **Testing Utilities Package**
Consider adding `pkg/testing` with utilities like:
```go
// Helper for middleware testing
func NewMockDBForTesting() (*mocks.MockDB, *mocks.MockQuery) {
    mockDB := new(mocks.MockDB)
    mockQuery := new(mocks.MockQuery)
    
    // Common setup for typical operations
    mockDB.On("Model", mock.Anything).Return(mockQuery)
    
    return mockDB, mockQuery
}

// Factory interface for dependency injection
type DBFactory interface {
    CreateDB(config session.Config) (DB, error)
}

type MockDBFactory struct {
    mockDB *mocks.MockDB
}

func (f *MockDBFactory) CreateDB(config session.Config) (DB, error) {
    return f.mockDB, nil
}
```

#### 2. **Documentation Examples**
Add examples for:
- Testing middleware that uses DynamORM
- Multi-tenant data isolation testing
- Transaction rollback scenarios
- Error handling patterns

#### 3. **Mock Helpers**
Consider adding convenience methods:
```go
// In mocks package
func (m *MockQuery) ExpectCreate() *MockQuery {
    m.On("Create").Return(nil)
    return m
}

func (m *MockQuery) ExpectCreateError(err error) *MockQuery {
    m.On("Create").Return(err)
    return m
}

func (m *MockQuery) ExpectFind(result interface{}) *MockQuery {
    m.On("First", mock.Anything).Run(func(args mock.Arguments) {
        // Populate result
    }).Return(nil)
    return m
}
```

### Example Use Cases We're Testing

1. **CRUD Operations**: Create, Read, Update, Delete with validation
2. **Multi-Tenant Isolation**: Ensuring tenant A can't access tenant B's data
3. **Transaction Management**: Automatic rollback on errors
4. **Error Handling**: Database failures, validation errors, not found scenarios
5. **Performance**: Benchmarking with realistic mock data

### Questions for the Team

1. **Architecture Guidance**: What patterns do you recommend for testing middleware that uses DynamORM?

2. **Mock Limitations**: Are there any known limitations or gotchas with the current mock implementations?

3. **Contribution Opportunity**: Would you be interested in PRs that add testing utilities or documentation examples?

4. **Best Practices**: Any specific patterns you recommend for:
   - Testing complex queries with multiple conditions
   - Mocking transaction scenarios
   - Testing error conditions
   - Performance testing approaches

### Our Commitment

We're committed to:
- Following DynamORM best practices
- Contributing back improvements that benefit the community
- Providing feedback on testing experience
- Helping improve documentation based on real-world usage

### Environment

- **DynamORM Version**: v1.0.9
- **Go Version**: 1.23
- **Testing Framework**: testify/mock, testify/assert
- **Use Case**: Serverless framework middleware

---

**Thank you for the excellent work on DynamORM and the comprehensive mock implementations! We're excited to build on this foundation and contribute back to the community.** 