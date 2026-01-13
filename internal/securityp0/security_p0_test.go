//go:build security_p0
// +build security_p0

// Package securityp0 implements hermetic P0 security regression tests for Lift.
// These tests enforce critical security invariants for CHD/auth-sensitive environments.
//
// Run with: go test -tags=security_p0 ./internal/securityp0
package securityp0

import (
	"strings"
	"testing"

	"github.com/pay-theory/lift/pkg/logger"
	"github.com/pay-theory/lift/pkg/utils/sanitization"
)

// TestLogFieldRedaction verifies that sensitive fields are redacted in log output.
// P0 Invariant: Secret/token field redaction must prevent credential leakage in logs.
func TestLogFieldRedaction(t *testing.T) {
	testCases := []struct {
		name            string
		fields          map[string]any
		sensitiveKeys   []string
		forbiddenValues []string
	}{
		{
			name: "authorization_header",
			fields: map[string]any{
				"authorization_header": "Bearer token_secret_12345",
				"user_id":              "user-123",
			},
			sensitiveKeys:   []string{"authorization_header"},
			forbiddenValues: []string{"Bearer", "token_secret", "12345"},
		},
		{
			name: "api_token",
			fields: map[string]any{
				"api_token": "tok_live_4b3403665fea6",
				"request":   "get_account",
			},
			sensitiveKeys:   []string{"api_token"},
			forbiddenValues: []string{"tok_live", "4b3403665fea6"},
		},
		{
			name: "authorization",
			fields: map[string]any{
				"authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				"method":        "POST",
			},
			sensitiveKeys:   []string{"authorization"},
			forbiddenValues: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "Bearer"},
		},
		{
			name: "secret_and_password",
			fields: map[string]any{
				"secret":   "super_secret_value_789",
				"password": "P@ssw0rd!2024",
				"action":   "authenticate",
			},
			sensitiveKeys:   []string{"secret", "password"},
			forbiddenValues: []string{"super_secret", "P@ssw0rd", "789", "2024"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sanitized := logger.SanitizeLogFields(tc.fields)

			// Verify sensitive keys are redacted
			for _, key := range tc.sensitiveKeys {
				value, exists := sanitized[key]
				if !exists {
					t.Errorf("Expected key %q to exist in sanitized fields", key)
					continue
				}

				valueStr, ok := value.(string)
				if !ok {
					valueStr = ""
				}

				// Verify the redacted value does not contain any forbidden substrings
				for _, forbidden := range tc.forbiddenValues {
					if strings.Contains(valueStr, forbidden) {
						t.Errorf("Sanitized field %q still contains forbidden substring %q: got %q", key, forbidden, valueStr)
					}
				}

				// Verify the value is actually redacted (not original value)
				originalValue := tc.fields[key]
				if valueStr == originalValue {
					t.Errorf("Field %q was not redacted: still equals original value %q", key, originalValue)
				}
			}
		})
	}
}

// TestSanitizeFieldValue_RedactsPaymentFields verifies CHD/SAD field sanitization.
// P0 Invariant: Card numbers must show BIN+last4 only, CVV must be fully redacted.
func TestSanitizeFieldValue_RedactsPaymentFields(t *testing.T) {
	testCases := []struct {
		name            string
		key             string
		value           string
		forbiddenValues []string
		expectedPattern string
	}{
		{
			name:            "card_number_shows_bin_last4",
			key:             "card_number",
			value:           "4111111111111111",
			forbiddenValues: []string{"4111111111111111", "11111111"},
			expectedPattern: "411111****1111", // BIN (6) + mask + last4
		},
		{
			name:            "cvv_fully_redacted",
			key:             "cvv",
			value:           "123",
			forbiddenValues: []string{"123"},
			expectedPattern: "[REDACTED]",
		},
		{
			name:            "security_code_fully_redacted",
			key:             "security_code",
			value:           "456",
			forbiddenValues: []string{"456"},
			expectedPattern: "[REDACTED]",
		},
		{
			name:            "authorization_fully_redacted",
			key:             "authorization",
			value:           "Bearer secret_token_xyz",
			forbiddenValues: []string{"secret_token_xyz", "Bearer"},
			expectedPattern: "[REDACTED]",
		},
		{
			name:            "password_fully_redacted",
			key:             "password",
			value:           "MyP@ssw0rd!",
			forbiddenValues: []string{"MyP@ssw0rd!", "P@ssw0rd"},
			expectedPattern: "[REDACTED]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sanitized := sanitization.SanitizeFieldValue(tc.key, tc.value)
			sanitizedStr, ok := sanitized.(string)
			if !ok {
				t.Fatalf("Expected sanitized value to be a string, got %T", sanitized)
			}

			// Verify forbidden values are not present
			for _, forbidden := range tc.forbiddenValues {
				if strings.Contains(sanitizedStr, forbidden) {
					t.Errorf("Sanitized value for %q still contains forbidden substring %q: got %q", tc.key, forbidden, sanitizedStr)
				}
			}

			// Verify the value was actually changed (not the original)
			if sanitizedStr == tc.value {
				t.Errorf("Field %q was not sanitized: still equals original value %q", tc.key, tc.value)
			}

			// For specific expected patterns, verify they match
			if tc.expectedPattern != "" {
				if tc.key == "card_number" {
					// For card numbers, verify BIN+mask+last4 pattern
					if !strings.HasPrefix(sanitizedStr, "411111") {
						t.Errorf("Card number should preserve BIN (first 6): got %q", sanitizedStr)
					}
					if !strings.HasSuffix(sanitizedStr, "1111") {
						t.Errorf("Card number should preserve last 4: got %q", sanitizedStr)
					}
					if !strings.Contains(sanitizedStr, "*") {
						t.Errorf("Card number should contain masking: got %q", sanitizedStr)
					}
				} else {
					// For redacted fields, verify exact match
					if sanitizedStr != tc.expectedPattern {
						t.Errorf("Expected %q to be redacted as %q, got %q", tc.key, tc.expectedPattern, sanitizedStr)
					}
				}
			}
		})
	}
}

