package errors

import (
	"fmt"
	"time"
)

// LiftError represents a structured error in the Lift framework
type LiftError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	StatusCode int                    `json:"-"`
	Cause      error                  `json:"-"`

	// Observability
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
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

// BadRequest creates a 400 Bad Request error
func BadRequest(message string) *LiftError {
	return &LiftError{
		Code:       "BAD_REQUEST",
		Message:    message,
		StatusCode: 400,
		Timestamp:  time.Now().Unix(),
	}
}

// Unauthorized creates a 401 Unauthorized error
func Unauthorized(message string) *LiftError {
	return &LiftError{
		Code:       "UNAUTHORIZED",
		Message:    message,
		StatusCode: 401,
		Timestamp:  time.Now().Unix(),
	}
}

// Forbidden creates a 403 Forbidden error
func Forbidden(message string) *LiftError {
	return &LiftError{
		Code:       "FORBIDDEN",
		Message:    message,
		StatusCode: 403,
		Timestamp:  time.Now().Unix(),
	}
}

// NotFound creates a 404 Not Found error
func NotFound(message string) *LiftError {
	return &LiftError{
		Code:       "NOT_FOUND",
		Message:    message,
		StatusCode: 404,
		Timestamp:  time.Now().Unix(),
	}
}

// InternalError creates a 500 Internal Server Error
func InternalError(message string) *LiftError {
	return &LiftError{
		Code:       "INTERNAL_ERROR",
		Message:    message,
		StatusCode: 500,
		Timestamp:  time.Now().Unix(),
	}
}

// ValidationError creates a validation error with field details
func ValidationError(field, message string) *LiftError {
	return &LiftError{
		Code:       "VALIDATION_ERROR",
		Message:    "Validation failed",
		StatusCode: 400,
		Details: map[string]interface{}{
			"field":   field,
			"message": message,
		},
		Timestamp: time.Now().Unix(),
	}
}

// WithDetails adds details to an error
func (e *LiftError) WithDetails(key string, value interface{}) *LiftError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
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
