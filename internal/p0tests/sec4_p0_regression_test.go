package p0tests

import (
	"strings"
	"testing"

	"github.com/pay-theory/lift/pkg/logger"
	"github.com/pay-theory/lift/pkg/utils/sanitization"
)

func TestSEC4HeaderRedaction(t *testing.T) {
	headers := map[string][]string{
		"Authorization": {"Bearer abc"},
		"Cookie":        {"session=abc"},
		"X-Api-Key":     {"key"},
		"X-Auth-Token":  {"token"},
		"X-Csrf-Token":  {"csrf"},
		"X-Session-Id":  {"session"},
		"Content-Type":  {"application/json"},
	}

	sanitized := sanitization.SanitizeHeaders(headers)
	redactedKeys := []string{
		"Authorization",
		"Cookie",
		"X-Api-Key",
		"X-Auth-Token",
		"X-Csrf-Token",
		"X-Session-Id",
	}

	for _, key := range redactedKeys {
		if sanitized[key] != "[REDACTED]" {
			t.Fatalf("expected header %s to be redacted, got %q", key, sanitized[key])
		}
	}

	if sanitized["Content-Type"] != "application/json" {
		t.Fatalf("expected Content-Type to remain, got %q", sanitized["Content-Type"])
	}
}

func TestSEC4QueryParamSanitization(t *testing.T) {
	params := map[string][]string{
		"token":    {"abc"},
		"api_key":  {"key"},
		"apikey":   {"key2"},
		"password": {"pw"},
		"secret":   {"secret"},
		"q":        {"search"},
	}

	sanitized := sanitization.SanitizeQueryParams(params)
	sanitizedKeys := []string{"token", "api_key", "apikey", "password", "secret"}
	for _, key := range sanitizedKeys {
		if sanitized[key] != "[SANITIZED_QUERY_PARAMS]" {
			t.Fatalf("expected query param %s to be sanitized, got %q", key, sanitized[key])
		}
	}

	if sanitized["q"] != "search" {
		t.Fatalf("expected q to remain, got %q", sanitized["q"])
	}
}

func TestSEC4FieldValueRedaction(t *testing.T) {
	if got := sanitization.SanitizeFieldValue("authorization", "Bearer abc"); got != "[REDACTED]" {
		t.Fatalf("authorization redaction failed: %v", got)
	}

	if got := sanitization.SanitizeFieldValue("cvv", "123"); got != "[REDACTED]" {
		t.Fatalf("cvv redaction failed: %v", got)
	}

	card := "4111111111111111"
	masked := sanitization.SanitizeFieldValue("card_number", card)
	maskedStr, ok := masked.(string)
	if !ok {
		t.Fatalf("expected masked card number to be string, got %T", masked)
	}
	if maskedStr == card {
		t.Fatalf("expected card number to be masked, got original")
	}
	if maskedStr != "411111******1111" {
		t.Fatalf("expected masked card number to be 411111******1111, got %q", maskedStr)
	}
}

func TestSEC4LogForgeryPrevention(t *testing.T) {
	cleaned := logger.SanitizeLogString("a\r\nb\n")
	if strings.Contains(cleaned, "\n") || strings.Contains(cleaned, "\r") {
		t.Fatalf("expected sanitized log string to remove newlines, got %q", cleaned)
	}
	if cleaned != "ab" {
		t.Fatalf("expected sanitized log string to be %q, got %q", "ab", cleaned)
	}

	fields := map[string]any{"authorization": "Bearer abc"}
	sanitized := logger.SanitizeLogFields(fields)
	if sanitized["authorization"] != "[REDACTED]" {
		t.Fatalf("expected authorization field redacted, got %v", sanitized["authorization"])
	}
}
