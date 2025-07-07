package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/suite"
)

// DynamORMIntegrationSuite provides a base suite for DynamORM integration tests
type DynamORMIntegrationSuite struct {
	suite.Suite
	helper       *DynamORMTestHelper
	client       *dynamodb.Client
	tablePrefix  string
	createdTables []string
}

// SetupSuite runs once before all tests in the suite
func (s *DynamORMIntegrationSuite) SetupSuite() {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TEST") != "true" {
		s.T().Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create test helper
	s.helper = NewDynamORMTestHelper(s.T())
	s.client = s.helper.DynamoClient
	
	// Generate unique table prefix to avoid conflicts
	s.tablePrefix = fmt.Sprintf("test-%d-", time.Now().Unix())
}

// TearDownSuite runs once after all tests in the suite
func (s *DynamORMIntegrationSuite) TearDownSuite() {
	// Clean up all created tables
	for _, tableName := range s.createdTables {
		s.helper.DeleteTestTable(s.T(), tableName)
	}
}

// CreateTestTable creates a table with the suite's prefix
func (s *DynamORMIntegrationSuite) CreateTestTable(name string, opts ...TestTableOption) string {
	tableName := s.tablePrefix + name
	s.helper.CreateTestTable(s.T(), tableName, opts...)
	s.createdTables = append(s.createdTables, tableName)
	return tableName
}

// DynamORMTestEnvironment provides environment setup for DynamORM tests
type DynamORMTestEnvironment struct {
	LocalEndpoint string
	Region        string
	IsLocal       bool
}

// NewDynamORMTestEnvironment creates a new test environment
func NewDynamORMTestEnvironment() *DynamORMTestEnvironment {
	endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	// Check if we're using local DynamoDB
	isLocal := os.Getenv("USE_REAL_AWS") != "true"

	return &DynamORMTestEnvironment{
		LocalEndpoint: endpoint,
		Region:        region,
		IsLocal:       isLocal,
	}
}

// GetDynamoDBClient returns a configured DynamoDB client for testing
func (e *DynamORMTestEnvironment) GetDynamoDBClient(t *testing.T) *dynamodb.Client {
	t.Helper()

	var cfg aws.Config
	var err error

	if e.IsLocal {
		// Configure for local DynamoDB
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(e.Region),
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					if service == dynamodb.ServiceID {
						return aws.Endpoint{
							URL: e.LocalEndpoint,
						}, nil
					}
					return aws.Endpoint{}, &aws.EndpointNotFoundError{}
				})),
			config.WithCredentialsProvider(GetTestCredentials(true)),
		)
	} else {
		// Use real AWS credentials
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(e.Region),
		)
	}

	if err != nil {
		t.Fatalf("Failed to load AWS config: %v", err)
	}

	return dynamodb.NewFromConfig(cfg)
}

// DynamORMTableTestSuite tests DynamORM table operations
type DynamORMTableTestSuite struct {
	DynamORMIntegrationSuite
}

// TestCreateAndQueryTable tests basic table operations
func (s *DynamORMTableTestSuite) TestCreateAndQueryTable() {
	// Create a table with GSI
	tableName := s.CreateTestTable("users",
		WithGSI("email-index", "email", "sk"),
		WithTTL("ttl"),
	)

	// Put test items
	items := []map[string]types.AttributeValue{
		{
			"pk":    &types.AttributeValueMemberS{Value: "USER#123"},
			"sk":    &types.AttributeValueMemberS{Value: "PROFILE"},
			"email": &types.AttributeValueMemberS{Value: "user@example.com"},
			"name":  &types.AttributeValueMemberS{Value: "Test User"},
		},
		{
			"pk":    &types.AttributeValueMemberS{Value: "USER#456"},
			"sk":    &types.AttributeValueMemberS{Value: "PROFILE"},
			"email": &types.AttributeValueMemberS{Value: "another@example.com"},
			"name":  &types.AttributeValueMemberS{Value: "Another User"},
		},
	}

	for _, item := range items {
		s.helper.PutTestItem(s.T(), tableName, item)
	}

	// Query by email using GSI
	ctx := context.Background()
	resp, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("email-index"),
		KeyConditionExpression: aws.String("email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: "user@example.com"},
		},
	})

	s.NoError(err)
	s.Len(resp.Items, 1)
	s.Equal("Test User", resp.Items[0]["name"].(*types.AttributeValueMemberS).Value)
}

// TestTTLFunctionality tests TTL configuration
func (s *DynamORMTableTestSuite) TestTTLFunctionality() {
	tableName := s.CreateTestTable("sessions", WithTTL("expires_at"))

	// Verify TTL is enabled
	ctx := context.Background()
	ttlResp, err := s.client.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{
		TableName: aws.String(tableName),
	})

	s.NoError(err)
	s.Equal("expires_at", *ttlResp.TimeToLiveDescription.AttributeName)
	s.Equal(types.TimeToLiveStatusEnabled, ttlResp.TimeToLiveDescription.TimeToLiveStatus)
}

// TestStreamProcessing tests DynamoDB streams
func (s *DynamORMTableTestSuite) TestStreamProcessing() {
	tableName := s.CreateTestTable("events", WithStream())

	// Verify stream is enabled
	ctx := context.Background()
	descResp, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})

	s.NoError(err)
	s.NotNil(descResp.Table.StreamSpecification)
	s.True(*descResp.Table.StreamSpecification.StreamEnabled)
}

