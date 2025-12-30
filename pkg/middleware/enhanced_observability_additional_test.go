package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/observability"
	"github.com/stretchr/testify/require"
)

type stubMetricsCollector struct {
	stats observability.MetricsStats
}

func (s *stubMetricsCollector) Counter(string, ...map[string]string) lift.Counter                { return &noopCounter{} }
func (s *stubMetricsCollector) Histogram(string, ...map[string]string) lift.Histogram            { return &noopHistogram{} }
func (s *stubMetricsCollector) Gauge(string, ...map[string]string) lift.Gauge                    { return &noopGauge{} }
func (s *stubMetricsCollector) Flush() error                                                     { return nil }
func (s *stubMetricsCollector) WithTags(map[string]string) observability.MetricsCollector        { return s }
func (s *stubMetricsCollector) WithTag(string, string) observability.MetricsCollector            { return s }
func (s *stubMetricsCollector) RecordBatch([]*observability.MetricEntry) error                   { return nil }
func (s *stubMetricsCollector) Close() error                                                     { return nil }
func (s *stubMetricsCollector) GetStats() observability.MetricsStats                             { return s.stats }
func (s *stubMetricsCollector) RecordLatency(string, time.Duration)                              {}
func (s *stubMetricsCollector) RecordError(string)                                               {}
func (s *stubMetricsCollector) RecordSuccess(string)                                             {}

type noopCounter struct{}

func (noopCounter) Inc()        {}
func (noopCounter) Add(float64) {}

type noopHistogram struct{}

func (noopHistogram) Observe(float64) {}

type noopGauge struct{}

func (noopGauge) Set(float64) {}
func (noopGauge) Inc()        {}
func (noopGauge) Dec()        {}
func (noopGauge) Add(float64) {}

func TestEnhancedObservability_LoggingBodiesAndMetricsSizing(t *testing.T) {
	logger := &mockLogger{}
	metrics := newMockMetricsCollector()

	cfg := EnhancedObservabilityConfig{
		EnableLogging:   true,
		EnableMetrics:   true,
		EnableTracing:   false,
		SampleRate:      1,
		Logger:          logger,
		Metrics:         metrics,
		LogRequestBody:  true,
		LogResponseBody: true,
	}

	mw := EnhancedObservabilityMiddleware(cfg)

	req := lift.NewRequest(&adapters.Request{
		Method:      "POST",
		Path:        "/test",
		Headers:     map[string]string{"User-Agent": "ua", "X-Forwarded-For": "10.0.0.1"},
		QueryParams: map[string]string{"q": "1"},
		Body:        []byte("hello"),
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.RequestID = "req-1"
	ctx.SetTenantID("tenant-1")
	ctx.SetUserID("user-1")

	require.NoError(t, mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.Body = "ok"
		ctx.Response.StatusCode = 200
		return nil
	})).Handle(ctx))

	require.Contains(t, metrics.metrics, "requests.total")
	require.Contains(t, metrics.metrics, "requests.active")
	require.Contains(t, metrics.metrics, "requests.duration")
	require.Contains(t, metrics.metrics, "response.size")
	require.Greater(t, len(logger.logs), 0)

	// bytes response branch
	ctx2 := lift.NewContext(context.Background(), req)
	require.NoError(t, mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.Body = []byte("bytes")
		ctx.Response.StatusCode = 200
		return nil
	})).Handle(ctx2))
}

func TestObservabilityHandler_HelperBranches(t *testing.T) {
	cfg := EnhancedObservabilityConfig{
		EnableLogging: true,
		Logger:        &mockLogger{},
		EnableMetrics: false,
		SampleRate:    1,
	}
	h := newObservabilityHandler(cfg)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/x"})
	ctx := lift.NewContext(context.Background(), req)

	h.config.OperationNameFunc = nil
	h.config.TenantIDFunc = nil
	h.config.UserIDFunc = nil

	require.Equal(t, "GET_/x", h.extractOperation(ctx))
	require.Equal(t, "", h.extractTenantID(ctx))
	require.Equal(t, "", h.extractUserID(ctx))

	ctx.Response.StatusCode = 0
	require.Equal(t, 200, h.determineStatusCode(ctx, nil))
	require.Equal(t, 500, h.determineStatusCode(ctx, assertErr{}))
	ctx.Response.StatusCode = 418
	require.Equal(t, 418, h.determineStatusCode(ctx, nil))
}

type assertErr struct{}

func (assertErr) Error() string { return "err" }

func TestObservabilityHandler_ShouldSample_Branches(t *testing.T) {
	h := newObservabilityHandler(EnhancedObservabilityConfig{SampleRate: 0})
	require.False(t, h.shouldSample())

	h = newObservabilityHandler(EnhancedObservabilityConfig{SampleRate: 1})
	require.True(t, h.shouldSample())

	h = newObservabilityHandler(EnhancedObservabilityConfig{
		SampleRate: 0.5,
		Sampler:    func() float64 { return 0.0 },
	})
	require.True(t, h.shouldSample())

	h = newObservabilityHandler(EnhancedObservabilityConfig{
		SampleRate: 0.5,
		Sampler:    func() float64 { return 1.0 },
	})
	require.False(t, h.shouldSample())

	// Nil sampler path (non-deterministic); just ensure it doesn't panic.
	_ = newObservabilityHandler(EnhancedObservabilityConfig{SampleRate: 0.5}).shouldSample()
}

func TestHealthCheckObservability_DetectsUnhealthyLoggerAndMetrics(t *testing.T) {
	unhealthyLogger := &mockLogger{healthy: false}
	cfg := EnhancedObservabilityConfig{EnableLogging: true, Logger: unhealthyLogger}
	require.Error(t, HealthCheckObservability(cfg)())

	metrics := &stubMetricsCollector{stats: observability.MetricsStats{ErrorCount: 10, MetricsRecorded: 1, LastError: "boom"}}
	cfg2 := EnhancedObservabilityConfig{EnableMetrics: true, Metrics: metrics}
	require.Error(t, HealthCheckObservability(cfg2)())

	metricsOK := &stubMetricsCollector{stats: observability.MetricsStats{ErrorCount: 1, MetricsRecorded: 100, LastError: "boom"}}
	cfg3 := EnhancedObservabilityConfig{EnableMetrics: true, Metrics: metricsOK}
	require.NoError(t, HealthCheckObservability(cfg3)())
}

