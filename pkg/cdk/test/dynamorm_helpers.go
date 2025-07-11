package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

// DynamORMTestHelper provides utilities for testing DynamORM-based CDK constructs
type DynamORMTestHelper struct {
	DynamoClient *dynamodb.Client
	TableName    string
	Region       string
	Endpoint     string
}

// NewDynamORMTestHelper creates a new test helper instance
// It uses DynamoDB Local if DYNAMODB_LOCAL_ENDPOINT is set, otherwise uses mock client
func NewDynamORMTestHelper(t *testing.T) *DynamORMTestHelper {
	t.Helper()

	endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	// Create config for local testing
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == dynamodb.ServiceID && endpoint != "" {
					return aws.Endpoint{
						URL: endpoint,
					}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			})),
		config.WithCredentialsProvider(GetTestCredentials(endpoint != "")),
	)
	require.NoError(t, err)

	client := dynamodb.NewFromConfig(cfg)

	return &DynamORMTestHelper{
		DynamoClient: client,
		Region:       region,
		Endpoint:     endpoint,
	}
}

// CreateTestTable creates a DynamORM-compatible table for testing
func (h *DynamORMTestHelper) CreateTestTable(t *testing.T, tableName string, opts ...TestTableOption) {
	t.Helper()

	config := &testTableConfig{
		tableName:      tableName,
		billingMode:    types.BillingModePayPerRequest,
		partitionKey:   "pk",
		sortKey:        "sk",
		ttlAttribute:   "",
		streamEnabled:  false,
		gsiDefinitions: []gsiDefinition{},
		lsiDefinitions: []lsiDefinition{},
		tags:           map[string]string{},
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	h.TableName = tableName

	// Build attribute definitions
	attrs := []types.AttributeDefinition{
		{
			AttributeName: aws.String(config.partitionKey),
			AttributeType: types.ScalarAttributeTypeS,
		},
	}
	if config.sortKey != "" {
		attrs = append(attrs, types.AttributeDefinition{
			AttributeName: aws.String(config.sortKey),
			AttributeType: types.ScalarAttributeTypeS,
		})
	}

	// Add GSI attributes
	for _, gsi := range config.gsiDefinitions {
		attrs = append(attrs, types.AttributeDefinition{
			AttributeName: aws.String(gsi.partitionKey),
			AttributeType: types.ScalarAttributeTypeS,
		})
		if gsi.sortKey != "" {
			attrs = append(attrs, types.AttributeDefinition{
				AttributeName: aws.String(gsi.sortKey),
				AttributeType: types.ScalarAttributeTypeS,
			})
		}
	}

	// Build key schema
	keySchema := []types.KeySchemaElement{
		{
			AttributeName: aws.String(config.partitionKey),
			KeyType:       types.KeyTypeHash,
		},
	}
	if config.sortKey != "" {
		keySchema = append(keySchema, types.KeySchemaElement{
			AttributeName: aws.String(config.sortKey),
			KeyType:       types.KeyTypeRange,
		})
	}

	// Build GSIs
	var gsis []types.GlobalSecondaryIndex
	for _, gsi := range config.gsiDefinitions {
		gsiKeySchema := []types.KeySchemaElement{
			{
				AttributeName: aws.String(gsi.partitionKey),
				KeyType:       types.KeyTypeHash,
			},
		}
		if gsi.sortKey != "" {
			gsiKeySchema = append(gsiKeySchema, types.KeySchemaElement{
				AttributeName: aws.String(gsi.sortKey),
				KeyType:       types.KeyTypeRange,
			})
		}

		gsis = append(gsis, types.GlobalSecondaryIndex{
			IndexName: aws.String(gsi.name),
			KeySchema: gsiKeySchema,
			Projection: &types.Projection{
				ProjectionType: types.ProjectionTypeAll,
			},
		})
	}

	// Create table input
	input := &dynamodb.CreateTableInput{
		TableName:            aws.String(tableName),
		AttributeDefinitions: attrs,
		KeySchema:            keySchema,
		BillingMode:          config.billingMode,
		Tags:                 h.buildTags(config.tags),
	}

	if len(gsis) > 0 {
		input.GlobalSecondaryIndexes = gsis
	}

	if config.streamEnabled {
		input.StreamSpecification = &types.StreamSpecification{
			StreamEnabled:  aws.Bool(true),
			StreamViewType: types.StreamViewTypeNewAndOldImages,
		}
	}

	// Create the table
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := h.DynamoClient.CreateTable(ctx, input)
	if err != nil {
		// Check if table already exists
		var resourceInUse *types.ResourceInUseException
		if !errors.As(err, &resourceInUse) {
			require.NoError(t, err, fmt.Sprintf("Failed to create table %s", tableName))
		}
	}

	// Wait for table to be active
	h.WaitForTableActive(t, tableName)

	// Set TTL if specified
	if config.ttlAttribute != "" {
		h.EnableTTL(t, tableName, config.ttlAttribute)
	}
}

// WaitForTableActive waits for a table to become active
func (h *DynamORMTestHelper) WaitForTableActive(t *testing.T, tableName string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	waiter := dynamodb.NewTableExistsWaiter(h.DynamoClient)
	err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}, 2*time.Minute)
	require.NoError(t, err)
}

