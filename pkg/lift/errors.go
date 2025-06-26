package lift

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
	Timestamp int64  `json:"timestamp"`
}

// Error implements the error interface
func (e *LiftError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements the unwrap interface for error chaining
func (e *LiftError) Unwrap() error {
	return e.Cause
}

// NewLiftError creates a new LiftError
func NewLiftError(code, message string, statusCode int) *LiftError {
	return &LiftError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Timestamp:  time.Now().Unix(),
	}
}

// WithDetails adds details to the error
func (e *LiftError) WithDetails(details map[string]interface{}) *LiftError {
	e.Details = details
	return e
}

// WithCause adds a cause error
func (e *LiftError) WithCause(cause error) *LiftError {
	e.Cause = cause
	return e
}

// HTTP error constructors

// BadRequest creates a 400 Bad Request error
// Deprecated: For validation errors, use ValidationError(message) instead
func BadRequest(message string) *LiftError {
	return NewLiftError("BAD_REQUEST", message, 400)
}

func Unauthorized(message string) *LiftError {
	return NewLiftError("UNAUTHORIZED", message, 401)
}

// Deprecated: Use AuthorizationError instead
func Forbidden(message string) *LiftError {
	return NewLiftError("FORBIDDEN", message, 403)
}

// AuthorizationError creates a 403 Forbidden error
func AuthorizationError(message string) *LiftError {
	return NewLiftError("AUTHORIZATION_ERROR", message, 403)
}

func NotFound(message string) *LiftError {
	return NewLiftError("NOT_FOUND", message, 404)
}

func Conflict(message string) *LiftError {
	return NewLiftError("CONFLICT", message, 409)
}

func SystemError(message string) *LiftError {
	return NewLiftError("INTERNAL_ERROR", message, 500)
}

// ParameterError creates a parameter validation error with field details
func ParameterError(field, message string) *LiftError {
	return NewLiftError("PARAMETER_ERROR", message, 400).WithDetails(map[string]interface{}{
		"field":   field,
		"message": message,
	})
}

// ValidationError creates a 422 Unprocessable Entity error for validation failures
func ValidationError(message string) *LiftError {
	return NewLiftError("VALIDATION_ERROR", message, 422)
}
