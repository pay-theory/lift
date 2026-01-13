package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/dynamorm"
)

type MockDynamORM struct {
	data            map[string]map[string]any
	transactions    map[string]*MockTransaction
	FailOnOperation map[string]error
	Delays          map[string]time.Duration
	config          *dynamorm.DynamORMConfig
	mu              sync.RWMutex
}

// NewMockDynamORM creates a new mock DynamORM instance
func NewMockDynamORM() *MockDynamORM {
	return &MockDynamORM{
		data:            make(map[string]map[string]any),
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
func (m *MockDynamORM) WithData(table, key string, item any) *MockDynamORM {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data[table] == nil {
		m.data[table] = make(map[string]any)
	}
	m.data[table][key] = item
	return m
}

// Get retrieves an item by key
func (m *MockDynamORM) Get(_ context.Context, table, key string, result any) error {
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
func (m *MockDynamORM) Put(_ context.Context, table, key string, item any) error {
	if err := m.simulateOperation("put"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data[table] == nil {
		m.data[table] = make(map[string]any)
	}

	m.data[table][key] = item
	return nil
}

// Delete removes an item
func (m *MockDynamORM) Delete(_ context.Context, table, key string) error {
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
func (m *MockDynamORM) Query(_ context.Context, table string, _ *dynamorm.Query) (*dynamorm.QueryResult, error) {
	if err := m.simulateOperation("query"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	tableData, exists := m.data[table]
	if !exists {
		return &dynamorm.QueryResult{
			Items: []any{},
			Count: 0,
		}, nil
	}

	// Simple implementation - return all items (for now)
	items := make([]any, 0, len(tableData))
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
func (m *MockDynamORM) GetAllData() map[string]map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy to prevent external modification
	result := make(map[string]map[string]any)
	for table, tableData := range m.data {
		result[table] = make(map[string]any)
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

	m.data = make(map[string]map[string]any)
	m.transactions = make(map[string]*MockTransaction)
	m.FailOnOperation = make(map[string]error)
	m.Delays = make(map[string]time.Duration)
}

// MockTransaction represents a mock DynamORM transaction
// Memory optimized for better alignment
type MockTransaction struct {
	mock       *MockDynamORM
	id         string
	operations []TransactionOperation
	mu         sync.RWMutex
	committed  bool
	rolledBack bool
}

// TransactionOperation represents an operation within a transaction
// Memory optimized for better alignment
type TransactionOperation struct {
	// Interface first (24 bytes)
	Item any
	// Strings (16 bytes each)
	Type  string
	Table string
	Key   string
}

// Put adds a put operation to the transaction
func (tx *MockTransaction) Put(_ context.Context, table, key string, item any) error {
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
func (tx *MockTransaction) Delete(_ context.Context, table, key string) error {
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
				tx.mock.data[op.Table] = make(map[string]any)
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
// Memory optimized for better alignment
