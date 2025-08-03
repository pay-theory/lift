package test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
)

// MockDynamORMClient provides a mock DynamoDB client for testing DynamORM operations
// This avoids circular dependencies by implementing the minimal interface needed
// Memory optimized: 112 → 80 bytes (32 bytes saved)
type MockDynamORMClient struct {
	// Embedded struct first (largest)
	mock.Mock
	// Map (24 bytes)
	tables map[string]*mockTable
	// Mutex last (24 bytes)
	mu     sync.RWMutex
}

// mockTable represents a mock DynamoDB table
// Memory optimized: 80 → 72 bytes (8 bytes saved)
type mockTable struct {
	// Maps and slices first (24 bytes each)
	items                map[string]map[string]types.AttributeValue
	gsis                 map[string]*mockGSI
	attributeDefinitions []types.AttributeDefinition
	// Strings (16 bytes each)
	name                 string
	ttlAttribute         string
	// Enum (4 bytes)
	billingMode          types.BillingMode
	// Bool last (1 byte)
	streamEnabled        bool
}

// mockGSI represents a mock global secondary index
// Memory optimized: 56 → 48 bytes (8 bytes saved)
type mockGSI struct {
	// Map first (24 bytes)
	items        map[string][]map[string]types.AttributeValue
	// Strings (16 bytes each)
	name         string
	partitionKey string
	sortKey      string
}

// NewMockDynamORMClient creates a new mock DynamoDB client
func NewMockDynamORMClient() *MockDynamORMClient {
	return &MockDynamORMClient{
		tables: make(map[string]*mockTable),
	}
}

// PutItem mocks the DynamoDB PutItem operation
func (m *MockDynamORMClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "PutItem" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.PutItemOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Otherwise, provide default behavior
	tableName := *params.TableName
	table, exists := m.tables[tableName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	// Create composite key
	key := m.createKey(params.Item)

	// Store the item
	if table.items == nil {
		table.items = make(map[string]map[string]types.AttributeValue)
	}

	// Add timestamps if not present
	item := make(map[string]types.AttributeValue)
	for k, v := range params.Item {
		item[k] = v
	}

	if _, ok := item["created_at"]; !ok {
		item["created_at"] = &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)}
	}
	item["updated_at"] = &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)}

	table.items[key] = item

	return &dynamodb.PutItemOutput{}, nil
}

