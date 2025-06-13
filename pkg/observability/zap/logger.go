package zap

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// ZapLogger implements the StructuredLogger interface using Zap
type ZapLogger struct {
	logger        *zap.Logger
	sugar         *zap.SugaredLogger
	config        observability.LoggerConfig
	stats         *loggerStats
	contextFields map[string]interface{}
}

// loggerStats tracks logger performance metrics
type loggerStats struct {
	entriesLogged  int64
	entriesDropped int64
	flushCount     int64
	lastFlush      int64 // Unix timestamp
	errorCount     int64
	lastError      string
}

// NewZapLogger creates a new Zap-based logger
func NewZapLogger(config observability.LoggerConfig) (*ZapLogger, error) {
	zapConfig := buildZapConfig(config)

	logger, err := zapConfig.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return &ZapLogger{
		logger:        logger,
		sugar:         logger.Sugar(),
		config:        config,
		stats:         &loggerStats{},
		contextFields: make(map[string]interface{}),
	}, nil
}

// buildZapConfig creates a Zap configuration from our LoggerConfig
func buildZapConfig(config observability.LoggerConfig) zap.Config {
	level := zapcore.InfoLevel
	switch config.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	zapConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if config.Format == "console" {
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	return zapConfig
}

// Debug logs a debug message
func (z *ZapLogger) Debug(message string, fields ...map[string]interface{}) {
	z.log(zapcore.DebugLevel, message, fields...)
}

// Info logs an info message
func (z *ZapLogger) Info(message string, fields ...map[string]interface{}) {
	z.log(zapcore.InfoLevel, message, fields...)
}

// Warn logs a warning message
func (z *ZapLogger) Warn(message string, fields ...map[string]interface{}) {
	z.log(zapcore.WarnLevel, message, fields...)
}

// Error logs an error message
func (z *ZapLogger) Error(message string, fields ...map[string]interface{}) {
	z.log(zapcore.ErrorLevel, message, fields...)
}

// WithField returns a new logger with an additional field
func (z *ZapLogger) WithField(key string, value interface{}) lift.Logger {
	return z.WithFields(map[string]interface{}{key: value})
}

// WithFields returns a new logger with additional fields
func (z *ZapLogger) WithFields(fields map[string]interface{}) lift.Logger {
	newFields := make(map[string]interface{})
	for k, v := range z.contextFields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &ZapLogger{
		logger:        z.logger,
		sugar:         z.sugar,
		config:        z.config,
		stats:         z.stats,
		contextFields: newFields,
	}
}

// WithRequestID adds request ID to logger context
func (z *ZapLogger) WithRequestID(requestID string) observability.StructuredLogger {
	return z.WithField("request_id", requestID).(observability.StructuredLogger)
}

// WithTenantID adds tenant ID to logger context
func (z *ZapLogger) WithTenantID(tenantID string) observability.StructuredLogger {
	return z.WithField("tenant_id", tenantID).(observability.StructuredLogger)
}

// WithUserID adds user ID to logger context
func (z *ZapLogger) WithUserID(userID string) observability.StructuredLogger {
	return z.WithField("user_id", userID).(observability.StructuredLogger)
}

// WithTraceID adds trace ID to logger context
func (z *ZapLogger) WithTraceID(traceID string) observability.StructuredLogger {
	return z.WithField("trace_id", traceID).(observability.StructuredLogger)
}

// WithSpanID adds span ID to logger context
func (z *ZapLogger) WithSpanID(spanID string) observability.StructuredLogger {
	return z.WithField("span_id", spanID).(observability.StructuredLogger)
}

// log is the internal logging method
func (z *ZapLogger) log(level zapcore.Level, message string, fieldMaps ...map[string]interface{}) {
	// Increment counter
	atomic.AddInt64(&z.stats.entriesLogged, 1)

	// Build fields slice
	var zapFields []zap.Field

	// Add context fields
	for k, v := range z.contextFields {
		zapFields = append(zapFields, zap.Any(k, v))
	}

	// Add provided fields
	for _, fieldMap := range fieldMaps {
		for k, v := range fieldMap {
			zapFields = append(zapFields, zap.Any(k, v))
		}
	}

	// Log with appropriate level
	switch level {
	case zapcore.DebugLevel:
		z.logger.Debug(message, zapFields...)
	case zapcore.InfoLevel:
		z.logger.Info(message, zapFields...)
	case zapcore.WarnLevel:
		z.logger.Warn(message, zapFields...)
	case zapcore.ErrorLevel:
		z.logger.Error(message, zapFields...)
	}
}

// Flush syncs the logger (Zap handles this automatically)
func (z *ZapLogger) Flush(ctx context.Context) error {
	atomic.AddInt64(&z.stats.flushCount, 1)
	atomic.StoreInt64(&z.stats.lastFlush, time.Now().Unix())
	return z.logger.Sync()
}

// Close closes the logger
func (z *ZapLogger) Close() error {
	return z.logger.Sync()
}

// IsHealthy returns true if the logger is functioning properly
func (z *ZapLogger) IsHealthy() bool {
	return atomic.LoadInt64(&z.stats.errorCount) == 0
}

// GetStats returns logger performance statistics
func (z *ZapLogger) GetStats() observability.LoggerStats {
	return observability.LoggerStats{
		EntriesLogged:    atomic.LoadInt64(&z.stats.entriesLogged),
		EntriesDropped:   atomic.LoadInt64(&z.stats.entriesDropped),
		FlushCount:       atomic.LoadInt64(&z.stats.flushCount),
		LastFlush:        time.Unix(atomic.LoadInt64(&z.stats.lastFlush), 0),
		BufferSize:       0, // Zap manages its own buffering
		BufferCapacity:   0,
		AverageFlushTime: 0, // Would need more complex tracking
		ErrorCount:       atomic.LoadInt64(&z.stats.errorCount),
		LastError:        z.stats.lastError,
	}
}

// ZapLoggerFactory implements the LoggerFactory interface
type ZapLoggerFactory struct{}

// NewZapLoggerFactory creates a new Zap logger factory
func NewZapLoggerFactory() *ZapLoggerFactory {
	return &ZapLoggerFactory{}
}

// CreateConsoleLogger creates a console-based logger
func (f *ZapLoggerFactory) CreateConsoleLogger(config observability.LoggerConfig) (observability.StructuredLogger, error) {
	config.Format = "console"
	return NewZapLogger(config)
}

// CreateCloudWatchLogger creates a CloudWatch-integrated logger
func (f *ZapLoggerFactory) CreateCloudWatchLogger(config observability.LoggerConfig, client observability.CloudWatchLogsClient) (observability.StructuredLogger, error) {
	// For now, return a console logger - CloudWatch integration will be in the CloudWatch package
	config.Format = "json"
	return NewZapLogger(config)
}

// CreateTestLogger creates a logger suitable for testing
func (f *ZapLoggerFactory) CreateTestLogger() observability.StructuredLogger {
	config := observability.LoggerConfig{
		Level:  "debug",
		Format: "json",
	}
	logger, _ := NewZapLogger(config)
	return logger
}

// CreateNoOpLogger creates a no-op logger
func (f *ZapLoggerFactory) CreateNoOpLogger() observability.StructuredLogger {
	return &NoOpStructuredLogger{}
}

// NoOpStructuredLogger is a no-op implementation for testing
type NoOpStructuredLogger struct {
	*lift.NoOpLogger
}

func (n *NoOpStructuredLogger) WithRequestID(requestID string) observability.StructuredLogger {
	return n
}
func (n *NoOpStructuredLogger) WithTenantID(tenantID string) observability.StructuredLogger { return n }
func (n *NoOpStructuredLogger) WithUserID(userID string) observability.StructuredLogger     { return n }
func (n *NoOpStructuredLogger) WithTraceID(traceID string) observability.StructuredLogger   { return n }
func (n *NoOpStructuredLogger) WithSpanID(spanID string) observability.StructuredLogger     { return n }
func (n *NoOpStructuredLogger) Flush(ctx context.Context) error                             { return nil }
func (n *NoOpStructuredLogger) Close() error                                                { return nil }
func (n *NoOpStructuredLogger) IsHealthy() bool                                             { return true }
func (n *NoOpStructuredLogger) GetStats() observability.LoggerStats {
	return observability.LoggerStats{}
}
