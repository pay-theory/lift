package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Example 1: Unit testing with mocks
func TestPaymentService_CreatePayment_WithMocks(t *testing.T) {
	// Create mock client
	mockClient := NewMockDynamORMClient()
	
	// Set up expectations
	mockClient.On("PutItem", mock.Anything, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)
	
	// Create a payment
	payment := map[string]types.AttributeValue{
		"pk":     &types.AttributeValueMemberS{Value: "PAYMENT#123"},
		"sk":     &types.AttributeValueMemberS{Value: "METADATA"},
		"amount": &types.AttributeValueMemberN{Value: "1000"},
		"status": &types.AttributeValueMemberS{Value: "pending"},
	}
	
	// Call the service (this would be your actual service code)
	ctx := context.Background()
	_, err := mockClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("payments"),
		Item:      payment,
	})
	
	// Verify
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// Example 2: Testing with mock tables
func TestUserService_QueryUsers_WithMockTable(t *testing.T) {
	// Create mock client with table
	mockClient := NewMockDynamORMClient()
	mockClient.AddMockTable("users", 
		WithGSI("email-index", "email", "sk"),
		WithTTL("expires_at"),
	)
	
	// Don't set expectations - use default behavior
	
	// Add test data
	ctx := context.Background()
	testUser := map[string]types.AttributeValue{
		"pk":    &types.AttributeValueMemberS{Value: "USER#123"},
		"sk":    &types.AttributeValueMemberS{Value: "PROFILE"},
		"email": &types.AttributeValueMemberS{Value: "test@example.com"},
		"name":  &types.AttributeValueMemberS{Value: "Test User"},
	}
	
	_, err := mockClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("users"),
		Item:      testUser,
	})
	assert.NoError(t, err)
	
	// Query the data
	output, err := mockClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("users"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "USER#123"},
			"sk": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	
	assert.NoError(t, err)
	assert.NotNil(t, output.Item)
	assert.Equal(t, "Test User", output.Item["name"].(*types.AttributeValueMemberS).Value)
}

// Example 3: Integration testing with DynamoDB Local
func TestPaymentService_Integration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Create test helper
	helper := NewDynamORMTestHelper(t)
	
	// Create test table
	tableName := "test-payments-" + time.Now().Format("20060102150405")
	helper.CreateTestTable(t, tableName,
		WithGSI("user-index", "user_id", "created_at"),
		WithTTL("expires_at"),
	)
	defer helper.DeleteTestTable(t, tableName)
	
	// Test data
	payment := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: "PAYMENT#123"},
		"sk":         &types.AttributeValueMemberS{Value: "METADATA"},
		"user_id":    &types.AttributeValueMemberS{Value: "USER#456"},
		"amount":     &types.AttributeValueMemberN{Value: "1000"},
		"created_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		"status":     &types.AttributeValueMemberS{Value: "pending"},
	}
	
	// Put item
	helper.PutTestItem(t, tableName, payment)
	
	// Get item back
	retrieved := helper.GetTestItem(t, tableName, map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "PAYMENT#123"},
		"sk": &types.AttributeValueMemberS{Value: "METADATA"},
	})
	
	assert.NotNil(t, retrieved)
	assert.Equal(t, "1000", retrieved["amount"].(*types.AttributeValueMemberN).Value)
	assert.Equal(t, "pending", retrieved["status"].(*types.AttributeValueMemberS).Value)
	
	// Query by user ID using GSI
	ctx := context.Background()
	queryResp, err := helper.DynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("user-index"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: "USER#456"},
		},
	})
	
	assert.NoError(t, err)
	assert.Len(t, queryResp.Items, 1)
}

