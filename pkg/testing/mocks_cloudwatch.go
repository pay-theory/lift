package testing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

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
type MockCloudWatchMetricsClient struct {
	metrics   map[string][]*MockMetricDatum
	callCount map[string]int
	errors    map[string]error
	config    *MockCloudWatchConfig
	mu        sync.RWMutex
}

// NewMockCloudWatchMetricsClient creates a new mock CloudWatch Metrics client
func NewMockCloudWatchMetricsClient() *MockCloudWatchMetricsClient {
	return &MockCloudWatchMetricsClient{
		metrics:   make(map[string][]*MockMetricDatum),
		callCount: make(map[string]int),
		errors:    make(map[string]error),
		config:    DefaultMockCloudWatchConfig(),
	}
}

// WithConfig sets the mock configuration
func (m *MockCloudWatchMetricsClient) WithConfig(config *MockCloudWatchConfig) *MockCloudWatchMetricsClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return m
}

// WithError configures an error for a specific operation
func (m *MockCloudWatchMetricsClient) WithError(operation string, err error) *MockCloudWatchMetricsClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[operation] = err
	return m
}

// PutMetricData publishes metric data to CloudWatch
func (m *MockCloudWatchMetricsClient) PutMetricData(_ context.Context, namespace string, metricData []*MockMetricDatum) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track call count
	m.callCount["PutMetricData"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors["PutMetricData"]; exists {
		return err
	}

	// Validate input
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	if len(metricData) == 0 {
		return fmt.Errorf("at least one metric datum is required")
	}

	if len(metricData) > m.config.MaxMetricsPerCall {
		return fmt.Errorf("too many metrics: %d, maximum allowed: %d", len(metricData), m.config.MaxMetricsPerCall)
	}

	// Store metrics
	if m.metrics[namespace] == nil {
		m.metrics[namespace] = make([]*MockMetricDatum, 0)
	}

	// Add timestamps if not provided
	now := time.Now()
	for _, datum := range metricData {
		if datum.Timestamp.IsZero() {
			datum.Timestamp = now
		}

		// Create a copy to prevent external modification
		datumCopy := *datum
		if datum.Dimensions != nil {
			datumCopy.Dimensions = make(map[string]string)
			for k, v := range datum.Dimensions {
				datumCopy.Dimensions[k] = v
			}
		}
		if datum.Metadata != nil {
			datumCopy.Metadata = make(map[string]any)
			for k, v := range datum.Metadata {
				datumCopy.Metadata[k] = v
			}
		}

		m.metrics[namespace] = append(m.metrics[namespace], &datumCopy)
	}

	return nil
}

