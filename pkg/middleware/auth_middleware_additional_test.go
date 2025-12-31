package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/security"
	"github.com/stretchr/testify/require"
)

func TestJWTMiddleware_ConfigError_ReturnsSystemError(t *testing.T) {
	mw := JWT(security.JWTConfig{SigningMethod: "unsupported"})
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 500, liftErr.StatusCode)
}

func TestJWTMiddleware_MissingToken_ReturnsUnauthorized(t *testing.T) {
	mw := JWT(security.JWTConfig{SigningMethod: "HS256", SecretKey: "secret"})
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 401, liftErr.StatusCode)
}

func TestJWTMiddleware_InvalidToken_LogsAndReturnsUnauthorized(t *testing.T) {
	logger := &mockLogger{}
	mw := JWT(security.JWTConfig{SigningMethod: "HS256", SecretKey: "secret"})

	badToken := newHS256JWT(t, "wrong-secret", &JWTClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user-1"}})
	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Authorization": "Bearer " + badToken},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.Logger = logger

	err := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.Error(t, err)
	require.Greater(t, len(logger.logs), 0)
}

func TestJWTMiddleware_ValidToken_SetsPrincipalAndAuthorizesRoleScopeTenant(t *testing.T) {
	secret := "secret"
	token := newHS256JWT(t, secret, &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		TenantID: "tenant-1",
		Roles:    []string{"admin"},
		Scopes:   []string{"read"},
	})

	cfg := security.JWTConfig{SigningMethod: "HS256", SecretKey: secret}

	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Authorization": "Bearer " + token},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.RequestID = "req-1"
	ctx.Request.Headers["X-Real-IP"] = "10.0.0.1"
	ctx.Request.Headers["User-Agent"] = "ua"

	called := false
	handler := JWT(cfg)(
		RequireRole("admin")(
			RequireScope("read")(
				RequireTenant("tenant-1")(
					lift.HandlerFunc(func(ctx *lift.Context) error {
						called = true
						require.Equal(t, "user-1", ctx.UserID())
						require.Equal(t, "tenant-1", ctx.TenantID())

						secCtx := lift.WithSecurity(ctx)
						require.NotNil(t, secCtx.GetPrincipal())
						require.True(t, secCtx.GetPrincipal().HasRole("admin"))
						return nil
					}),
				),
			),
		),
	)

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestRequireRoleScopeTenant_DenyPaths(t *testing.T) {
	secret := "secret"
	token := newHS256JWT(t, secret, &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TenantID: "tenant-1",
		Roles:    []string{"user"},
		Scopes:   []string{"read"},
	})

	cfg := security.JWTConfig{SigningMethod: "HS256", SecretKey: secret}
	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Authorization": "Bearer " + token},
	})

	logger := &mockLogger{}

	// RequireRole denies
	ctx := lift.NewContext(context.Background(), req)
	ctx.Logger = logger
	err := JWT(cfg)(RequireRole("admin")(lift.HandlerFunc(func(_ *lift.Context) error { return nil }))).Handle(ctx)
	require.Error(t, err)

	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 403, liftErr.StatusCode)

	// RequireScope denies
	ctx2 := lift.NewContext(context.Background(), req)
	ctx2.Logger = logger
	err = JWT(cfg)(RequireScope("write")(lift.HandlerFunc(func(_ *lift.Context) error { return nil }))).Handle(ctx2)
	require.Error(t, err)

	// RequireTenant denies
	ctx3 := lift.NewContext(context.Background(), req)
	err = JWT(cfg)(RequireTenant("tenant-2")(lift.HandlerFunc(func(_ *lift.Context) error { return nil }))).Handle(ctx3)
	require.Error(t, err)
}

func TestJWTOptional_SetsAnonymousOrAuthenticatedPrincipal(t *testing.T) {
	secret := "secret"
	cfg := security.JWTConfig{SigningMethod: "HS256", SecretKey: secret}

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test", Headers: map[string]string{}})
	ctx := lift.NewContext(context.Background(), req)
	require.NoError(t, JWTOptional(cfg)(lift.HandlerFunc(func(ctx *lift.Context) error {
		require.Equal(t, "anonymous", ctx.UserID())
		return nil
	})).Handle(ctx))

	invalidCtx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Authorization": "Bearer not.a.jwt"},
	}))
	require.NoError(t, JWTOptional(cfg)(lift.HandlerFunc(func(ctx *lift.Context) error {
		require.Equal(t, "anonymous", ctx.UserID())
		return nil
	})).Handle(invalidCtx))

	token := newHS256JWT(t, secret, &JWTClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user-1", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}})
	validCtx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Authorization": "Bearer " + token},
	}))
	require.NoError(t, JWTOptional(cfg)(lift.HandlerFunc(func(ctx *lift.Context) error {
		require.Equal(t, "user-1", ctx.UserID())
		return nil
	})).Handle(validCtx))
}

