package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/observability"
)

// Mock implementations for testing

type mockLogger struct {
	logs    []map[string]interface{}
	healthy bool
}

func (m *mockLogger) Info(msg string, fields ...map[string]interface{}) {
	entry := map[string]interface{}{"level": "info", "message": msg}
	for _, fieldMap := range fields {
		for k, v := range fieldMap {
			entry[k] = v
		}
	}
	m.logs = append(m.logs, entry)
}

func (m *mockLogger) Error(msg string, fields ...map[string]interface{}) {
	entry := map[string]interface{}{"level": "error", "message": msg}
	for _, fieldMap := range fields {
		for k, v := range fieldMap {
			entry[k] = v
		}
	}
	m.logs = append(m.logs, entry)
}

func (m *mockLogger) Warn(msg string, fields ...map[string]interface{}) {
	entry := map[string]interface{}{"level": "warn", "message": msg}
	for _, fieldMap := range fields {
		for k, v := range fieldMap {
			entry[k] = v
		}
	}
	m.logs = append(m.logs, entry)
}

func (m *mockLogger) Debug(msg string, fields ...map[string]interface{}) {
	entry := map[string]interface{}{"level": "debug", "message": msg}
	for _, fieldMap := range fields {
		for k, v := range fieldMap {
			entry[k] = v
		}
	}
	m.logs = append(m.logs, entry)
}

func (m *mockLogger) WithField(key string, value interface{}) lift.Logger  { return m }
func (m *mockLogger) WithFields(fields map[string]interface{}) lift.Logger { return m }

func (m *mockLogger) WithRequestID(requestID string) observability.StructuredLogger { return m }
func (m *mockLogger) WithTenantID(tenantID string) observability.StructuredLogger   { return m }
func (m *mockLogger) WithUserID(userID string) observability.StructuredLogger       { return m }
func (m *mockLogger) WithTraceID(traceID string) observability.StructuredLogger     { return m }
func (m *mockLogger) WithSpanID(spanID string) observability.StructuredLogger       { return m }

func (m *mockLogger) Flush(ctx context.Context) error { return nil }
func (m *mockLogger) Close() error                    { return nil }
func (m *mockLogger) IsHealthy() bool                 { return m.healthy }
func (m *mockLogger) GetStats() observability.LoggerStats {
	return observability.LoggerStats{
		EntriesLogged:  int64(len(m.logs)),
		BufferSize:     0,
		BufferCapacity: 1000,
	}
}

type mockMetrics struct {
	metrics map[string]interface{}
	tags    map[string]string
}

func (m *mockMetrics) Counter(name string, tags ...map[string]string) lift.Counter {
	return &mockCounter{metrics: m.metrics, name: name}
}

func (m *mockMetrics) Histogram(name string, tags ...map[string]string) lift.Histogram {
	return &mockHistogram{metrics: m.metrics, name: name}
}

func (m *mockMetrics) Gauge(name string, tags ...map[string]string) lift.Gauge {
	return &mockGauge{metrics: m.metrics, name: name}
}

func (m *mockMetrics) Flush() error { return nil }

func (m *mockMetrics) WithTags(tags map[string]string) observability.MetricsCollector {
	newTags := make(map[string]string)
	for k, v := range m.tags {
		newTags[k] = v
	}
	for k, v := range tags {
		newTags[k] = v
	}
	return &mockMetrics{metrics: m.metrics, tags: newTags}
}

func (m *mockMetrics) WithTag(key, value string) observability.MetricsCollector {
	return m.WithTags(map[string]string{key: value})
}

func (m *mockMetrics) RecordBatch(entries []*observability.MetricEntry) error { return nil }
func (m *mockMetrics) Close() error                                           { return nil }
func (m *mockMetrics) GetStats() observability.MetricsStats {
	return observability.MetricsStats{
		MetricsRecorded: int64(len(m.metrics)),
	}
}

// Add missing methods to implement updated MetricsCollector interface
func (m *mockMetrics) RecordLatency(operation string, duration time.Duration) {
	m.metrics[operation+".latency"] = duration.Milliseconds()
}

func (m *mockMetrics) RecordError(operation string) {
	key := operation + ".errors"
	if val, exists := m.metrics[key]; exists {
		m.metrics[key] = val.(int) + 1
	} else {
		m.metrics[key] = 1
	}
}

