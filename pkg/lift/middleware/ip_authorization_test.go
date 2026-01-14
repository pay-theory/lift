package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/security"
	"github.com/stretchr/testify/require"
)

func newIPAuthService(t *testing.T, allowedList string, statusCode int) *security.IPAuthorizationService {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(statusCode)
		if statusCode >= 400 {
			_, _ = w.Write([]byte(`{"__type":"InternalServerError","message":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"Parameter":{"Name":"param","Type":"String","Value":"%s","Version":1}}`, allowedList)))
	}))
	t.Cleanup(srv.Close)

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")),
	}

	client := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
	})

	return security.NewIPAuthorizationService(client, "param")
}

func newLiftCtx(method, path, sourceIP string) *lift.Context {
	req := &adapters.Request{
		Method:  method,
		Path:    path,
		Headers: map[string]string{},
	}
	if sourceIP != "" {
		req.Headers["X-Forwarded-For"] = sourceIP
	}
	return lift.NewContext(context.Background(), lift.NewRequest(req))
}

func TestIPAuthorizationMiddleware_AllowsAuthorizedRequests(t *testing.T) {
	ipAuthService := newIPAuthService(t, "1.2.3.4", http.StatusOK)

	mw := IPAuthorization(IPAuthorizationConfig{IPAuthService: ipAuthService})

	ctx := newLiftCtx(http.MethodGet, "/ok", "1.2.3.4")

	called := false
	handler := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		called = true
		return nil
	}))

	require.NoError(t, handler.Handle(ctx))
	require.True(t, called)

	require.NoError(t, CheckIPAuthorization(ctx, ipAuthService))
}

func TestIPAuthorizationMiddleware_BlocksUnauthorizedRequests(t *testing.T) {
	ipAuthService := newIPAuthService(t, "1.2.3.4", http.StatusOK)

	mw := IPAuthorization(IPAuthorizationConfig{IPAuthService: ipAuthService})

	ctx := newLiftCtx(http.MethodGet, "/nope", "9.9.9.9")
	err := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		t.Fatalf("next handler should not run for unauthorized IP")
		return nil
	})).Handle(ctx)

	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 403, liftErr.StatusCode)

	require.Error(t, CheckIPAuthorization(ctx, ipAuthService))
}

func TestIPAuthorizationMiddleware_PropagatesServiceErrors(t *testing.T) {
	ipAuthService := newIPAuthService(t, "", http.StatusInternalServerError)
	mw := IPAuthorization(IPAuthorizationConfig{IPAuthService: ipAuthService})

	ctx := newLiftCtx(http.MethodGet, "/err", "1.2.3.4")
	err := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)

	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 500, liftErr.StatusCode)
}

func TestIPAuthorizationMiddleware_ReturnsParameterErrorWhenIPMissing(t *testing.T) {
	ipAuthService := newIPAuthService(t, "1.2.3.4", http.StatusOK)
	mw := IPAuthorization(IPAuthorizationConfig{IPAuthService: ipAuthService})

	ctx := newLiftCtx(http.MethodGet, "/missing", "")
	err := mw(lift.HandlerFunc(func(_ *lift.Context) error { return nil })).Handle(ctx)

	require.Error(t, err)
	var liftErr *lift.LiftError
	require.ErrorAs(t, err, &liftErr)
	require.Equal(t, 400, liftErr.StatusCode)
}
