package lift

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/security"
	"github.com/stretchr/testify/require"
)

func TestWithSecurityMiddleware_AddsMiddleware(t *testing.T) {
	app := New(WithSecurityMiddleware(SecurityConfig{}))
	require.Len(t, app.middleware, 1)
}

func TestSecurityMiddleware_IPWhitelistAndHeaders(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/",
		Headers: map[string]string{"X-Forwarded-For": "203.0.113.10"},
	})
	ctx := NewContext(context.Background(), req)

	processor := newSecurityProcessor(SecurityConfig{
		IPWhitelist:           []string{"203.0.113.0/24"},
		EnableSecurityHeaders: true,
	})

	nextCalled := false
	next := HandlerFunc(func(*Context) error {
		nextCalled = true
		return nil
	})

	require.NoError(t, processor.process(ctx, next))
	require.True(t, nextCalled)
	require.Equal(t, "nosniff", ctx.Response.Headers["X-Content-Type-Options"])
	require.Equal(t, "DENY", ctx.Response.Headers["X-Frame-Options"])
}

func TestSecurityMiddleware_IPWhitelist_Denies(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/",
		Headers: map[string]string{"X-Forwarded-For": "198.51.100.10"},
	})
	ctx := NewContext(context.Background(), req)

	processor := newSecurityProcessor(SecurityConfig{
		IPWhitelist: []string{"203.0.113.0/24"},
	})

	err := processor.process(ctx, HandlerFunc(func(*Context) error { return nil }))
	require.Error(t, err)
	require.Equal(t, ErrorCodeAuthorizationError, err.(*LiftError).Code)
}

func TestSecurityMiddleware_RoleValidation(t *testing.T) {
	t.Run("unauthenticated requests skip role checks", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "GET", Path: "/"})
		ctx := NewContext(context.Background(), req)
		processor := newSecurityProcessor(SecurityConfig{RequiredRoles: []string{"admin"}})

		called := false
		require.NoError(t, processor.process(ctx, HandlerFunc(func(*Context) error {
			called = true
			return nil
		})))
		require.True(t, called)
	})

	t.Run("authenticated requests without role are denied", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "GET", Path: "/"})
		ctx := NewContext(context.Background(), req)
		ctx.SetClaims(map[string]any{"sub": "user-1"})
		ctx.Set("principal", &security.Principal{
			UserID:    "user-1",
			Roles:     []string{"user"},
			ExpiresAt: time.Now().Add(time.Hour),
		})

		processor := newSecurityProcessor(SecurityConfig{RequiredRoles: []string{"admin"}})
		err := processor.process(ctx, HandlerFunc(func(*Context) error { return nil }))
		require.Error(t, err)
		require.Equal(t, ErrorCodeAuthorizationError, err.(*LiftError).Code)
	})

	t.Run("authenticated requests with role are allowed", func(t *testing.T) {
		req := NewRequest(&adapters.Request{Method: "GET", Path: "/"})
		ctx := NewContext(context.Background(), req)
		ctx.SetClaims(map[string]any{"sub": "user-1"})
		ctx.Set("principal", &security.Principal{
			UserID:    "user-1",
			Roles:     []string{"admin"},
			ExpiresAt: time.Now().Add(time.Hour),
		})

		processor := newSecurityProcessor(SecurityConfig{RequiredRoles: []string{"admin"}})
		called := false
		require.NoError(t, processor.process(ctx, HandlerFunc(func(*Context) error {
			called = true
			return nil
		})))
		require.True(t, called)
	})
}

func TestSecurityMiddleware_AuditLoggerAndCustomHandler(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/",
		Headers: map[string]string{"X-Forwarded-For": "203.0.113.10"},
	})
	ctx := NewContext(context.Background(), req)

	auditCalled := false
	customErr := errors.New("custom handler failed")
	processor := newSecurityProcessor(SecurityConfig{
		AuditLogger: func(_ *Context, event string, data map[string]any) {
			auditCalled = true
			require.Equal(t, "request", event)
			require.NotEmpty(t, data["request_id"])
		},
		Handler: func(*Context) error {
			return customErr
		},
	})

	nextCalled := false
	err := processor.process(ctx, HandlerFunc(func(*Context) error {
		nextCalled = true
		return nil
	}))
	require.ErrorIs(t, err, customErr)
	require.False(t, nextCalled)
	require.True(t, auditCalled)
}
