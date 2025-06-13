package cloudwatch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCloudWatchMetricsClient implements CloudWatchMetricsClient for testing
type MockCloudWatchMetricsClient struct {
	mu            sync.Mutex
	putMetricData []cloudwatch.PutMetricDataInput
	errors        map[int]error
	callCount     int
}

func NewMockCloudWatchMetricsClient() *MockCloudWatchMetricsClient {
	return &MockCloudWatchMetricsClient{
		putMetricData: make([]cloudwatch.PutMetricDataInput, 0),
		errors:        make(map[int]error),
	}
}

func (m *MockCloudWatchMetricsClient) PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.putMetricData = append(m.putMetricData, *params)
	m.callCount++

	if err, ok := m.errors[m.callCount-1]; ok {
		return nil, err
	}

	return &cloudwatch.PutMetricDataOutput{}, nil
}

func (m *MockCloudWatchMetricsClient) GetPutMetricDataCalls() []cloudwatch.PutMetricDataInput {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]cloudwatch.PutMetricDataInput, len(m.putMetricData))
	copy(result, m.putMetricData)
	return result
}

func (m *MockCloudWatchMetricsClient) SetError(callIndex int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[callIndex] = err
}

func TestCloudWatchMetrics_BasicMetrics(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    100,
		FlushSize:     5,
		FlushInterval: 1 * time.Second,
		Dimensions: map[string]string{
			"Environment": "test",
		},
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Record various metric types
	metrics.RecordCount("test.count", 5)
	metrics.RecordDuration("test.duration", 100*time.Millisecond)
	metrics.RecordGauge("test.gauge", 42.5)
	metrics.RecordMetric("test.custom", 123.45, types.StandardUnitBytes)

	// Record one more to trigger flush (flushSize = 5)
	metrics.RecordCount("test.trigger", 1)

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Verify metrics were sent
	calls := client.GetPutMetricDataCalls()
	require.Len(t, calls, 1)

	call := calls[0]
	assert.Equal(t, "TestNamespace", *call.Namespace)
	assert.Len(t, call.MetricData, 5)

	// Verify first metric
	metric := call.MetricData[0]
	assert.Equal(t, "test.count", *metric.MetricName)
	assert.Equal(t, float64(5), *metric.Value)
	assert.Equal(t, types.StandardUnitCount, metric.Unit)

	// Verify dimensions
	assert.Len(t, metric.Dimensions, 1)
	assert.Equal(t, "Environment", *metric.Dimensions[0].Name)
	assert.Equal(t, "test", *metric.Dimensions[0].Value)
}

func TestCloudWatchMetrics_MultiTenantDimensions(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    100,
		FlushSize:     10,
		FlushInterval: 1 * time.Hour, // Long interval to control flushing
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Create tenant-specific metrics
	tenant1Metrics := metrics.WithTenant("tenant-1")
	tenant2Metrics := metrics.WithTenant("tenant-2")

	// Record metrics for different tenants
	tenant1Metrics.RecordCount("api.requests", 10)
	tenant2Metrics.RecordCount("api.requests", 20)

	// Force flush
	err := metrics.Flush()
	require.NoError(t, err)

	// Verify metrics
	calls := client.GetPutMetricDataCalls()
	require.Len(t, calls, 1)

	data := calls[0].MetricData
	require.Len(t, data, 2)

	// Find tenant dimensions
	var tenant1Found, tenant2Found bool
	for _, metric := range data {
		for _, dim := range metric.Dimensions {
			if *dim.Name == "TenantID" {
				if *dim.Value == "tenant-1" {
					tenant1Found = true
					assert.Equal(t, float64(10), *metric.Value)
				} else if *dim.Value == "tenant-2" {
					tenant2Found = true
					assert.Equal(t, float64(20), *metric.Value)
				}
			}
		}
	}

	assert.True(t, tenant1Found, "Tenant 1 metric not found")
	assert.True(t, tenant2Found, "Tenant 2 metric not found")
}

func TestCloudWatchMetrics_BufferOverflow(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    5,
		FlushSize:     10, // Higher than buffer size
		FlushInterval: 1 * time.Hour,
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Record more metrics than buffer size
	for i := 0; i < 10; i++ {
		metrics.RecordCount("overflow.test", int64(i))
	}

	// Force flush
	err := metrics.Flush()
	require.NoError(t, err)

	// Should have only the last 5 metrics (buffer size)
	calls := client.GetPutMetricDataCalls()
	require.Len(t, calls, 1)
	assert.Len(t, calls[0].MetricData, 5)

	// Verify we have the last 5 values (5-9)
	values := make([]float64, 0)
	for _, metric := range calls[0].MetricData {
		values = append(values, *metric.Value)
	}

	// Values should be 5, 6, 7, 8, 9 (oldest metrics dropped)
	for i, v := range values {
		assert.Equal(t, float64(i+5), v)
	}
}

