package testing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MockAPIGatewayManagementClient struct {
	connections map[string]*MockConnection
	messages    map[string][][]byte
	errors      map[string]error
	callCount   map[string]int
	config      *MockAPIGatewayConfig
	mu          sync.RWMutex
}

// NewMockAPIGatewayManagementClient creates a new mock API Gateway Management client
func NewMockAPIGatewayManagementClient() *MockAPIGatewayManagementClient {
	return &MockAPIGatewayManagementClient{
		connections: make(map[string]*MockConnection),
		messages:    make(map[string][][]byte),
		errors:      make(map[string]error),
		callCount:   make(map[string]int),
		config:      DefaultMockAPIGatewayConfig(),
	}
}

// WithConfig sets the mock configuration
func (m *MockAPIGatewayManagementClient) WithConfig(config *MockAPIGatewayConfig) *MockAPIGatewayManagementClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return m
}

// WithConnection adds a connection to the mock
func (m *MockAPIGatewayManagementClient) WithConnection(connectionID string, conn *MockConnection) *MockAPIGatewayManagementClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn == nil {
		conn = &MockConnection{
			ID:           connectionID,
			State:        ConnectionStateActive,
			CreatedAt:    time.Now(),
			LastActiveAt: time.Now(),
			SourceIP:     "127.0.0.1",
			UserAgent:    "MockClient/1.0",
			Metadata:     make(map[string]any),
		}
	}

	m.connections[connectionID] = conn
	return m
}

// WithError configures an error for a specific connection
func (m *MockAPIGatewayManagementClient) WithError(connectionID string, err error) *MockAPIGatewayManagementClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[connectionID] = err
	return m
}

// PostToConnection sends data to a WebSocket connection
func (m *MockAPIGatewayManagementClient) PostToConnection(_ context.Context, connectionID string, data []byte) error {
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
		return &MockAPIGatewayError{
			Code:       "GoneException",
			Message:    fmt.Sprintf("Connection %s not found", connectionID),
			StatusCode: 410,
			Retryable:  false,
		}
	}

	// Check connection state
	if conn.State != ConnectionStateActive {
		return &MockAPIGatewayError{
			Code:       "GoneException",
			Message:    fmt.Sprintf("Connection %s is not active", connectionID),
			StatusCode: 410,
			Retryable:  false,
		}
	}

	// Check if connection is stale (TTL expired)
	if time.Since(conn.CreatedAt).Seconds() > float64(m.config.ConnectionTTL) {
		conn.State = ConnectionStateStale
		return &MockAPIGatewayError{
			Code:       "GoneException",
			Message:    fmt.Sprintf("Connection %s has expired", connectionID),
			StatusCode: 410,
			Retryable:  false,
		}
	}

	// Check message size
	if int64(len(data)) > m.config.MaxMessageSize {
		return &MockAPIGatewayError{
			Code:       "PayloadTooLargeException",
			Message:    fmt.Sprintf("Message size %d exceeds maximum %d", len(data), m.config.MaxMessageSize),
			StatusCode: 413,
			Retryable:  false,
		}
	}

	// Store the message
	m.messages[connectionID] = append(m.messages[connectionID], data)

	// Update last active time
	conn.LastActiveAt = time.Now()

	return nil
}

// DeleteConnection terminates a WebSocket connection
func (m *MockAPIGatewayManagementClient) DeleteConnection(_ context.Context, connectionID string) error {
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
		return &MockAPIGatewayError{
			Code:       "GoneException",
			Message:    fmt.Sprintf("Connection %s not found", connectionID),
			StatusCode: 410,
			Retryable:  false,
		}
	}

	// Mark connection as disconnected
	conn.State = ConnectionStateDisconnected
	conn.LastActiveAt = time.Now()

	return nil
}

