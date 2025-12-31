package lift

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketActionRouter_DispatchesActions(t *testing.T) {
	app := New(WithWebSocketSupport())

	actionCalled := false
	router := app.WebSocketActions()
	router.On("sendMessage", func(ctx *Context) error {
		actionCalled = true
		assert.Equal(t, "sendMessage", ctx.Get(contextKeyWebSocketAction))
		return nil
	})

	handler := app.WebSocketHandler()
	wsHandler, ok := handler.(func(context.Context, events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error))
	require.True(t, ok, "Handler should be a WebSocket handler function")

	event := createTestWebSocketEvent("$default", "conn123", `{"action":"sendMessage","message":"hello"}`)
	resp, err := wsHandler(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, actionCalled)
}

func TestWebSocketActionRouter_DefaultHandlerOnUnknownAction(t *testing.T) {
	app := New(WithWebSocketSupport())

	defaultCalled := false
	router := app.WebSocketActions()
	router.Default(func(ctx *Context) error {
		defaultCalled = true
		assert.Equal(t, "unknown", ctx.Get(contextKeyWebSocketAction))
		assert.Nil(t, ctx.Get(contextKeyWebSocketActionRouterError))
		return nil
	})

	handler := app.WebSocketHandler()
	wsHandler, ok := handler.(func(context.Context, events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error))
	require.True(t, ok, "Handler should be a WebSocket handler function")

	event := createTestWebSocketEvent("$default", "conn123", `{"action":"unknown","message":"hello"}`)
	resp, err := wsHandler(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, defaultCalled)
}

func TestWebSocketActionRouter_DefaultHandlerOnInvalidJSON(t *testing.T) {
	app := New(WithWebSocketSupport())

	defaultCalled := false
	router := app.WebSocketActions()
	router.Default(func(ctx *Context) error {
		defaultCalled = true
		assert.Equal(t, "", ctx.Get(contextKeyWebSocketAction))
		assert.NotNil(t, ctx.Get(contextKeyWebSocketActionRouterError))
		return nil
	})

	handler := app.WebSocketHandler()
	wsHandler, ok := handler.(func(context.Context, events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error))
	require.True(t, ok, "Handler should be a WebSocket handler function")

	event := createTestWebSocketEvent("$default", "conn123", "{")
	resp, err := wsHandler(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, defaultCalled)
}

func TestWebSocketActionRouter_ReturnsErrorsWithoutDefaultHandler(t *testing.T) {
	app := New(WithWebSocketSupport())
	app.WebSocket("$default", NewWebSocketActionRouter().Handle)

	handler := app.WebSocketHandler()
	wsHandler, ok := handler.(func(context.Context, events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error))
	require.True(t, ok, "Handler should be a WebSocket handler function")

	t.Run("Missing action", func(t *testing.T) {
		event := createTestWebSocketEvent("$default", "conn123", `{"message":"hello"}`)
		resp, err := wsHandler(context.Background(), event)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		event := createTestWebSocketEvent("$default", "conn123", "{")
		resp, err := wsHandler(context.Background(), event)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("Unknown action", func(t *testing.T) {
		event := createTestWebSocketEvent("$default", "conn123", `{"action":"unknown"}`)
		resp, err := wsHandler(context.Background(), event)
		require.NoError(t, err)
		assert.Equal(t, 404, resp.StatusCode)
	})
}
