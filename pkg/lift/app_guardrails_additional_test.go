package lift

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestApp_HandleTestRequest_Branches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		app := New()
		require.NoError(t, app.GET("/ok", func(*Context) error { return nil }))

		req := NewRequest(&adapters.Request{Method: "GET", Path: "/ok", TriggerType: TriggerAPIGateway})
		ctx := NewContext(context.Background(), req)
		require.NoError(t, app.HandleTestRequest(ctx))
	})

	t.Run("lift error is written to response", func(t *testing.T) {
		app := New()
		require.NoError(t, app.GET("/bad", func(*Context) error {
			return NewLiftError("BAD", "no", 400)
		}))

		req := NewRequest(&adapters.Request{Method: "GET", Path: "/bad", TriggerType: TriggerAPIGateway})
		ctx := NewContext(context.Background(), req)

		require.NoError(t, app.HandleTestRequest(ctx))
		require.Equal(t, 400, ctx.Response.StatusCode)
	})

	t.Run("non-lift error is written to response", func(t *testing.T) {
		app := New()
		require.NoError(t, app.GET("/boom", func(*Context) error { return errors.New("boom") }))

		req := NewRequest(&adapters.Request{Method: "GET", Path: "/boom", TriggerType: TriggerAPIGateway})
		ctx := NewContext(context.Background(), req)

		require.NoError(t, app.HandleTestRequest(ctx))
		require.Equal(t, 500, ctx.Response.StatusCode)
	})

	t.Run("write failure returns error", func(t *testing.T) {
		app := New()
		require.NoError(t, app.GET("/bad", func(*Context) error {
			return NewLiftError("BAD", "no", 400)
		}))

		req := NewRequest(&adapters.Request{Method: "GET", Path: "/bad", TriggerType: TriggerAPIGateway})
		ctx := NewContext(context.Background(), req)
		require.NoError(t, ctx.JSON(map[string]string{"written": "true"}))

		require.Error(t, app.HandleTestRequest(ctx))
	})
}

func TestApp_TryPreferredAdapters_And_DebugLogging(t *testing.T) {
	logger := &recordingLogger{}

	app := New(WithDebug())
	app.logger = logger
	app.WithPreferredAdapters(adapters.TriggerAPIGatewayV2)

	require.NoError(t, app.GET("/ok", func(ctx *Context) error {
		return ctx.JSON(map[string]string{"ok": "true"})
	}))

	event := newAPIGatewayV2Event("GET", "/ok")
	_, err := app.HandleRequest(context.Background(), event)
	require.NoError(t, err)
	require.NotEmpty(t, logger.debugMessages)
}

func TestApp_RequestAndResponseGuardrails(t *testing.T) {
	t.Run("request body too large triggers PAYLOAD_TOO_LARGE", func(t *testing.T) {
		app := New(WithConfig(&Config{MaxRequestSize: 10, MaxResponseSize: 6 * 1024 * 1024, Timeout: 30, LogLevel: "INFO"}))
		require.NoError(t, app.POST("/ok", func(ctx *Context) error {
			return ctx.JSON(map[string]string{"ok": "true"})
		}))

		body := strings.Repeat("a", 64)
		event := newAPIGatewayV1Event("POST", "/ok")
		event["body"] = `{"data":"` + body + `"}`

		respAny, err := app.HandleRequest(context.Background(), event)
		require.NoError(t, err)
		resp := respAny.(*Response)
		require.Equal(t, 413, resp.StatusCode)
	})

	t.Run("response body too large triggers PAYLOAD_TOO_LARGE", func(t *testing.T) {
		app := New(WithConfig(&Config{MaxRequestSize: 10 * 1024 * 1024, MaxResponseSize: 10, Timeout: 30, LogLevel: "INFO"}))
		require.NoError(t, app.GET("/big", func(ctx *Context) error {
			return ctx.JSON(map[string]string{"data": strings.Repeat("b", 64)})
		}))

		respAny, err := app.HandleRequest(context.Background(), newAPIGatewayV1Event("GET", "/big"))
		require.NoError(t, err)
		resp := respAny.(*Response)
		require.Equal(t, 413, resp.StatusCode)
	})
}

