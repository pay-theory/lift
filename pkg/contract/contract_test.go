//go:build contract
// +build contract

package contract_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/dynamorm"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
	"github.com/stretchr/testify/require"
)

func TestContract_HTTP_JWTAuth_Behavior(t *testing.T) {
	// #nosec G101 -- test secret
	secret := "contract-secret"

	app := lift.New()
	app.Use(middleware.JWTAuth(middleware.JWTConfig{
		Secret:      secret,
		Algorithm:   "HS256",
		TokenLookup: "header:Authorization",
	}))

	handlerCalled := false
	require.NoError(t, app.GET("/protected", func(ctx *lift.Context) error {
		handlerCalled = true
		return ctx.OK(map[string]any{
			"user_id":         ctx.UserID(),
			"tenant_id":       ctx.TenantID(),
			"authenticated":   ctx.IsAuthenticated(),
			"auth_claims_set": ctx.Claims() != nil,
		})
	}))

	t.Run("missing_token_is_rejected", func(t *testing.T) {
		handlerCalled = false

		event := apiGatewayV1Event("GET", "/protected", nil, "")
		respAny, err := app.HandleRequest(context.Background(), event)
		require.NoError(t, err)

		resp, ok := respAny.(*lift.Response)
		require.True(t, ok)
		require.Equal(t, 401, resp.StatusCode)
		require.Equal(t, lift.ContentTypeJSON, resp.Headers[lift.HeaderContentType])

		body, ok := resp.Body.(map[string]any)
		require.True(t, ok)
		require.Equal(t, lift.ErrorCodeUnauthorized, body["code"])
		require.Equal(t, "Invalid or missing token", body["message"])
		require.False(t, handlerCalled)
	})

	t.Run("valid_token_allows_access_and_sets_context", func(t *testing.T) {
		handlerCalled = false

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":   "user-123",
			"tenant_id": "tenant-abc",
			"exp":       time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, err := token.SignedString([]byte(secret))
		require.NoError(t, err)

		event := apiGatewayV1Event("GET", "/protected", map[string]string{
			"Authorization": "Bearer " + tokenString,
		}, "")
		respAny, err := app.HandleRequest(context.Background(), event)
		require.NoError(t, err)

		resp, ok := respAny.(*lift.Response)
		require.True(t, ok)
		require.Equal(t, 200, resp.StatusCode)

		body, ok := resp.Body.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "user-123", body["user_id"])
		require.Equal(t, "tenant-abc", body["tenant_id"])
		require.Equal(t, true, body["authenticated"])
		require.Equal(t, true, body["auth_claims_set"])
		require.True(t, handlerCalled)
	})
}

func TestContract_HTTP_ErrorResponseShaping(t *testing.T) {
	app := lift.New()
	require.NoError(t, app.GET("/bad", func(_ *lift.Context) error {
		return lift.NewLiftError("VALIDATION_ERROR", "Validation failed", 400).
			WithDetail("field", "email").
			WithDetail("reason", "invalid format")
	}))

	event := apiGatewayV1Event("GET", "/bad", nil, "")
	respAny, err := app.HandleRequest(context.Background(), event)
	require.NoError(t, err)

	resp, ok := respAny.(*lift.Response)
	require.True(t, ok)
	require.Equal(t, 400, resp.StatusCode)
	require.Equal(t, lift.ContentTypeJSON, resp.Headers[lift.HeaderContentType])

	body, ok := resp.Body.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "VALIDATION_ERROR", body["code"])
	require.Equal(t, "Validation failed", body["message"])

	details, ok := body["details"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "email", details["field"])
	require.Equal(t, "invalid format", details["reason"])
}

func TestContract_HTTP_MiddlewareOrdering(t *testing.T) {
	app := lift.New()

	var order []string
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			order = append(order, "mw1_before")
			err := next.Handle(ctx)
			order = append(order, "mw1_after")
			return err
		})
	})
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			order = append(order, "mw2_before")
			err := next.Handle(ctx)
			order = append(order, "mw2_after")
			return err
		})
	})
	require.NoError(t, app.GET("/ok", func(ctx *lift.Context) error {
		order = append(order, "handler")
		return ctx.OK(map[string]bool{"ok": true})
	}))

	event := apiGatewayV1Event("GET", "/ok", nil, "")
	_, err := app.HandleRequest(context.Background(), event)
	require.NoError(t, err)
	require.Equal(t, []string{"mw1_before", "mw2_before", "handler", "mw2_after", "mw1_after"}, order)
}

