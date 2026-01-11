package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestDefaultJWTConfig_SetsExpectedDefaults(t *testing.T) {
	cfg := DefaultJWTConfig()
	require.Equal(t, algorithmHS256, cfg.Algorithm)
	require.Equal(t, "header:Authorization", cfg.TokenLookup)
	require.NotNil(t, cfg.ErrorHandler)
}

func TestJWTAuth_SkipsPaths(t *testing.T) {
	mw := JWTAuth(JWTConfig{
		Secret:      "secret",
		Algorithm:   algorithmHS256,
		TokenLookup: "header:Authorization",
		SkipPaths:   []string{"/public"},
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/public/health"})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestJWTAuth_MissingToken_UsesErrorHandler(t *testing.T) {
	mw := JWTAuth(JWTConfig{
		Secret:    "secret",
		Algorithm: algorithmHS256,
	})

	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/private",
		Headers: map[string]string{},
	})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	err := handler.Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, lift.ErrorCodeUnauthorized, liftErr.Code)
	require.Equal(t, 401, liftErr.StatusCode)
	require.False(t, called)
}

func TestJWTAuth_ValidToken_SetsClaimsAndCallsNext(t *testing.T) {
	secret := "secret"
	tokenString := newHS256JWT(t, secret, jwt.MapClaims{
		"user_id":   "user-123",
		"tenant_id": "tenant-abc",
		"sub":       "ignored-because-user-id",
	})

	mw := JWTAuth(JWTConfig{
		Secret:    secret,
		Algorithm: algorithmHS256,
	})

	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/private",
		Headers: map[string]string{"Authorization": "Bearer " + tokenString},
	})
	ctx := lift.NewContext(context.Background(), req)

	called := false
	handler := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		called = true
		require.True(t, ctx.IsAuthenticated())
		require.Equal(t, "user-123", ctx.UserID())
		require.Equal(t, "tenant-abc", ctx.TenantID())
		require.Equal(t, "user-123", ctx.GetClaim("user_id"))
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)
}

func TestJWTAuth_ValidatorError_UsesErrorHandler(t *testing.T) {
	secret := "secret"
	tokenString := newHS256JWT(t, secret, jwt.MapClaims{"sub": "user-1"})

	mw := JWTAuth(JWTConfig{
		Secret:    secret,
		Algorithm: algorithmHS256,
		Validator: func(_ jwt.MapClaims) error {
			return errors.New("validator rejected")
		},
	})

	req := lift.NewRequest(&adapters.Request{
		Method:  "GET",
		Path:    "/private",
		Headers: map[string]string{"Authorization": "Bearer " + tokenString},
	})
	ctx := lift.NewContext(context.Background(), req)

	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		t.Fatalf("next handler should not run when validator fails")
		return nil
	}))

	err := handler.Handle(ctx)
	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, lift.ErrorCodeUnauthorized, liftErr.Code)
	require.Equal(t, 401, liftErr.StatusCode)
}

func TestCreateExtractor_ParsesHeaderQueryCookieLookups(t *testing.T) {
	headerExtractor := createExtractor("header:Authorization")
	queryExtractor := createExtractor("query:token")
	cookieExtractor := createExtractor("cookie:jwt")

	req := lift.NewRequest(&adapters.Request{
		Method: "GET",
		Path:   "/test",
		Headers: map[string]string{
			"Authorization": "Bearer abc",
			"Cookie":        `jwt="a.b.c"; badcookie; other=xyz`,
		},
		QueryParams: map[string]string{"token": "q-token"},
	})
	ctx := lift.NewContext(context.Background(), req)

	token, err := headerExtractor(ctx)
	require.NoError(t, err)
	require.Equal(t, "abc", token)

	token, err = queryExtractor(ctx)
	require.NoError(t, err)
	require.Equal(t, "q-token", token)

	token, err = cookieExtractor(ctx)
	require.NoError(t, err)
	require.Equal(t, "a.b.c", token)
}

func TestCreateExtractor_RejectsInvalidLookups(t *testing.T) {
	invalid := createExtractor("Authorization")
	unsupported := createExtractor("body:token")

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	_, err := invalid(ctx)
	require.Error(t, err)

	_, err = unsupported(ctx)
	require.Error(t, err)
}

func TestParseToken_ValidatesAlgorithm(t *testing.T) {
	secret := "secret"
	hsToken := newHS256JWT(t, secret, jwt.MapClaims{"sub": "user-1"})

	parsed, err := parseToken(hsToken, JWTConfig{Secret: secret, Algorithm: algorithmHS256})
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rsToken := newRS256JWT(t, privateKey, jwt.MapClaims{"sub": "user-1"})

	_, err = parseToken(rsToken, JWTConfig{Secret: secret, Algorithm: algorithmHS256})
	require.Error(t, err)

	_, err = parseToken(hsToken, JWTConfig{Secret: secret, Algorithm: "none"})
	require.Error(t, err)

	parsed, err = parseToken(rsToken, JWTConfig{PublicKey: &privateKey.PublicKey, Algorithm: algorithmRS256})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
}

func TestJWTClaimsHandler_ExtractClaims_HandlesNonMapClaims(t *testing.T) {
	handler := newJWTClaimsHandler(JWTConfig{})
	_, err := handler.extractClaims(&jwt.Token{Claims: &jwt.RegisteredClaims{}})
	require.Error(t, err)

	handler = newJWTClaimsHandler(JWTConfig{Claims: &jwt.RegisteredClaims{}})
	claims, err := handler.extractClaims(&jwt.Token{Claims: &jwt.RegisteredClaims{}})
	require.NoError(t, err)
	require.NotNil(t, claims)
}

func TestExtractJWTFromCookie_ValidationErrors(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test", Headers: map[string]string{}})
	ctx := lift.NewContext(context.Background(), req)

	_, err := extractJWTFromCookie(ctx, "jwt")
	require.Error(t, err)

	ctx.Request.Headers["Cookie"] = "other=a.b.c"
	_, err = extractJWTFromCookie(ctx, "jwt")
	require.Error(t, err)

	ctx.Request.Headers["Cookie"] = "jwt=not-a-jwt"
	_, err = extractJWTFromCookie(ctx, "jwt")
	require.Error(t, err)
}

func TestValidateJWTCookie_RejectsInvalidValues(t *testing.T) {
	require.Error(t, validateJWTCookie(&CookieToken{Name: "", Value: "a.b.c"}))
	require.Error(t, validateJWTCookie(&CookieToken{Name: "jwt", Value: ""}))
	require.Error(t, validateJWTCookie(&CookieToken{Name: "jwt", Value: "a.b"}))
	require.Error(t, validateJWTCookie(&CookieToken{Name: "jwt", Value: "a..c"}))
	require.Error(t, validateJWTCookie(&CookieToken{Name: "jwt", Value: "a.b+c.d"}))

	longValue := strings.Repeat("a", 8193) + ".b.c"
	require.Error(t, validateJWTCookie(&CookieToken{Name: "jwt", Value: longValue}))

	require.NoError(t, validateJWTCookie(&CookieToken{Name: "jwt", Value: "a.b.c"}))
}

func newHS256JWT(t *testing.T, secret string, claims jwt.Claims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func newRS256JWT(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.Claims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return signed
}
