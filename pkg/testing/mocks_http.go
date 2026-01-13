package testing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MockHTTPClient struct {
	responses map[string]*MockHTTPResponse
	callCount map[string]int
	mu        sync.RWMutex
}

// MockHTTPResponse represents a mock HTTP response
// Memory optimized for better alignment
type MockHTTPResponse struct {
	Headers    map[string]string
	Body       string
	Delay      time.Duration
	StatusCode int
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
func (w *MockDynamORMWrapper) Get(ctx context.Context, key any, result any) error {
	keyStr := fmt.Sprintf("%v", key)
	return w.mock.Get(ctx, w.tableName, keyStr, result)
}

// Put saves an item (matches DynamORMWrapper interface)
func (w *MockDynamORMWrapper) Put(ctx context.Context, item any) error {
	// Extract ID from item for the key
	keyStr := w.extractID(item)
	return w.mock.Put(ctx, w.tableName, keyStr, item)
}

// Delete removes an item (matches DynamORMWrapper interface)
func (w *MockDynamORMWrapper) Delete(ctx context.Context, key any) error {
	keyStr := fmt.Sprintf("%v", key)
	return w.mock.Delete(ctx, w.tableName, keyStr)
}

// Query performs a query operation (matches DynamORMWrapper interface)
func (w *MockDynamORMWrapper) Query(ctx context.Context, _ any) (any, error) {
	// Simple implementation for testing
	return w.mock.Query(ctx, w.tableName, nil)
}

// BeginTransaction starts a mock transaction
func (w *MockDynamORMWrapper) BeginTransaction() (any, error) {
	return w.mock.BeginTransaction()
}

// extractID extracts the ID field from an item using reflection or type assertion
func (w *MockDynamORMWrapper) extractID(item any) string {
	// Try to extract ID field using type assertion for common types
	if v, ok := item.(map[string]any); ok {
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
func (w *MockDynamORMWrapper) WithData(key string, item any) *MockDynamORMWrapper {
	w.mock.WithData(w.tableName, key, item)
	return w
}

// WithFailure configures the mock to fail on specific operations
func (w *MockDynamORMWrapper) WithFailure(operation string, err error) *MockDynamORMWrapper {
	w.mock.WithFailure(operation, err)
	return w
}

// GetAllData returns all data for testing inspection
func (w *MockDynamORMWrapper) GetAllData() map[string]map[string]any {
	return w.mock.GetAllData()
}

// =============================================================================
// API Gateway Management API Mocks
// =============================================================================

// ConnectionState represents the state of a WebSocket connection
type ConnectionState string

const (
	ConnectionStateActive       ConnectionState = "ACTIVE"
	ConnectionStateDisconnected ConnectionState = "DISCONNECTED"
	ConnectionStateStale        ConnectionState = "STALE"
)

// MockConnection represents a WebSocket connection in the mock
type MockConnection struct {
	CreatedAt    time.Time       `json:"created_at"`
	LastActiveAt time.Time       `json:"last_active_at"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
	ID           string          `json:"id"`
	State        ConnectionState `json:"state"`
	SourceIP     string          `json:"source_ip"`
	UserAgent    string          `json:"user_agent"`
}

// MockAPIGatewayConfig configures the behavior of the API Gateway mock
type MockAPIGatewayConfig struct {
	ErrorRates     map[string]float64
	ConnectionTTL  int64
	MaxMessageSize int64
	NetworkDelay   time.Duration
}

// DefaultMockAPIGatewayConfig returns default configuration
func DefaultMockAPIGatewayConfig() *MockAPIGatewayConfig {
	return &MockAPIGatewayConfig{
		ConnectionTTL:  7200,   // 2 hours
		MaxMessageSize: 131072, // 128KB
		NetworkDelay:   0,
		ErrorRates:     make(map[string]float64),
	}
}

// MockAPIGatewayManagementClient provides a mock implementation of API Gateway Management API
