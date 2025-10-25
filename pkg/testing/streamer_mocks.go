package testing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/streamer"
)

// =============================================================================
// Streamer API Gateway Client Mock
// =============================================================================

// This mock implements the streamer.Client interface for testing purposes.

// StreamerMockConnection represents a WebSocket connection in the mock.
type StreamerMockConnection struct {
	CreatedTime  time.Time
	Metadata     map[string]any
	ConnectionID string
	ConnectedAt  time.Time
	LastActiveAt time.Time
	SourceIP     string
	UserAgent    string
	State        streamer.ConnectionState
}

// ToConnectionInfo converts a mock connection to a streamer.ConnectionInfo.
func (c *StreamerMockConnection) ToConnectionInfo() *streamer.ConnectionInfo {
	return &streamer.ConnectionInfo{
		ConnectionID: c.ConnectionID,
		ConnectedAt:  c.ConnectedAt,
		LastActiveAt: c.LastActiveAt,
		SourceIP:     c.SourceIP,
		UserAgent:    c.UserAgent,
		Identity:     c.Metadata,
	}
}

// StreamerMockConfig configures the behavior of the streamer mock.
type StreamerMockConfig struct {
	DefaultSourceIP  string
	DefaultUserAgent string
	ConnectionTTL    int64
	MaxMessageSize   int64
	NetworkDelay     time.Duration
}

// DefaultStreamerMockConfig returns default configuration for streamer mocks.
func DefaultStreamerMockConfig() *StreamerMockConfig {
	return &StreamerMockConfig{
		ConnectionTTL:    7200,   // 2 hours
		MaxMessageSize:   131072, // 128KB
		NetworkDelay:     0,
		DefaultSourceIP:  "127.0.0.1",
		DefaultUserAgent: "MockClient/1.0",
	}
}

// StreamerClientMock implements the streamer.Client interface for testing.
type StreamerClientMock struct {
	connections map[string]*StreamerMockConnection
	messages    map[string][][]byte
	errors      map[string]error
	callCount   map[string]int
	config      *StreamerMockConfig
	mu          sync.RWMutex
}

// NewStreamerClientMock creates a new streamer client mock.
func NewStreamerClientMock() *StreamerClientMock {
	return &StreamerClientMock{
		connections: make(map[string]*StreamerMockConnection),
		messages:    make(map[string][][]byte),
		errors:      make(map[string]error),
		callCount:   make(map[string]int),
		config:      DefaultStreamerMockConfig(),
	}
}

// Ensure StreamerClientMock implements streamer.Client interface
var _ streamer.Client = (*StreamerClientMock)(nil)

// WithConfig sets the mock configuration.
func (m *StreamerClientMock) WithConfig(config *StreamerMockConfig) *StreamerClientMock {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return m
}

// WithConnection adds a connection to the mock.
func (m *StreamerClientMock) WithConnection(connectionID string, conn *StreamerMockConnection) *StreamerClientMock {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn == nil {
		now := time.Now()
		conn = &StreamerMockConnection{
			ConnectionID: connectionID,
			ConnectedAt:  now,
			LastActiveAt: now,
			SourceIP:     m.config.DefaultSourceIP,
			UserAgent:    m.config.DefaultUserAgent,
			State:        streamer.ConnectionStateActive,
			CreatedTime:  now,
			Metadata:     make(map[string]any),
		}
	}

	m.connections[connectionID] = conn
	return m
}

// WithError configures an error for a specific connection.
func (m *StreamerClientMock) WithError(connectionID string, err error) *StreamerClientMock {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[connectionID] = err
	return m
}

// PostToConnection sends data to a WebSocket connection.
// Implements streamer.Client interface
func (m *StreamerClientMock) PostToConnection(_ context.Context, connectionID string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track call count
	m.callCount["PostToConnection"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors[connectionID]; exists {
		return err
	}

	// Check if connection exists
	conn, exists := m.connections[connectionID]
	if !exists {
		return streamer.GoneError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("Connection %s not found", connectionID),
		}
	}

	// Check connection state
	if conn.State != streamer.ConnectionStateActive {
		return streamer.GoneError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("Connection %s is not active", connectionID),
		}
	}

	// Check if connection is stale (TTL expired)
	if time.Since(conn.CreatedTime).Seconds() > float64(m.config.ConnectionTTL) {
		conn.State = streamer.ConnectionStateStale
		return streamer.GoneError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("Connection %s has expired", connectionID),
		}
	}

	// Check message size
	if int64(len(data)) > m.config.MaxMessageSize {
		return streamer.PayloadTooLargeError{
			ConnectionID: connectionID,
			PayloadSize:  len(data),
			MaxSize:      int(m.config.MaxMessageSize),
			Message:      fmt.Sprintf("Message size %d exceeds maximum %d", len(data), m.config.MaxMessageSize),
		}
	}

	// Store the message
	m.messages[connectionID] = append(m.messages[connectionID], data)

	// Update last active time
	now := time.Now()
	conn.LastActiveAt = now

	return nil
}

