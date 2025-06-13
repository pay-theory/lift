package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/dynamorm"
)

// MockDynamORM provides a mock implementation of DynamORM for testing
type MockDynamORM struct {
	mu           sync.RWMutex
	data         map[string]map[string]interface{} // table -> key -> item
	transactions map[string]*MockTransaction       // transaction ID -> transaction
	config       *dynamorm.DynamORMConfig

	// Behavior configuration
	FailOnOperation map[string]error         // operation -> error to return
	Delays          map[string]time.Duration // operation -> delay to add
}

// NewMockDynamORM creates a new mock DynamORM instance
func NewMockDynamORM() *MockDynamORM {
	return &MockDynamORM{
		data:            make(map[string]map[string]interface{}),
		transactions:    make(map[string]*MockTransaction),
		config:          dynamorm.DefaultConfig(),
		FailOnOperation: make(map[string]error),
		Delays:          make(map[string]time.Duration),
	}
}

// WithFailure configures the mock to fail on specific operations
func (m *MockDynamORM) WithFailure(operation string, err error) *MockDynamORM {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailOnOperation[operation] = err
	return m
}

// WithDelay configures the mock to add delays to operations
func (m *MockDynamORM) WithDelay(operation string, delay time.Duration) *MockDynamORM {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Delays[operation] = delay
	return m
}

// WithData pre-populates the mock with test data
func (m *MockDynamORM) WithData(table, key string, item interface{}) *MockDynamORM {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data[table] == nil {
		m.data[table] = make(map[string]interface{})
	}
	m.data[table][key] = item
	return m
}

// Get retrieves an item by key
func (m *MockDynamORM) Get(ctx context.Context, table, key string, result interface{}) error {
	if err := m.simulateOperation("get"); err != nil {
		return err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	tableData, exists := m.data[table]
	if !exists {
		return fmt.Errorf("item not found")
	}

	item, exists := tableData[key]
	if !exists {
		return fmt.Errorf("item not found")
	}

	// Marshal and unmarshal to simulate DynamoDB behavior
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, result)
}

// Put saves an item
func (m *MockDynamORM) Put(ctx context.Context, table, key string, item interface{}) error {
	if err := m.simulateOperation("put"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data[table] == nil {
		m.data[table] = make(map[string]interface{})
	}

	m.data[table][key] = item
	return nil
}

// Delete removes an item
func (m *MockDynamORM) Delete(ctx context.Context, table, key string) error {
	if err := m.simulateOperation("delete"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if tableData, exists := m.data[table]; exists {
		delete(tableData, key)
	}

	return nil
}

// Query performs a query operation
func (m *MockDynamORM) Query(ctx context.Context, table string, query *dynamorm.Query) (*dynamorm.QueryResult, error) {
	if err := m.simulateOperation("query"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	tableData, exists := m.data[table]
	if !exists {
		return &dynamorm.QueryResult{
			Items: []interface{}{},
			Count: 0,
		}, nil
	}

	// Simple implementation - return all items (for now)
	items := make([]interface{}, 0, len(tableData))
	for _, item := range tableData {
		items = append(items, item)
	}

	return &dynamorm.QueryResult{
		Items: items,
		Count: len(items),
	}, nil
}

// BeginTransaction starts a mock transaction
func (m *MockDynamORM) BeginTransaction() (*MockTransaction, error) {
	if err := m.simulateOperation("begin_transaction"); err != nil {
		return nil, err
	}

	tx := &MockTransaction{
		id:         fmt.Sprintf("tx_%d", time.Now().UnixNano()),
		mock:       m,
		committed:  false,
		rolledBack: false,
		operations: make([]TransactionOperation, 0),
	}

	m.mu.Lock()
	m.transactions[tx.id] = tx
	m.mu.Unlock()

	return tx, nil
}

// simulateOperation handles failure simulation and delays
func (m *MockDynamORM) simulateOperation(operation string) error {
	// Check for configured delays
	if delay, exists := m.Delays[operation]; exists {
		time.Sleep(delay)
	}

	// Check for configured failures
	if err, exists := m.FailOnOperation[operation]; exists {
		return err
	}

	return nil
}

// GetAllData returns all data for testing inspection
func (m *MockDynamORM) GetAllData() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy to prevent external modification
	result := make(map[string]map[string]interface{})
	for table, tableData := range m.data {
		result[table] = make(map[string]interface{})
		for key, item := range tableData {
			result[table][key] = item
		}
	}

	return result
}

// Reset clears all data and resets the mock state
func (m *MockDynamORM) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]map[string]interface{})
	m.transactions = make(map[string]*MockTransaction)
	m.FailOnOperation = make(map[string]error)
	m.Delays = make(map[string]time.Duration)
}

