package middleware

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestChain_AppliesMiddlewareInOrder(t *testing.T) {
	var order []string

	a := func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			order = append(order, "a")
			return next.Handle(ctx)
		})
	}
	b := func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			order = append(order, "b")
			return next.Handle(ctx)
		})
	}

	handler := Chain(a, b)(lift.HandlerFunc(func(_ *lift.Context) error {
		order = append(order, "handler")
		return nil
	}))

	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{Method: "GET", Path: "/"}))
	require.NoError(t, handler.Handle(ctx))
	require.Equal(t, []string{"a", "b", "handler"}, order)
}

func TestLoggerMiddleware_InitializesDefaultLoggerWhenMissing(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	require.Nil(t, ctx.Logger)

	mw := Logger()
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx))
	require.NotNil(t, ctx.Logger)
}

func TestRecoverMiddleware_RecoversPanicsAndWritesResponse(t *testing.T) {
	logger := &mockLogger{}

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/panic"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Logger = logger

	mw := Recover()
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		panic("boom")
	})).Handle(ctx))

	require.Equal(t, 500, ctx.Response.StatusCode)
	_, ok := ctx.Response.Body.(map[string]any)
	require.True(t, ok)

	// Already-written response triggers the error logging branch.
	ctx2 := lift.NewContext(context.Background(), req)
	ctx2.Logger = logger
	require.NoError(t, ctx2.JSON(map[string]any{"ok": true}))

	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		panic("boom")
	})).Handle(ctx2))
	require.Greater(t, len(logger.logs), 0)
}

func TestCORSMiddleware_AllowsOriginsAndHandlesPreflight(t *testing.T) {
	mw := CORS([]string{"https://example.com"})

	req := lift.NewRequest(&adapters.Request{
		Method:  "OPTIONS",
		Path:    "/test",
		Headers: map[string]string{"Origin": "https://example.com"},
	})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	})).Handle(ctx))
	require.False(t, called)
	require.Equal(t, 204, ctx.Response.StatusCode)
	require.Equal(t, "https://example.com", ctx.Response.Headers["Access-Control-Allow-Origin"])

	mwAny := CORS([]string{"*"})
	req2 := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Origin": "https://other.com"},
	})
	ctx2 := lift.NewContext(context.Background(), req2)
	require.NoError(t, mwAny(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx2))
	require.Equal(t, "https://other.com", ctx2.Response.Headers["Access-Control-Allow-Origin"])
}

func TestTimeoutMiddleware_UsesContextWithTimeout(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/slow"})
	ctx := lift.NewContext(context.Background(), req)

	mw := Timeout(5 * time.Millisecond)
	err := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})).Handle(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deadline")

	req2 := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/ok"})
	ctx2 := lift.NewContext(context.Background(), req2)
	mw2 := Timeout(50 * time.Millisecond)
	require.NoError(t, mw2(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx2))
}

func TestMetricsMiddleware_RecordsCountersAndErrors(t *testing.T) {
	metrics := newMockMetricsCollector()
	mw := Metrics()

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/ok"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Metrics = metrics
	ctx.Response.StatusCode = 200

	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx))
	require.Contains(t, metrics.metrics, "requests_total")
	require.Contains(t, metrics.metrics, "request_duration_ms")

	errCtx := lift.NewContext(context.Background(), req)
	errCtx.Metrics = metrics
	errCtx.Response.StatusCode = 500
	require.Error(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return errors.New("boom") })).Handle(errCtx))
	require.Contains(t, metrics.metrics, "errors_total")
}

func TestRequestIDMiddleware_UsesHeaderOrGenerates(t *testing.T) {
	mw := RequestID()

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test", Headers: map[string]string{"X-Request-ID": "rid"}})
	ctx := lift.NewContext(context.Background(), req)
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx))
	require.Equal(t, "rid", ctx.RequestID)
	require.Equal(t, "rid", ctx.Response.Headers["X-Request-ID"])

	req2 := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test", Headers: map[string]string{}})
	ctx2 := lift.NewContext(context.Background(), req2)
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx2))
	require.True(t, strings.HasPrefix(ctx2.RequestID, "req_"))
}

func TestErrorHandlerMiddleware_WritesLiftAndGenericErrors(t *testing.T) {
	logger := &mockLogger{}
	mw := ErrorHandler()

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/lift"})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Logger = logger

	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return lift.NewLiftError("CODE", "msg", 418)
	})).Handle(ctx))
	require.Equal(t, 418, ctx.Response.StatusCode)

	req2 := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/generic"})
	ctx2 := lift.NewContext(context.Background(), req2)
	ctx2.Logger = logger

	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return errors.New("boom")
	})).Handle(ctx2))
	require.Equal(t, 500, ctx2.Response.StatusCode)

	// Already-written response triggers the logging branch.
	ctx3 := lift.NewContext(context.Background(), req)
	ctx3.Logger = logger
	require.NoError(t, ctx3.JSON(map[string]any{"ok": true}))
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return lift.NewLiftError("CODE", "msg", 418)
	})).Handle(ctx3))

	ctx4 := lift.NewContext(context.Background(), req2)
	ctx4.Logger = logger
	require.NoError(t, ctx4.JSON(map[string]any{"ok": true}))
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return errors.New("boom")
	})).Handle(ctx4))
	require.Greater(t, len(logger.logs), 0)
}
