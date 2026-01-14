package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

type stubHealthChecker struct {
	name     string
	required bool
	err      error
}

func (s stubHealthChecker) Name() string { return s.name }
func (s stubHealthChecker) Check(_ context.Context) error {
	return s.err
}
func (s stubHealthChecker) IsRequired() bool { return s.required }

func TestHealthCheckMiddleware_HealthAndDetailEndpoints(t *testing.T) {
	metrics := newMockMetricsCollector()

	mw := HealthCheckMiddleware(HealthCheckConfig{
		EnableMetrics: true,
		Metrics:       metrics,
		Dependencies: []HealthChecker{
			stubHealthChecker{name: "db", required: true, err: errors.New("down")},
			stubHealthChecker{name: "cache", required: false, err: nil},
		},
		GracePeriod: time.Nanosecond,
	})

	healthCtx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/health"}))
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(healthCtx))
	require.Equal(t, 503, healthCtx.Response.StatusCode)

	detailCtx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/health/detail"}))
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(detailCtx))
	require.Equal(t, 503, detailCtx.Response.StatusCode)

	require.Contains(t, metrics.metrics, "health_checks.total")
	require.Contains(t, metrics.metrics, "health_checks.duration")
}

func TestHealthCheckMiddleware_Readiness_GracePeriod(t *testing.T) {
	mw := HealthCheckMiddleware(HealthCheckConfig{
		GracePeriod: time.Hour,
	})

	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/ready"}))
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx))
	require.Equal(t, 503, ctx.Response.StatusCode)
}

func TestHealthCheckMiddleware_Readiness_AfterGracePeriod(t *testing.T) {
	mw := HealthCheckMiddleware(HealthCheckConfig{
		GracePeriod: time.Nanosecond,
		Dependencies: []HealthChecker{
			stubHealthChecker{name: "db", required: true, err: nil},
		},
	})

	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/ready"}))
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx))
	require.Equal(t, 200, ctx.Response.StatusCode)
}

func TestHealthCheckMiddleware_Liveness(t *testing.T) {
	mw := HealthCheckMiddleware(HealthCheckConfig{})

	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/live"}))
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx))
	require.Equal(t, 200, ctx.Response.StatusCode)
}

func TestHealthMonitor_RunSingleCheck_CircuitBreakerOpen(t *testing.T) {
	monitor := &healthMonitor{
		config: HealthCheckConfig{FailureThreshold: 3},
		failureCounts: map[string]int{
			"db": 3,
		},
	}

	result := monitor.runSingleCheck(context.Background(), stubHealthChecker{name: "db", required: true})
	require.Equal(t, HealthStatusUnhealthy, result.Status)
	require.Contains(t, result.Message, "Circuit breaker open")
}

func TestHealthMonitor_RunSingleCheck_FailureAndRecovery(t *testing.T) {
	logger := &mockLogger{}
	monitor := &healthMonitor{
		config: HealthCheckConfig{FailureThreshold: 3, Logger: logger},
		failureCounts: map[string]int{
			"db": 0,
		},
	}

	result := monitor.runSingleCheck(context.Background(), stubHealthChecker{name: "db", required: true, err: errors.New("down")})
	require.Equal(t, HealthStatusUnhealthy, result.Status)
	require.Equal(t, 1, monitor.failureCounts["db"])

	result = monitor.runSingleCheck(context.Background(), stubHealthChecker{name: "db", required: true, err: nil})
	require.Equal(t, HealthStatusHealthy, result.Status)
	require.Equal(t, 0, monitor.failureCounts["db"])
}

func TestHealthMonitor_GetHealthStatus_CachedAndUnknown(t *testing.T) {
	monitor := &healthMonitor{
		config:    HealthCheckConfig{Interval: time.Hour},
		lastCheck: time.Now(),
		results: map[string]*HealthCheckResult{
			"db": {Name: "db", Status: HealthStatusUnhealthy, Required: true},
		},
	}

	cached := monitor.GetHealthStatus()
	require.Equal(t, HealthStatusUnhealthy, cached.Status)
	require.NotNil(t, cached.Summary)

	monitor.lastCheck = time.Now().Add(-2 * time.Hour)
	monitor.config.Interval = time.Second
	unknown := monitor.GetHealthStatus()
	require.Equal(t, HealthStatusUnknown, unknown.Status)
}

func TestHealthCheckers_BuiltinsAndAlias(t *testing.T) {
	dbChecker := NewDatabaseHealthChecker("db", true, func(_ context.Context) error { return nil })
	require.Equal(t, "db", dbChecker.Name())
	require.True(t, dbChecker.IsRequired())
	require.NoError(t, dbChecker.Check(context.Background()))

	httpChecker := NewHTTPHealthChecker("upstream", "https://example.com", false, time.Second)
	require.Equal(t, "upstream", httpChecker.Name())
	require.NoError(t, httpChecker.Check(context.Background()))
	require.False(t, httpChecker.IsRequired())

	memChecker := NewMemoryHealthChecker("mem", 0.9)
	require.Equal(t, "mem", memChecker.Name())
	require.NoError(t, memChecker.Check(context.Background()))
	require.False(t, memChecker.IsRequired())

	require.NotNil(t, HealthMiddleware(HealthConfig{}))
}
