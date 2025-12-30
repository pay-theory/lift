package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/security"
	"github.com/stretchr/testify/require"
)

func TestWebSocketAuth_PassThroughWhenNotWebSocket(t *testing.T) {
	mw := WebSocketAuth(WebSocketAuthConfig{
		JWTConfig: security.JWTConfig{SigningMethod: "HS256", SecretKey: "secret"},
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestWebSocketAuth_SkipRoutes(t *testing.T) {
	mw := WebSocketAuth(WebSocketAuthConfig{
		SkipRoutes: []string{"$connect"},
		JWTConfig:  security.JWTConfig{SigningMethod: "HS256", SecretKey: "secret"},
	})

	ctx := newWebSocketTestContext("$connect", "CONNECT", "c-1", nil)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestWebSocketAuth_ConnectEvent_MissingToken_DefaultHandler(t *testing.T) {
	mw := WebSocketAuth(WebSocketAuthConfig{
		JWTConfig: security.JWTConfig{SigningMethod: "HS256", SecretKey: "secret"},
	})

	ctx := newWebSocketTestContext("$connect", "CONNECT", "c-1", nil)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.False(t, called)
	require.Equal(t, 401, ctx.Response.StatusCode)
}

func TestWebSocketAuth_ConnectEvent_ValidToken_SetsContext(t *testing.T) {
	secret := "secret"
	token := newHS256JWT(t, secret, &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		TenantID:  "tenant-1",
		AccountID: "account-1",
		Roles:     []string{"role-1"},
		Scopes:    []string{"scope-1"},
	})

	mw := WebSocketAuth(WebSocketAuthConfig{
		JWTConfig: security.JWTConfig{SigningMethod: "HS256", SecretKey: secret, RequireTenantID: true},
	})

	ctx := newWebSocketTestContext("$connect", "CONNECT", "c-1", nil)
	ctx.Request.QueryParams = map[string]string{"token": "Bearer " + token}

	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		require.Equal(t, "user-1", ctx.UserID())
		require.Equal(t, "tenant-1", ctx.TenantID())
		require.NotNil(t, ctx.Get("claims"))
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
}

func TestWebSocketAuth_NonConnectEvent_RequiresUserID(t *testing.T) {
	mw := WebSocketAuth(WebSocketAuthConfig{
		JWTConfig: security.JWTConfig{SigningMethod: "HS256", SecretKey: "secret"},
	})

	ctx := newWebSocketTestContext("message", "MESSAGE", "c-1", nil)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.False(t, called)
	require.Equal(t, 401, ctx.Response.StatusCode)
}

func TestWebSocketAuth_CustomOnError(t *testing.T) {
	customErr := errors.New("custom")
	mw := WebSocketAuth(WebSocketAuthConfig{
		OnError:   func(_ *lift.Context, _ error) error { return customErr },
		JWTConfig: security.JWTConfig{SigningMethod: "HS256", SecretKey: "secret"},
	})

	ctx := newWebSocketTestContext("$connect", "CONNECT", "c-1", nil)
	err := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.ErrorIs(t, err, customErr)
}

func TestWebSocketAuthFromQueryAndHeader(t *testing.T) {
	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/ws",
		TriggerType: adapters.TriggerWebSocket,
		QueryParams: map[string]string{"token": "q"},
		Headers:     map[string]string{"Authorization": "h"},
		Metadata:    map[string]any{"routeKey": "$connect", "eventType": "CONNECT", "connectionId": "c-1"},
	}))

	require.Equal(t, "q", WebSocketAuthFromQuery("token")(ctx))
	require.Equal(t, "h", WebSocketAuthFromHeader("Authorization")(ctx))
}