func (m *mockMetrics) RecordSuccess(operation string) {
	key := operation + ".success"
	if val, exists := m.metrics[key]; exists {
		m.metrics[key] = val.(int) + 1
	} else {
		m.metrics[key] = 1
	}
}

// Helper method to get metrics count for testing
func (m *mockMetrics) GetMetricsCount() int {
	return len(m.metrics)
}

type mockCounter struct {
	metrics map[string]interface{}
	name    string
}

func (c *mockCounter) Inc() {
	if val, exists := c.metrics[c.name]; exists {
		c.metrics[c.name] = val.(int) + 1
	} else {
		c.metrics[c.name] = 1
	}
}

func (c *mockCounter) Add(value float64) {
	if val, exists := c.metrics[c.name]; exists {
		c.metrics[c.name] = val.(float64) + value
	} else {
		c.metrics[c.name] = value
	}
}

type mockHistogram struct {
	metrics map[string]interface{}
	name    string
}

func (h *mockHistogram) Observe(value float64) {
	h.metrics[h.name] = value
}

type mockGauge struct {
	metrics map[string]interface{}
	name    string
}

func (g *mockGauge) Set(value float64) {
	g.metrics[g.name] = value
}

func (g *mockGauge) Inc() {
	if val, exists := g.metrics[g.name]; exists {
		g.metrics[g.name] = val.(float64) + 1
	} else {
		g.metrics[g.name] = 1.0
	}
}

func (g *mockGauge) Dec() {
	if val, exists := g.metrics[g.name]; exists {
		g.metrics[g.name] = val.(float64) - 1
	} else {
		g.metrics[g.name] = -1.0
	}
}

func (g *mockGauge) Add(value float64) {
	if val, exists := g.metrics[g.name]; exists {
		g.metrics[g.name] = val.(float64) + value
	} else {
		g.metrics[g.name] = value
	}
}

func TestEnhancedObservabilityMiddleware(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{
		metrics: make(map[string]interface{}),
		tags:    make(map[string]string),
	}

	config := EnhancedObservabilityConfig{
		EnableLogging: true,
		EnableMetrics: true,
		EnableTracing: false, // Skip X-Ray for unit tests
		Logger:        logger,
		Metrics:       metrics,
	}

	middleware := EnhancedObservabilityMiddleware(config)

	handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{"message": "success"})
	}))

	// Create a proper Request using the adapters package
	adapterRequest := &adapters.Request{
		Method:      "GET",
		Path:        "/test",
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
	}

	ctx := &lift.Context{
		Context: context.Background(),
		Request: &lift.Request{Request: adapterRequest},
		Response: &lift.Response{
			StatusCode: 200,
			Headers:    make(map[string]string),
		},
	}

	err := handler.Handle(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify logs were generated
	if len(logger.logs) == 0 {
		t.Error("Expected logs to be generated")
	}

	// Verify metrics were recorded
	if metrics.GetMetricsCount() == 0 {
		t.Error("Expected metrics to be recorded")
	}
}

func TestObservabilityWithTenantContext(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{
		metrics: make(map[string]interface{}),
		tags:    make(map[string]string),
	}

	config := EnhancedObservabilityConfig{
		EnableLogging: true,
		EnableMetrics: true,
		EnableTracing: false,
		Logger:        logger,
		Metrics:       metrics,
	}

	middleware := EnhancedObservabilityMiddleware(config)

	handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
		return ctx.JSON(map[string]string{"tenant": ctx.TenantID()})
	}))

	// Create a proper Request using the adapters package
	adapterRequest := &adapters.Request{
		Method:      "GET",
		Path:        "/test",
		Headers:     map[string]string{"X-Tenant-ID": "test-tenant"},
		QueryParams: make(map[string]string),
	}

	ctx := &lift.Context{
		Context: context.Background(),
		Request: &lift.Request{Request: adapterRequest},
		Response: &lift.Response{
			StatusCode: 200,
			Headers:    make(map[string]string),
		},
	}
	ctx.Set("tenant_id", "test-tenant")

	err := handler.Handle(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify tenant context was captured
	found := false
	for _, log := range logger.logs {
		if tenantID, exists := log["tenant_id"]; exists && tenantID == "test-tenant" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected tenant context in logs")
	}
}