// TestSanitizeLogString_StripsNewlines verifies log forging protection.
// P0 Invariant: User input must not be able to inject newlines to forge log entries.
func TestSanitizeLogString_StripsNewlines(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips_newline",
			input:    "hello\nworld",
			expected: "helloworld",
		},
		{
			name:     "strips_carriage_return",
			input:    "hello\rworld",
			expected: "helloworld",
		},
		{
			name:     "strips_multiple_newlines",
			input:    "line1\nline2\nline3",
			expected: "line1line2line3",
		},
		{
			name:     "strips_mixed_control_chars",
			input:    "hello\r\nworld\n\rtest",
			expected: "helloworldtest",
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "",
		},
		{
			name:     "no_control_chars",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "log_injection_attempt",
			input:    "user@example.com\n[ERROR] Fake log entry",
			expected: "user@example.com[ERROR] Fake log entry",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := logger.SanitizeLogString(tc.input)

			// Verify expected output
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}

			// Critical: Verify NO newlines or carriage returns remain
			if strings.Contains(result, "\n") {
				t.Errorf("Result still contains newline character: %q", result)
			}
			if strings.Contains(result, "\r") {
				t.Errorf("Result still contains carriage return character: %q", result)
			}
		})
	}
}

// TestSanitizeHeaders verifies header sanitization redacts sensitive headers.
// P0 Invariant: Authorization, Cookie, and API key headers must be redacted.
func TestSanitizeHeaders(t *testing.T) {
	testCases := []struct {
		name            string
		headers         map[string][]string
		sensitiveKeys   []string
		forbiddenValues []string
	}{
		{
			name: "authorization_header",
			headers: map[string][]string{
				"Authorization": {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
				"Content-Type":  {"application/json"},
			},
			sensitiveKeys:   []string{"Authorization"},
			forbiddenValues: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "Bearer"},
		},
		{
			name: "cookie_header",
			headers: map[string][]string{
				"Cookie":     {"session=abc123; token=xyz789"},
				"User-Agent": {"Mozilla/5.0"},
			},
			sensitiveKeys:   []string{"Cookie"},
			forbiddenValues: []string{"abc123", "xyz789", "session"},
		},
		{
			name: "x_api_key_header",
			headers: map[string][]string{
				"X-Api-Key": {"secret_api_key_12345"},
				"Accept":    {"*/*"},
			},
			sensitiveKeys:   []string{"X-Api-Key"},
			forbiddenValues: []string{"secret_api_key_12345", "secret"},
		},
		{
			name: "multiple_sensitive_headers",
			headers: map[string][]string{
				"Authorization": {"Bearer token123"},
				"X-Auth-Token":  {"auth_token_456"},
				"X-CSRF-Token":  {"csrf789"},
				"Content-Type":  {"text/plain"},
			},
			sensitiveKeys:   []string{"Authorization", "X-Auth-Token", "X-CSRF-Token"},
			forbiddenValues: []string{"token123", "auth_token_456", "csrf789"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sanitized := sanitization.SanitizeHeaders(tc.headers)

			// Verify sensitive headers are redacted
			for _, key := range tc.sensitiveKeys {
				value, exists := sanitized[key]
				if !exists {
					t.Errorf("Expected key %q to exist in sanitized headers", key)
					continue
				}

				// Verify the redacted value does not contain forbidden substrings
				for _, forbidden := range tc.forbiddenValues {
					if strings.Contains(value, forbidden) {
						t.Errorf("Sanitized header %q still contains forbidden substring %q: got %q", key, forbidden, value)
					}
				}

				// Verify it was actually redacted
				originalValue := tc.headers[key][0]
				if value == originalValue {
					t.Errorf("Header %q was not redacted: still equals original value %q", key, originalValue)
				}
			}
		})
	}
}

