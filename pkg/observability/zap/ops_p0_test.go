package zap

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stretchr/testify/require"
)

func TestOps_ZapLogger_SanitizesFieldsRecursively(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	base := zap.New(core)

	logger := &ZapLogger{
		logger:        base,
		sugar:         base.Sugar(),
		stats:         &loggerStats{},
		contextFields: map[string]any{},
	}

	logger.Info("hello", map[string]any{
		"password":      "secret123",
		"card_number":   "4111111111111111",
		"authorization": "Bearer secret-token",
		"raw_bytes":     []byte("a\r\nb"),
		"nested": map[string]any{
			"cvv":  "123",
			"safe": "ok",
		},
		"list": []any{
			map[string]any{"ssn": "123-45-6789"},
			"line1\nline2",
		},
	})

	entries := observed.All()
	require.Len(t, entries, 1)

	fields := entries[0].ContextMap()
	require.Equal(t, "[REDACTED]", fields["password"])
	require.Equal(t, "411111******1111", fields["card_number"])
	require.Equal(t, "[REDACTED]", fields["authorization"])
	require.Equal(t, "ab", fields["raw_bytes"])

	nested, ok := fields["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", nested["cvv"])
	require.Equal(t, "ok", nested["safe"])

	list, ok := fields["list"].([]any)
	require.True(t, ok)
	require.Len(t, list, 2)

	item0, ok := list[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "*****6789", item0["ssn"])
	require.Equal(t, "line1line2", list[1])
}
