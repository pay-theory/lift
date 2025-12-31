package lift

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestCreateSecurityMiddleware_WrapsProcessor(t *testing.T) {
	req := NewRequest(&adapters.Request{Method: "GET", Path: "/"})
	ctx := NewContext(context.Background(), req)

	customCalled := false
	nextCalled := false

	mw := createSecurityMiddleware(SecurityConfig{
		Handler: func(*Context) error {
			customCalled = true
			return nil
		},
	})

	next := HandlerFunc(func(*Context) error {
		nextCalled = true
		return nil
	})

	require.NoError(t, mw(next).Handle(ctx))
	require.True(t, customCalled)
	require.True(t, nextCalled)
}