// DeleteTestTable deletes a test table
func (h *DynamORMTestHelper) DeleteTestTable(t *testing.T, tableName string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := h.DynamoClient.DeleteTable(ctx, &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if !errors.As(err, &notFound) {
			require.NoError(t, err)
		}
	}
}

// EnableTTL enables TTL on a table
func (h *DynamORMTestHelper) EnableTTL(t *testing.T, tableName, ttlAttribute string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := h.DynamoClient.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(tableName),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			AttributeName: aws.String(ttlAttribute),
			Enabled:       aws.Bool(true),
		},
	})
	require.NoError(t, err)
}

// PutTestItem puts a test item into the table
func (h *DynamORMTestHelper) PutTestItem(t *testing.T, tableName string, item map[string]types.AttributeValue) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.DynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	require.NoError(t, err)
}

// GetTestItem retrieves a test item from the table
func (h *DynamORMTestHelper) GetTestItem(t *testing.T, tableName string, key map[string]types.AttributeValue) map[string]types.AttributeValue {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.DynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	require.NoError(t, err)

	return resp.Item
}

// ScanTable performs a scan on the table and returns all items
func (h *DynamORMTestHelper) ScanTable(t *testing.T, tableName string) []map[string]types.AttributeValue {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var items []map[string]types.AttributeValue
	paginator := dynamodb.NewScanPaginator(h.DynamoClient, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		require.NoError(t, err)
		items = append(items, page.Items...)
	}

	return items
}

// buildTags converts a map to DynamoDB tags
func (h *DynamORMTestHelper) buildTags(tags map[string]string) []types.Tag {
	var result []types.Tag
	for k, v := range tags {
		result = append(result, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return result
}

// Test table configuration
type testTableConfig struct {
	tableName      string
	billingMode    types.BillingMode
	partitionKey   string
	sortKey        string
	ttlAttribute   string
	streamEnabled  bool
	gsiDefinitions []gsiDefinition
	lsiDefinitions []lsiDefinition
	tags           map[string]string
}

type gsiDefinition struct {
	name         string
	partitionKey string
	sortKey      string
}

type lsiDefinition struct {
	name    string
	sortKey string
}

// TestTableOption configures a test table
type TestTableOption func(*testTableConfig)

// WithBillingMode sets the billing mode
func WithBillingMode(mode types.BillingMode) TestTableOption {
	return func(c *testTableConfig) {
		c.billingMode = mode
	}
}

// WithKeySchema sets custom key schema
func WithKeySchema(pk, sk string) TestTableOption {
	return func(c *testTableConfig) {
		c.partitionKey = pk
		c.sortKey = sk
	}
}

// WithTTL enables TTL on the specified attribute
func WithTTL(attribute string) TestTableOption {
	return func(c *testTableConfig) {
		c.ttlAttribute = attribute
	}
}

// WithStream enables DynamoDB streams
func WithStream() TestTableOption {
	return func(c *testTableConfig) {
		c.streamEnabled = true
	}
}

// WithGSI adds a global secondary index
func WithGSI(name, pk, sk string) TestTableOption {
	return func(c *testTableConfig) {
		c.gsiDefinitions = append(c.gsiDefinitions, gsiDefinition{
			name:         name,
			partitionKey: pk,
			sortKey:      sk,
		})
	}
}

// WithTags adds tags to the table
func WithTags(tags map[string]string) TestTableOption {
	return func(c *testTableConfig) {
		for k, v := range tags {
			c.tags[k] = v
		}
	}
}

// DynamORMTestItem represents a test item for DynamORM tables
type DynamORMTestItem struct {
	PK        string            ``
	SK        string            ``
	Type      string            ``
	Data      map[string]string ``
	TTL       int64             ``
	GSI1PK    string            ``
	GSI1SK    string            ``
	CreatedAt time.Time         ``
	UpdatedAt time.Time         ``
}

// CreateDynamORMItem creates a standard DynamORM test item
func CreateDynamORMItem(pk, sk, itemType string) DynamORMTestItem {
	now := time.Now()
	return DynamORMTestItem{
		PK:        pk,
		SK:        sk,
		Type:      itemType,
		Data:      make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