// Example of how to use the test suite
func ExampleDynamORMIntegrationTest(t *testing.T) {
	suite.Run(t, new(DynamORMTableTestSuite))
}

// DynamORMTestScenarios provides common test scenarios
type DynamORMTestScenarios struct {
	helper *DynamORMTestHelper
}

// NewDynamORMTestScenarios creates a new test scenarios helper
func NewDynamORMTestScenarios(helper *DynamORMTestHelper) *DynamORMTestScenarios {
	return &DynamORMTestScenarios{helper: helper}
}

// TestMultiTenantAccess tests multi-tenant data isolation
func (s *DynamORMTestScenarios) TestMultiTenantAccess(t *testing.T, tableName string) {
	// Create items for different tenants
	tenant1Items := []map[string]types.AttributeValue{
		{
			"pk": &types.AttributeValueMemberS{Value: "TENANT#abc123"},
			"sk": &types.AttributeValueMemberS{Value: "USER#1"},
			"data": &types.AttributeValueMemberS{Value: "Tenant ABC User 1"},
		},
		{
			"pk": &types.AttributeValueMemberS{Value: "TENANT#abc123"},
			"sk": &types.AttributeValueMemberS{Value: "USER#2"},
			"data": &types.AttributeValueMemberS{Value: "Tenant ABC User 2"},
		},
	}

	tenant2Items := []map[string]types.AttributeValue{
		{
			"pk": &types.AttributeValueMemberS{Value: "TENANT#xyz789"},
			"sk": &types.AttributeValueMemberS{Value: "USER#1"},
			"data": &types.AttributeValueMemberS{Value: "Tenant XYZ User 1"},
		},
	}

	// Insert items
	for _, item := range append(tenant1Items, tenant2Items...) {
		s.helper.PutTestItem(t, tableName, item)
	}

	// Query tenant 1 items
	ctx := context.Background()
	resp, err := s.helper.DynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "TENANT#abc123"},
		},
	})

	if err != nil {
		t.Fatalf("Failed to query tenant items: %v", err)
	}

	// Verify only tenant 1 items are returned
	if len(resp.Items) != 2 {
		t.Errorf("Expected 2 items for tenant abc123, got %d", len(resp.Items))
	}

	// Verify tenant isolation
	for _, item := range resp.Items {
		pk := item["pk"].(*types.AttributeValueMemberS).Value
		if pk != "TENANT#abc123" {
			t.Errorf("Got item from wrong tenant: %s", pk)
		}
	}
}

// TestPaginatedQueries tests pagination handling
func (s *DynamORMTestScenarios) TestPaginatedQueries(t *testing.T, tableName string) {
	// Create many items
	for i := 0; i < 25; i++ {
		item := map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "PAGINATED"},
			"sk": &types.AttributeValueMemberS{Value: fmt.Sprintf("ITEM#%03d", i)},
			"data": &types.AttributeValueMemberS{Value: fmt.Sprintf("Item %d", i)},
		}
		s.helper.PutTestItem(t, tableName, item)
	}

	// Query with pagination
	ctx := context.Background()
	var allItems []map[string]types.AttributeValue
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		input := &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			KeyConditionExpression: aws.String("pk = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: "PAGINATED"},
			},
			Limit: aws.Int32(10), // Small page size to test pagination
		}

		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		resp, err := s.helper.DynamoClient.Query(ctx, input)
		if err != nil {
			t.Fatalf("Failed to query with pagination: %v", err)
		}

		allItems = append(allItems, resp.Items...)

		if resp.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = resp.LastEvaluatedKey
	}

	// Verify all items were retrieved
	if len(allItems) != 25 {
		t.Errorf("Expected 25 items, got %d", len(allItems))
	}
}

// TestConditionalWrites tests conditional write operations
func (s *DynamORMTestScenarios) TestConditionalWrites(t *testing.T, tableName string) {
	ctx := context.Background()
	
	// First write should succeed
	item := map[string]types.AttributeValue{
		"pk":      &types.AttributeValueMemberS{Value: "CONDITIONAL"},
		"sk":      &types.AttributeValueMemberS{Value: "TEST"},
		"version": &types.AttributeValueMemberN{Value: "1"},
		"data":    &types.AttributeValueMemberS{Value: "Initial data"},
	}

	_, err := s.helper.DynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Second write with same key should fail
	item["version"] = &types.AttributeValueMemberN{Value: "2"}
	_, err = s.helper.DynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	
	if err == nil {
		t.Error("Expected conditional write to fail, but it succeeded")
	}

	// Update with correct version should succeed
	_, err = s.helper.DynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "CONDITIONAL"},
			"sk": &types.AttributeValueMemberS{Value: "TEST"},
		},
		UpdateExpression: aws.String("SET #data = :data, #version = :newVersion"),
		ConditionExpression: aws.String("#version = :oldVersion"),
		ExpressionAttributeNames: map[string]string{
			"#data":    "data",
			"#version": "version",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":data":       &types.AttributeValueMemberS{Value: "Updated data"},
			":oldVersion": &types.AttributeValueMemberN{Value: "1"},
			":newVersion": &types.AttributeValueMemberN{Value: "2"},
		},
	})
	
	if err != nil {
		t.Errorf("Conditional update failed: %v", err)
	}
}