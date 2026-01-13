package testing

import (
	"sync"
)

type MockAWSService struct {
	responses map[string]any
	errors    map[string]error
	callCount map[string]int
	mu        sync.RWMutex
}

// NewMockAWSService creates a new mock AWS service
func NewMockAWSService() *MockAWSService {
	return &MockAWSService{
		responses: make(map[string]any),
		errors:    make(map[string]error),
		callCount: make(map[string]int),
	}
}

// WithResponse configures a response for an operation
func (m *MockAWSService) WithResponse(operation string, response any) *MockAWSService {
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
func (m *MockAWSService) Call(operation string, _ any) (any, error) {
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

	m.responses = make(map[string]any)
	m.errors = make(map[string]error)
	m.callCount = make(map[string]int)
}

// MockHTTPClient provides a mock HTTP client for external API testing
// Memory optimized for better alignment
