package errors

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

// LiftError represents a structured error in the Lift framework
type LiftError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]any         `json:"details,omitempty"`
	StatusCode int                    `json:"-"`
	Cause      error                  `json:"-"`

	// Observability
	RequestID  string `json:"request_id,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	Timestamp  string `json:"timestamp"`
	StackTrace string `json:"stack_trace,omitempty"`

	// AppSync
	ErrorData map[string]any `json:"errorData,omitempty"`
	ErrorInfo map[string]any `json:"errorInfo,omitempty"`

	// Logging
	LogError bool `json:"-"`
}

// Error implements the error interface
func (e *LiftError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *LiftError) Unwrap() error {
	return e.Cause
}

// HTTP error constructors

// ValidationError creates a 422 Unprocessable Entity error for validation failures
func ValidationError(message string) *LiftError {
	return &LiftError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		StatusCode: 422,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// Unauthorized creates a 401 Unauthorized error
func Unauthorized(message string) *LiftError {
	return &LiftError{
		Code:       "UNAUTHORIZED",
		Message:    message,
		StatusCode: 401,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// AuthorizationError creates a 403 Forbidden error
func AuthorizationError(message string) *LiftError {
	return &LiftError{
		Code:       "AUTHORIZATION_ERROR",
		Message:    message,
		StatusCode: 403,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// NotFound creates a 404 Not Found error
func NotFound(message string) *LiftError {
	return &LiftError{
		Code:       "NOT_FOUND",
		Message:    message,
		StatusCode: 404,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// SystemError creates a 500 Internal Server Error
func SystemError(message string) *LiftError {
	return &LiftError{
		Code:       "SYSTEM_ERROR",
		Message:    message,
		StatusCode: 500,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		TraceID:    uuid.New().String(),
		StackTrace: string(debug.Stack()),
		LogError:   true, // 5xx errors are logged by default
	}
}

// NetworkError creates a 500 Internal Server Error for network-related failures
func NetworkError(message string) *LiftError {
	return &LiftError{
		Code:       "NETWORK_ERROR",
		Message:    message,
		StatusCode: 500,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		TraceID:    uuid.New().String(),
		StackTrace: string(debug.Stack()),
		LogError:   true, // 5xx errors are logged by default
	}
}

// ProcessingError creates a 500 Internal Server Error for processing failures
func ProcessingError(message string) *LiftError {
	return &LiftError{
		Code:       "PROCESSING_ERROR",
		Message:    message,
		StatusCode: 500,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		TraceID:    uuid.New().String(),
		StackTrace: string(debug.Stack()),
		LogError:   true, // 5xx errors are logged by default
	}
}

// TokenizationFailure creates a 500 Internal Server Error for tokenization failures
func TokenizationFailure(message string) *LiftError {
	return &LiftError{
		Code:       "TOKENIZATION_FAILURE",
		Message:    message,
		StatusCode: 500,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		TraceID:    uuid.New().String(),
		StackTrace: string(debug.Stack()),
		LogError:   true, // 5xx errors are logged by default
	}
}

// ParameterError creates a parameter validation error with field details
func ParameterError(field, message string) *LiftError {
	return &LiftError{
		Code:       "PARAMETER_ERROR",
		Message:    message,
		StatusCode: 400,
		Details: map[string]any{
			"field":   field,
			"message": message,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// WithDetails adds details to an error
func (e *LiftError) WithDetails(key string, value any) *LiftError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// WithCause adds a cause error
func (e *LiftError) WithCause(cause error) *LiftError {
	e.Cause = cause
	return e
}

// WithRequestID adds a request ID for tracing
func (e *LiftError) WithRequestID(requestID string) *LiftError {
	e.RequestID = requestID
	return e
}

// WithTraceID adds a trace ID for distributed tracing
func (e *LiftError) WithTraceID(traceID string) *LiftError {
	if traceID == "" {
		// Generate a new trace ID if not provided
		traceID = uuid.New().String()
	}
	e.TraceID = traceID
	return e
}

// WithStackTrace adds a stack trace to the error
func (e *LiftError) WithStackTrace() *LiftError {
	e.StackTrace = string(debug.Stack())
	return e
}

// WithErrorData adds AppSync error data
func (e *LiftError) WithErrorData(key string, value any) *LiftError {
	if e.ErrorData == nil {
		e.ErrorData = make(map[string]any)
	}
	e.ErrorData[key] = value
	return e
}

// WithErrorInfo adds AppSync error info
func (e *LiftError) WithErrorInfo(key string, value any) *LiftError {
	if e.ErrorInfo == nil {
		e.ErrorInfo = make(map[string]any)
	}
	e.ErrorInfo[key] = value
	return e
}

// WithLogging sets whether this error should be logged
func (e *LiftError) WithLogging(shouldLog bool) *LiftError {
	e.LogError = shouldLog
	return e
}