// DeleteConnection terminates a WebSocket connection.
// Implements streamer.Client interface
func (m *StreamerClientMock) DeleteConnection(_ context.Context, connectionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track call count
	m.callCount["DeleteConnection"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors[connectionID]; exists {
		return err
	}

	// Check if connection exists
	conn, exists := m.connections[connectionID]
	if !exists {
		return streamer.GoneError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("Connection %s not found", connectionID),
		}
	}

	// Mark connection as disconnected
	conn.State = streamer.ConnectionStateDisconnected
	now := time.Now()
	conn.LastActiveAt = now

	return nil
}

// GetConnection retrieves connection information.
// Implements streamer.Client interface
func (m *StreamerClientMock) GetConnection(_ context.Context, connectionID string) (*streamer.ConnectionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Track call count
	m.callCount["GetConnection"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors[connectionID]; exists {
		return nil, err
	}

	// Check if connection exists
	conn, exists := m.connections[connectionID]
	if !exists {
		return nil, streamer.GoneError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("Connection %s not found", connectionID),
		}
	}

	// Return connection info using the streamer.ConnectionInfo type
	return conn.ToConnectionInfo(), nil
}

// =============================================================================
// Helper Methods for Testing
// =============================================================================

// GetMessages returns all messages sent to a connection.
func (m *StreamerClientMock) GetMessages(connectionID string) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages, exists := m.messages[connectionID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([][]byte, len(messages))
	for i, msg := range messages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}

	return result
}

// GetMessageCount returns the number of messages sent to a connection.
func (m *StreamerClientMock) GetMessageCount(connectionID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages[connectionID])
}

// GetCallCount returns the number of times an operation was called.
func (m *StreamerClientMock) GetCallCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[operation]
}

// GetConnectionState returns a copy of the connection state.
func (m *StreamerClientMock) GetConnectionState(connectionID string) *StreamerMockConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[connectionID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	connCopy := *conn
	if conn.Metadata != nil {
		connCopy.Metadata = make(map[string]any)
		for k, v := range conn.Metadata {
			connCopy.Metadata[k] = v
		}
	}

	return &connCopy
}

// GetActiveConnections returns all active connections.
func (m *StreamerClientMock) GetActiveConnections() map[string]*StreamerMockConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := make(map[string]*StreamerMockConnection)
	for id, conn := range m.connections {
		if conn.State == streamer.ConnectionStateActive {
			// Return a copy
			connCopy := *conn
			if conn.Metadata != nil {
				connCopy.Metadata = make(map[string]any)
				for k, v := range conn.Metadata {
					connCopy.Metadata[k] = v
				}
			}
			active[id] = &connCopy
		}
	}

	return active
}

// Reset clears all mock state.
func (m *StreamerClientMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connections = make(map[string]*StreamerMockConnection)
	m.messages = make(map[string][][]byte)
	m.errors = make(map[string]error)
	m.callCount = make(map[string]int)
	m.config = DefaultStreamerMockConfig()
}

// SimulateConnectionExpiry marks connections as stale based on TTL.
func (m *StreamerClientMock) SimulateConnectionExpiry() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, conn := range m.connections {
		if conn.State == streamer.ConnectionStateActive &&
			now.Sub(conn.CreatedTime).Seconds() > float64(m.config.ConnectionTTL) {
			conn.State = streamer.ConnectionStateStale
		}
	}
}

// =============================================================================
// Error Helper Functions
// =============================================================================

// WithGoneError configures a GoneError for a specific connection.
func (m *StreamerClientMock) WithGoneError(connectionID, message string) *StreamerClientMock {
	return m.WithError(connectionID, streamer.GoneError{
		ConnectionID: connectionID,
		Message:      message,
	})
}

// WithForbiddenError configures a ForbiddenError for a specific connection.
func (m *StreamerClientMock) WithForbiddenError(connectionID, message string) *StreamerClientMock {
	return m.WithError(connectionID, streamer.ForbiddenError{
		ConnectionID: connectionID,
		Message:      message,
	})
}

// WithPayloadTooLargeError configures a PayloadTooLargeError for a specific connection.
func (m *StreamerClientMock) WithPayloadTooLargeError(connectionID string, payloadSize, maxSize int, message string) *StreamerClientMock {
	return m.WithError(connectionID, streamer.PayloadTooLargeError{
		ConnectionID: connectionID,
		PayloadSize:  payloadSize,
		MaxSize:      maxSize,
		Message:      message,
	})
}

// WithThrottlingError configures a ThrottlingError for a specific connection.
func (m *StreamerClientMock) WithThrottlingError(connectionID string, retryAfter int, message string) *StreamerClientMock {
	return m.WithError(connectionID, streamer.ThrottlingError{
		ConnectionID: connectionID,
		RetryAfter:   retryAfter,
		Message:      message,
	})
}

// WithInternalServerError configures an InternalServerError for a specific connection.
func (m *StreamerClientMock) WithInternalServerError(connectionID, message string) *StreamerClientMock {
	return m.WithError(connectionID, streamer.InternalServerError{
		Message: message,
	})
}
