package lift

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestWebSocketActionRouter_Options(t *testing.T) {
	opt := WithWebSocketActionField("kind")
	opt(nil)

	router := NewWebSocketActionRouter(WithWebSocketActionField("kind"))
	require.Equal(t, "kind", router.actionField)

	NewWebSocketActionRouter(WithWebSocketActionField(""))
	require.Equal(t, defaultWebSocketActionField, NewWebSocketActionRouter(WithWebSocketActionField("")).actionField)

	extractorCalled := false
	router = NewWebSocketActionRouter(WithWebSocketActionExtractor(func(*Context) (string, error) {
		extractorCalled = true
		return "ping", nil
	}))

	handled := false
	router.On("ping", func(*Context) error {
		handled = true
		return nil
	})

	ctx := NewContext(context.Background(), NewRequest(&adapters.Request{Body: []byte{}}))
	require.NoError(t, router.Handle(ctx))
	require.True(t, extractorCalled)
	require.True(t, handled)
}

func TestWebSocketActionRouter_Handle_NilRouter(t *testing.T) {
	var router *WebSocketActionRouter
	err := router.Handle(&Context{})
	require.Error(t, err)
	require.Equal(t, ErrorCodeSystemError, err.(*LiftError).Code)
}

