package middleware

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestLoadSheddingHandler_HandleShedding_InvokesHandlerAndRecordsMetrics(t *testing.T) {
	logger := &mockLogger{}
	metrics := newMockMetricsCollector()

	cfg := applyLoadSheddingDefaults(LoadSheddingConfig{
		Enabled:       true,
		EnableMetrics: true,
		Logger:        logger,
		Metrics:       metrics,
		Strategy:      LoadSheddingCustom,
		CustomShedder: func(_ *lift.Context, _ *LoadMetrics) bool { return true },
	})

	manager := newLoadSheddingManager(cfg)
	manager.setCurrentSheddingRate(0.42)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/shed", Headers: map[string]string{"X-Priority": "high"}})
	ctx := lift.NewContext(context.Background(), req)

	nextCalled := false
	require.NoError(t, manager.handleRequest(ctx, lift.HandlerFunc(func(_ *lift.Context) error {
		nextCalled = true
		return nil
	})))

	require.False(t, nextCalled)
	require.Equal(t, 503, ctx.Response.StatusCode)
	require.GreaterOrEqual(t, atomic.LoadInt64(&manager.metrics.ShedRequests), int64(1))
	require.Contains(t, metrics.metrics, "load_shedding.requests.total")
	require.Contains(t, metrics.metrics, "load_shedding.rate")
	require.Greater(t, len(logger.logs), 0)
}

func TestLoadSheddingHandler_ExecutesRequest_RecordsSuccessAndError(t *testing.T) {
	metrics := newMockMetricsCollector()

	cfg := applyLoadSheddingDefaults(LoadSheddingConfig{
		Enabled:       true,
		EnableMetrics: true,
		Metrics:       metrics,
		Strategy:      LoadSheddingCustom,
		CustomShedder: func(_ *lift.Context, _ *LoadMetrics) bool { return false },
	})

	manager := newLoadSheddingManager(cfg)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/ok"})
	ctx := lift.NewContext(context.Background(), req)
	require.NoError(t, manager.handleRequest(ctx, lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	})))

	errCtx := lift.NewContext(context.Background(), req)
	require.Error(t, manager.handleRequest(errCtx, lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 500
		return lift.SystemError("boom")
	})))

	require.Contains(t, metrics.metrics, "load_shedding.latency")
	require.Contains(t, metrics.metrics, "load_shedding.active_requests")
}

func TestLoadSheddingStrategies_PriorityAdaptiveCircuit(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	priorityCfg := applyLoadSheddingDefaults(LoadSheddingConfig{
		Enabled:            true,
		Strategy:           LoadSheddingPriority,
		MinSheddingRate:    1,
		MaxSheddingRate:    1,
		PriorityExtractor:  func(_ *lift.Context) int { return 0 },
		PriorityThresholds: map[int]float64{0: 0},
	})
	priorityMgr := newLoadSheddingManager(priorityCfg)
	require.False(t, priorityMgr.priorityShedding(ctx))
	priorityMgr.config.PriorityThresholds = nil
	require.True(t, priorityMgr.priorityShedding(ctx))

	adaptiveCfg := applyLoadSheddingDefaults(LoadSheddingConfig{
		Enabled:         true,
		Strategy:        LoadSheddingAdaptive,
		TargetLatency:   100 * time.Millisecond,
		AdaptationRate:  1,
		MinSheddingRate: 0,
		MaxSheddingRate: 1,
	})
	adaptiveMgr := newLoadSheddingManager(adaptiveCfg)
	adaptiveMgr.metrics.AverageLatency = 50 * time.Millisecond
	adaptiveMgr.setCurrentSheddingRate(0)
	require.False(t, adaptiveMgr.adaptiveShedding())

	adaptiveMgr.metrics.AverageLatency = 300 * time.Millisecond
	adaptiveMgr.setCurrentSheddingRate(0)
	require.True(t, adaptiveMgr.adaptiveShedding())

	circuitCfg := applyLoadSheddingDefaults(LoadSheddingConfig{
		Enabled:         true,
		Strategy:        LoadSheddingCircuit,
		MaxSheddingRate: 1,
	})
	circuitMgr := newLoadSheddingManager(circuitCfg)
	circuitMgr.metrics.CPUUsage = 0
	circuitMgr.metrics.MemoryUsage = 0
	circuitMgr.metrics.AverageLatency = 0
	circuitMgr.metrics.ErrorRate = 0
	require.False(t, circuitMgr.circuitShedding())

	circuitMgr.metrics.CPUUsage = 1
	circuitMgr.metrics.MemoryUsage = 1
	circuitMgr.metrics.AverageLatency = 10 * time.Second
	circuitMgr.metrics.ErrorRate = 1
	require.True(t, circuitMgr.circuitShedding())

	require.GreaterOrEqual(t, circuitMgr.getCurrentSheddingRate(), 0.0)
}

func TestLoadSheddingManager_UpdateMetricsAndGetStats(t *testing.T) {
	cfg := applyLoadSheddingDefaults(LoadSheddingConfig{
		Enabled:       true,
		EnableMetrics: false,
		Strategy:      LoadSheddingRandom,
		MetricsWindow: 1 * time.Millisecond,
	})
	manager := newLoadSheddingManager(cfg)

	atomic.StoreInt64(&manager.metrics.TotalRequests, 10)
	atomic.StoreInt64(&manager.metrics.ShedRequests, 2)
	manager.metrics.WindowStart = time.Now().Add(-10 * time.Second)
	manager.recordError()
	manager.recordError()

	manager.updateMetrics()
	stats := manager.GetStats()
	require.Equal(t, cfg.Name, stats.Name)
	require.GreaterOrEqual(t, stats.SheddingRatio, 0.0)
}

func TestLoadSheddingDefaults_ExtractorAndHandlerAndBuilders(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"X-Priority": "critical"},
	})
	ctx := lift.NewContext(context.Background(), req)
	require.Equal(t, 10, defaultLoadSheddingPriorityExtractor(ctx))

	ctx.Request.Headers["X-Priority"] = priorityHigh
	require.Equal(t, 8, defaultLoadSheddingPriorityExtractor(ctx))

	ctx.Request.Headers["X-Priority"] = "normal"
	require.Equal(t, 5, defaultLoadSheddingPriorityExtractor(ctx))

	ctx.Request.Headers["X-Priority"] = priorityLow
	require.Equal(t, 2, defaultLoadSheddingPriorityExtractor(ctx))

	ctx.Request.Headers["X-Priority"] = "background"
	require.Equal(t, 1, defaultLoadSheddingPriorityExtractor(ctx))

	ctx.Request.Headers["X-Priority"] = "unknown"
	require.Equal(t, 5, defaultLoadSheddingPriorityExtractor(ctx))

	handler := defaultSheddingHandler(418, "teapot")
	ctx2 := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/shed"}))
	require.NoError(t, handler(ctx2))
	require.Equal(t, 418, ctx2.Response.StatusCode)

	priorityCfg := NewPriorityLoadShedding("p", map[int]float64{0: 0.1})
	require.Equal(t, LoadSheddingPriority, priorityCfg.Strategy)

	adaptiveCfg := NewAdaptiveLoadShedding("a", 250*time.Millisecond)
	require.Equal(t, LoadSheddingAdaptive, adaptiveCfg.Strategy)

	customCfg := NewCustomLoadShedding("c", func(_ *lift.Context, _ *LoadMetrics) bool { return false })
	require.Equal(t, LoadSheddingCustom, customCfg.Strategy)
	require.NotNil(t, customCfg.CustomShedder)
}

