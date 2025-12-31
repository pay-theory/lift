package lift

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func newAPIGatewayV1EventWithBody(method, path string, body any) map[string]any {
	event := newAPIGatewayV1Event(method, path)
	event["headers"] = map[string]any{"Content-Type": "application/json"}

	if body == nil {
		return event
	}

	switch v := body.(type) {
	case string:
		event["body"] = v
	default:
		encoded, _ := json.Marshal(v)
		event["body"] = string(encoded)
	}

	return event
}

func TestConvertHandlerUsingReflection_Validation(t *testing.T) {
	_, err := convertHandlerUsingReflection(123)
	require.Error(t, err)

	_, err = convertHandlerUsingReflection(func(int, int) error { return nil })
	require.Error(t, err)
}

func TestConvertHandlerUsingReflection_ContextErrorPattern(t *testing.T) {
	h, err := convertHandlerUsingReflection(func(*Context) error { return nil })
	require.NoError(t, err)

	req := NewRequest(&adapters.Request{Method: "GET", Path: "/", TriggerType: TriggerAPIGateway})
	ctx := NewContext(context.Background(), req)

	require.NoError(t, h.Handle(ctx))
}

func TestApp_HandleRequest_ReflectedHandlers(t *testing.T) {
	app := New()

	require.NoError(t, app.GET("/ctx-resp", func(*Context) (any, error) {
		return map[string]string{"hello": "world"}, nil
	}))
	require.NoError(t, app.GET("/simple-error", func() error { return nil }))
	require.NoError(t, app.GET("/simple-resp", func() (any, error) {
		return map[string]string{"ok": "true"}, nil
	}))

	type requestModel struct {
		Name string `json:"name"`
	}
	type responseModel struct {
		Hello string `json:"hello"`
	}
	require.NoError(t, app.POST("/model-resp", func(in requestModel) (responseModel, error) {
		return responseModel{Hello: "hi " + in.Name}, nil
	}))
	require.NoError(t, app.POST("/model-err", func(requestModel) error { return nil }))

	respAny, err := app.HandleRequest(context.Background(), newAPIGatewayV1EventWithBody("GET", "/ctx-resp", nil))
	require.NoError(t, err)
	resp := respAny.(*Response)
	require.Equal(t, map[string]string{"hello": "world"}, resp.Body)

	respAny, err = app.HandleRequest(context.Background(), newAPIGatewayV1EventWithBody("GET", "/simple-error", nil))
	require.NoError(t, err)
	resp = respAny.(*Response)
	require.Nil(t, resp.Body)

	respAny, err = app.HandleRequest(context.Background(), newAPIGatewayV1EventWithBody("GET", "/simple-resp", nil))
	require.NoError(t, err)
	resp = respAny.(*Response)
	require.Equal(t, map[string]string{"ok": "true"}, resp.Body)

	respAny, err = app.HandleRequest(context.Background(), newAPIGatewayV1EventWithBody("POST", "/model-resp", requestModel{Name: "bob"}))
	require.NoError(t, err)
	resp = respAny.(*Response)
	require.Equal(t, responseModel{Hello: "hi bob"}, resp.Body)

	respAny, err = app.HandleRequest(context.Background(), newAPIGatewayV1EventWithBody("POST", "/model-err", requestModel{Name: "bob"}))
	require.NoError(t, err)
	resp = respAny.(*Response)
	require.Nil(t, resp.Body)
}

func TestRouteGroup_Prefix_GroupAndMethods(t *testing.T) {
	app := New()

	group := app.Group("/v1")
	sub := group.Group("/sub")

	require.NoError(t, group.GET("/get", func(*Context) error { return nil }))
	require.NoError(t, group.POST("/post", func(*Context) error { return nil }))
	require.NoError(t, group.PUT("/put", func(*Context) error { return nil }))
	require.NoError(t, group.DELETE("/delete", func(*Context) error { return nil }))
	require.NoError(t, group.PATCH("/patch", func(*Context) error { return nil }))

	mwCalled := false
	sub.Use(func(next Handler) Handler {
		return HandlerFunc(func(ctx *Context) error {
			mwCalled = true
			return next.Handle(ctx)
		})
	})
	require.NoError(t, sub.GET("/ping", func(ctx *Context) error {
		return ctx.Text("ok")
	}))

	respAny, err := app.HandleRequest(context.Background(), newAPIGatewayV1EventWithBody("GET", "/v1/sub/ping", nil))
	require.NoError(t, err)
	resp := respAny.(*Response)
	require.Equal(t, 200, resp.StatusCode)
	require.True(t, mwCalled)
}

func TestApp_SettersAndEventHelpers(t *testing.T) {
	app := New()

	logger := &NoOpLogger{}
	metrics := &NoOpMetrics{}
	db := struct{ Name string }{Name: "db"}

	app.WithLogger(logger).
		WithMetrics(metrics).
		WithDatabase(db).
		WithTracer("tracer").
		WithPreferredAdapters(adapters.TriggerAPIGatewayV2)

	require.Equal(t, logger, app.logger)
	require.Equal(t, metrics, app.metrics)
	require.Equal(t, db, app.db)
	require.Equal(t, "tracer", app.tracer)
	require.Equal(t, []adapters.TriggerType{adapters.TriggerAPIGatewayV2}, app.preferredAdapters)

	require.Equal(t, app.eventRouter, app.GetEventRouter())

	require.NoError(t, app.SQS("*", func(*Context) error { return nil }))
	require.NoError(t, app.S3("*", func(*Context) error { return nil }))
	require.NoError(t, app.EventBridge("*", func(*Context) error { return nil }))

	require.Error(t, app.SQS("*", 123))
}

func TestParseTriggerType_Mappings(t *testing.T) {
	require.Equal(t, TriggerSQS, parseTriggerType("SQS"))
	require.Equal(t, TriggerS3, parseTriggerType("S3"))
	require.Equal(t, TriggerEventBridge, parseTriggerType("EventBridge"))
	require.Equal(t, TriggerWebSocket, parseTriggerType("CONNECT"))
	require.Equal(t, TriggerAPIGateway, parseTriggerType("GET"))
	require.Equal(t, TriggerUnknown, parseTriggerType("Nope"))
}