func TestCreatePrincipalFromClaims_LoadsTimesAndRequestMetadata(t *testing.T) {
	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user-1",
		},
		TenantID: "tenant-1",
		Roles:    []string{"r"},
		Scopes:   []string{"s"},
	}

	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"X-Real-IP": "10.0.0.1", "User-Agent": "ua"},
	})
	ctx := lift.NewContext(context.Background(), req)
	ctx.RequestID = "req-1"

	principal := createPrincipalFromClaims(claims, ctx)
	require.Equal(t, "user-1", principal.UserID)
	require.Equal(t, "tenant-1", principal.TenantID)
	require.Equal(t, "10.0.0.1", principal.IPAddress)
	require.Equal(t, "ua", principal.UserAgent)
	require.Equal(t, "req-1", principal.RequestID)
	require.True(t, principal.IssuedAt.After(time.Time{}))
}

func TestLoadRSAPublicKey_LoadsKeyAndRejectsBadPaths(t *testing.T) {
	_, err := loadRSAPublicKey("../bad.pem")
	require.Error(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	pemData := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	require.NotEmpty(t, pemData)

	path := filepath.Join(t.TempDir(), "pub.pem")
	require.NoError(t, os.WriteFile(path, pemData, 0600))

	key, err := loadRSAPublicKey(path)
	require.NoError(t, err)
	require.NotNil(t, key)

	// NewJWTValidator RS256 path
	validator, err := NewJWTValidator(security.JWTConfig{SigningMethod: "RS256", PublicKeyPath: path})
	require.NoError(t, err)
	require.NotNil(t, validator)

	// Invalid PEM
	badPath := filepath.Join(t.TempDir(), "bad.pem")
	require.NoError(t, os.WriteFile(badPath, []byte("not pem"), 0600))
	_, err = loadRSAPublicKey(badPath)
	require.Error(t, err)
}

func TestJWTOptional_ConfigError_ReturnsSystemError(t *testing.T) {
	mw := JWTOptional(security.JWTConfig{SigningMethod: "unsupported"})
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 500, liftErr.StatusCode)
}

func TestRequireRole_Unauthenticated_ReturnsUnauthorized(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := RequireRole("admin")(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 401, liftErr.StatusCode)
}

func TestRequireScope_Unauthenticated_ReturnsUnauthorized(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := RequireScope("scope")(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.Error(t, err)
}

func TestRequireTenant_Unauthenticated_ReturnsUnauthorized(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := RequireTenant("tenant")(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)
	require.Error(t, err)
}

func TestLoadRSAPublicKey_ReadErrors(t *testing.T) {
	_, err := loadRSAPublicKey(filepath.Join(t.TempDir(), "missing.pem"))
	require.Error(t, err)

	// Parse error
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "raw.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: privateKey.PublicKey.N.Bytes()}), 0600))
	_, err = loadRSAPublicKey(path)
	require.Error(t, err)

	// Non-RSA key
	path2 := filepath.Join(t.TempDir(), "other.pem")
	require.NoError(t, os.WriteFile(path2, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("not a key")}), 0600))
	_, err = loadRSAPublicKey(path2)
	require.Error(t, err)
}

func TestJWTValidator_CustomClaimsValidation(t *testing.T) {
	config := security.JWTConfig{
		SigningMethod:   "HS256",
		SecretKey:       "secret",
		RequireTenantID: true,
		ValidateTenant: func(tenantID string) error {
			if tenantID != "tenant-1" {
				return errors.New("bad tenant")
			}
			return nil
		},
	}

	validator, err := NewJWTValidator(config)
	require.NoError(t, err)

	token := newHS256JWT(t, "secret", &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TenantID: "tenant-1",
	})
	_, err = validator.ValidateToken(token)
	require.NoError(t, err)

	badTenantToken := newHS256JWT(t, "secret", &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TenantID: "tenant-2",
	})
	_, err = validator.ValidateToken(badTenantToken)
	require.Error(t, err)
}

