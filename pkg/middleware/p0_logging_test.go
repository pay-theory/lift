package middleware

import (
	"context"
	"fmt"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestP0_EnhancedObservability_RedactsBodiesAndDoesNotLogAuthorization(t *testing.T) {
	logger := &mockLogger{}

	config := EnhancedObservabilityConfig{
		EnableLogging:   true,
		EnableTracing:   false,
		EnableMetrics:   false,
		Logger:          logger,
		SampleRate:      1.0,
		LogRequestBody:  true,
		LogResponseBody: true,
	}

	middleware := EnhancedObservabilityMiddleware(config)
	handler := middleware(lift.HandlerFunc(func(ctx *lift.Context) error {
		ctx.Response.StatusCode = 200
		ctx.Response.Body = []byte(`{"token":"secret-token","card_number":"4111111111111111"}`)
		return nil
	}))

	reqBody := []byte(`{"password":"secret123","card_number":"4111111111111111"}`)
	adapterRequest := &adapters.Request{
		Method: "POST",
		Path:   "/payments",
		Headers: map[string]string{
			"Authorization":   "Bearer secret-token",
			"X-Forwarded-For": "203.0.113.1",
			"User-Agent":      "lift-test",
		},
		QueryParams: map[string]string{},
		Body:        reqBody,
	}

	ctx := &lift.Context{
		Context:  context.Background(),
		Request:  lift.NewRequest(adapterRequest),
		Response: &lift.Response{Headers: make(map[string]string)},
	}

	require.NoError(t, handler.Handle(ctx))
	require.NotEmpty(t, logger.logs)

	var started, completed map[string]any
	for _, entry := range logger.logs {
		if entry["message"] == "Request started" {
			started = entry
		}
		if entry["message"] == "Request completed" {
			completed = entry
		}
	}

	require.NotNil(t, started)
	require.Equal(t, "[USER_CONTENT_REDACTED]", started["request_body"])
	require.Equal(t, len(reqBody), started["request_body_size"])

	require.NotNil(t, completed)
	require.Equal(t, "[RESPONSE_CONTENT_REDACTED]", completed["response_body"])
	require.Equal(t, len(ctx.Response.Body.([]byte)), completed["response_body_size"])

	for _, entry := range logger.logs {
		require.NotContains(t, entry, "Authorization")
		require.NotContains(t, entry, "authorization")

		// Ensure no raw secrets leak into log values.
		for k, v := range entry {
			s := fmt.Sprintf("%v", v)
			require.NotContainsf(t, s, "secret-token", "field %q leaked secret-token", k)
			require.NotContainsf(t, s, "secret123", "field %q leaked secret123", k)
			require.NotContainsf(t, s, "4111111111111111", "field %q leaked card_number", k)
		}
	}
}
