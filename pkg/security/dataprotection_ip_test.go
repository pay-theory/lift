package security

import (
	"testing"
)

func TestIPAddressNotRedacted(t *testing.T) {
	// Create a data protection manager with default config
	config := DataProtectionConfig{
		DefaultClassification: DataInternal,
		FieldClassifications:  make(map[string]DataClassification),
		EncryptionKey:         "test-key-for-encryption",
		MaskingRules:          make(map[string]MaskingRule),
	}

	manager, err := NewDataProtectionManager(config)
	if err != nil {
		t.Fatalf("Failed to create data protection manager: %v", err)
	}

	// Test various IP address field names
	ipFields := []string{
		"ip",
		"ip_address",
		"ipaddress",
		"client_ip",
		"source_ip",
		"remote_ip",
		"x_forwarded_for",
		"x_real_ip",
		"cf_connecting_ip",
		"sourceIP",
		"clientIP",
		"remoteIP",
		"server_ip",
		"user_ip",
		"host_ip",
	}

	for _, field := range ipFields {
		t.Run(field, func(t *testing.T) {
			// Test that IP address fields are classified as public
			classification := manager.classifyField(field, "192.168.1.1")
			if classification != DataPublic {
				t.Errorf("Field %s should be classified as DataPublic, got %s", field, classification)
			}

			// Test that IP values are not masked
			maskedValue := manager.applyDefaultMasking("192.168.1.1", DataPublic)
			if maskedValue != "192.168.1.1" {
				t.Errorf("IP address should not be masked, got %v", maskedValue)
			}
		})
	}

	// Test that IP addresses in structured data are not redacted
	t.Run("StructuredData", func(t *testing.T) {
		testData := map[string]any{
			"user_id":     "12345",
			"email":       "user@example.com",
			"source_ip":   "192.168.1.100",
			"client_ip":   "10.0.0.1",
			"card_number": "4111111111111111",
			"name":        "John Doe",
		}

		dataCtx := manager.ClassifyData(testData, map[string]any{})

		// Check classifications
		expectedClassifications := map[string]DataClassification{
			"user_id":     DataInternal, // Should not match IP patterns
			"email":       DataInternal,
			"source_ip":   DataPublic,
			"client_ip":   DataPublic,
			"card_number": DataRestricted,
			"name":        DataInternal,
		}

		for field, expected := range expectedClassifications {
			if dataCtx.Fields[field] != expected {
				t.Errorf("Field %s: expected %s, got %s", field, expected, dataCtx.Fields[field])
			}
		}

		// Test masking
		maskedData, err := manager.maskData(testData, dataCtx.Fields)
		if err != nil {
			t.Fatalf("Failed to mask data: %v", err)
		}
		maskedMap, ok := maskedData.(map[string]any)
		if !ok {
			t.Fatalf("Failed to assert maskedData as map[string]any")
		}

		// IP addresses should not be masked
		if maskedMap["source_ip"] != "192.168.1.100" {
			t.Errorf("source_ip should not be masked, got %v", maskedMap["source_ip"])
		}
		if maskedMap["client_ip"] != "10.0.0.1" {
			t.Errorf("client_ip should not be masked, got %v", maskedMap["client_ip"])
		}

		// Card number should be fully masked
		if maskedStr, ok := maskedMap["card_number"].(string); !ok || maskedStr != "****************" {
			t.Errorf("card_number should be fully masked, got %v", maskedMap["card_number"])
		}

		// Name should show metadata only
		if nameStr, ok := maskedMap["name"].(string); !ok || nameStr != "[INTERNAL_8_chars]" {
			t.Errorf("name should show metadata only, got %v", maskedMap["name"])
		}
	})

	// Test edge cases
	t.Run("EdgeCases", func(t *testing.T) {
		// Test field names that contain "ip" but aren't IP addresses
		nonIPFields := map[string]DataClassification{
			"description":    DataInternal, // Contains "ip" but not an IP field
			"recipient":      DataInternal, // Contains "ip" but not an IP field
			"participation":  DataInternal, // Contains "ip" but not an IP field
		}

		for field, expected := range nonIPFields {
			classification := manager.classifyField(field, "some value")
			if classification != expected {
				t.Errorf("Field %s should be classified as %s, got %s", field, expected, classification)
			}
		}

		// Test actual IP patterns are still public
		ipPatternFields := map[string]string{
			"X-Forwarded-For": "203.0.113.1",
			"X-Real-IP":       "198.51.100.1",
			"CF-Connecting-IP": "192.0.2.1",
		}

		for field, value := range ipPatternFields {
			classification := manager.classifyField(field, value)
			if classification != DataPublic {
				t.Errorf("Field %s should be classified as DataPublic, got %s", field, classification)
			}
		}
	})
}

// TestDataClassificationPriority verifies that IP classification takes precedence
func TestDataClassificationPriority(t *testing.T) {
	config := DataProtectionConfig{
		DefaultClassification: DataInternal,
		FieldClassifications:  make(map[string]DataClassification),
		EncryptionKey:         "test-key",
		MaskingRules:          make(map[string]MaskingRule),
	}

	manager, err := NewDataProtectionManager(config)
	if err != nil {
		t.Fatalf("Failed to create data protection manager: %v", err)
	}

	// Test that even if a field contains other patterns, IP pattern takes precedence
	testCases := []struct {
		field    string
		expected DataClassification
	}{
		{"client_ip_address", DataPublic},    // Contains both "client" and "ip"
		{"source_ip_token", DataPublic},      // Contains both "token" and "ip"
		{"api_key", DataRestricted},          // Contains "api" and "key" but not IP pattern
		{"stripe_customer_id", DataInternal}, // Contains "stripe" but not a sensitive pattern
	}

	for _, tc := range testCases {
		t.Run(tc.field, func(t *testing.T) {
			classification := manager.classifyField(tc.field, "test-value")
			if classification != tc.expected {
				t.Errorf("Field %s: expected %s, got %s", tc.field, tc.expected, classification)
			}
		})
	}
}

// TestMaskingBehavior verifies the masking behavior for each classification
func TestMaskingBehavior(t *testing.T) {
	config := DataProtectionConfig{
		DefaultClassification: DataInternal,
		FieldClassifications:  make(map[string]DataClassification),
		EncryptionKey:         "test-key",
		MaskingRules:          make(map[string]MaskingRule),
	}

	manager, err := NewDataProtectionManager(config)
	if err != nil {
		t.Fatalf("Failed to create data protection manager: %v", err)
	}

	testCases := []struct {
		name           string
		value          string
		classification DataClassification
		expected       string
	}{
		{"Public IP", "192.168.1.1", DataPublic, "192.168.1.1"},
		{"Internal Data", "John Doe", DataInternal, "[INTERNAL_8_chars]"},
		{"Confidential Short", "ABC", DataConfidential, "***"},
		{"Confidential Long", "1234567890", DataConfidential, "12******90"},
		{"Restricted", "secret-key", DataRestricted, "**********"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			masked := manager.applyDefaultMasking(tc.value, tc.classification)
			if masked != tc.expected {
				t.Errorf("Expected %s, got %v", tc.expected, masked)
			}
		})
	}
}