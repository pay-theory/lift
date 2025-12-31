package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrincipalRoleAndScopeChecks(t *testing.T) {
	t.Parallel()

	p := &Principal{
		Roles:  []string{"user", "manager"},
		Scopes: []string{"payments:read", "payments:write"},
	}

	assert.True(t, p.HasRole("user"))
	assert.False(t, p.HasRole("admin"))
	assert.True(t, p.HasScope("payments:read"))
	assert.False(t, p.HasScope("users:read"))

	assert.True(t, p.HasAnyRole("admin", "manager"))
	assert.False(t, p.HasAnyRole("admin", "system"))
	assert.True(t, p.HasAllRoles("user", "manager"))
	assert.False(t, p.HasAllRoles("user", "admin"))
}

func TestPrincipalTenantAndExpirationChecks(t *testing.T) {
	t.Parallel()

	now := time.Now()
	p := &Principal{
		TenantID:  "t1",
		IssuedAt:  now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Minute),
	}

	assert.False(t, p.IsExpired())
	assert.True(t, p.IsSameTenant("t1"))
	assert.False(t, p.IsSameTenant("t2"))
	assert.True(t, p.IsValidForTenant("t1"))
	assert.False(t, p.IsValidForTenant("t2"))

	p.ExpiresAt = now.Add(-time.Second)
	assert.True(t, p.IsExpired())
	assert.False(t, p.IsValidForTenant("t1"))
}

func TestPrincipalCanAccessResource(t *testing.T) {
	t.Parallel()

	t.Run("admin can access everything", func(t *testing.T) {
		t.Parallel()

		p := &Principal{Roles: []string{"admin"}}
		assert.True(t, p.CanAccessResource("unknown", "read"))
	})

	t.Run("health is public", func(t *testing.T) {
		t.Parallel()

		p := &Principal{Roles: []string{}}
		assert.True(t, p.CanAccessResource("health", "read"))
	})

	t.Run("resource-specific role checks", func(t *testing.T) {
		t.Parallel()

		assert.True(t, (&Principal{Roles: []string{"user"}}).CanAccessResource("users", "read"))
		assert.True(t, (&Principal{Roles: []string{"manager"}}).CanAccessResource("users", "read"))
		assert.False(t, (&Principal{Roles: []string{}}).CanAccessResource("users", "read"))

		assert.True(t, (&Principal{Roles: []string{"payment_processor"}}).CanAccessResource("payments", "read"))
		assert.True(t, (&Principal{Roles: []string{"manager"}}).CanAccessResource("payments", "read"))
		assert.False(t, (&Principal{Roles: []string{"user"}}).CanAccessResource("payments", "read"))

		assert.True(t, (&Principal{Roles: []string{"account_manager"}}).CanAccessResource("accounts", "read"))
		assert.True(t, (&Principal{Roles: []string{"manager"}}).CanAccessResource("accounts", "read"))
		assert.False(t, (&Principal{Roles: []string{"user"}}).CanAccessResource("accounts", "read"))

		assert.False(t, (&Principal{Roles: []string{"user"}}).CanAccessResource("unknown", "read"))
	})
}

func TestPrincipalToAuditMap(t *testing.T) {
	t.Parallel()

	issuedAt := time.Now().Add(-time.Minute)
	expiresAt := time.Now().Add(time.Minute)

	p := &Principal{
		UserID:     "u1",
		TenantID:   "t1",
		AccountID:  "a1",
		Roles:      []string{"user"},
		Scopes:     []string{"s1"},
		AuthMethod: "jwt",
		IPAddress:  "127.0.0.1",
		UserAgent:  "ua",
		SessionID:  "sess",
		RequestID:  "req",
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
	}

	m := p.ToAuditMap()
	assert.Equal(t, "u1", m["user_id"])
	assert.Equal(t, "t1", m["tenant_id"])
	assert.Equal(t, "a1", m["account_id"])
	assert.Equal(t, []string{"user"}, m["roles"])
	assert.Equal(t, []string{"s1"}, m["scopes"])
	assert.Equal(t, "jwt", m["auth_method"])
	assert.Equal(t, "127.0.0.1", m["ip_address"])
	assert.Equal(t, "ua", m["user_agent"])
	assert.Equal(t, "sess", m["session_id"])
	assert.Equal(t, "req", m["request_id"])
	assert.Equal(t, issuedAt, m["issued_at"])
	assert.Equal(t, expiresAt, m["expires_at"])
}

