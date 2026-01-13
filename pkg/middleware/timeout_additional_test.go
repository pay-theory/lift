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

func TestTimeoutMiddleware_CompletesWithinTimeout_RecordsSuccess(t *testing.T) {
	logger := &mockLogger{}
	metrics := newMockMetricsCollector()

	mw := TimeoutMiddleware(TimeoutConfig{
		Name:           "test",
		DefaultTimeout: 50 * time.Millisecond,
		EnableMetrics:  true,
		Logger:         logger,
		Metrics:        metrics,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/ok"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.Contains(t, metrics.metrics, "timeout.requests.total")
	require.Contains(t, metrics.metrics, "timeout.ratio")
	require.Greater(t, len(logger.logs), 0)
}

func TestTimeoutMiddleware_TimesOut_InvokesTimeoutHandler(t *testing.T) {
	logger := &mockLogger{}
	metrics := newMockMetricsCollector()

	mw := TimeoutMiddleware(TimeoutConfig{
		Name:           "test",
		DefaultTimeout: 5 * time.Millisecond,
		EnableMetrics:  true,
		Logger:         logger,
		Metrics:        metrics,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/slow"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		localCtx := ctx.Context
		<-localCtx.Done()
		return localCtx.Err()
	}))

	require.NoError(t, handler.Handle(ctx))
	require.Equal(t, 408, ctx.Response.StatusCode)
	require.Contains(t, metrics.metrics, "timeout.requests.total")
	require.Greater(t, len(logger.logs), 0)
}

func TestTimeoutManager_CalculateTimeout_PrefersDynamicTenantOperationThenDefault(t *testing.T) {
	manager := &timeoutManager{config: TimeoutConfig{
		DefaultTimeout:       10 * time.Second,
		EnableDynamicTimeout: true,
		TimeoutCalculator: func(_ *lift.Context) time.Duration {
			return 1 * time.Second
		},
		TenantTimeouts:    map[string]time.Duration{"tenant-1": 2 * time.Second},
		OperationTimeouts: map[string]time.Duration{"GET:/op": 3 * time.Second},
	}}

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/op"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.SetTenantID("tenant-1")

	require.Equal(t, 1*time.Second, manager.calculateTimeout(ctx))

	manager.config.EnableDynamicTimeout = false
	require.Equal(t, 2*time.Second, manager.calculateTimeout(ctx))

	ctx.SetTenantID("")
	require.Equal(t, 3*time.Second, manager.calculateTimeout(ctx))

	ctx.Request.Path = "/other"
	require.Equal(t, 10*time.Second, manager.calculateTimeout(ctx))
}

func TestTimeoutManager_GetStats_ReturnsCopy(t *testing.T) {
	manager := &timeoutManager{stats: &TimeoutStats{Name: "t"}}
	stats := manager.GetStats()
	require.Equal(t, "t", stats.Name)
}

func TestTimeoutHelpersAndCalculators(t *testing.T) {
	cfg := NewBasicTimeout("basic", 12*time.Second)
	require.Equal(t, 12*time.Second, cfg.DefaultTimeout)

	opCfg := NewOperationTimeout("op", time.Second, map[string]time.Duration{"GET:/x": 2 * time.Second})
	require.Equal(t, 2*time.Second, opCfg.OperationTimeouts["GET:/x"])

	tenantCfg := NewTenantTimeout("tenant", time.Second, map[string]time.Duration{"t": 3 * time.Second})
	require.Equal(t, 3*time.Second, tenantCfg.TenantTimeouts["t"])

	dynCfg := NewDynamicTimeout("dyn", time.Second, func(_ *lift.Context) time.Duration { return 4 * time.Second })
	require.True(t, dynCfg.EnableDynamicTimeout)
	require.NotNil(t, dynCfg.TimeoutCalculator)

	adaptive := AdaptiveTimeoutCalculator(4 * time.Second)
	priority := PriorityTimeoutCalculator(4 * time.Second)

	req := lift.NewRequest(&adapters.Request{
		Method:      "POST",
		Path:        "/x",
		Headers:     map[string]string{"X-Priority": "high"},
		QueryParams: map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5", "f": "6"},
		Body:        []byte("small"),
	})
	ctx := lift.NewContext(context.Background(), req)

	require.Greater(t, adaptive(ctx), 4*time.Second)
	require.Equal(t, 8*time.Second, priority(ctx))

	loadMetrics := &LoadMetrics{CPUUsage: 0.9, MemoryUsage: 0.9, ActiveRequests: 200, ErrorRate: 0.2}
	loadBased := LoadBasedTimeoutCalculator(4*time.Second, loadMetrics)
	require.GreaterOrEqual(t, loadBased(ctx), 4*time.Second)
	require.LessOrEqual(t, loadBased(ctx), 20*time.Second)

	nilLoadBased := LoadBasedTimeoutCalculator(4*time.Second, nil)
	require.Equal(t, 4*time.Second, nilLoadBased(ctx))

	handler := defaultTimeoutHandler(418, "teapot")
	ctx2 := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/t"}))
	require.NoError(t, handler(ctx2))
	require.Equal(t, 418, ctx2.Response.StatusCode)

	require.NotNil(t, ctx2.Response.Body)
	_, ok := ctx2.Response.Body.(map[string]any)
	require.True(t, ok)
}

func TestTimeoutMiddleware_RecoversFromPanicInHandler(t *testing.T) {
	mw := TimeoutMiddleware(TimeoutConfig{
		DefaultTimeout: 50 * time.Millisecond,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/panic"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		panic("boom")
	}))

	err := handler.Handle(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "panic in handler")
}

func TestTimeoutMiddleware_DoesNotDecrementOnErrorOrBadStatus(t *testing.T) {
	metrics := newMockMetricsCollector()
	mw := TimeoutMiddleware(TimeoutConfig{
		DefaultTimeout: 50 * time.Millisecond,
		EnableMetrics:  true,
		Metrics:        metrics,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/err"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 500
		return errors.New("boom")
	}))

	require.Error(t, handler.Handle(ctx))
	require.Contains(t, metrics.metrics, "timeout.requests.total")
}
