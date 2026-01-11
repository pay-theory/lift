package sanitization

import (
	"regexp"
	"strings"
)

// XMLSanitizationPattern defines a regex-based sanitization rule for XML elements
type XMLSanitizationPattern struct {
	Pattern     *regexp.Regexp            // Regex to match XML elements (both regular and HTML-escaped)
	MaskingFunc func(match string) string // Function to mask the matched value
	Name        string                    // Descriptive name for the pattern (e.g., "AcctNum", "CVV")
}

// SanitizeXML sanitizes XML content using configurable patterns
// Supports both regular XML (<AcctNum>...</AcctNum>) and HTML-escaped XML (&lt;AcctNum&gt;...&lt;/AcctNum&gt;)
//
// Example:
//
//	xmlRequest := buildRapidConnectXML(...)
//	logger.Info("Sending request", map[string]any{
//	    "xml_request": sanitization.SanitizeXML(xmlRequest, sanitization.PaymentXMLPatterns),
//	})
//
// The function applies each pattern in sequence to the XML string,
// allowing for comprehensive masking of sensitive data.
func SanitizeXML(xmlString string, patterns []XMLSanitizationPattern) string {
	result := xmlString

	for _, pattern := range patterns {
		result = pattern.Pattern.ReplaceAllStringFunc(result, pattern.MaskingFunc)
	}

	return result
}

// MaskCardNumber shows BIN + last 4 digits of card numbers (PCI DSS compliant)
// Handles both <AcctNum>1234567890123456</AcctNum> and HTML-escaped variants
func MaskCardNumber(match string) string {
	// Determine if this is HTML-escaped XML
	isEscaped := strings.Contains(match, "&gt;")

	var start, end int
	if isEscaped {
		// For &lt;AcctNum&gt;1234567890123456&lt;/AcctNum&gt;
		start = strings.Index(match, "&gt;") + 4 // Length of "&gt;"
		end = strings.LastIndex(match, "&lt;")
	} else {
		// For <AcctNum>1234567890123456</AcctNum>
		start = strings.Index(match, ">") + 1
		end = strings.LastIndex(match, "<")
	}

	if end > start {
		number := match[start:end]
		if len(number) > 10 {
			// Show BIN + last 4 digits for PCI compliance
			masked := number[:6] + strings.Repeat("*", len(number)-10) + number[len(number)-4:]
			return match[:start] + masked + match[end:]
		}
		if len(number) > 4 {
			masked := strings.Repeat("*", len(number)-4) + number[len(number)-4:]
			return match[:start] + masked + match[end:]
		}
	}

	return match
}

// MaskCompletelyFunc returns a function that masks a field completely with a replacement string
// Used for highly sensitive fields like CVV, SSN, expiry dates
func MaskCompletelyFunc(replacement string) func(string) string {
	return func(match string) string {
		// Determine if this is HTML-escaped XML
		isEscaped := strings.Contains(match, "&gt;")

		var start, end int
		if isEscaped {
			// For &lt;CVV&gt;123&lt;/CVV&gt;
			start = strings.Index(match, "&gt;") + 4 // Length of "&gt;"
			end = strings.LastIndex(match, "&lt;")
		} else {
			// For <CVV>123</CVV>
			start = strings.Index(match, ">") + 1
			end = strings.LastIndex(match, "<")
		}

		if end >= start { // >= to handle empty tags
			return match[:start] + replacement + match[end:]
		}

		return match
	}
}

// MaskTokenLastFour shows only last 4 characters of tokens
// Used for TransArmorToken and similar processor tokens
func MaskTokenLastFour(match string) string {
	// Determine if this is HTML-escaped XML
	isEscaped := strings.Contains(match, "&gt;")

	// Handle empty tokens
	if strings.Contains(match, "><") || strings.Contains(match, "&gt;&lt;") {
		return match
	}

	var start, end int
	if isEscaped {
		// For &lt;TransArmorToken&gt;abc123def456ghi789&lt;/TransArmorToken&gt;
		start = strings.Index(match, "&gt;") + 4 // Length of "&gt;"
		end = strings.LastIndex(match, "&lt;")
	} else {
		// For <TransArmorToken>abc123def456ghi789</TransArmorToken>
		start = strings.Index(match, ">") + 1
		end = strings.LastIndex(match, "<")
	}

	if end > start {
		token := match[start:end]
		if len(token) > 4 {
			masked := strings.Repeat("*", len(token)-4) + token[len(token)-4:]
			return match[:start] + masked + match[end:]
		}
	}

	return match
}
