package streamer

import (
	"errors"
	"fmt"
)

// Sentinel errors for common conditions
var (
	ErrConnectionGone    = errors.New("connection no longer exists")
	ErrForbidden         = errors.New("operation forbidden")
	ErrPayloadTooLarge   = errors.New("payload exceeds maximum size")
	ErrThrottled         = errors.New("request throttled")
	ErrInternalServer    = errors.New("internal server error")
	ErrInvalidConnection = errors.New("invalid connection ID")
)

// APIError represents an error from the API Gateway Management API.
type APIError interface {
	error
	HTTPStatusCode() int
	ErrorCode() string
	IsRetryable() bool
}

// GoneError indicates a connection no longer exists (410).
type GoneError struct {
	ConnectionID string
	Message      string
}

func (e GoneError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("connection %s is gone", e.ConnectionID)
}

func (e GoneError) HTTPStatusCode() int { return 410 }
func (e GoneError) ErrorCode() string   { return "GoneException" }
func (e GoneError) IsRetryable() bool   { return false }
func (e GoneError) Unwrap() error       { return ErrConnectionGone }

// ForbiddenError indicates the operation is not permitted (403).
type ForbiddenError struct {
	ConnectionID string
	Message      string
}

func (e ForbiddenError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("operation forbidden for connection %s", e.ConnectionID)
}

func (e ForbiddenError) HTTPStatusCode() int { return 403 }
func (e ForbiddenError) ErrorCode() string   { return "ForbiddenException" }
func (e ForbiddenError) IsRetryable() bool   { return false }
func (e ForbiddenError) Unwrap() error       { return ErrForbidden }

// PayloadTooLargeError indicates the message payload exceeds the maximum size (413).
type PayloadTooLargeError struct {
	ConnectionID string
	Message      string
	PayloadSize  int
	MaxSize      int
}

func (e PayloadTooLargeError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("payload size %d exceeds maximum %d for connection %s",
		e.PayloadSize, e.MaxSize, e.ConnectionID)
}

func (e PayloadTooLargeError) HTTPStatusCode() int { return 413 }
func (e PayloadTooLargeError) ErrorCode() string   { return "PayloadTooLargeException" }
func (e PayloadTooLargeError) IsRetryable() bool   { return false }
func (e PayloadTooLargeError) Unwrap() error       { return ErrPayloadTooLarge }

// ThrottlingError indicates the request is being rate limited (429).
type ThrottlingError struct {
	ConnectionID string
	Message      string
	RetryAfter   int // Seconds to wait before retrying
}

func (e ThrottlingError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("throttled for connection %s, retry after %d seconds",
			e.ConnectionID, e.RetryAfter)
	}
	return fmt.Sprintf("throttled for connection %s", e.ConnectionID)
}

func (e ThrottlingError) HTTPStatusCode() int { return 429 }
func (e ThrottlingError) ErrorCode() string   { return "ThrottlingException" }
func (e ThrottlingError) IsRetryable() bool   { return true }
func (e ThrottlingError) Unwrap() error       { return ErrThrottled }

// InternalServerError indicates an AWS service error (500).
type InternalServerError struct {
	Message string
}

func (e InternalServerError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "internal server error"
}

func (e InternalServerError) HTTPStatusCode() int { return 500 }
func (e InternalServerError) ErrorCode() string   { return "InternalServerError" }
func (e InternalServerError) IsRetryable() bool   { return true }
func (e InternalServerError) Unwrap() error       { return ErrInternalServer }
