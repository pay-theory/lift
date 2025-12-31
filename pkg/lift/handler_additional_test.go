package lift

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestHandlerFunc_ImplementsHandler(t *testing.T) {
	called := false
	h := HandlerFunc(func(*Context) error {
		called = true
		return nil
	})

	require.NoError(t, h.Handle(&Context{}))
	require.True(t, called)
}

func TestSimpleHandler_TypedAdapter_Success(t *testing.T) {
	type requestModel struct {
		Name string `json:"name"`
	}
	type responseModel struct {
		Greeting string `json:"greeting"`
	}

	req := NewRequest(&adapters.Request{
		Method:      "POST",
		Path:        "/",
		Headers:     map[string]string{"Content-Type": "application/json"},
		QueryParams: map[string]string{},
		Body:        []byte(`{"name":"bob"}`),
		TriggerType: TriggerAPIGateway,
	})
	ctx := NewContext(context.Background(), req)

	handler := SimpleHandler(func(_ *Context, in requestModel) (responseModel, error) {
		return responseModel{Greeting: "hi " + in.Name}, nil
	})

	require.NoError(t, handler.Handle(ctx))
	require.Equal(t, ContentTypeJSON, ctx.Response.Headers[HeaderContentType])
	require.Equal(t, responseModel{Greeting: "hi bob"}, ctx.Response.Body)
}

func TestSimpleHandler_TypedAdapter_ParseError(t *testing.T) {
	type requestModel struct {
		Name string `json:"name"`
	}
	type responseModel struct {
		Greeting string `json:"greeting"`
	}

	req := NewRequest(&adapters.Request{
		Method:      "POST",
		Path:        "/",
		Headers:     map[string]string{"Content-Type": "application/json"},
		QueryParams: map[string]string{},
		Body:        []byte(`{bad-json}`),
		TriggerType: TriggerAPIGateway,
	})
	ctx := NewContext(context.Background(), req)

	handler := SimpleHandler(func(*Context, requestModel) (responseModel, error) {
		return responseModel{Greeting: "nope"}, nil
	})

	err := handler.Handle(ctx)
	require.Error(t, err)
	require.Equal(t, "INVALID_JSON", err.(*LiftError).Code)
}

func TestSimpleHandler_TypedAdapter_HandlerError(t *testing.T) {
	type requestModel struct {
		Name string `json:"name"`
	}
	type responseModel struct {
		Greeting string `json:"greeting"`
	}

	req := NewRequest(&adapters.Request{
		Method:      "POST",
		Path:        "/",
		Headers:     map[string]string{"Content-Type": "application/json"},
		QueryParams: map[string]string{},
		Body:        []byte(`{"name":"bob"}`),
		TriggerType: TriggerAPIGateway,
	})
	ctx := NewContext(context.Background(), req)

	sentinel := errors.New("handler failed")
	handler := SimpleHandler(func(*Context, requestModel) (responseModel, error) {
		return responseModel{}, sentinel
	})

	require.ErrorIs(t, handler.Handle(ctx), sentinel)
	require.False(t, ctx.Response.IsWritten())
}

func TestResponseBuffer_GetReturnsCopy(t *testing.T) {
	buf := NewResponseBuffer()
	buf.SetHeader("X-Test", "value")
	buf.SetStatusCode(201)
	buf.SetBody("body", "captured")

	body, status, headers, captured := buf.Get()
	require.Equal(t, "body", body)
	require.Equal(t, "captured", captured)
	require.Equal(t, 201, status)
	require.Equal(t, "value", headers["X-Test"])

	headers["X-Test"] = "mutated"
	_, _, headers2, _ := buf.Get()
	require.Equal(t, "value", headers2["X-Test"])
}

func TestMiddlewareTags_GlobalMiddlewareRegistry(t *testing.T) {
	global := Middleware(func(next Handler) Handler { return next })
	require.False(t, middlewareAppliesToEvents(global))

	MarkGlobalMiddleware(global)
	require.True(t, middlewareAppliesToEvents(global))

	require.Nil(t, MarkGlobalMiddleware(nil))
	require.False(t, middlewareAppliesToEvents(nil))
}

func TestInterceptingMiddleware_Behavior(t *testing.T) {
	inner := Middleware(func(next Handler) Handler { return next })
	im := NewInterceptingMiddleware(inner)

	require.True(t, im.NeedsResponseInterception())
	applied := im.Apply()
	require.NotNil(t, applied)
	require.Equal(t, reflect.ValueOf(inner).Pointer(), reflect.ValueOf(applied).Pointer())
}
