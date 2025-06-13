package cloudwatch

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// CloudWatchMetricsClient defines the interface for CloudWatch metrics operations
type CloudWatchMetricsClient interface {
	PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error)
}

// MetricsBuffer manages buffering of metric data points
type MetricsBuffer struct {
	mu        sync.Mutex
	data      []types.MetricDatum
	maxSize   int
	flushSize int
}

// NewMetricsBuffer creates a new metrics buffer
func NewMetricsBuffer(maxSize, flushSize int) *MetricsBuffer {
	return &MetricsBuffer{
		data:      make([]types.MetricDatum, 0, maxSize),
		maxSize:   maxSize,
		flushSize: flushSize,
	}
}

// Add adds a metric datum to the buffer
func (b *MetricsBuffer) Add(datum types.MetricDatum) (shouldFlush bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If buffer is full, drop oldest metrics
	if len(b.data) >= b.maxSize {
		copy(b.data, b.data[1:])
		b.data = b.data[:len(b.data)-1]
	}

	b.data = append(b.data, datum)
	return len(b.data) >= b.flushSize
}

// Drain removes and returns all metrics from the buffer
func (b *MetricsBuffer) Drain() []types.MetricDatum {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.data) == 0 {
		return nil
	}

	result := make([]types.MetricDatum, len(b.data))
	copy(result, b.data)
	b.data = b.data[:0]

	return result
}

// Size returns the current number of metrics in the buffer
func (b *MetricsBuffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.data)
}

// CloudWatchMetrics implements metrics collection for CloudWatch
type CloudWatchMetrics struct {
	client        CloudWatchMetricsClient
	namespace     string
	buffer        *MetricsBuffer
	flushInterval time.Duration
	dimensions    []types.Dimension

	// Performance tracking
	metricsRecorded int64
	metricsDropped  int64
	flushCount      int64
	errorCount      int64
	lastError       atomic.Value
	lastFlush       atomic.Value

	// Control
	stopCh   chan struct{}
	doneCh   chan struct{}
	flushNow chan struct{}
	mu       sync.RWMutex
}

// CloudWatchMetricsConfig holds configuration for CloudWatch metrics
type CloudWatchMetricsConfig struct {
	Namespace     string
	BufferSize    int
	FlushSize     int
	FlushInterval time.Duration
	Dimensions    map[string]string
}

// NewCloudWatchMetrics creates a new CloudWatch metrics collector
func NewCloudWatchMetrics(client CloudWatchMetricsClient, config CloudWatchMetricsConfig) *CloudWatchMetrics {
	// Set defaults
	if config.BufferSize == 0 {
		config.BufferSize = 1000
	}
	if config.FlushSize == 0 {
		config.FlushSize = 20 // CloudWatch allows up to 1000 metrics per request, but we'll batch smaller
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 60 * time.Second
	}

	// Convert dimensions map to slice
	dimensions := make([]types.Dimension, 0, len(config.Dimensions))
	for name, value := range config.Dimensions {
		dimensions = append(dimensions, types.Dimension{
			Name:  aws.String(name),
			Value: aws.String(value),
		})
	}

	m := &CloudWatchMetrics{
		client:        client,
		namespace:     config.Namespace,
		buffer:        NewMetricsBuffer(config.BufferSize, config.FlushSize),
		flushInterval: config.FlushInterval,
		dimensions:    dimensions,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		flushNow:      make(chan struct{}, 1),
	}

	// Initialize last flush time
	m.lastFlush.Store(time.Now())

	// Start background flusher
	go m.backgroundFlusher()

	return m
}

// RecordMetric records a single metric value
func (m *CloudWatchMetrics) RecordMetric(name string, value float64, unit types.StandardUnit) {
	atomic.AddInt64(&m.metricsRecorded, 1)

	datum := types.MetricDatum{
		MetricName: aws.String(name),
		Value:      aws.Float64(value),
		Unit:       unit,
		Timestamp:  aws.Time(time.Now()),
		Dimensions: m.getDimensions(),
	}

	if shouldFlush := m.buffer.Add(datum); shouldFlush {
		// Trigger flush without blocking
		select {
		case m.flushNow <- struct{}{}:
		default:
		}
	}
}

// RecordCount records a count metric
func (m *CloudWatchMetrics) RecordCount(name string, count int64) {
	m.RecordMetric(name, float64(count), types.StandardUnitCount)
}

// RecordDuration records a duration metric in milliseconds
func (m *CloudWatchMetrics) RecordDuration(name string, duration time.Duration) {
	m.RecordMetric(name, float64(duration.Milliseconds()), types.StandardUnitMilliseconds)
}