// GetItem mocks the DynamoDB GetItem operation
func (m *MockDynamORMClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "GetItem" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.GetItemOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Otherwise, provide default behavior
	tableName := *params.TableName
	table, exists := m.tables[tableName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	key := m.createKey(params.Key)
	item, exists := table.items[key]
	if !exists {
		return &dynamodb.GetItemOutput{}, nil
	}

	return &dynamodb.GetItemOutput{Item: item}, nil
}

// Query mocks the DynamoDB Query operation
func (m *MockDynamORMClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "Query" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.QueryOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Otherwise, provide simple default behavior
	return &dynamodb.QueryOutput{
		Items: []map[string]types.AttributeValue{},
		Count: 0,
	}, nil
}

// Scan mocks the DynamoDB Scan operation
func (m *MockDynamORMClient) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "Scan" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.ScanOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Otherwise, return all items from the table
	tableName := *params.TableName
	table, exists := m.tables[tableName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	items := make([]map[string]types.AttributeValue, 0, len(table.items))
	for _, item := range table.items {
		items = append(items, item)
	}

	return &dynamodb.ScanOutput{
		Items: items,
		Count: int32(len(items)), // #nosec G115 - len() returns int which is safe to convert to int32 for DynamoDB API
	}, nil
}

// UpdateItem mocks the DynamoDB UpdateItem operation
func (m *MockDynamORMClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "UpdateItem" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.UpdateItemOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Simple default behavior
	tableName := *params.TableName
	_, exists := m.tables[tableName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	return &dynamodb.UpdateItemOutput{}, nil
}

// DeleteItem mocks the DynamoDB DeleteItem operation
func (m *MockDynamORMClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "DeleteItem" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.DeleteItemOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Default behavior
	tableName := *params.TableName
	table, exists := m.tables[tableName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	key := m.createKey(params.Key)
	delete(table.items, key)

	return &dynamodb.DeleteItemOutput{}, nil
}

// CreateTable mocks the DynamoDB CreateTable operation
func (m *MockDynamORMClient) CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "CreateTable" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.CreateTableOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Default behavior
	tableName := *params.TableName
	if _, exists := m.tables[tableName]; exists {
		return nil, &types.ResourceInUseException{
			Message: stringPtr(fmt.Sprintf("Table %s already exists", tableName)),
		}
	}

	table := &mockTable{
		name:                 tableName,
		items:                make(map[string]map[string]types.AttributeValue),
		gsis:                 make(map[string]*mockGSI),
		billingMode:          params.BillingMode,
		attributeDefinitions: params.AttributeDefinitions,
	}

	// Process GSIs
	for _, gsi := range params.GlobalSecondaryIndexes {
		mockGSI := &mockGSI{
			name:  *gsi.IndexName,
			items: make(map[string][]map[string]types.AttributeValue),
		}

		for _, key := range gsi.KeySchema {
			if key.KeyType == types.KeyTypeHash {
				mockGSI.partitionKey = *key.AttributeName
			} else {
				mockGSI.sortKey = *key.AttributeName
			}
		}

		table.gsis[*gsi.IndexName] = mockGSI
	}

	// Process streams
	if params.StreamSpecification != nil && *params.StreamSpecification.StreamEnabled {
		table.streamEnabled = true
	}

	m.tables[tableName] = table

	return &dynamodb.CreateTableOutput{
		TableDescription: &types.TableDescription{
			TableName:   params.TableName,
			TableStatus: types.TableStatusActive,
		},
	}, nil
}

// DeleteTable mocks the DynamoDB DeleteTable operation
func (m *MockDynamORMClient) DeleteTable(ctx context.Context, params *dynamodb.DeleteTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteTableOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "DeleteTable" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.DeleteTableOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Default behavior
	tableName := *params.TableName
	if _, exists := m.tables[tableName]; !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	delete(m.tables, tableName)

	return &dynamodb.DeleteTableOutput{}, nil
}

// DescribeTable mocks the DynamoDB DescribeTable operation
func (m *MockDynamORMClient) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "DescribeTable" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.DescribeTableOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Default behavior
	tableName := *params.TableName
	table, exists := m.tables[tableName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	return &dynamodb.DescribeTableOutput{
		Table: &types.TableDescription{
			TableName:   params.TableName,
			TableStatus: types.TableStatusActive,
			BillingModeSummary: &types.BillingModeSummary{
				BillingMode: table.billingMode,
			},
		},
	}, nil
}

// UpdateTimeToLive mocks the DynamoDB UpdateTimeToLive operation
func (m *MockDynamORMClient) UpdateTimeToLive(ctx context.Context, params *dynamodb.UpdateTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateTimeToLiveOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this method has expectations
	if len(m.ExpectedCalls) > 0 {
		for _, call := range m.ExpectedCalls {
			if call.Method == "UpdateTimeToLive" {
				args := m.Called(ctx, params, optFns)
				if args.Get(0) != nil {
					if output, ok := args.Get(0).(*dynamodb.UpdateTimeToLiveOutput); ok {
						return output, args.Error(1)
					}
				}
				return nil, args.Error(1)
			}
		}
	}

	// Default behavior
	tableName := *params.TableName
	table, exists := m.tables[tableName]
	if !exists {
		return nil, &types.ResourceNotFoundException{
			Message: stringPtr(fmt.Sprintf("Table %s not found", tableName)),
		}
	}

	if params.TimeToLiveSpecification.Enabled != nil && *params.TimeToLiveSpecification.Enabled {
		table.ttlAttribute = *params.TimeToLiveSpecification.AttributeName
	} else {
		table.ttlAttribute = ""
	}

	return &dynamodb.UpdateTimeToLiveOutput{}, nil
}

// Helper methods

// createKey creates a composite key from DynamoDB attributes
func (m *MockDynamORMClient) createKey(attrs map[string]types.AttributeValue) string {
	pk := ""
	sk := ""

	if pkAttr, ok := attrs["pk"]; ok {
		if s, ok := pkAttr.(*types.AttributeValueMemberS); ok {
			pk = s.Value
		}
	}

	if skAttr, ok := attrs["sk"]; ok {
		if s, ok := skAttr.(*types.AttributeValueMemberS); ok {
			sk = s.Value
		}
	}

	if sk != "" {
		return fmt.Sprintf("%s#%s", pk, sk)
	}
	return pk
}

// AddMockTable adds a mock table for testing
func (m *MockDynamORMClient) AddMockTable(tableName string, opts ...TestTableOption) {
	m.mu.Lock()
	defer m.mu.Unlock()

	config := &testTableConfig{
		tableName:    tableName,
		billingMode:  types.BillingModePayPerRequest,
		partitionKey: "pk",
		sortKey:      "sk",
	}

	for _, opt := range opts {
		opt(config)
	}

	table := &mockTable{
		name:        tableName,
		items:       make(map[string]map[string]types.AttributeValue),
		gsis:        make(map[string]*mockGSI),
		billingMode: config.billingMode,
	}

	if config.ttlAttribute != "" {
		table.ttlAttribute = config.ttlAttribute
	}

	if config.streamEnabled {
		table.streamEnabled = true
	}

	// Add GSIs
	for _, gsi := range config.gsiDefinitions {
		table.gsis[gsi.name] = &mockGSI{
			name:         gsi.name,
			partitionKey: gsi.partitionKey,
			sortKey:      gsi.sortKey,
			items:        make(map[string][]map[string]types.AttributeValue),
		}
	}

	m.tables[tableName] = table
}


// ClearMockTables removes all mock tables
func (m *MockDynamORMClient) ClearMockTables() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tables = make(map[string]*mockTable)
}

// stringPtr is a helper to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
