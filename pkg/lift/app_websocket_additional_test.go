package lift

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestWebSocketEventProcessor_HandleNonWebSocketRequest_ConvertsResponse(t *testing.T) {
	app := New()
	require.NoError(t, app.GET("/ok", func(ctx *Context) error {
		ctx.Status(202)
		return ctx.JSON(map[string]string{"ok": "true"})
	}))

	processor := newWebSocketEventProcessor(app)
	req := NewRequest(&adapters.Request{
		TriggerType: TriggerAPIGateway,
		Method:      "GET",
		Path:        "/ok",
	})
	liftCtx := processor.createLiftContext(context.Background(), req)

	resp, err := processor.handleNonWebSocketRequest(liftCtx)
	require.NoError(t, err)
	require.Equal(t, 202, resp.StatusCode)
	require.Equal(t, `{"ok":"true"}`, resp.Body)
	require.Equal(t, ContentTypeJSON, resp.Headers[HeaderContentType])
}

func TestWebSocketEventProcessor_convertResponseBody_Variants(t *testing.T) {
	app := New()
	processor := newWebSocketEventProcessor(app)

	require.Equal(t, "s", processor.convertResponseBody("s"))
	require.Equal(t, "b", processor.convertResponseBody([]byte("b")))
	require.Equal(t, `{"a":"b"}`, processor.convertResponseBody(map[string]string{"a": "b"}))
	require.Equal(t, "", processor.convertResponseBody(map[string]any{"bad": make(chan int)}))
}

func TestWebSocketEventProcessor_handleRoutingError_WhenHandleErrorFails(t *testing.T) {
	app := New()
	processor := newWebSocketEventProcessor(app)

	req := NewRequest(&adapters.Request{
		TriggerType: TriggerAPIGateway,
		Method:      "GET",
		Path:        "/missing",
	})
	liftCtx := processor.createLiftContext(context.Background(), req)

	require.NoError(t, liftCtx.JSON(map[string]string{"written": "true"}))

	resp, err := processor.handleRoutingError(liftCtx, NewLiftError("NOT_FOUND", "missing", 404))
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)
}

func TestWebSocketEventProcessor_convertResponse_DefaultsToSuccess(t *testing.T) {
	app := New()
	processor := newWebSocketEventProcessor(app)

	liftCtx := &Context{Response: nil}
	resp := processor.convertResponse(liftCtx)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "OK", resp.Body)
}