// RecordGauge records a gauge metric
func (m *CloudWatchMetrics) RecordGauge(name string, value float64) {
	m.RecordMetric(name, value, types.StandardUnitNone)
}

// WithDimensions returns a new metrics collector with additional dimensions
func (m *CloudWatchMetrics) WithDimensions(dims map[string]string) *CloudWatchMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copy existing dimensions
	newDims := make([]types.Dimension, len(m.dimensions))
	copy(newDims, m.dimensions)

	// Add new dimensions
	for name, value := range dims {
		newDims = append(newDims, types.Dimension{
			Name:  aws.String(name),
			Value: aws.String(value),
		})
	}

	return &CloudWatchMetrics{
		client:          m.client,
		namespace:       m.namespace,
		buffer:          m.buffer, // Share the buffer
		flushInterval:   m.flushInterval,
		dimensions:      newDims,
		stopCh:          m.stopCh,
		doneCh:          m.doneCh,
		flushNow:        m.flushNow,
		metricsRecorded: m.metricsRecorded,
		metricsDropped:  m.metricsDropped,
		flushCount:      m.flushCount,
		errorCount:      m.errorCount,
	}
}

// WithTenant returns a new metrics collector with tenant dimension
func (m *CloudWatchMetrics) WithTenant(tenantID string) *CloudWatchMetrics {
	return m.WithDimensions(map[string]string{"TenantID": tenantID})
}

// WithTag adds a single tag/dimension
func (m *CloudWatchMetrics) WithTag(key, value string) observability.MetricsCollector {
	return m.WithDimensions(map[string]string{key: value})
}

// WithTags adds multiple tags/dimensions
func (m *CloudWatchMetrics) WithTags(tags map[string]string) observability.MetricsCollector {
	return m.WithDimensions(tags)
}

// RecordBatch records multiple metric entries at once
func (m *CloudWatchMetrics) RecordBatch(entries []*observability.MetricEntry) error {
	for _, entry := range entries {
		// Convert unit string to StandardUnit
		unit := m.parseUnit(entry.Unit)

		// Build dimensions
		dims := m.getDimensions()
		for k, v := range entry.Tags {
			dims = append(dims, types.Dimension{
				Name:  aws.String(k),
				Value: aws.String(v),
			})
		}

		datum := types.MetricDatum{
			MetricName: aws.String(entry.Name),
			Value:      aws.Float64(entry.Value),
			Unit:       unit,
			Timestamp:  aws.Time(entry.Timestamp),
			Dimensions: dims,
		}

		if shouldFlush := m.buffer.Add(datum); shouldFlush {
			select {
			case m.flushNow <- struct{}{}:
			default:
			}
		}
	}

	return nil
}

// Flush forces a flush of buffered metrics
func (m *CloudWatchMetrics) Flush() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return m.flush(ctx)
}

// Close stops the metrics collector and flushes remaining metrics
func (m *CloudWatchMetrics) Close() error {
	// Signal shutdown
	close(m.stopCh)

	// Wait for background flusher to stop
	select {
	case <-m.doneCh:
	case <-time.After(5 * time.Second):
		// Timeout waiting for graceful shutdown
	}

	// Final flush
	return m.Flush()
}

// GetStats returns metrics collection statistics
func (m *CloudWatchMetrics) GetStats() observability.MetricsStats {
	lastFlush, _ := m.lastFlush.Load().(time.Time)
	lastError, _ := m.lastError.Load().(string)

	return observability.MetricsStats{
		MetricsRecorded: atomic.LoadInt64(&m.metricsRecorded),
		MetricsDropped:  atomic.LoadInt64(&m.metricsDropped),
		LastFlush:       lastFlush,
		ErrorCount:      atomic.LoadInt64(&m.errorCount),
		LastError:       lastError,
	}
}

// backgroundFlusher runs the periodic flush loop
func (m *CloudWatchMetrics) backgroundFlusher() {
	defer close(m.doneCh)

	ticker := time.NewTicker(m.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = m.flush(ctx)
			cancel()
		case <-m.flushNow:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = m.flush(ctx)
			cancel()
		}
	}
}

