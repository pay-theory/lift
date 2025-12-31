package lift

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/security"
	"github.com/stretchr/testify/require"
)

func TestSecurityContext_NewSecurityContext_RestoresPrincipal(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/",
		Headers:     map[string]string{"X-Forwarded-For": "203.0.113.10", "User-Agent": "ua"},
		QueryParams: map[string]string{},
		RawEvent: map[string]any{
			"requestContext": map[string]any{
				"http": map[string]any{"sourceIp": "203.0.113.10"},
			},
		},
	})
	ctx := NewContext(context.Background(), req)

	principal := &security.Principal{
		UserID:     "user-1",
		TenantID:   "tenant-1",
		AccountID:  "acct-1",
		Roles:      []string{"admin"},
		Scopes:     []string{"*"},
		AuthMethod: "jwt",
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	ctx.Set("principal", principal)

	secCtx := NewSecurityContext(ctx)
	require.Equal(t, principal, secCtx.GetPrincipal())
	require.NotEmpty(t, secCtx.RequestID())
}

func TestSecurityContext_SetPrincipal_PopulatesContextValuesAndTracking(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/",
		Headers:     map[string]string{"X-Forwarded-For": "203.0.113.10", "User-Agent": "ua"},
		QueryParams: map[string]string{},
	})
	ctx := NewContext(context.Background(), req)
	secCtx := NewSecurityContext(ctx)

	principal := &security.Principal{
		UserID:     "user-1",
		TenantID:   "tenant-1",
		AccountID:  "acct-1",
		Roles:      []string{"admin"},
		Scopes:     []string{"*"},
		AuthMethod: "jwt",
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	secCtx.SetPrincipal(principal)

	require.Equal(t, principal, secCtx.GetPrincipal())
	require.Equal(t, "user-1", secCtx.UserID())
	require.Equal(t, "tenant-1", secCtx.TenantID())
	require.Equal(t, "acct-1", secCtx.AccountID())
	require.Equal(t, secCtx.RequestID(), principal.RequestID)
	require.Equal(t, "203.0.113.10", principal.IPAddress)
	require.Equal(t, "ua", principal.UserAgent)

	audit := secCtx.ToAuditMap()
	require.Equal(t, "user-1", audit["user_id"])
	require.Equal(t, "tenant-1", audit["tenant_id"])
	require.Equal(t, "acct-1", audit["account_id"])
	require.Equal(t, secCtx.RequestID(), audit["request_id"])
	require.NotNil(t, audit["timestamp"])
}

func TestSecurityContext_RequireMethods(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/",
		Headers:     map[string]string{"X-Forwarded-For": "203.0.113.10"},
		QueryParams: map[string]string{},
	})
	ctx := NewContext(context.Background(), req)
	secCtx := NewSecurityContext(ctx)

	require.Error(t, secCtx.RequireAuthentication())
	require.Error(t, secCtx.RequireRole("admin"))
	require.Error(t, secCtx.RequirePermission("health", "read"))
	require.Error(t, secCtx.RequireTenant("tenant-1"))

	secCtx.SetPrincipal(&security.Principal{
		UserID:    "user-1",
		TenantID:  "tenant-1",
		Roles:     []string{"user"},
		ExpiresAt: time.Now().Add(time.Hour),
	})

	require.NoError(t, secCtx.RequireAuthentication())
	require.Error(t, secCtx.RequireRole("admin"))
	require.NoError(t, secCtx.RequireRole("user"))

	require.NoError(t, secCtx.RequirePermission("health", "read"))
	require.Error(t, secCtx.RequirePermission("payments", "read"))

	require.Error(t, secCtx.RequireTenant("tenant-other"))
	require.NoError(t, secCtx.RequireTenant("tenant-1"))
}

func TestSecurityContext_GetClientIP_Unknown(t *testing.T) {
	req := NewRequest(&adapters.Request{
		Method:      "GET",
		Path:        "/",
		Headers:     map[string]string{},
		QueryParams: map[string]string{},
	})
	ctx := NewContext(context.Background(), req)
	secCtx := NewSecurityContext(ctx)

	require.Equal(t, "unknown", secCtx.GetClientIP())
	require.False(t, secCtx.ValidateIP([]string{"203.0.113.0/24"}))
}

func TestSecurityContext_WithSecurity_AndNilPrincipalBranches(t *testing.T) {
	req := NewRequest(&adapters.Request{Method: "GET", Path: "/"})
	ctx := NewContext(context.Background(), req)

	secCtx := WithSecurity(ctx)
	require.NotNil(t, secCtx)
	require.NotEmpty(t, secCtx.RequestID())

	require.False(t, secCtx.HasRole("admin"))
	require.False(t, secCtx.HasPermission("resource", "action"))
	require.False(t, secCtx.ValidateTenant("tenant-1"))
	require.Equal(t, "", secCtx.AccountID())
}