func TestContract_TenantIsolationInvariants_WithDynamORM(t *testing.T) {
	cfg := dynamorm.DefaultConfig()
	cfg.AutoTransaction = false

	factory := dynamorm.NewMockDBFactory()

	t.Run("missing_tenant_is_rejected", func(t *testing.T) {
		app := lift.New()
		app.Use(dynamorm.WithDynamORM(cfg, factory))

		handlerCalled := false
		require.NoError(t, app.GET("/items", func(ctx *lift.Context) error {
			handlerCalled = true
			return ctx.OK(map[string]bool{"ok": true})
		}))

		event := apiGatewayV1Event("GET", "/items", nil, "")
		respAny, err := app.HandleRequest(context.Background(), event)
		require.NoError(t, err)

		resp, ok := respAny.(*lift.Response)
		require.True(t, ok)
		require.Equal(t, 401, resp.StatusCode)
		require.False(t, handlerCalled)
	})

	t.Run("tenant_scoped_db_is_available_when_tenant_set", func(t *testing.T) {
		app := lift.New()

		// Simulate upstream auth/tenant identification middleware.
		app.Use(func(next lift.Handler) lift.Handler {
			return lift.HandlerFunc(func(ctx *lift.Context) error {
				ctx.SetTenantID("tenant-123")
				return next.Handle(ctx)
			})
		})
		app.Use(dynamorm.WithDynamORM(cfg, factory))

		require.NoError(t, app.GET("/items", func(ctx *lift.Context) error {
			db, err := dynamorm.DB(ctx)
			require.NoError(t, err)
			tenantDB, err := dynamorm.TenantDB(ctx)
			require.NoError(t, err)
			require.NotSame(t, db, tenantDB)
			return ctx.OK(map[string]bool{"ok": true})
		}))

		event := apiGatewayV1Event("GET", "/items", nil, "")
		respAny, err := app.HandleRequest(context.Background(), event)
		require.NoError(t, err)

		resp, ok := respAny.(*lift.Response)
		require.True(t, ok)
		require.Equal(t, 200, resp.StatusCode)
	})
}

func TestContract_WebSocketRoutingAndMiddlewareOrdering(t *testing.T) {
	app := lift.New(lift.WithWebSocketSupport())

	var order []string
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			order = append(order, "mw1")
			return next.Handle(ctx)
		})
	})
	app.Use(func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			order = append(order, "mw2")
			return next.Handle(ctx)
		})
	})

	connectCalled := false
	defaultCalled := false
	app.WebSocket("$connect", func(ctx *lift.Context) error {
		connectCalled = true
		order = append(order, "connect")
		return ctx.JSON(map[string]bool{"connected": true})
	})
	app.WebSocket("$default", func(ctx *lift.Context) error {
		defaultCalled = true
		order = append(order, "default")
		return ctx.JSON(map[string]bool{"default": true})
	})

	t.Run("connect_route_is_routed_and_wraps_middleware", func(t *testing.T) {
		order = nil
		connectCalled = false
		defaultCalled = false

		event := webSocketEvent("$connect", "conn-1")
		respAny, err := app.HandleRequest(context.Background(), event)
		require.NoError(t, err)

		resp, ok := respAny.(*lift.Response)
		require.True(t, ok)
		require.Equal(t, 200, resp.StatusCode)
		require.True(t, connectCalled)
		require.False(t, defaultCalled)
		require.Equal(t, []string{"mw1", "mw2", "connect"}, order)
	})

	t.Run("unknown_route_falls_back_to_default", func(t *testing.T) {
		order = nil
		connectCalled = false
		defaultCalled = false

		event := webSocketEvent("unknownRoute", "conn-2")
		respAny, err := app.HandleRequest(context.Background(), event)
		require.NoError(t, err)

		resp, ok := respAny.(*lift.Response)
		require.True(t, ok)
		require.Equal(t, 200, resp.StatusCode)
		require.False(t, connectCalled)
		require.True(t, defaultCalled)
		require.Equal(t, []string{"mw1", "mw2", "default"}, order)
	})
}

func apiGatewayV1Event(method, path string, headers map[string]string, body string) map[string]any {
	eventHeaders := make(map[string]any)
	for k, v := range headers {
		eventHeaders[k] = v
	}

	event := map[string]any{
		"resource":   path,
		"httpMethod": method,
		"path":       path,
		"requestContext": map[string]any{
			"requestId": "contract-request-id",
		},
		"headers": eventHeaders,
	}

	if body != "" {
		event["body"] = body
	}

	return event
}

func webSocketEvent(routeKey, connectionID string) map[string]any {
	return map[string]any{
		"requestContext": map[string]any{
			"routeKey":     routeKey,
			"connectionId": connectionID,
			"eventType":    "MESSAGE",
			"stage":        "test",
			"requestId":    "contract-ws-request-id",
			"domainName":   "test.execute-api.us-east-1.amazonaws.com",
			"apiId":        "contract-api-id",
		},
		"headers": map[string]any{
			"Content-Type": "application/json",
		},
	}
}