func TestPrincipalFactories(t *testing.T) {
	t.Parallel()

	anon := AnonymousPrincipal()
	require.NotNil(t, anon)
	assert.Equal(t, "anonymous", anon.UserID)
	assert.Equal(t, "none", anon.AuthMethod)
	assert.Contains(t, anon.Roles, "anonymous")
	assert.True(t, anon.ExpiresAt.After(anon.IssuedAt))

	system := SystemPrincipal()
	require.NotNil(t, system)
	assert.Equal(t, "system", system.UserID)
	assert.Equal(t, "system", system.AuthMethod)
	assert.Contains(t, system.Roles, "system")
	assert.Contains(t, system.Scopes, "*")
	assert.True(t, system.ExpiresAt.After(system.IssuedAt))

	svc := ServicePrincipal("svc", "tenant")
	require.NotNil(t, svc)
	assert.Equal(t, "svc", svc.UserID)
	assert.Equal(t, "tenant", svc.TenantID)
	assert.Equal(t, "service", svc.AuthMethod)
	assert.Contains(t, svc.Roles, "service")
	assert.Contains(t, svc.Scopes, "service")
}

func TestPrincipalBuilder(t *testing.T) {
	t.Parallel()

	builder := NewPrincipalBuilder().
		WithUserID("u1").
		WithTenantID("t1").
		WithAccountID("a1").
		WithRoles("user").
		AddRole("manager").
		WithScopes("s1").
		AddScope("s2").
		WithAuthMethod("jwt").
		WithRequest("127.0.0.1", "ua", "req").
		WithExpiration(2 * time.Hour)

	require.NoError(t, builder.Validate())

	p := builder.Build()
	assert.Equal(t, "u1", p.UserID)
	assert.Equal(t, "t1", p.TenantID)
	assert.Equal(t, "a1", p.AccountID)
	assert.Equal(t, []string{"user", "manager"}, p.Roles)
	assert.Equal(t, []string{"s1", "s2"}, p.Scopes)
	assert.Equal(t, "jwt", p.AuthMethod)
	assert.Equal(t, "127.0.0.1", p.IPAddress)
	assert.Equal(t, "ua", p.UserAgent)
	assert.Equal(t, "req", p.RequestID)
	assert.True(t, p.ExpiresAt.After(p.IssuedAt))
}

func TestPrincipalBuilderValidate_Errors(t *testing.T) {
	t.Parallel()

	t.Run("missing user id", func(t *testing.T) {
		t.Parallel()

		builder := NewPrincipalBuilder().WithAuthMethod("jwt")
		err := builder.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_PRINCIPAL", secErr.Code)
	})

	t.Run("missing auth method", func(t *testing.T) {
		t.Parallel()

		builder := NewPrincipalBuilder().WithUserID("u1")
		err := builder.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_PRINCIPAL", secErr.Code)
	})

	t.Run("expired principal", func(t *testing.T) {
		t.Parallel()

		builder := NewPrincipalBuilder().
			WithUserID("u1").
			WithAuthMethod("jwt").
			WithExpiration(-1 * time.Hour)

		err := builder.Validate()
		require.Error(t, err)
		var secErr *SecurityError
		require.ErrorAs(t, err, &secErr)
		assert.Equal(t, "INVALID_PRINCIPAL", secErr.Code)
	})
}

