package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetLiftLogger_NilDefaultsToNoop(t *testing.T) {
	original := GetLiftLogger()
	t.Cleanup(func() { SetLiftLogger(original) })

	SetLiftLogger(nil)
	require.NotNil(t, GetLiftLogger())
}

func TestSanitizeLogFields(t *testing.T) {
	fields := map[string]any{
		"plain":       "ok",
		"crlf":        "a\r\nb",
		"count":       42,
		"card_number": "4111111111111111",
		"nested": map[string]any{
			"inner": "x\ny",
		},
		"list": []any{"c\r\nd", 7},
	}

	sanitized := SanitizeLogFields(fields)

	require.Equal(t, "ok", sanitized["plain"])
	require.Equal(t, "ab", sanitized["crlf"])
	require.Equal(t, "42", sanitized["count"])
	require.Equal(t, "411111******1111", sanitized["card_number"])
	require.Equal(t, "xy", sanitized["nested"].(map[string]any)["inner"])
	require.Equal(t, "cd", sanitized["list"].([]any)[0])
	require.Equal(t, "7", sanitized["list"].([]any)[1])
}