func TestCloudWatchMetrics_ErrorHandling(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	client.SetError(0, errors.New("AWS error"))

	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    100,
		FlushSize:     2,
		FlushInterval: 1 * time.Hour,
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Record metrics that will trigger flush
	metrics.RecordCount("error.test", 1)
	metrics.RecordCount("error.test", 2)

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Check stats
	stats := metrics.GetStats()
	assert.Equal(t, int64(2), stats.MetricsRecorded)
	assert.Equal(t, int64(2), stats.MetricsDropped) // Dropped due to error
	assert.Equal(t, int64(1), stats.ErrorCount)
	assert.NotEmpty(t, stats.LastError)
}

func TestCloudWatchMetrics_PeriodicFlush(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    100,
		FlushSize:     100, // High flush size
		FlushInterval: 100 * time.Millisecond,
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Record a metric
	metrics.RecordCount("periodic.test", 1)

	// Wait for periodic flush
	time.Sleep(150 * time.Millisecond)

	// Should have flushed
	calls := client.GetPutMetricDataCalls()
	require.Len(t, calls, 1)
	assert.Len(t, calls[0].MetricData, 1)
}

func TestCloudWatchMetrics_LiftInterface(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    100,
		FlushSize:     10,
		FlushInterval: 1 * time.Hour,
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Test Counter
	counter := metrics.Counter("test.counter", map[string]string{"type": "api"})
	counter.Inc()
	counter.Add(5.5)

	// Test Histogram
	histogram := metrics.Histogram("test.histogram", map[string]string{"type": "latency"})
	histogram.Observe(100.5)
	histogram.Observe(200.5)

	// Test Gauge
	gauge := metrics.Gauge("test.gauge", map[string]string{"type": "memory"})
	gauge.Set(1024.5)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(512.5)

	// Force flush
	err := metrics.Flush()
	require.NoError(t, err)

	// Verify metrics
	calls := client.GetPutMetricDataCalls()
	require.Len(t, calls, 1)

	data := calls[0].MetricData
	assert.Len(t, data, 8) // 2 counter + 2 histogram + 4 gauge

	// Verify tags were applied
	for _, metric := range data {
		found := false
		for _, dim := range metric.Dimensions {
			if *dim.Name == "type" {
				found = true
				assert.Contains(t, []string{"api", "latency", "memory"}, *dim.Value)
			}
		}
		assert.True(t, found, "Tag not found on metric %s", *metric.MetricName)
	}
}

func TestCloudWatchMetrics_ConcurrentAccess(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    1000,
		FlushSize:     100,
		FlushInterval: 1 * time.Hour,
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Concurrent metric recording
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				metrics.RecordCount("concurrent.test", int64(id*10+j))
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Force flush
	err := metrics.Flush()
	require.NoError(t, err)

	// Verify all metrics were recorded
	stats := metrics.GetStats()
	assert.Equal(t, int64(100), stats.MetricsRecorded)

	calls := client.GetPutMetricDataCalls()
	totalMetrics := 0
	for _, call := range calls {
		totalMetrics += len(call.MetricData)
	}
	assert.Equal(t, 100, totalMetrics)
}

func TestCloudWatchMetrics_Stats(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    100,
		FlushSize:     5,
		FlushInterval: 1 * time.Hour,
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Initial stats
	stats := metrics.GetStats()
	assert.Equal(t, int64(0), stats.MetricsRecorded)
	assert.Equal(t, int64(0), stats.MetricsDropped)
	assert.Equal(t, int64(0), stats.ErrorCount)

	// Record some metrics
	for i := 0; i < 5; i++ {
		metrics.RecordCount("stats.test", 1)
	}

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Check updated stats
	stats = metrics.GetStats()
	assert.Equal(t, int64(5), stats.MetricsRecorded)
	assert.NotZero(t, stats.LastFlush)
}

func TestCloudWatchMetrics_Performance(t *testing.T) {
	client := NewMockCloudWatchMetricsClient()
	config := CloudWatchMetricsConfig{
		Namespace:     "TestNamespace",
		BufferSize:    10000,
		FlushSize:     1000,
		FlushInterval: 1 * time.Hour,
	}

	metrics := NewCloudWatchMetrics(client, config)
	defer metrics.Close()

	// Measure time to record 1000 metrics
	start := time.Now()
	for i := 0; i < 1000; i++ {
		metrics.RecordCount("perf.test", 1)
	}
	duration := time.Since(start)

	// Should be very fast (< 1ms per metric)
	perMetric := duration / 1000
	t.Logf("Time per metric: %v", perMetric)
	assert.Less(t, perMetric, 1*time.Millisecond)
}