// TestSanitizeQueryParams verifies query parameter sanitization.
// P0 Invariant: Token, password, and secret query params must be redacted.
func TestSanitizeQueryParams(t *testing.T) {
	testCases := []struct {
		name            string
		params          map[string][]string
		sensitiveKeys   []string
		forbiddenValues []string
	}{
		{
			name: "token_param",
			params: map[string][]string{
				"token": {"secret_token_abc123"},
				"page":  {"1"},
			},
			sensitiveKeys:   []string{"token"},
			forbiddenValues: []string{"secret_token_abc123", "abc123"},
		},
		{
			name: "api_key_param",
			params: map[string][]string{
				"api_key": {"live_key_xyz789"},
				"limit":   {"10"},
			},
			sensitiveKeys:   []string{"api_key"},
			forbiddenValues: []string{"live_key_xyz789", "xyz789"},
		},
		{
			name: "password_param",
			params: map[string][]string{
				"password": {"MyPassword123!"},
				"username": {"user@example.com"},
			},
			sensitiveKeys:   []string{"password"},
			forbiddenValues: []string{"MyPassword123!", "Password123"},
		},
		{
			name: "multiple_sensitive_params",
			params: map[string][]string{
				"token":    {"tok_123"},
				"secret":   {"sec_456"},
				"password": {"pass_789"},
				"user_id":  {"user-001"},
			},
			sensitiveKeys:   []string{"token", "secret", "password"},
			forbiddenValues: []string{"tok_123", "sec_456", "pass_789"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sanitized := sanitization.SanitizeQueryParams(tc.params)

			// Verify sensitive params are sanitized
			for _, key := range tc.sensitiveKeys {
				value, exists := sanitized[key]
				if !exists {
					t.Errorf("Expected key %q to exist in sanitized params", key)
					continue
				}

				// Verify the sanitized value does not contain forbidden substrings
				for _, forbidden := range tc.forbiddenValues {
					if strings.Contains(value, forbidden) {
						t.Errorf("Sanitized param %q still contains forbidden substring %q: got %q", key, forbidden, value)
					}
				}

				// Verify it was actually sanitized
				originalValue := tc.params[key][0]
				if value == originalValue {
					t.Errorf("Param %q was not sanitized: still equals original value %q", key, originalValue)
				}
			}
		})
	}
}

// TestCriticalFieldsNeverLeakSecrets is a comprehensive safety net test.
// P0 Invariant: Ensure common secret fields never leak actual secret values.
func TestCriticalFieldsNeverLeakSecrets(t *testing.T) {
	criticalTests := []struct {
		fieldName string
		testValue string
	}{
		{"password", "SuperSecretPassword123!"},
		{"secret", "my_secret_key_value"},
		{"private_key", "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg..."},
		{"api_token", "sk_live_1234567890abcdef"},
		{"authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature"},
		{"cvv", "999"},
		{"security_code", "777"},
	}

	for _, test := range criticalTests {
		t.Run("field_"+test.fieldName, func(t *testing.T) {
			sanitized := sanitization.SanitizeFieldValue(test.fieldName, test.testValue)
			sanitizedStr, ok := sanitized.(string)
			if !ok {
				// If not a string, that's acceptable (could be redacted to a type)
				return
			}

			// Critical safety check: sanitized value must NOT contain the original secret
			if strings.Contains(sanitizedStr, test.testValue) {
				t.Errorf("CRITICAL: Field %q leaked secret value! Sanitized: %q", test.fieldName, sanitizedStr)
			}

			// Additional check: ensure some form of redaction happened
			if sanitizedStr == test.testValue {
				t.Errorf("CRITICAL: Field %q was not sanitized at all! Value unchanged: %q", test.fieldName, test.testValue)
			}
		})
	}
}