// GetMetricStatistics retrieves statistics for a metric
func (m *MockCloudWatchMetricsClient) GetMetricStatistics(_ context.Context, namespace, metricName string, dimensions map[string]string, startTime, endTime time.Time, _ int32, statistics []Statistic) (map[Statistic]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Track call count
	m.callCount["GetMetricStatistics"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors["GetMetricStatistics"]; exists {
		return nil, err
	}

	// Create metric query and execute
	query := &metricQuery{
		namespace:  namespace,
		metricName: metricName,
		dimensions: dimensions,
		startTime:  startTime,
		endTime:    endTime,
	}

	matcher := newMetricMatcher(m.metrics)
	values := matcher.findMatchingValues(query)

	if len(values) == 0 {
		return make(map[Statistic]float64), nil
	}

	// Calculate requested statistics
	calculator := newStatisticsCalculator(values)
	return calculator.calculate(statistics), nil
}

// metricQuery represents a query for metrics
type metricQuery struct {
	startTime  time.Time
	endTime    time.Time
	dimensions map[string]string
	namespace  string
	metricName string
}

// metricMatcher handles metric filtering and matching
type metricMatcher struct {
	metrics map[string][]*MockMetricDatum
}

// newMetricMatcher creates a new metric matcher
func newMetricMatcher(metrics map[string][]*MockMetricDatum) *metricMatcher {
	return &metricMatcher{metrics: metrics}
}

// findMatchingValues finds all metric values matching the query
func (mm *metricMatcher) findMatchingValues(query *metricQuery) []float64 {
	namespaceMetrics, exists := mm.metrics[query.namespace]
	if !exists {
		return nil
	}

	var values []float64
	for _, metric := range namespaceMetrics {
		if mm.matches(metric, query) {
			values = append(values, metric.Value)
		}
	}

	return values
}

// matches checks if a metric matches the query criteria
func (mm *metricMatcher) matches(metric *MockMetricDatum, query *metricQuery) bool {
	// Check metric name
	if metric.MetricName != query.metricName {
		return false
	}

	// Check time range
	if metric.Timestamp.Before(query.startTime) || metric.Timestamp.After(query.endTime) {
		return false
	}

	// Check dimensions
	return mm.dimensionsMatch(metric.Dimensions, query.dimensions)
}

// dimensionsMatch checks if metric dimensions match the query dimensions
func (mm *metricMatcher) dimensionsMatch(metricDims, queryDims map[string]string) bool {
	if queryDims == nil {
		return true
	}

	for k, v := range queryDims {
		if metricDims[k] != v {
			return false
		}
	}

	return true
}

// statisticsCalculator handles statistics calculations
type statisticsCalculator struct {
	values []float64
}

// newStatisticsCalculator creates a new statistics calculator
func newStatisticsCalculator(values []float64) *statisticsCalculator {
	return &statisticsCalculator{values: values}
}

// calculate computes the requested statistics
func (sc *statisticsCalculator) calculate(statistics []Statistic) map[Statistic]float64 {
	result := make(map[Statistic]float64)

	for _, stat := range statistics {
		result[stat] = sc.computeStatistic(stat)
	}

	return result
}

// computeStatistic computes a single statistic
func (sc *statisticsCalculator) computeStatistic(stat Statistic) float64 {
	switch stat {
	case StatisticSampleCount:
		return float64(len(sc.values))
	case StatisticSum:
		return sc.sum()
	case StatisticAverage:
		return sc.average()
	case StatisticMinimum:
		return sc.minimum()
	case StatisticMaximum:
		return sc.maximum()
	default:
		return 0
	}
}

// sum calculates the sum of all values
func (sc *statisticsCalculator) sum() float64 {
	total := 0.0
	for _, v := range sc.values {
		total += v
	}
	return total
}

// average calculates the average of all values
func (sc *statisticsCalculator) average() float64 {
	if len(sc.values) == 0 {
		return 0
	}
	return sc.sum() / float64(len(sc.values))
}

// minimum finds the minimum value
func (sc *statisticsCalculator) minimum() float64 {
	if len(sc.values) == 0 {
		return 0
	}

	minVal := sc.values[0]
	for _, v := range sc.values[1:] {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

// maximum finds the maximum value
func (sc *statisticsCalculator) maximum() float64 {
	if len(sc.values) == 0 {
		return 0
	}

	maxVal := sc.values[0]
	for _, v := range sc.values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

// MockCloudWatchAlarmsClient provides a mock implementation of CloudWatch Alarms
type MockCloudWatchAlarmsClient struct {
	alarms        map[string]*MockAlarmDefinition
	callCount     map[string]int
	errors        map[string]error
	config        *MockCloudWatchConfig
	metricsClient *MockCloudWatchMetricsClient
	mu            sync.RWMutex
}

// NewMockCloudWatchAlarmsClient creates a new mock CloudWatch Alarms client
func NewMockCloudWatchAlarmsClient() *MockCloudWatchAlarmsClient {
	return &MockCloudWatchAlarmsClient{
		alarms:    make(map[string]*MockAlarmDefinition),
		callCount: make(map[string]int),
		errors:    make(map[string]error),
		config:    DefaultMockCloudWatchConfig(),
	}
}

// WithConfig sets the mock configuration
func (m *MockCloudWatchAlarmsClient) WithConfig(config *MockCloudWatchConfig) *MockCloudWatchAlarmsClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return m
}

// WithMetricsClient sets the metrics client for alarm evaluation
func (m *MockCloudWatchAlarmsClient) WithMetricsClient(client *MockCloudWatchMetricsClient) *MockCloudWatchAlarmsClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsClient = client
	return m
}

// WithError configures an error for a specific operation
func (m *MockCloudWatchAlarmsClient) WithError(operation string, err error) *MockCloudWatchAlarmsClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[operation] = err
	return m
}

// PutMetricAlarm creates or updates an alarm
func (m *MockCloudWatchAlarmsClient) PutMetricAlarm(_ context.Context, alarm *MockAlarmDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track call count
	m.callCount["PutMetricAlarm"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors["PutMetricAlarm"]; exists {
		return err
	}

	// Validate input
	if alarm.AlarmName == "" {
		return fmt.Errorf("alarm name is required")
	}
	if alarm.MetricName == "" {
		return fmt.Errorf("metric name is required")
	}
	if alarm.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	// Create a copy to prevent external modification
	alarmCopy := *alarm
	if alarm.Dimensions != nil {
		alarmCopy.Dimensions = make(map[string]string)
		for k, v := range alarm.Dimensions {
			alarmCopy.Dimensions[k] = v
		}
	}

	// Set timestamps
	now := time.Now()
	if alarmCopy.CreatedAt.IsZero() {
		alarmCopy.CreatedAt = now
	}
	alarmCopy.UpdatedAt = now

	// Set initial state if not provided
	if alarmCopy.State == "" {
		alarmCopy.State = AlarmStateInsufficientData
		alarmCopy.StateReason = "Insufficient Data"
		alarmCopy.StateUpdatedAt = now
	}

	m.alarms[alarm.AlarmName] = &alarmCopy

	return nil
}

// DescribeAlarms retrieves alarm information
func (m *MockCloudWatchAlarmsClient) DescribeAlarms(_ context.Context, alarmNames []string) ([]*MockAlarmDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Track call count
	m.callCount["DescribeAlarms"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors["DescribeAlarms"]; exists {
		return nil, err
	}

	var result []*MockAlarmDefinition

	if len(alarmNames) == 0 {
		// Return all alarms
		for _, alarm := range m.alarms {
			alarmCopy := *alarm
			if alarm.Dimensions != nil {
				alarmCopy.Dimensions = make(map[string]string)
				for k, v := range alarm.Dimensions {
					alarmCopy.Dimensions[k] = v
				}
			}
			result = append(result, &alarmCopy)
		}
	} else {
		// Return specific alarms
		for _, name := range alarmNames {
			if alarm, exists := m.alarms[name]; exists {
				alarmCopy := *alarm
				if alarm.Dimensions != nil {
					alarmCopy.Dimensions = make(map[string]string)
					for k, v := range alarm.Dimensions {
						alarmCopy.Dimensions[k] = v
					}
				}
				result = append(result, &alarmCopy)
			}
		}
	}

	return result, nil
}

// DeleteAlarms deletes one or more alarms
func (m *MockCloudWatchAlarmsClient) DeleteAlarms(_ context.Context, alarmNames []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Track call count
	m.callCount["DeleteAlarms"]++

	// Simulate network delay
	if m.config.NetworkDelay > 0 {
		time.Sleep(m.config.NetworkDelay)
	}

	// Check for configured error
	if err, exists := m.errors["DeleteAlarms"]; exists {
		return err
	}

	for _, name := range alarmNames {
		delete(m.alarms, name)
	}

	return nil
}

// EvaluateAlarms evaluates all alarms against current metrics
func (m *MockCloudWatchAlarmsClient) EvaluateAlarms(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.metricsClient == nil {
		return fmt.Errorf("metrics client not configured")
	}

	now := time.Now()

	for _, alarm := range m.alarms {
		// Calculate evaluation period
		evaluationDuration := time.Duration(alarm.Period*alarm.EvaluationPeriods) * time.Second
		startTime := now.Add(-evaluationDuration)

		// Get metric statistics
		stats, err := m.metricsClient.GetMetricStatistics(
			ctx,
			alarm.Namespace,
			alarm.MetricName,
			alarm.Dimensions,
			startTime,
			now,
			alarm.Period,
			[]Statistic{alarm.Statistic},
		)
		if err != nil {
			continue
		}

		value, exists := stats[alarm.Statistic]
		if !exists {
			// Insufficient data
			if alarm.State != AlarmStateInsufficientData {
				alarm.State = AlarmStateInsufficientData
				alarm.StateReason = "Insufficient Data"
				alarm.StateUpdatedAt = now
			}
			continue
		}

		// Evaluate threshold
		var inAlarmState bool
		switch alarm.ComparisonOperator {
		case ComparisonGreaterThanThreshold:
			inAlarmState = value > alarm.Threshold
		case ComparisonGreaterThanOrEqualToThreshold:
			inAlarmState = value >= alarm.Threshold
		case ComparisonLessThanThreshold:
			inAlarmState = value < alarm.Threshold
		case ComparisonLessThanOrEqualToThreshold:
			inAlarmState = value <= alarm.Threshold
		}

		// Update alarm state
		newState := AlarmStateOK
		newReason := fmt.Sprintf("Threshold Crossed: %f %s %f", value, alarm.ComparisonOperator, alarm.Threshold)

		if inAlarmState {
			newState = AlarmStateAlarm
		}

		if alarm.State != newState {
			alarm.State = newState
			alarm.StateReason = newReason
			alarm.StateUpdatedAt = now
		}
	}

	return nil
}

// Helper methods for testing

// GetCallCount returns the number of times an operation was called
func (m *MockCloudWatchMetricsClient) GetCallCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[operation]
}

// GetMetrics returns all metrics for a namespace
func (m *MockCloudWatchMetricsClient) GetMetrics(namespace string) []*MockMetricDatum {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[namespace]
	if !exists {
		return nil
	}

	// Return copies to prevent external modification
	result := make([]*MockMetricDatum, len(metrics))
	for i, metric := range metrics {
		metricCopy := *metric
		if metric.Dimensions != nil {
			metricCopy.Dimensions = make(map[string]string)
			for k, v := range metric.Dimensions {
				metricCopy.Dimensions[k] = v
			}
		}
		if metric.Metadata != nil {
			metricCopy.Metadata = make(map[string]any)
			for k, v := range metric.Metadata {
				metricCopy.Metadata[k] = v
			}
		}
		result[i] = &metricCopy
	}

	return result
}

// GetAllMetrics returns all metrics across all namespaces
func (m *MockCloudWatchMetricsClient) GetAllMetrics() map[string][]*MockMetricDatum {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*MockMetricDatum)
	for namespace, metrics := range m.metrics {
		result[namespace] = make([]*MockMetricDatum, len(metrics))
		for i, metric := range metrics {
			metricCopy := *metric
			if metric.Dimensions != nil {
				metricCopy.Dimensions = make(map[string]string)
				for k, v := range metric.Dimensions {
					metricCopy.Dimensions[k] = v
				}
			}
			if metric.Metadata != nil {
				metricCopy.Metadata = make(map[string]any)
				for k, v := range metric.Metadata {
					metricCopy.Metadata[k] = v
				}
			}
			result[namespace][i] = &metricCopy
		}
	}

	return result
}

// Reset clears all mock state
func (m *MockCloudWatchMetricsClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = make(map[string][]*MockMetricDatum)
	m.callCount = make(map[string]int)
	m.errors = make(map[string]error)
	m.config = DefaultMockCloudWatchConfig()
}

// GetCallCount returns the number of times an operation was called
func (m *MockCloudWatchAlarmsClient) GetCallCount(operation string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount[operation]
}

// GetAlarm returns a copy of an alarm definition
func (m *MockCloudWatchAlarmsClient) GetAlarm(alarmName string) *MockAlarmDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alarm, exists := m.alarms[alarmName]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	alarmCopy := *alarm
	if alarm.Dimensions != nil {
		alarmCopy.Dimensions = make(map[string]string)
		for k, v := range alarm.Dimensions {
			alarmCopy.Dimensions[k] = v
		}
	}

	return &alarmCopy
}

// GetAllAlarms returns all alarm definitions
func (m *MockCloudWatchAlarmsClient) GetAllAlarms() map[string]*MockAlarmDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*MockAlarmDefinition)
	for name, alarm := range m.alarms {
		alarmCopy := *alarm
		if alarm.Dimensions != nil {
			alarmCopy.Dimensions = make(map[string]string)
			for k, v := range alarm.Dimensions {
				alarmCopy.Dimensions[k] = v
			}
		}
		result[name] = &alarmCopy
	}

	return result
}

// Reset clears all mock state
func (m *MockCloudWatchAlarmsClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alarms = make(map[string]*MockAlarmDefinition)
	m.callCount = make(map[string]int)
	m.errors = make(map[string]error)
	m.config = DefaultMockCloudWatchConfig()
}

// =============================================================================
// End of CloudWatch Metrics & Alarms Mocks
// =============================================================================