// Example 4: Testing DynamORM models
func TestDynamORMModel_Serialization(t *testing.T) {
	// Create a test item
	item := CreateDynamORMItem("USER#123", "PROFILE", "user")
	item.Data["email"] = "test@example.com"
	item.Data["name"] = "Test User"
	item.GSI1PK = "EMAIL#test@example.com"
	item.GSI1SK = "USER#123"
	
	// Marshal to DynamoDB attributes
	av, err := attributevalue.MarshalMap(item)
	assert.NoError(t, err)
	
	// Verify attributes
	assert.Equal(t, "USER#123", av["pk"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "PROFILE", av["sk"].(*types.AttributeValueMemberS).Value)
	assert.Equal(t, "user", av["type"].(*types.AttributeValueMemberS).Value)
	assert.NotNil(t, av["created_at"])
	assert.NotNil(t, av["updated_at"])
	
	// Unmarshal back
	var unmarshaled DynamORMTestItem
	err = attributevalue.UnmarshalMap(av, &unmarshaled)
	assert.NoError(t, err)
	
	assert.Equal(t, item.PK, unmarshaled.PK)
	assert.Equal(t, item.SK, unmarshaled.SK)
	assert.Equal(t, item.Type, unmarshaled.Type)
	assert.Equal(t, item.Data["email"], unmarshaled.Data["email"])
}

// Example 5: Testing with scenarios helper
func TestMultiTenantScenario(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Create test helper and scenarios
	helper := NewDynamORMTestHelper(t)
	scenarios := NewDynamORMTestScenarios(helper)
	
	// Create multi-tenant table
	tableName := "test-multitenant-" + time.Now().Format("20060102150405")
	helper.CreateTestTable(t, tableName,
		WithGSI("tenant-user-index", "tenant_id", "user_id"),
	)
	defer helper.DeleteTestTable(t, tableName)
	
	// Run multi-tenant test scenario
	scenarios.TestMultiTenantAccess(t, tableName)
}

// Example 6: Testing pagination
func TestPaginationScenario(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Create test helper and scenarios
	helper := NewDynamORMTestHelper(t)
	scenarios := NewDynamORMTestScenarios(helper)
	
	// Create table
	tableName := "test-pagination-" + time.Now().Format("20060102150405")
	helper.CreateTestTable(t, tableName)
	defer helper.DeleteTestTable(t, tableName)
	
	// Run pagination test scenario
	scenarios.TestPaginatedQueries(t, tableName)
}

// Example 7: Testing conditional writes
func TestConditionalWritesScenario(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Create test helper and scenarios
	helper := NewDynamORMTestHelper(t)
	scenarios := NewDynamORMTestScenarios(helper)
	
	// Create table
	tableName := "test-conditional-" + time.Now().Format("20060102150405")
	helper.CreateTestTable(t, tableName)
	defer helper.DeleteTestTable(t, tableName)
	
	// Run conditional writes test scenario
	scenarios.TestConditionalWrites(t, tableName)
}

// Example 8: Benchmarking with mocks
func BenchmarkPutItem_WithMocks(b *testing.B) {
	// Create mock client
	mockClient := NewMockDynamORMClient()
	mockClient.AddMockTable("benchmark-table")
	
	// Prepare test item
	item := map[string]types.AttributeValue{
		"pk":   &types.AttributeValueMemberS{Value: "BENCH#123"},
		"sk":   &types.AttributeValueMemberS{Value: "DATA"},
		"data": &types.AttributeValueMemberS{Value: "benchmark data"},
	}
	
	ctx := context.Background()
	
	// Reset timer to exclude setup
	b.ResetTimer()
	
	// Run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = mockClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String("benchmark-table"),
			Item:      item,
		})
	}
}

// Example test suite using the integration suite base
type PaymentServiceTestSuite struct {
	DynamORMIntegrationSuite
	paymentsTable string
}

func (s *PaymentServiceTestSuite) SetupSuite() {
	s.DynamORMIntegrationSuite.SetupSuite()
	
	// Create payments table
	s.paymentsTable = s.CreateTestTable("payments",
		WithGSI("user-payments", "user_id", "created_at"),
		WithGSI("status-index", "status", "created_at"),
		WithTTL("expires_at"),
	)
}

func (s *PaymentServiceTestSuite) TestCreatePayment() {
	payment := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: "PAYMENT#789"},
		"sk":         &types.AttributeValueMemberS{Value: "METADATA"},
		"user_id":    &types.AttributeValueMemberS{Value: "USER#123"},
		"amount":     &types.AttributeValueMemberN{Value: "5000"},
		"status":     &types.AttributeValueMemberS{Value: "completed"},
		"created_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}
	
	s.helper.PutTestItem(s.T(), s.paymentsTable, payment)
	
	// Verify payment was created
	retrieved := s.helper.GetTestItem(s.T(), s.paymentsTable, map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "PAYMENT#789"},
		"sk": &types.AttributeValueMemberS{Value: "METADATA"},
	})
	
	s.NotNil(retrieved)
	s.Equal("5000", retrieved["amount"].(*types.AttributeValueMemberN).Value)
}

func (s *PaymentServiceTestSuite) TestQueryUserPayments() {
	// Create multiple payments for a user
	userID := "USER#999"
	for i := 0; i < 5; i++ {
		payment := map[string]types.AttributeValue{
			"pk":         &types.AttributeValueMemberS{Value: fmt.Sprintf("PAYMENT#%c", 'A'+i)},
			"sk":         &types.AttributeValueMemberS{Value: "METADATA"},
			"user_id":    &types.AttributeValueMemberS{Value: userID},
			"amount":     &types.AttributeValueMemberN{Value: "1000"},
			"status":     &types.AttributeValueMemberS{Value: "completed"},
			"created_at": &types.AttributeValueMemberS{Value: time.Now().Add(time.Duration(i) * time.Hour).Format(time.RFC3339)},
		}
		s.helper.PutTestItem(s.T(), s.paymentsTable, payment)
	}
	
	// Query user payments
	ctx := context.Background()
	resp, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.paymentsTable),
		IndexName:              aws.String("user-payments"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
		ScanIndexForward: aws.Bool(false), // Most recent first
	})
	
	s.NoError(err)
	s.Len(resp.Items, 5)
}

// Run the test suite
func TestPaymentServiceSuite(t *testing.T) {
	suite.Run(t, new(PaymentServiceTestSuite))
}