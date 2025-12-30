package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

type stubConnectionStore struct {
	count int64
	err   error
}

func (s stubConnectionStore) Save(_ context.Context, _ *lift.Connection) error           { return nil }
func (s stubConnectionStore) Get(_ context.Context, _ string) (*lift.Connection, error)  { return nil, nil }
func (s stubConnectionStore) Delete(_ context.Context, _ string) error                   { return nil }
func (s stubConnectionStore) ListByUser(_ context.Context, _ string) ([]*lift.Connection, error) {
	return nil, nil
}
func (s stubConnectionStore) ListByTenant(_ context.Context, _ string) ([]*lift.Connection, error) {
	return nil, nil
}
func (s stubConnectionStore) CountActive(_ context.Context) (int64, error) {
	return s.count, s.err
}

func TestWebSocketMetrics_PassThroughWhenNotWebSocket(t *testing.T) {
	metrics := newMockMetricsCollector()
	mw := WebSocketMetrics(metrics)

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
	require.Nil(t, ctx.Get("websocket.route_key"))
}

func TestWebSocketMetrics_ConnectDisconnectMessageAndErrors(t *testing.T) {
	metrics := newMockMetricsCollector()
	mw := WebSocketMetrics(metrics)

	connectCtx := newWebSocketTestContext("$connect", "CONNECT", "c-1", nil)
	require.NoError(t, mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	})).Handle(connectCtx))
	require.Equal(t, "$connect", connectCtx.Get("websocket.route_key"))
	require.Contains(t, metrics.metrics, "websocket.requests")
	require.Contains(t, metrics.metrics, "websocket.connections.new")
	require.Contains(t, metrics.metrics, "websocket.connections.active")

	disconnectCtx := newWebSocketTestContext("$disconnect", "DISCONNECT", "c-1", nil)
	require.NoError(t, mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	})).Handle(disconnectCtx))
	require.Contains(t, metrics.metrics, "websocket.connections.closed")

	messageCtx := newWebSocketTestContext("message", "MESSAGE", "c-1", []byte("hello"))
	require.NoError(t, mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		return nil
	})).Handle(messageCtx))
	require.Contains(t, metrics.metrics, "websocket.messages")
	require.Contains(t, metrics.metrics, "websocket.message.size")

	errorCtx := newWebSocketTestContext("message", "MESSAGE", "c-1", nil)
	require.Error(t, mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return lift.NewLiftError("UNAUTHORIZED", "unauthorized", 401)
	})).Handle(errorCtx))
	require.Contains(t, metrics.metrics, "websocket.errors")
}

func TestWebSocketConnectionMetrics_ConnectSpawnsCounterUpdate(t *testing.T) {
	metrics := newMockMetricsCollector()
	mw := WebSocketConnectionMetrics(metrics, nil)

	ctx := newWebSocketTestContext("$connect", "CONNECT", "c-1", nil)
	require.NoError(t, mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx))
}

func TestUpdateConnectionCount_ErrorAndSuccess(t *testing.T) {
	metrics := newMockMetricsCollector()

	updateConnectionCount(metrics, stubConnectionStore{err: errors.New("boom")})
	require.Contains(t, metrics.metrics, "websocket.connection_count_errors")

	updateConnectionCount(metrics, stubConnectionStore{count: 42})
	require.Contains(t, metrics.metrics, "websocket.connections.total")
}

func TestGetErrorType_CategorizesErrors(t *testing.T) {
	require.Equal(t, "", getErrorType(nil))
	require.Equal(t, "CODE", getErrorType(lift.NewLiftError("CODE", "msg", 400)))

	require.Equal(t, "unauthorized", getErrorType(errors.New("401 unauthorized")))
	require.Equal(t, "forbidden", getErrorType(errors.New("403 forbidden")))
	require.Equal(t, "not_found", getErrorType(errors.New("404 not found")))
	require.Equal(t, "timeout", getErrorType(errors.New("request timeout")))
	require.Equal(t, "connection_error", getErrorType(errors.New("connection reset")))
	require.Equal(t, "internal_error", getErrorType(errors.New("boom")))
}

func TestTrackConnectionCount_NoStore_NoOp(t *testing.T) {
	trackConnectionCount(newMockMetricsCollector(), nil)
}

func newWebSocketTestContext(routeKey, eventType, connectionID string, body []byte) *lift.Context {
	req := lift.NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/ws",
		TriggerType: adapters.TriggerWebSocket,
		Metadata: map[string]any{
			"routeKey":     routeKey,
			"eventType":    eventType,
			"connectionId": connectionID,
		},
		Body: body,
	})
	return lift.NewContext(context.Background(), req)
}

