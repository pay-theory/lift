package sanitization

import (
	"strings"
	"testing"
)

func TestSanitizeXML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		contains    []string // Strings that should appear in output
		notContains []string // Strings that should NOT appear in output
	}{
		{
			name: "Card number with last 4 digits",
			input: `<xml>
				<AcctNum>4111111111111111</AcctNum>
				<Amount>1000</Amount>
			</xml>`,
			contains: []string{
				"1111",   // Last 4 digits preserved
				"Amount", // Non-sensitive field preserved
				"1000",
			},
			notContains: []string{
				"4111111111111111", // Full card number should be masked
			},
		},
		{
			name: "CVV data masked completely",
			input: `<Transaction>
				<CCVData>123</CCVData>
				<CVV>456</CVV>
				<SecurityCode>789</SecurityCode>
			</Transaction>`,
			notContains: []string{
				"123", // CVV should be redacted
				"456",
				"789",
			},
			contains: []string{
				"***", // CVV replacement
			},
		},
		{
			name: "Card expiry date masked",
			input: `<Payment>
				<CardExpiryDate>1225</CardExpiryDate>
				<Amount>5000</Amount>
			</Payment>`,
			notContains: []string{
				"1225", // Expiry date should be masked
			},
			contains: []string{
				"****", // Expiry replacement
				"5000", // Amount preserved
			},
		},
		{
			name:  "HTML-escaped XML",
			input: `Response: &lt;AcctNum&gt;4005562231212149&lt;/AcctNum&gt; and &lt;CVV&gt;999&lt;/CVV&gt;`,
			contains: []string{
				"2149", // Last 4 digits preserved
			},
			notContains: []string{
				"4005562231212149", // Full card number masked
				"999",              // CVV masked
			},
		},
		{
			name: "SSN and Tax ID masked",
			input: `<Payor>
				<SSN>123-45-6789</SSN>
				<TaxID>98-7654321</TaxID>
				<TaxId>11-1111111</TaxId>
			</Payor>`,
			notContains: []string{
				"123-45-6789",
				"98-7654321",
				"11-1111111",
			},
			contains: []string{
				"****", // SSN/TaxID replacement
			},
		},
		{
			name: "ACH account and routing numbers",
			input: `<ACH>
				<AccountNumber>123456789</AccountNumber>
				<RoutingNumber>021000021</RoutingNumber>
			</ACH>`,
			contains: []string{
				"6789", // Last 4 of account
				"0021", // Last 4 of routing
			},
			notContains: []string{
				"123456789",
				"021000021",
			},
		},
		{
			name: "TransArmorToken with last 4",
			input: `<Token>
				<TransArmorToken>abc123def456ghi789</TransArmorToken>
			</Token>`,
			contains: []string{
				"i789", // Last 4 of token
			},
			notContains: []string{
				"abc123def456ghi789", // Full token masked
			},
		},
		{
			name: "Multiple sensitive fields in complex XML",
			input: `<RapidConnectRequest>
				<Payment>
					<AcctNum>4111111111111111</AcctNum>
					<CardExpiryDate>1225</CardExpiryDate>
					<CCVData>123</CCVData>
					<Amount>5000</Amount>
				</Payment>
				<Customer>
					<SSN>123-45-6789</SSN>
					<Email>test@example.com</Email>
				</Customer>
			</RapidConnectRequest>`,
			contains: []string{
				"1111",  // Card last 4
				"5000",  // Amount preserved
				"Email", // Non-sensitive field name
				"test@example.com",
			},
			notContains: []string{
				"4111111111111111", // Card masked
				"1225",             // Expiry masked
				"123",              // CVV masked (in CCVData)
				"123-45-6789",      // SSN masked
			},
		},
		{
			name: "Empty XML tags",
			input: `<Transaction>
				<AcctNum></AcctNum>
				<Amount>1000</Amount>
			</Transaction>`,
			contains: []string{
				"1000",
				"Amount",
			},
		},
		{
			name: "PIN masked completely",
			input: `<DebitTransaction>
				<PIN>1234</PIN>
				<Amount>500</Amount>
			</DebitTransaction>`,
			notContains: []string{
				"1234", // PIN should never appear
			},
			contains: []string{
				"****", // PIN replacement
				"500",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeXML(tt.input, PaymentXMLPatterns)

			// Check for expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("SanitizeXML() result missing expected string %q\nInput: %s\nResult: %s", expected, tt.input, result)
				}
			}

			// Check that sensitive strings are NOT present
			for _, notExpected := range tt.notContains {
				if strings.Contains(result, notExpected) {
					t.Errorf("SanitizeXML() result contains sensitive string %q\nInput: %s\nResult: %s", notExpected, tt.input, result)
				}
			}
		})
	}
}

func TestMaskCardNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "16-digit card number",
			input:    "<AcctNum>4111111111111111</AcctNum>",
			expected: "<AcctNum>************1111</AcctNum>",
		},
		{
			name:     "15-digit card number (Amex)",
			input:    "<AcctNum>378282246310005</AcctNum>",
			expected: "<AcctNum>***********0005</AcctNum>",
		},
		{
			name:     "HTML-escaped",
			input:    "&lt;AcctNum&gt;4005562231212149&lt;/AcctNum&gt;",
			expected: "&lt;AcctNum&gt;************2149&lt;/AcctNum&gt;",
		},
		{
			name:     "Short number (4 digits or less)",
			input:    "<AcctNum>1234</AcctNum>",
			expected: "<AcctNum>1234</AcctNum>", // Not masked if <= 4 digits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskCardNumber(tt.input)
			if result != tt.expected {
				t.Errorf("MaskCardNumber() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMaskCompletelyFunc(t *testing.T) {
	maskFunc := MaskCompletelyFunc("***")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "CVV data",
			input:    "<CCVData>123</CCVData>",
			expected: "<CCVData>***</CCVData>",
		},
		{
			name:     "Security code",
			input:    "<SecurityCode>456</SecurityCode>",
			expected: "<SecurityCode>***</SecurityCode>",
		},
		{
			name:     "HTML-escaped CVV",
			input:    "&lt;CVV&gt;789&lt;/CVV&gt;",
			expected: "&lt;CVV&gt;***&lt;/CVV&gt;",
		},
		{
			name:     "Empty tag",
			input:    "<CVV></CVV>",
			expected: "<CVV>***</CVV>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskFunc(tt.input)
			if result != tt.expected {
				t.Errorf("MaskCompletelyFunc() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMaskTokenLastFour(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "TransArmorToken",
			input:    "<TransArmorToken>abc123def456ghi789</TransArmorToken>",
			expected: "<TransArmorToken>**************i789</TransArmorToken>", // 18 chars - 4 = 14 asterisks
		},
		{
			name:     "Short token (4 chars or less)",
			input:    "<TransArmorToken>abcd</TransArmorToken>",
			expected: "<TransArmorToken>abcd</TransArmorToken>", // Not masked
		},
		{
			name:     "Empty token",
			input:    "<TransArmorToken></TransArmorToken>",
			expected: "<TransArmorToken></TransArmorToken>", // Not masked
		},
		{
			name:     "HTML-escaped token",
			input:    "&lt;TransArmorToken&gt;1234567890abcdef&lt;/TransArmorToken&gt;",
			expected: "&lt;TransArmorToken&gt;************cdef&lt;/TransArmorToken&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskTokenLastFour(tt.input)
			if result != tt.expected {
				t.Errorf("MaskTokenLastFour() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPaymentXMLPatterns(t *testing.T) {
	// Verify that PaymentXMLPatterns includes all expected patterns
	expectedPatterns := []string{
		"AcctNum",
		"AcctNum_Escaped",
		"CardExpiryDate",
		"CardExpiryDate_Escaped",
		"CCVData",
		"CCVData_Escaped",
		"CVV",
		"CVV_Escaped",
		"SecurityCode",
		"SecurityCode_Escaped",
		"TransArmorToken",
		"TransArmorToken_Escaped",
		"SSN",
		"SSN_Escaped",
		"TaxID",
		"TaxID_Escaped",
		"TaxId",
		"TaxId_Escaped",
		"PIN",
		"PIN_Escaped",
		"AccountNumber",
		"AccountNumber_Escaped",
		"RoutingNumber",
		"RoutingNumber_Escaped",
	}

	patternNames := make(map[string]bool)
	for _, pattern := range PaymentXMLPatterns {
		patternNames[pattern.Name] = true
	}

	for _, expectedName := range expectedPatterns {
		if !patternNames[expectedName] {
			t.Errorf("PaymentXMLPatterns missing expected pattern: %s", expectedName)
		}
	}

	if len(PaymentXMLPatterns) != len(expectedPatterns) {
		t.Errorf("PaymentXMLPatterns count = %d, want %d", len(PaymentXMLPatterns), len(expectedPatterns))
	}
}

func TestRapidConnectXMLPatterns(t *testing.T) {
	// Verify that RapidConnectXMLPatterns is an alias of PaymentXMLPatterns
	if len(RapidConnectXMLPatterns) != len(PaymentXMLPatterns) {
		t.Errorf("RapidConnectXMLPatterns length = %d, want %d", len(RapidConnectXMLPatterns), len(PaymentXMLPatterns))
	}
}

func BenchmarkSanitizeXML(b *testing.B) {
	input := `<RapidConnectRequest>
		<Payment>
			<AcctNum>4111111111111111</AcctNum>
			<CardExpiryDate>1225</CardExpiryDate>
			<CCVData>123</CCVData>
			<Amount>5000</Amount>
		</Payment>
		<Customer>
			<SSN>123-45-6789</SSN>
			<TaxID>98-7654321</TaxID>
			<Email>test@example.com</Email>
		</Customer>
		<Transaction>
			<TransArmorToken>abc123def456ghi789</TransArmorToken>
			<PIN>1234</PIN>
		</Transaction>
	</RapidConnectRequest>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeXML(input, PaymentXMLPatterns)
	}
}

func BenchmarkMaskCardNumber(b *testing.B) {
	input := "<AcctNum>4111111111111111</AcctNum>"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MaskCardNumber(input)
	}
}

func BenchmarkMaskCompletely(b *testing.B) {
	input := "<CCVData>123</CCVData>"
	maskFunc := MaskCompletelyFunc("***")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = maskFunc(input)
	}
}
