package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestSecureRandomFloat_ReturnsValueInRange(t *testing.T) {
	v := secureRandomFloat()
	require.GreaterOrEqual(t, v, 0.0)
	require.Less(t, v, 1.0)
}

func TestEnhancedObservability_TracingAndErrorPaths(t *testing.T) {
	metrics := newMockMetricsCollector()

	cfg := EnhancedObservabilityConfig{
		EnableTracing: true,
		EnableMetrics: true,
		EnableLogging: false,
		SampleRate:    1,
		Metrics:       metrics,
	}

	before := GetObservabilityStats(cfg).Tracing

	mw := EnhancedObservabilityMiddleware(cfg)

	okReq := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/ok"})
	okCtx := lift.NewContext(context.Background(), okReq)
	require.NoError(t, mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	})).Handle(okCtx))

	afterOK := GetObservabilityStats(cfg).Tracing
	require.Greater(t, afterOK.TracesGenerated, before.TracesGenerated)

	errReq := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/err"})
	errCtx := lift.NewContext(context.Background(), errReq)
	require.Error(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return errors.New("boom")
	})).Handle(errCtx))

	afterErr := GetObservabilityStats(cfg).Tracing
	require.Greater(t, afterErr.ErrorCount, before.ErrorCount)
	require.Contains(t, metrics.metrics, "requests.errors")
}

