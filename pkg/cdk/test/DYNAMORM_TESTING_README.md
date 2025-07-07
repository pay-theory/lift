# DynamORM CDK Testing Infrastructure

This directory contains testing utilities for DynamORM-based CDK constructs. The infrastructure supports both unit testing with mocks and integration testing with DynamoDB Local.

## Overview

The testing infrastructure provides:

1. **Mock DynamoDB Client** - For fast unit tests without AWS dependencies
2. **DynamoDB Local Helpers** - For integration tests with a real database
3. **Test Scenarios** - Common test patterns for DynamORM applications
4. **Integration Test Suite** - Base suite for comprehensive testing

## Files

- `dynamorm_helpers.go` - Core testing utilities and DynamoDB Local support
- `dynamorm_mocks.go` - Mock DynamoDB client implementation
- `dynamorm_integration.go` - Integration test framework and scenarios
- `dynamorm_example_test.go` - Examples of using the testing infrastructure

## Usage

### Unit Testing with Mocks

```go
func TestMyService_WithMocks(t *testing.T) {
    // Create mock client
    mockClient := NewMockDynamORMClient()
    
    // Option 1: Set specific expectations
    mockClient.On("PutItem", mock.Anything, mock.Anything, mock.Anything).
        Return(&dynamodb.PutItemOutput{}, nil)
    
    // Option 2: Use mock tables with default behavior
    mockClient.AddMockTable("my-table", 
        WithGSI("email-index", "email", "sk"),
        WithTTL("expires_at"),
    )
    
    // Test your service
    service := NewMyService(mockClient)
    err := service.CreateItem(item)
    
    // Verify
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}
```

### Integration Testing with DynamoDB Local

```go
func TestMyService_Integration(t *testing.T) {
    // Skip if not running integration tests
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Create test helper
    helper := NewDynamORMTestHelper(t)
    
    // Create test table
    tableName := "test-table-" + time.Now().Format("20060102150405")
    helper.CreateTestTable(t, tableName,
        WithGSI("user-index", "user_id", "created_at"),
        WithTTL("expires_at"),
    )
    defer helper.DeleteTestTable(t, tableName)
    
    // Test your service with real DynamoDB
    service := NewMyService(helper.DynamoClient)
    // ... test implementation
}
```

### Using Test Suites

```go
type MyServiceTestSuite struct {
    DynamORMIntegrationSuite
    tableName string
}

func (s *MyServiceTestSuite) SetupSuite() {
    s.DynamORMIntegrationSuite.SetupSuite()
    s.tableName = s.CreateTestTable("my-table", WithGSI("index", "gsi_pk", "gsi_sk"))
}

func (s *MyServiceTestSuite) TestCreateItem() {
    // Your test implementation
}

func TestMyServiceSuite(t *testing.T) {
    suite.Run(t, new(MyServiceTestSuite))
}
```

## Running Tests

### Unit Tests Only (Fast)
```bash
go test -short ./pkg/cdk/test
```

### Integration Tests with DynamoDB Local
```bash
# Start DynamoDB Local first
docker run -p 8000:8000 amazon/dynamodb-local

# Run integration tests
INTEGRATION_TEST=true go test ./pkg/cdk/test
```

### Custom DynamoDB Local Endpoint
```bash
DYNAMODB_LOCAL_ENDPOINT=http://localhost:8001 INTEGRATION_TEST=true go test ./pkg/cdk/test
```

### Using Real AWS (Not Recommended for Tests)
```bash
USE_REAL_AWS=true AWS_REGION=us-east-1 go test ./pkg/cdk/test
```

## Test Scenarios

The framework includes pre-built test scenarios:

1. **Multi-Tenant Access** - Tests tenant data isolation
2. **Paginated Queries** - Tests pagination handling
3. **Conditional Writes** - Tests optimistic locking

```go
scenarios := NewDynamORMTestScenarios(helper)
scenarios.TestMultiTenantAccess(t, tableName)
scenarios.TestPaginatedQueries(t, tableName)
scenarios.TestConditionalWrites(t, tableName)
```

## Best Practices

1. **Use Mocks for Unit Tests** - Fast and reliable
2. **Use Integration Tests Sparingly** - For critical paths only
3. **Clean Up Resources** - Always defer table deletion
4. **Unique Table Names** - Include timestamp to avoid conflicts
5. **Skip Integration Tests** - Use `testing.Short()` to skip in CI

## Environment Variables

- `INTEGRATION_TEST=true` - Enable integration tests
- `DYNAMODB_LOCAL_ENDPOINT` - DynamoDB Local endpoint (default: http://localhost:8000)
- `AWS_REGION` - AWS region (default: us-east-1)
- `USE_REAL_AWS=true` - Use real AWS instead of local (not recommended)

## Troubleshooting

### DynamoDB Local Not Running
```
Error: dial tcp 127.0.0.1:8000: connect: connection refused
```
Solution: Start DynamoDB Local with Docker

### Table Already Exists
```
Error: ResourceInUseException: Table already exists
```
Solution: Use unique table names with timestamps

### Permission Denied
```
Error: AccessDeniedException: User is not authorized
```
Solution: Check AWS credentials or use DynamoDB Local

## Examples

See `dynamorm_example_test.go` for complete examples including:
- Unit testing with mocks
- Integration testing
- DynamORM model testing
- Benchmarking
- Test suites
- Common scenarios