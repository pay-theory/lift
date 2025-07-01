// Package sanitization provides centralized data sanitization utilities
// to prevent sensitive data exposure across the Lift framework.
package sanitization

import (
	"fmt"
	"strings"

	"github.com/pay-theory/lift/pkg/security"
)

// AllowedFields are field names that should not be sanitized
var AllowedFields = map[string]bool{
	"card_bin":   true,
	"card_brand": true,
	"card_type":  true,
}

// Sanitizer provides methods for sanitizing various types of data
type Sanitizer struct {
	dataProtectionManager *security.DataProtectionManager
}

// New creates a new Sanitizer with a data protection manager
func New(dpm *security.DataProtectionManager) *Sanitizer {
	return &Sanitizer{dataProtectionManager: dpm}
}

// Default returns a sanitizer with default data protection configuration
func Default() *Sanitizer {
	// Create a default data protection config for sanitization
	config := security.DataProtectionConfig{
		DefaultClassification: security.DataPublic,
		FieldClassifications:  make(map[string]security.DataClassification),
		EncryptionKey:         "default-sanitization-key", // This is only for classification, not actual encryption
	}

	dpm, err := security.NewDataProtectionManager(config)
	if err != nil {
		// This should not happen with basic config
		panic(fmt.Sprintf("Failed to create default data protection manager: %v", err))
	}

	return &Sanitizer{dataProtectionManager: dpm}
}

// SanitizeFieldValue sanitizes a field value based on its key name and data classification
func (s *Sanitizer) SanitizeFieldValue(key string, value any) any {
	keyLower := strings.ToLower(key)

	// Check if field is explicitly allowed
	if AllowedFields[keyLower] {
		return value
	}

	// Use the data protection manager to classify the field
	dataCtx := s.dataProtectionManager.ClassifyData(
		map[string]any{key: value},
		map[string]any{"source": "sanitizer"},
	)

	// Get the classification for this specific field
	var classification security.DataClassification
	if fieldClass, exists := dataCtx.Fields[key]; exists {
		classification = fieldClass
	} else {
		// Use overall classification if field-specific not found
		classification = dataCtx.Classification
	}

	// Apply sanitization based on classification
	switch classification {
	case security.DataRestricted:
		// For restricted data, show last 4 if it's a number field, otherwise redact
		if str, ok := value.(string); ok {
			// Clean the string to check if it's a number
			cleaned := strings.ReplaceAll(strings.ReplaceAll(str, " ", ""), "-", "")
			if len(cleaned) >= 4 && isNumeric(cleaned) {
				// Show last 4 digits, mask the rest
				masked := strings.Repeat("*", len(cleaned)-4) + cleaned[len(cleaned)-4:]
				return masked
			}
		}
		return "[REDACTED]"

	case security.DataConfidential:
		// For confidential data, redact completely
		return "[REDACTED]"

	case security.DataInternal:
		// For internal data (like user content), show metadata only
		if str, ok := value.(string); ok {
			// User-generated content fields show length
			if isUserContentField(keyLower) {
				if len(str) > 0 {
					return fmt.Sprintf("[USER_CONTENT_%d_CHARS]", len(str))
				}
				return "[USER_CONTENT]"
			}

			// Error messages might contain sensitive info
			if keyLower == "error" || strings.Contains(keyLower, "error") {
				if len(str) > 50 ||
					strings.Contains(strings.ToLower(str), "input") ||
					strings.Contains(strings.ToLower(str), "invalid") {
					return "[SANITIZED_ERROR]"
				}
			}

			// For other internal data, check if it's large
			if len(str) > 200 {
				return fmt.Sprintf("[LARGE_STRING_%d_CHARS]", len(str))
			}
		}
		// For small internal data or non-strings, return as-is
		return value

	case security.DataPublic:
		// Public data doesn't need sanitization
		return value

	default:
		// Unknown classification - be safe and redact
		return "[REDACTED]"
	}
}

// isUserContentField checks if a field contains user-generated content
func isUserContentField(fieldLower string) bool {
	userContentFields := []string{
		"body", "request_body", "response_body", "user_input",
		"query", "search", "message", "comment", "description",
	}
	for _, field := range userContentFields {
		if strings.Contains(fieldLower, field) {
			return true
		}
	}
	return false
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// SanitizeHeaders removes sensitive headers from a map
func (s *Sanitizer) SanitizeHeaders(headers map[string][]string) map[string]string {
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"cookie":        true,
		"x-api-key":     true,
		"x-auth-token":  true,
		"x-csrf-token":  true,
		"x-session-id":  true,
	}

	result := make(map[string]string)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			result[key] = "[REDACTED]"
		} else if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// SanitizeQueryParams sanitizes query parameters
func (s *Sanitizer) SanitizeQueryParams(params map[string][]string) map[string]string {
	sensitiveParams := map[string]bool{
		"token":    true,
		"api_key":  true,
		"apikey":   true,
		"password": true,
		"secret":   true,
		"auth":     true,
		"session":  true,
		"key":      true,
	}

	result := make(map[string]string)
	for key, values := range params {
		lowerKey := strings.ToLower(key)
		if sensitiveParams[lowerKey] {
			result[key] = "[SANITIZED_QUERY_PARAMS]"
		} else if len(values) > 0 {
			// Also apply field value sanitization to query param values
			sanitizedValue := s.SanitizeFieldValue(key, values[0])
			result[key] = fmt.Sprintf("%v", sanitizedValue)
		}
	}
	return result
}

// SanitizeMap applies sanitization to all values in a map
func (s *Sanitizer) SanitizeMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	for key, value := range data {
		result[key] = s.SanitizeFieldValue(key, value)
	}
	return result
}

// Global default sanitizer for convenience
var defaultSanitizer = Default()

// SanitizeFieldValue uses the default sanitizer
func SanitizeFieldValue(key string, value any) any {
	return defaultSanitizer.SanitizeFieldValue(key, value)
}

// SanitizeHeaders uses the default sanitizer
func SanitizeHeaders(headers map[string][]string) map[string]string {
	return defaultSanitizer.SanitizeHeaders(headers)
}

// SanitizeQueryParams uses the default sanitizer
func SanitizeQueryParams(params map[string][]string) map[string]string {
	return defaultSanitizer.SanitizeQueryParams(params)
}

// SanitizeMap uses the default sanitizer
func SanitizeMap(data map[string]any) map[string]any {
	return defaultSanitizer.SanitizeMap(data)
}