// GetConnection retrieves connection information
func (m *MockAPIGatewayManagementClient) GetConnection(_ context.Context, connectionID string) (*MockConnectionInfo, error) {
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
		return nil, &MockAPIGatewayError{
			Code:       "GoneException",
			Message:    fmt.Sprintf("Connection %s not found", connectionID),
			StatusCode: 410,
			Retryable:  false,
		}
	}

	return &MockConnectionInfo{
		ConnectionID: conn.ID,
		ConnectedAt:  conn.CreatedAt.Format(time.RFC3339),
		LastActiveAt: conn.LastActiveAt.Format(time.RFC3339),
		SourceIP:     conn.SourceIP,
		UserAgent:    conn.UserAgent,
	}, nil
}

// MockConnectionInfo represents connection information returned by GetConnection
type MockConnectionInfo struct {
	ConnectionID string `json:"connectionId"`
	ConnectedAt  string `json:"connectedAt"`
	LastActiveAt string `json:"lastActiveAt"`
	SourceIP     string `json:"sourceIp"`
	UserAgent    string `json:"userAgent"`
}

// MockAPIGatewayError implements the APIError interface for testing
type MockAPIGatewayError struct {
	Code       string
	Message    string
	StatusCode int
	Retryable  bool
}

func (e *MockAPIGatewayError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *MockAPIGatewayError) HTTPStatusCode() int {
	return e.StatusCode
}

func (e *MockAPIGatewayError) ErrorCode() string {
	return e.Code
}

func (e *MockAPIGatewayError) IsRetryable() bool {
	return e.Retryable
}

// Helper methods for testing

// GetMessages returns all messages sent to a connection
func (m *MockAPIGatewayManagementClient) GetMessages(connectionID string) [][]byte {
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

// GetMessageCount returns the number of messages sent to a connection
func (m *MockAPIGatewayManagementClient) GetMessageCount(connectionID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages[connectionID])
}

// GetCallCount returns the number of times an operation was called
func (m *MockAPIGatewayManagementClient) GetCallCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[operation]
}

