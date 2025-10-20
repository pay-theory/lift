package sanitization

import (
	"encoding/json"
	"fmt"
)

// SanitizeJSON recursively sanitizes JSON data for logging
// Returns a formatted JSON string with sensitive data masked using Lift's field sanitization
//
// Example:
//
//	reqJSON, _ := json.Marshal(paymentRequest)
//	logger.Info("Processing payment", map[string]any{
//	    "request_json": sanitization.SanitizeJSON(reqJSON),
//	})
//
// Output will mask sensitive fields like card_number, cvv, ssn, etc. based on
// Lift's DataClassification system while preserving the JSON structure.
func SanitizeJSON(jsonBytes []byte) string {
	if len(jsonBytes) == 0 {
		return "(empty)"
	}

	// Parse JSON into generic interface
	var data any
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Sprintf("(malformed JSON: %s)", err.Error())
	}

	// Recursively sanitize using existing SanitizeFieldValue
	sanitized := sanitizeJSONValue(data)

	// Marshal back to pretty JSON for readability in logs
	result, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return "(error marshaling sanitized JSON)"
	}

	return string(result)
}

// sanitizeJSONValue recursively processes any JSON value
func sanitizeJSONValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return sanitizeJSONObject(v)
	case []any:
		return sanitizeJSONArray(v)
	default:
		return value
	}
}

// sanitizeJSONObject processes a JSON object recursively
func sanitizeJSONObject(obj map[string]any) map[string]any {
	result := make(map[string]any)

	for key, value := range obj {
		// Special handling for "body" field which may contain JSON as a string
		if key == "body" {
			if bodyStr, ok := value.(string); ok {
				// Try to parse the body as JSON
				var bodyData any
				if err := json.Unmarshal([]byte(bodyStr), &bodyData); err == nil {
					// Successfully parsed - sanitize and re-serialize
					sanitizedBody := sanitizeJSONValue(bodyData)
					if bodyJSON, err := json.Marshal(sanitizedBody); err == nil {
						result[key] = string(bodyJSON)
						continue
					}
				}
				// If parsing failed or re-serialization failed, fall through to normal sanitization
			}
		}

		// Sanitize the value using existing field sanitization
		sanitizedValue := SanitizeFieldValue(key, value)

		// If the value is still a complex type after sanitization, recurse into it
		// This handles nested objects and arrays that weren't redacted
		switch sv := sanitizedValue.(type) {
		case map[string]any:
			result[key] = sanitizeJSONObject(sv)
		case []any:
			result[key] = sanitizeJSONArray(sv)
		default:
			result[key] = sanitizedValue
		}
	}

	return result
}

// sanitizeJSONArray processes a JSON array recursively
func sanitizeJSONArray(arr []any) []any {
	result := make([]any, len(arr))

	for i, value := range arr {
		result[i] = sanitizeJSONValue(value)
	}

	return result
}