// flush sends buffered metrics to CloudWatch
func (m *CloudWatchMetrics) flush(ctx context.Context) error {
	data := m.buffer.Drain()
	if len(data) == 0 {
		return nil
	}

	atomic.AddInt64(&m.flushCount, 1)

	// CloudWatch allows up to 1000 metrics per request
	// We'll send in batches of 20 for better error handling
	const batchSize = 20

	for i := 0; i < len(data); i += batchSize {
		end := i + batchSize
		if end > len(data) {
			end = len(data)
		}

		batch := data[i:end]
		input := &cloudwatch.PutMetricDataInput{
			Namespace:  aws.String(m.namespace),
			MetricData: batch,
		}

		_, err := m.client.PutMetricData(ctx, input)
		if err != nil {
			atomic.AddInt64(&m.errorCount, 1)
			atomic.AddInt64(&m.metricsDropped, int64(len(batch)))
			m.lastError.Store(err.Error())
			// Continue with next batch even if this one fails
		}
	}

	m.lastFlush.Store(time.Now())
	return nil
}

// getDimensions returns a copy of the current dimensions
func (m *CloudWatchMetrics) getDimensions() []types.Dimension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dims := make([]types.Dimension, len(m.dimensions))
	copy(dims, m.dimensions)
	return dims
}

// parseUnit converts a string unit to StandardUnit
func (m *CloudWatchMetrics) parseUnit(unit string) types.StandardUnit {
	switch unit {
	case "Count":
		return types.StandardUnitCount
	case "Milliseconds":
		return types.StandardUnitMilliseconds
	case "Seconds":
		return types.StandardUnitSeconds
	case "Bytes":
		return types.StandardUnitBytes
	case "Percent":
		return types.StandardUnitPercent
	default:
		return types.StandardUnitNone
	}
}

// Implement lift.MetricsCollector interface methods

// RecordLatency records the latency of an operation
func (m *CloudWatchMetrics) RecordLatency(operation string, duration time.Duration) {
	m.RecordDuration(fmt.Sprintf("%s.latency", operation), duration)
}

// RecordError records that an error occurred
func (m *CloudWatchMetrics) RecordError(operation string) {
	m.RecordCount(fmt.Sprintf("%s.errors", operation), 1)
}

// RecordSuccess records a successful operation
func (m *CloudWatchMetrics) RecordSuccess(operation string) {
	m.RecordCount(fmt.Sprintf("%s.success", operation), 1)
}

// Counter returns a counter metric implementation
func (m *CloudWatchMetrics) Counter(name string, tags ...map[string]string) lift.Counter {
	// Apply tags if provided
	collector := m
	if len(tags) > 0 && tags[0] != nil {
		collector = m.WithDimensions(tags[0])
	}

	return &cloudWatchCounter{
		metrics: collector,
		name:    name,
	}
}

// Histogram returns a histogram metric implementation
func (m *CloudWatchMetrics) Histogram(name string, tags ...map[string]string) lift.Histogram {
	// Apply tags if provided
	collector := m
	if len(tags) > 0 && tags[0] != nil {
		collector = m.WithDimensions(tags[0])
	}

	return &cloudWatchHistogram{
		metrics: collector,
		name:    name,
	}
}

// Gauge returns a gauge metric implementation
func (m *CloudWatchMetrics) Gauge(name string, tags ...map[string]string) lift.Gauge {
	// Apply tags if provided
	collector := m
	if len(tags) > 0 && tags[0] != nil {
		collector = m.WithDimensions(tags[0])
	}

	return &cloudWatchGauge{
		metrics: collector,
		name:    name,
	}
}

// cloudWatchCounter implements lift.Counter
type cloudWatchCounter struct {
	metrics *CloudWatchMetrics
	name    string
}

func (c *cloudWatchCounter) Inc() {
	c.metrics.RecordCount(c.name, 1)
}

func (c *cloudWatchCounter) Add(value float64) {
	c.metrics.RecordMetric(c.name, value, types.StandardUnitCount)
}

// cloudWatchHistogram implements lift.Histogram
type cloudWatchHistogram struct {
	metrics *CloudWatchMetrics
	name    string
}

func (h *cloudWatchHistogram) Observe(value float64) {
	h.metrics.RecordMetric(h.name, value, types.StandardUnitNone)
}

// cloudWatchGauge implements lift.Gauge
type cloudWatchGauge struct {
	metrics *CloudWatchMetrics
	name    string
}

func (g *cloudWatchGauge) Set(value float64) {
	g.metrics.RecordGauge(g.name, value)
}

func (g *cloudWatchGauge) Inc() {
	// For CloudWatch, we'll just record the increment as a gauge value
	// In a real implementation, you might want to track the current value
	g.metrics.RecordMetric(g.name, 1, types.StandardUnitNone)
}

func (g *cloudWatchGauge) Dec() {
	// For CloudWatch, we'll just record the decrement as a gauge value
	g.metrics.RecordMetric(g.name, -1, types.StandardUnitNone)
}

func (g *cloudWatchGauge) Add(value float64) {
	g.metrics.RecordMetric(g.name, value, types.StandardUnitNone)
}