// GetConnectionState returns a copy of the connection state
func (m *MockAPIGatewayManagementClient) GetConnectionState(connectionID string) *MockConnection {
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

// GetActiveConnections returns all active connections
func (m *MockAPIGatewayManagementClient) GetActiveConnections() map[string]*MockConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := make(map[string]*MockConnection)
	for id, conn := range m.connections {
		if conn.State == ConnectionStateActive {
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

// Reset clears all mock state
func (m *MockAPIGatewayManagementClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connections = make(map[string]*MockConnection)
	m.messages = make(map[string][][]byte)
	m.errors = make(map[string]error)
	m.callCount = make(map[string]int)
	m.config = DefaultMockAPIGatewayConfig()
}

// SimulateConnectionExpiry marks connections as stale based on TTL
func (m *MockAPIGatewayManagementClient) SimulateConnectionExpiry() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, conn := range m.connections {
		if conn.State == ConnectionStateActive &&
			now.Sub(conn.CreatedAt).Seconds() > float64(m.config.ConnectionTTL) {
			conn.State = ConnectionStateStale
		}
	}
}

// =============================================================================
// End of API Gateway Management API Mocks
// =============================================================================

// =============================================================================
// CloudWatch Metrics & Alarms Mocks
// =============================================================================

// MetricUnit represents CloudWatch metric units
type MetricUnit string

const (
	MetricUnitNone           MetricUnit = "None"
	MetricUnitSeconds        MetricUnit = "Seconds"
	MetricUnitMicroseconds   MetricUnit = "Microseconds"
	MetricUnitMilliseconds   MetricUnit = "Milliseconds"
	MetricUnitBytes          MetricUnit = "Bytes"
	MetricUnitKilobytes      MetricUnit = "Kilobytes"
	MetricUnitMegabytes      MetricUnit = "Megabytes"
	MetricUnitGigabytes      MetricUnit = "Gigabytes"
	MetricUnitTerabytes      MetricUnit = "Terabytes"
	MetricUnitBits           MetricUnit = "Bits"
	MetricUnitKilobits       MetricUnit = "Kilobits"
	MetricUnitMegabits       MetricUnit = "Megabits"
	MetricUnitGigabits       MetricUnit = "Gigabits"
	MetricUnitTerabits       MetricUnit = "Terabits"
	MetricUnitPercent        MetricUnit = "Percent"
	MetricUnitCount          MetricUnit = "Count"
	MetricUnitCountPerSecond MetricUnit = "Count/Second"
)

// MockMetricDatum represents a single metric data point
type MockMetricDatum struct {
	Timestamp  time.Time         `json:"timestamp"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	MetricName string            `json:"metric_name"`
	Unit       MetricUnit        `json:"unit"`
	Value      float64           `json:"value"`
}

// AlarmState represents the state of a CloudWatch alarm
type AlarmState string

const (
	AlarmStateOK               AlarmState = "OK"
	AlarmStateAlarm            AlarmState = "ALARM"
	AlarmStateInsufficientData AlarmState = "INSUFFICIENT_DATA"
)

// ComparisonOperator represents alarm comparison operators
type ComparisonOperator string

const (
	ComparisonGreaterThanThreshold                     ComparisonOperator = "GreaterThanThreshold"
	ComparisonGreaterThanOrEqualToThreshold            ComparisonOperator = "GreaterThanOrEqualToThreshold"
	ComparisonLessThanThreshold                        ComparisonOperator = "LessThanThreshold"
	ComparisonLessThanOrEqualToThreshold               ComparisonOperator = "LessThanOrEqualToThreshold"
	ComparisonLessThanLowerOrGreaterThanUpperThreshold ComparisonOperator = "LessThanLowerOrGreaterThanUpperThreshold"
	ComparisonLessThanLowerThreshold                   ComparisonOperator = "LessThanLowerThreshold"
	ComparisonGreaterThanUpperThreshold                ComparisonOperator = "GreaterThanUpperThreshold"
)

// Statistic represents CloudWatch statistics
type Statistic string

const (
	StatisticSampleCount Statistic = "SampleCount"
	StatisticAverage     Statistic = "Average"
	StatisticSum         Statistic = "Sum"
	StatisticMinimum     Statistic = "Minimum"
	StatisticMaximum     Statistic = "Maximum"
)

// MockAlarmDefinition represents a CloudWatch alarm
type MockAlarmDefinition struct {
	UpdatedAt          time.Time          `json:"updated_at"`
	CreatedAt          time.Time          `json:"created_at"`
	StateUpdatedAt     time.Time          `json:"state_updated_at"`
	Dimensions         map[string]string  `json:"dimensions,omitempty"`
	StateReason        string             `json:"state_reason"`
	Statistic          Statistic          `json:"statistic"`
	ComparisonOperator ComparisonOperator `json:"comparison_operator"`
	TreatMissingData   string             `json:"treat_missing_data"`
	State              AlarmState         `json:"state"`
	AlarmName          string             `json:"alarm_name"`
	Namespace          string             `json:"namespace"`
	MetricName         string             `json:"metric_name"`
	AlarmDescription   string             `json:"alarm_description"`
	Threshold          float64            `json:"threshold"`
	Period             int32              `json:"period"`
	EvaluationPeriods  int32              `json:"evaluation_periods"`
}

// MockCloudWatchConfig configures the behavior of CloudWatch mocks
type MockCloudWatchConfig struct {
	// Maximum number of metrics per PutMetricData call
	MaxMetricsPerCall int
	// Simulate network delays
	NetworkDelay time.Duration
	// Auto-evaluate alarms when metrics are published
	AutoEvaluateAlarms bool
	// Metric retention period in hours
	MetricRetentionHours int
}

// DefaultMockCloudWatchConfig returns default configuration
func DefaultMockCloudWatchConfig() *MockCloudWatchConfig {
	return &MockCloudWatchConfig{
		MaxMetricsPerCall:    20,
		NetworkDelay:         0,
		AutoEvaluateAlarms:   true,
		MetricRetentionHours: 24 * 15, // 15 days
	}
}

// MockCloudWatchMetricsClient provides a mock implementation of CloudWatch Metrics
