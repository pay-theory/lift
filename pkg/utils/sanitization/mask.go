package sanitization

import "strings"

const (
	emptyMaskedValue = "(empty)"
	maskedValue      = "***masked***"
)

// MaskFirstLast keeps the first prefixLen and last suffixLen characters and masks the middle.
func MaskFirstLast(value string, prefixLen, suffixLen int) string {
	if value == "" {
		return emptyMaskedValue
	}
	if prefixLen < 0 || suffixLen < 0 {
		return maskedValue
	}
	if len(value) <= prefixLen+suffixLen {
		return maskedValue
	}
	return value[:prefixLen] + "***" + value[len(value)-suffixLen:]
}

// MaskFirstLast4 keeps the first and last 4 characters and masks the middle.
func MaskFirstLast4(value string) string {
	return MaskFirstLast(value, 4, 4)
}

// MaskBINLast4 keeps the first 6 digits (BIN) and last 4 digits of a card number.
func MaskBINLast4(value string) string {
	if value == "" {
		return emptyMaskedValue
	}
	cleaned := strings.ReplaceAll(strings.ReplaceAll(value, " ", ""), "-", "")
	if cleaned == "" {
		return maskedValue
	}
	if !isNumeric(cleaned) || len(cleaned) <= 10 {
		return maskedValue
	}
	masked := strings.Repeat("*", len(cleaned)-10)
	return cleaned[:6] + masked + cleaned[len(cleaned)-4:]
}
