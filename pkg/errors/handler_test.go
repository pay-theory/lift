package errors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDefaultErrorHandler_HandleError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedCode   string
		expectedStatus int
		shouldLog      bool
	}{
		{
			name:           "nil error",
			err:            nil,
			expectedCode:   "",
			expectedStatus: 200,
			shouldLog:      false,
		},
		{
			name:           "LiftError",
			err:            BadRequest("invalid input"),
			expectedCode:   "BAD_REQUEST",
			expectedStatus: 400,
			shouldLog:      false,
		},
		{
			name:           "generic error",
			err:            errors.New("something went wrong"),
			expectedCode:   "INTERNAL_ERROR",
			expectedStatus: 500,
			shouldLog:      true,
		},
		{
			name:           "internal server error",
			err:            InternalError("database connection failed"),
			expectedCode:   "INTERNAL_ERROR",
			expectedStatus: 500,
			shouldLog:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewDefaultErrorHandler()
			ctx := context.WithValue(context.Background(), "request_id", "test-123")

			result := handler.HandleError(ctx, tt.err)

			if tt.err == nil {
				if result != nil {
					t.Errorf("expected nil result for nil error, got %v", result)
				}
				return
			}

			liftErr, ok := result.(*LiftError)
			if !ok {
				t.Errorf("expected LiftError, got %T", result)
				return
			}

			if liftErr.Code != tt.expectedCode {
				t.Errorf("expected code %s, got %s", tt.expectedCode, liftErr.Code)
			}

			if liftErr.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, liftErr.StatusCode)
			}

			if liftErr.RequestID != "test-123" {
				t.Errorf("expected request ID test-123, got %s", liftErr.RequestID)
			}

			shouldLog := handler.ShouldLog(result)
			if shouldLog != tt.shouldLog {
				t.Errorf("expected shouldLog %v, got %v", tt.shouldLog, shouldLog)
			}
		})
	}
}

func TestDefaultErrorHandler_HandlePanic(t *testing.T) {
	tests := []struct {
		name        string
		panicValue  interface{}
		expectedMsg string
	}{
		{
			name:        "string panic",
			panicValue:  "something went wrong",
			expectedMsg: "Internal server error",
		},
		{
			name:        "error panic",
			panicValue:  errors.New("test error"),
			expectedMsg: "Internal server error",
		},
		{
			name:        "other panic",
			panicValue:  42,
			expectedMsg: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewDefaultErrorHandler()
			handler.EnableStackTrace = true
			ctx := context.Background()

			result := handler.HandlePanic(ctx, tt.panicValue)

			liftErr, ok := result.(*LiftError)
			if !ok {
				t.Errorf("expected LiftError, got %T", result)
				return
			}

			if liftErr.Code != "PANIC" {
				t.Errorf("expected code PANIC, got %s", liftErr.Code)
			}

			if liftErr.StatusCode != 500 {
				t.Errorf("expected status 500, got %d", liftErr.StatusCode)
			}

			if liftErr.Message != tt.expectedMsg {
				t.Errorf("expected message %s, got %s", tt.expectedMsg, liftErr.Message)
			}

			// Check stack trace is included
			if liftErr.Details == nil || liftErr.Details["stacktrace"] == nil {
				t.Error("expected stack trace in details")
			}
		})
	}
}