func BenchmarkEnhancedObservabilityMiddleware(b *testing.B) {
	config := EnhancedObservabilityConfig{
		EnableLogging: false, // Disable for pure performance test
		EnableMetrics: false,
		EnableTracing: false,
	}

	middleware := EnhancedObservabilityMiddleware(config)

	handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
		return nil
	}))

	// Create a proper Request using the adapters package
	adapterRequest := &adapters.Request{
		Method:      "GET",
		Path:        "/test",
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
	}

	ctx := &lift.Context{
		Context: context.Background(),
		Request: &lift.Request{Request: adapterRequest},
		Response: &lift.Response{
			StatusCode: 200,
			Headers:    make(map[string]string),
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler.Handle(ctx)
	}
}

func TestEnhancedObservabilityDefaults(t *testing.T) {
	config := EnhancedObservabilityConfig{}
	middleware := EnhancedObservabilityMiddleware(config)

	// Test that defaults are set correctly
	ctx := &lift.Context{
		Context: context.Background(),
		Request: &lift.Request{
			Method:  "GET",
			Path:    "/test",
			Headers: make(map[string]string),
		},
		Response: &lift.Response{},
	}

	handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
		return nil
	}))

	err := handler.Handle(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGetObservabilityStats(t *testing.T) {
	logger := &mockLogger{healthy: true}
	metrics := &mockMetrics{metrics: make(map[string]interface{}), tags: make(map[string]string)}

	config := EnhancedObservabilityConfig{
		Logger:  logger,
		Metrics: metrics,
	}

	stats := GetObservabilityStats(config)

	if stats.Logger == nil {
		t.Error("Expected logger stats to be present")
	}

	if stats.Metrics == nil {
		t.Error("Expected metrics stats to be present")
	}

	if stats.Tracing == nil {
		t.Error("Expected tracing stats to be present")
	}
}

func TestHealthCheckObservability(t *testing.T) {
	tests := []struct {
		name        string
		config      EnhancedObservabilityConfig
		expectError bool
	}{
		{
			name: "healthy logger",
			config: EnhancedObservabilityConfig{
				Logger:        &mockLogger{healthy: true},
				EnableLogging: true,
			},
			expectError: false,
		},
		{
			name: "unhealthy logger",
			config: EnhancedObservabilityConfig{
				Logger:        &mockLogger{healthy: false},
				EnableLogging: true,
			},
			expectError: true,
		},
		{
			name: "healthy metrics",
			config: EnhancedObservabilityConfig{
				Metrics:       &mockMetrics{metrics: make(map[string]interface{}), tags: make(map[string]string)},
				EnableMetrics: true,
			},
			expectError: false,
		},
		{
			name: "disabled components",
			config: EnhancedObservabilityConfig{
				EnableLogging: false,
				EnableMetrics: false,
				EnableTracing: false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthCheck := HealthCheckObservability(tt.config)
			err := healthCheck()

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func BenchmarkEnhancedObservabilityLoggingOnly(b *testing.B) {
	logger := &mockLogger{healthy: true}

	config := EnhancedObservabilityConfig{
		Logger:        logger,
		EnableLogging: true,
		EnableMetrics: false,
		EnableTracing: false,
	}

	middleware := EnhancedObservabilityMiddleware(config)

	handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
		return nil
	}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := &lift.Context{
			Context: context.Background(),
			Request: &lift.Request{
				Method:  "GET",
				Path:    "/test",
				Headers: make(map[string]string),
			},
			Response: &lift.Response{},
		}

		_ = handler.Handle(ctx)
	}
}

func BenchmarkEnhancedObservabilityMetricsOnly(b *testing.B) {
	metrics := &mockMetrics{metrics: make(map[string]interface{}), tags: make(map[string]string)}

	config := EnhancedObservabilityConfig{
		Metrics:       metrics,
		EnableLogging: false,
		EnableMetrics: true,
		EnableTracing: false,
	}

	middleware := EnhancedObservabilityMiddleware(config)

	handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
		return nil
	}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := &lift.Context{
			Context: context.Background(),
			Request: &lift.Request{
				Method:  "GET",
				Path:    "/test",
				Headers: make(map[string]string),
			},
			Response: &lift.Response{},
		}

		_ = handler.Handle(ctx)
	}
}
