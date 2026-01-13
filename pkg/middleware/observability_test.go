package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestObservabilityMiddleware_PassThroughWhenLoggerAndMetricsNil(t *testing.T) {
	mw := ObservabilityMiddleware(ObservabilityConfig{})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestObservabilityMiddleware_LogsAndRecordsMetrics(t *testing.T) {
	logger := &mockLogger{}
	metrics := newMockMetricsCollector()

	mw := ObservabilityMiddleware(ObservabilityConfig{
		Logger:  logger,
		Metrics: metrics,
	})

	req := lift.NewRequest(&adapters.Request{
		Method: "GET",
		Path:   "/test",
		Headers: map[string]string{
			"X-Trace-Id": "trace-1",
			"X-Span-Id":  "span-1",
		},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.RequestID = "req-1"
	ctx.SetTenantID("tenant-1")
	ctx.SetUserID("user-1")

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.Body = map[string]any{"ok": true}
		ctx.Response.StatusCode = 200
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.NotNil(t, ctx.Logger)
	require.Greater(t, len(logger.logs), 0)

	require.Contains(t, metrics.metrics, "requests.total")
	require.Contains(t, metrics.metrics, "requests.duration")
	require.Contains(t, metrics.metrics, "response.size")
	require.Contains(t, metrics.metrics, "operation.GET_/test")
	require.Contains(t, metrics.metrics, "operation.GET_/test.duration")
}

func TestObservabilityMiddleware_RecordsErrorMetricsAndLogsError(t *testing.T) {
	logger := &mockLogger{}
	metrics := newMockMetricsCollector()

	mw := ObservabilityMiddleware(ObservabilityConfig{
		Logger:  logger,
		Metrics: metrics,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/err"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.RequestID = "req-1"

	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return errors.New("boom")
	}))

	require.Error(t, handler.Handle(ctx))
	require.Contains(t, metrics.metrics, "requests.errors")
}

func TestMetricsOnlyMiddleware_RecordsRequestDurationAndErrors(t *testing.T) {
	metrics := newMockMetricsCollector()
	mw := MetricsOnlyMiddleware(metrics)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.Contains(t, metrics.metrics, "http.requests")
	require.Contains(t, metrics.metrics, "http.duration")

	errCtx := lift.NewContext(context.Background(), req)
	errHandler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return errors.New("boom")
	}))

	require.Error(t, errHandler.Handle(errCtx))
	require.Contains(t, metrics.metrics, "http.errors")
}
