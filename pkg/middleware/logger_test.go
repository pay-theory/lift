package middleware

import (
	"context"
	"testing"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerMiddleware(t *testing.T) {
	t.Run("Logger middleware initializes real logger when ctx.Logger is nil", func(t *testing.T) {
		middleware := Logger()

		handler := lift.HandlerFunc(func(ctx *lift.Context) error {
			// Verify logger is now initialized
			assert.NotNil(t, ctx.Logger, "Logger should be initialized")

			// Verify we can safely use the logger and it actually logs
			ctx.Logger.Info("test message", map[string]any{"key": "value"})
			ctx.Logger.Error("test error", map[string]any{"error": "test"})

			return nil
		})

		wrappedHandler := middleware(handler)
		ctx := createLoggerTestContext("GET", "/test", nil)

		// Explicitly verify logger is nil before middleware runs
		assert.Nil(t, ctx.Logger, "Logger should be nil before middleware runs")

		err := wrappedHandler.Handle(ctx)
		assert.NoError(t, err, "Handler should execute successfully")

		// Verify logger was initialized
		assert.NotNil(t, ctx.Logger, "Logger should be initialized after middleware runs")
	})

	t.Run("Logger middleware augments existing logger with request_id", func(t *testing.T) {
		middleware := Logger()

		// Create a mock logger that tracks calls
		mockLogger := &MockLogger{
			withFieldCalls: make(map[string]any),
		}

		handler := lift.HandlerFunc(func(ctx *lift.Context) error {
			// Verify logger was augmented
			assert.NotNil(t, ctx.Logger, "Logger should be set")
			return nil
		})

		wrappedHandler := middleware(handler)
		ctx := createLoggerTestContext("GET", "/test", nil)
		ctx.Logger = mockLogger
		ctx.RequestID = "test-request-123"

		err := wrappedHandler.Handle(ctx)
		assert.NoError(t, err, "Handler should execute successfully")

		// Verify WithField was called with request_id
		assert.Contains(t, mockLogger.withFieldCalls, "request_id", "Should add request_id to logger")
		assert.Equal(t, "test-request-123", mockLogger.withFieldCalls["request_id"])
	})

	t.Run("Logger middleware logs request completion", func(t *testing.T) {
		middleware := Logger()

		mockLogger := &MockLogger{
			infoCalls:      []MockLogCall{},
			withFieldCalls: make(map[string]any),
		}

		handler := lift.HandlerFunc(func(ctx *lift.Context) error {
			ctx.Response.StatusCode = 200
			return nil
		})

		wrappedHandler := middleware(handler)
		ctx := createLoggerTestContext("GET", "/api/users", nil)
		ctx.Logger = mockLogger
		ctx.RequestID = "req-123"

		err := wrappedHandler.Handle(ctx)
		assert.NoError(t, err, "Handler should execute successfully")

		// Verify Info was called with request completion details
		assert.Len(t, mockLogger.infoCalls, 1, "Should log request completion")
		assert.Equal(t, "Request completed", mockLogger.infoCalls[0].Message)
		assert.Contains(t, mockLogger.infoCalls[0].Fields, "method")
		assert.Contains(t, mockLogger.infoCalls[0].Fields, "path")
		assert.Contains(t, mockLogger.infoCalls[0].Fields, "status")
		assert.Contains(t, mockLogger.infoCalls[0].Fields, "duration")
	})

	t.Run("Logger middleware logs request failure", func(t *testing.T) {
		middleware := Logger()

		mockLogger := &MockLogger{
			errorCalls:     []MockLogCall{},
			withFieldCalls: make(map[string]any),
		}

		handler := lift.HandlerFunc(func(ctx *lift.Context) error {
			return lift.NewLiftError("TEST_ERROR", "test error", 500)
		})

		wrappedHandler := middleware(handler)
		ctx := createLoggerTestContext("POST", "/api/payments", nil)
		ctx.Logger = mockLogger
		ctx.RequestID = "req-456"

		err := wrappedHandler.Handle(ctx)
		assert.Error(t, err, "Handler should return error")

		// Verify Error was called with request failure details
		assert.Len(t, mockLogger.errorCalls, 1, "Should log request failure")
		assert.Equal(t, "Request failed", mockLogger.errorCalls[0].Message)
		assert.Contains(t, mockLogger.errorCalls[0].Fields, "error")
		assert.Equal(t, "[REDACTED_ERROR_DETAIL]", mockLogger.errorCalls[0].Fields["error"], "Error should be redacted for security")
	})

	t.Run("Logger middleware makes logger available to downstream middleware", func(t *testing.T) {
		loggerMiddleware := Logger()

		// Simulate another middleware that uses the logger
		downstreamMiddleware := func(next lift.Handler) lift.Handler {
			return lift.HandlerFunc(func(ctx *lift.Context) error {
				// This should not panic because Logger middleware initialized it
				require.NotNil(t, ctx.Logger, "Logger should be available to downstream middleware")

				// Should be safe to use
				ctx.Logger.Info("downstream middleware message", map[string]any{
					"key": "value",
				})

				return next.Handle(ctx)
			})
		}

		handler := lift.HandlerFunc(func(ctx *lift.Context) error {
			// Final handler also has access
			require.NotNil(t, ctx.Logger, "Logger should be available to final handler")
			ctx.Logger.Debug("final handler", map[string]any{"test": true})
			return nil
		})

		// Chain the middleware
		chain := Chain(loggerMiddleware, downstreamMiddleware)
		wrappedHandler := chain(handler)

		ctx := createLoggerTestContext("GET", "/test", nil)
		assert.Nil(t, ctx.Logger, "Logger should start as nil")

		err := wrappedHandler.Handle(ctx)
		assert.NoError(t, err, "Handler chain should execute successfully")
		assert.NotNil(t, ctx.Logger, "Logger should be initialized after middleware chain")
	})
}

// MockLogger for testing
type MockLogger struct {
	debugCalls     []MockLogCall
	infoCalls      []MockLogCall
	warnCalls      []MockLogCall
	errorCalls     []MockLogCall
	withFieldCalls map[string]any
}

type MockLogCall struct {
	Message string
	Fields  map[string]any
}

func (m *MockLogger) Debug(message string, fields ...map[string]any) {
	call := MockLogCall{Message: message}
	if len(fields) > 0 {
		call.Fields = fields[0]
	}
	m.debugCalls = append(m.debugCalls, call)
}

func (m *MockLogger) Info(message string, fields ...map[string]any) {
	call := MockLogCall{Message: message}
	if len(fields) > 0 {
		call.Fields = fields[0]
	}
	m.infoCalls = append(m.infoCalls, call)
}

func (m *MockLogger) Warn(message string, fields ...map[string]any) {
	call := MockLogCall{Message: message}
	if len(fields) > 0 {
		call.Fields = fields[0]
	}
	m.warnCalls = append(m.warnCalls, call)
}

func (m *MockLogger) Error(message string, fields ...map[string]any) {
	call := MockLogCall{Message: message}
	if len(fields) > 0 {
		call.Fields = fields[0]
	}
	m.errorCalls = append(m.errorCalls, call)
}

func (m *MockLogger) WithField(key string, value any) lift.Logger {
	m.withFieldCalls[key] = value
	return m
}

func (m *MockLogger) WithFields(fields map[string]any) lift.Logger {
	for k, v := range fields {
		m.withFieldCalls[k] = v
	}
	return m
}

// Helper function to create test context
func createLoggerTestContext(method, path string, body []byte) *lift.Context {
	adapterReq := &adapters.Request{
		Method:      method,
		Path:        path,
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
		PathParams:  make(map[string]string),
		Body:        body,
	}
	req := lift.NewRequest(adapterReq)
	ctx := lift.NewContext(context.Background(), req)

	// Initialize response headers if not already done
	if ctx.Response.Headers == nil {
		ctx.Response.Headers = make(map[string]string)
	}

	return ctx
}