// MockTransaction represents a mock DynamORM transaction
type MockTransaction struct {
	id         string
	mock       *MockDynamORM
	committed  bool
	rolledBack bool
	operations []TransactionOperation
	mu         sync.RWMutex
}

// TransactionOperation represents an operation within a transaction
type TransactionOperation struct {
	Type  string // "put", "delete", etc.
	Table string
	Key   string
	Item  interface{}
}

// Put adds a put operation to the transaction
func (tx *MockTransaction) Put(ctx context.Context, table, key string, item interface{}) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed || tx.rolledBack {
		return fmt.Errorf("transaction already completed")
	}

	tx.operations = append(tx.operations, TransactionOperation{
		Type:  "put",
		Table: table,
		Key:   key,
		Item:  item,
	})

	return nil
}

// Delete adds a delete operation to the transaction
func (tx *MockTransaction) Delete(ctx context.Context, table, key string) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed || tx.rolledBack {
		return fmt.Errorf("transaction already completed")
	}

	tx.operations = append(tx.operations, TransactionOperation{
		Type:  "delete",
		Table: table,
		Key:   key,
	})

	return nil
}

// Commit commits the transaction
func (tx *MockTransaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed || tx.rolledBack {
		return fmt.Errorf("transaction already completed")
	}

	// Apply all operations atomically
	for _, op := range tx.operations {
		switch op.Type {
		case "put":
			tx.mock.mu.Lock()
			if tx.mock.data[op.Table] == nil {
				tx.mock.data[op.Table] = make(map[string]interface{})
			}
			tx.mock.data[op.Table][op.Key] = op.Item
			tx.mock.mu.Unlock()
		case "delete":
			tx.mock.mu.Lock()
			if tableData, exists := tx.mock.data[op.Table]; exists {
				delete(tableData, op.Key)
			}
			tx.mock.mu.Unlock()
		}
	}

	tx.committed = true
	return nil
}

// Rollback rolls back the transaction
func (tx *MockTransaction) Rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.committed || tx.rolledBack {
		return fmt.Errorf("transaction already completed")
	}

	// Simply mark as rolled back - no operations to undo since they weren't applied
	tx.rolledBack = true
	return nil
}

// MockAWSService provides a generic mock for AWS services
type MockAWSService struct {
	mu        sync.RWMutex
	responses map[string]interface{} // operation -> response
	errors    map[string]error       // operation -> error
	callCount map[string]int         // operation -> call count
}

// NewMockAWSService creates a new mock AWS service
func NewMockAWSService() *MockAWSService {
	return &MockAWSService{
		responses: make(map[string]interface{}),
		errors:    make(map[string]error),
		callCount: make(map[string]int),
	}
}

// WithResponse configures a response for an operation
func (m *MockAWSService) WithResponse(operation string, response interface{}) *MockAWSService {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[operation] = response
	return m
}

// WithError configures an error for an operation
func (m *MockAWSService) WithError(operation string, err error) *MockAWSService {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[operation] = err
	return m
}

// Call simulates calling an AWS service operation
func (m *MockAWSService) Call(operation string, input interface{}) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track call count
	m.callCount[operation]++

	// Return configured error if exists
	if err, exists := m.errors[operation]; exists {
		return nil, err
	}

	// Return configured response if exists
	if response, exists := m.responses[operation]; exists {
		return response, nil
	}

	// Default empty response
	return nil, nil
}

// GetCallCount returns the number of times an operation was called
func (m *MockAWSService) GetCallCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[operation]
}

// Reset clears all mock state
func (m *MockAWSService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses = make(map[string]interface{})
	m.errors = make(map[string]error)
	m.callCount = make(map[string]int)
}

// MockHTTPClient provides a mock HTTP client for external API testing
type MockHTTPClient struct {
	mu        sync.RWMutex
	responses map[string]*MockHTTPResponse // URL -> response
	callCount map[string]int               // URL -> call count
}

// MockHTTPResponse represents a mock HTTP response
type MockHTTPResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Delay      time.Duration
}

