package logger

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pay-theory/lift/pkg/observability"
	liftzap "github.com/pay-theory/lift/pkg/observability/zap"
	"github.com/pay-theory/lift/pkg/utils/sanitization"
)

var (
	// LiftLogger is the global logger instance used by applications that want a singleton.
	// It defaults to a no-op logger until SetLiftLogger is called.
	LiftLogger   observability.StructuredLogger
	liftLoggerMu sync.RWMutex
)

func init() {
	// Default to a no-op logger so package-level logging is always safe.
	LiftLogger = liftzap.NewZapLoggerFactory().CreateNoOpLogger()
}

// SetLiftLogger sets the global logger instance.
func SetLiftLogger(logger observability.StructuredLogger) {
	liftLoggerMu.Lock()
	defer liftLoggerMu.Unlock()
	if logger == nil {
		LiftLogger = liftzap.NewZapLoggerFactory().CreateNoOpLogger()
		return
	}
	LiftLogger = logger
}

// GetLiftLogger returns the global logger instance.
func GetLiftLogger() observability.StructuredLogger {
	liftLoggerMu.RLock()
	defer liftLoggerMu.RUnlock()
	return LiftLogger
}

// SanitizeLogString removes control characters that could enable log forging.
func SanitizeLogString(value string) string {
	if value == "" {
		return value
	}
	value = strings.ReplaceAll(value, "\r", "")
	return strings.ReplaceAll(value, "\n", "")
}

// SanitizeLogFields returns a sanitized copy of log fields.
func SanitizeLogFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}

	sanitized := make(map[string]any, len(fields))
	for key, value := range fields {
		sanitized[key] = sanitizeLogValue(key, value)
	}

	return sanitized
}

func sanitizeLogValue(key string, value any) any {
	sanitized := sanitization.SanitizeFieldValue(key, value)
	switch typed := sanitized.(type) {
	case nil:
		return nil
	case string:
		return SanitizeLogString(typed)
	case []byte:
		return SanitizeLogString(string(typed))
	case map[string]any:
		return SanitizeLogFields(typed)
	case []any:
		return sanitizeLogSlice(key, typed)
	default:
		return SanitizeLogString(fmt.Sprintf("%v", typed))
	}
}

func sanitizeLogSlice(key string, values []any) []any {
	sanitized := make([]any, len(values))
	for index, item := range values {
		sanitized[index] = sanitizeLogValue(key, item)
	}
	return sanitized
}
