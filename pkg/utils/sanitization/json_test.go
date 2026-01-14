package sanitization

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		contains    []string // Strings that should appear in output
		notContains []string // Strings that should NOT appear in output
	}{
		{
			name: "Card payment with sensitive data",
			input: `{
				"payment_method": {
					"card_number": "4111111111111111",
					"security_code": "123",
					"exp_date": {"month": "12", "year": "2025"}
				},
				"amount": 1000
			}`,
			contains: []string{
				"amount",
				"1000",
				"exp_date", // Non-sensitive field names preserved
			},
			notContains: []string{
				"4111111111111111", // Full card number should be masked
				"123",              // CVV should be redacted
			},
		},
		{
			name: "Nested objects with multiple sensitive fields",
			input: `{
				"transaction": {
					"payment": {
						"card_number": "4005562231212149",
						"cvv": "456",
						"account_number": "123456789"
					},
					"payor": {
						"ssn": "123-45-6789",
						"email": "test@example.com"
					}
				}
			}`,
			notContains: []string{
				"4005562231212149", // Card number
				"456",              // CVV
				"123456789",        // Account number
				"123-45-6789",      // SSN
			},
		},
		{
			name: "Arrays with sensitive data",
			input: `{
				"payments": [
					{"card_number": "4111111111111111", "cvv": "123"},
					{"card_number": "5555555555554444", "cvv": "456"}
				]
			}`,
			notContains: []string{
				"4111111111111111",
				"5555555555554444",
				"123",
				"456",
			},
		},
		{
			name:     "Empty JSON",
			input:    "",
			contains: []string{"(empty)"},
		},
		{
			name:     "Malformed JSON",
			input:    `{"invalid": json}`,
			contains: []string{"(malformed JSON"},
		},
		{
			name: "Mixed sensitive and non-sensitive",
			input: `{
				"merchant_uid": "merchant_123",
				"transaction_id": "txn_456",
				"card_number": "4111111111111111",
				"amount": 5000,
				"currency": "USD"
			}`,
			contains: []string{
				"merchant_uid",
				"merchant_123",
				"transaction_id",
				"txn_456",
				"amount",
				"5000",
				"currency",
				"USD",
			},
			notContains: []string{
				"4111111111111111", // Only sensitive data masked
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeJSON([]byte(tt.input))

			// Check for expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("SanitizeJSON() result missing expected string %q\nResult: %s", expected, result)
				}
			}

			// Check that sensitive strings are NOT present
			for _, notExpected := range tt.notContains {
				if strings.Contains(result, notExpected) {
					t.Errorf("SanitizeJSON() result contains sensitive string %q\nResult: %s", notExpected, result)
				}
			}
		})
	}
}

func TestSanitizeJSON_ValidJSON(t *testing.T) {
	input := `{
		"card_number": "4111111111111111",
		"amount": 1000
	}`

	result := SanitizeJSON([]byte(input))

	// Result should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("SanitizeJSON() result is not valid JSON: %v\nResult: %s", err, result)
	}

	// Check that amount is preserved (non-sensitive)
	if amount, ok := parsed["amount"].(float64); !ok || amount != 1000 {
		t.Errorf("SanitizeJSON() did not preserve non-sensitive field 'amount'")
	}
}

func TestSanitizeJSON_DeepNesting(t *testing.T) {
	input := `{
		"level1": {
			"level2": {
				"level3": {
					"card_number": "4111111111111111",
					"safe_field": "safe_value"
				}
			}
		}
	}`

	result := SanitizeJSON([]byte(input))

	// Check that deeply nested sensitive data is masked
	if strings.Contains(result, "4111111111111111") {
		t.Errorf("SanitizeJSON() did not mask deeply nested card_number")
	}

	// Check that safe field is preserved
	if !strings.Contains(result, "safe_value") {
		t.Errorf("SanitizeJSON() removed non-sensitive deeply nested field")
	}
}

func TestSanitizeJSON_BodyFieldJSONString(t *testing.T) {
	input := `{
		"body": "{\"card_number\":\"4111111111111111\",\"safe\":\"ok\"}",
		"other": "value"
	}`

	result := SanitizeJSON([]byte(input))

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("SanitizeJSON() result is not valid JSON: %v\nResult: %s", err, result)
	}

	body, ok := parsed["body"].(string)
	if !ok {
		t.Fatalf("expected body to be a string, got %T", parsed["body"])
	}
	if strings.Contains(body, "4111111111111111") {
		t.Fatalf("expected body JSON string to be sanitized, got %s", body)
	}

	var bodyParsed map[string]any
	if err := json.Unmarshal([]byte(body), &bodyParsed); err != nil {
		t.Fatalf("expected body to contain valid JSON: %v\nBody: %s", err, body)
	}
	if bodyParsed["card_number"] != "411111******1111" {
		t.Fatalf("expected masked card_number in body, got %v", bodyParsed["card_number"])
	}
	if bodyParsed["safe"] != "ok" {
		t.Fatalf("expected safe field preserved in body, got %v", bodyParsed["safe"])
	}
}

func BenchmarkSanitizeJSON(b *testing.B) {
	input := []byte(`{
		"transaction": {
			"payment_method": {
				"card_number": "4111111111111111",
				"cvv": "123",
				"exp_date": {"month": "12", "year": "2025"}
			},
			"payor": {
				"email": "test@example.com",
				"phone": "555-1234"
			},
			"amount": 1000,
			"currency": "USD"
		}
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeJSON(input)
	}
}