// NewMockHTTPClient creates a new mock HTTP client
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*MockHTTPResponse),
		callCount: make(map[string]int),
	}
}

// WithResponse configures a response for a URL
func (m *MockHTTPClient) WithResponse(url string, response *MockHTTPResponse) *MockHTTPClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[url] = response
	return m
}

// Get simulates an HTTP GET request
func (m *MockHTTPClient) Get(url string) (*MockHTTPResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount[url]++

	response, exists := m.responses[url]
	if !exists {
		return &MockHTTPResponse{
			StatusCode: 404,
			Body:       "Not Found",
			Headers:    make(map[string]string),
		}, nil
	}

	// Simulate delay if configured
	if response.Delay > 0 {
		time.Sleep(response.Delay)
	}

	return response, nil
}

// Reset clears all mock state
func (m *MockHTTPClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses = make(map[string]*MockHTTPResponse)
	m.callCount = make(map[string]int)
}

// MockDynamORMWrapper wraps MockDynamORM to match the DynamORMWrapper interface
type MockDynamORMWrapper struct {
	mock      *MockDynamORM
	tableName string
	tenantID  string
}

// NewMockDynamORMWrapper creates a wrapper that matches DynamORMWrapper interface
func NewMockDynamORMWrapper(tableName string) *MockDynamORMWrapper {
	return &MockDynamORMWrapper{
		mock:      NewMockDynamORM(),
		tableName: tableName,
	}
}

// WithTenant creates a tenant-scoped wrapper
func (w *MockDynamORMWrapper) WithTenant(tenantID string) *MockDynamORMWrapper {
	return &MockDynamORMWrapper{
		mock:      w.mock,
		tableName: w.tableName,
		tenantID:  tenantID,
	}
}

// Get retrieves an item by key (matches DynamORMWrapper interface)
func (w *MockDynamORMWrapper) Get(ctx context.Context, key interface{}, result interface{}) error {
	keyStr := fmt.Sprintf("%v", key)
	return w.mock.Get(ctx, w.tableName, keyStr, result)
}

// Put saves an item (matches DynamORMWrapper interface)
func (w *MockDynamORMWrapper) Put(ctx context.Context, item interface{}) error {
	// Extract ID from item for the key
	keyStr := w.extractID(item)
	return w.mock.Put(ctx, w.tableName, keyStr, item)
}

// Delete removes an item (matches DynamORMWrapper interface)
func (w *MockDynamORMWrapper) Delete(ctx context.Context, key interface{}) error {
	keyStr := fmt.Sprintf("%v", key)
	return w.mock.Delete(ctx, w.tableName, keyStr)
}

// Query performs a query operation (matches DynamORMWrapper interface)
func (w *MockDynamORMWrapper) Query(ctx context.Context, query interface{}) (interface{}, error) {
	// Simple implementation for testing
	return w.mock.Query(ctx, w.tableName, nil)
}

// BeginTransaction starts a mock transaction
func (w *MockDynamORMWrapper) BeginTransaction() (interface{}, error) {
	return w.mock.BeginTransaction()
}

// extractID extracts the ID field from an item using reflection or type assertion
func (w *MockDynamORMWrapper) extractID(item interface{}) string {
	// Try to extract ID field using type assertion for common types
	switch v := item.(type) {
	case map[string]interface{}:
		if id, ok := v["id"].(string); ok {
			return id
		}
		if id, ok := v["ID"].(string); ok {
			return id
		}
	}

	// For struct types, try to access ID field via interface
	if idGetter, ok := item.(interface{ GetID() string }); ok {
		return idGetter.GetID()
	}

	// Fallback to string representation
	return fmt.Sprintf("%v", item)
}

// WithData pre-populates the mock with test data
func (w *MockDynamORMWrapper) WithData(key string, item interface{}) *MockDynamORMWrapper {
	w.mock.WithData(w.tableName, key, item)
	return w
}

// WithFailure configures the mock to fail on specific operations
func (w *MockDynamORMWrapper) WithFailure(operation string, err error) *MockDynamORMWrapper {
	w.mock.WithFailure(operation, err)
	return w
}

// GetAllData returns all data for testing inspection
func (w *MockDynamORMWrapper) GetAllData() map[string]map[string]interface{} {
	return w.mock.GetAllData()
}