func TestErrorTransformers(t *testing.T) {
	t.Run("SanitizeErrorTransformer", func(t *testing.T) {
		// Test 5xx error sanitization
		internalErr := &LiftError{
			Code:       "INTERNAL_ERROR",
			Message:    "Database connection failed: password=secret123",
			StatusCode: 500,
			Details: map[string]interface{}{
				"password": "secret123",
			},
		}

		sanitized := SanitizeErrorTransformer(internalErr)
		sanitizedErr := sanitized.(*LiftError)

		if sanitizedErr.Message != "An internal error occurred" {
			t.Errorf("expected sanitized message, got %s", sanitizedErr.Message)
		}

		if sanitizedErr.Details != nil {
			t.Error("expected details to be removed")
		}

		// Test 4xx error (should not be sanitized)
		clientErr := BadRequest("invalid email format")
		result := SanitizeErrorTransformer(clientErr)

		if result.(*LiftError).Message != "invalid email format" {
			t.Error("4xx errors should not be sanitized")
		}
	})

	t.Run("RateLimitErrorTransformer", func(t *testing.T) {
		rateLimitErr := &LiftError{
			Code:       "RATE_LIMITED",
			Message:    "Too many requests",
			StatusCode: 429,
		}

		transformed := RateLimitErrorTransformer(rateLimitErr)
		transformedErr := transformed.(*LiftError)

		if transformedErr.Details == nil || transformedErr.Details["retry_after"] == nil {
			t.Error("expected retry_after in details")
		}

		if transformedErr.Details["retry_after"] != 60 {
			t.Errorf("expected retry_after 60, got %v", transformedErr.Details["retry_after"])
		}
	})
}

func TestErrorHandlerWithRecoveryStrategies(t *testing.T) {
	t.Run("with recovery strategy", func(t *testing.T) {
		handler := NewDefaultErrorHandler()

		// Add a simple recovery strategy
		handler.RecoveryStrategies = []RecoveryStrategy{
			&mockRecoveryStrategy{
				canRecover: true,
				recovered:  BadRequest("recovered error"),
			},
		}

		originalErr := InternalError("original error")
		ctx := context.Background()

		result := handler.HandleError(ctx, originalErr)

		liftErr := result.(*LiftError)
		if liftErr.Code != "BAD_REQUEST" {
			t.Errorf("expected recovered error code BAD_REQUEST, got %s", liftErr.Code)
		}

		if liftErr.Message != "recovered error" {
			t.Errorf("expected recovered error message, got %s", liftErr.Message)
		}
	})
}

func TestErrorHandlerWithTransformers(t *testing.T) {
	handler := NewDefaultErrorHandler()

	// Add transformer that adds a prefix
	handler.ErrorTransformers = []ErrorTransformer{
		func(err error) error {
			if liftErr, ok := err.(*LiftError); ok {
				liftErr.Message = "TRANSFORMED: " + liftErr.Message
			}
			return err
		},
	}

	originalErr := BadRequest("test error")
	ctx := context.Background()

	result := handler.HandleError(ctx, originalErr)

	liftErr := result.(*LiftError)
	if !strings.HasPrefix(liftErr.Message, "TRANSFORMED: ") {
		t.Errorf("expected transformed message, got %s", liftErr.Message)
	}
}

func TestErrorHandlerLogging(t *testing.T) {
	var loggedMessages []string

	handler := NewDefaultErrorHandler()
	handler.Logger = func(format string, args ...interface{}) {
		loggedMessages = append(loggedMessages, fmt.Sprintf(format, args...))
	}

	ctx := context.Background()

	// Test 5xx error (should log)
	internalErr := InternalError("database error")
	handler.HandleError(ctx, internalErr)

	if len(loggedMessages) != 1 {
		t.Errorf("expected 1 log message for 5xx error, got %d", len(loggedMessages))
	}

	// Test 4xx error (should not log)
	loggedMessages = nil
	clientErr := BadRequest("invalid input")
	handler.HandleError(ctx, clientErr)

	if len(loggedMessages) != 0 {
		t.Errorf("expected 0 log messages for 4xx error, got %d", len(loggedMessages))
	}
}

// Mock recovery strategy for testing
type mockRecoveryStrategy struct {
	canRecover bool
	recovered  error
}

func (m *mockRecoveryStrategy) CanRecover(err error) bool {
	return m.canRecover
}

func (m *mockRecoveryStrategy) Recover(ctx context.Context, err error) error {
	return m.recovered
}

func BenchmarkErrorHandler_HandleError(b *testing.B) {
	handler := NewDefaultErrorHandler()
	ctx := context.Background()
	err := BadRequest("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.HandleError(ctx, err)
	}
}

func BenchmarkErrorHandler_HandlePanic(b *testing.B) {
	handler := NewDefaultErrorHandler()
	ctx := context.Background()
	panicValue := "test panic"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.HandlePanic(ctx, panicValue)
	}
}
