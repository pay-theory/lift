package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"time"
)

// ErrorHandler defines the interface for handling errors in the Lift framework
type ErrorHandler interface {
	// HandleError processes an error and returns a client-safe error response
	HandleError(ctx context.Context, err error) error

	// HandlePanic recovers from panics and converts them to errors
	HandlePanic(ctx context.Context, v any) error

	// ShouldLog determines if an error should be logged
	ShouldLog(err error) bool

	// GetStatusCode returns the HTTP status code for an error
	GetStatusCode(err error) int
}

// RecoveryStrategy defines how to recover from errors
type RecoveryStrategy interface {
	// Recover attempts to recover from an error
	Recover(ctx context.Context, err error) error

	// CanRecover determines if this strategy can handle the error
	CanRecover(err error) bool
}

// DefaultErrorHandler provides standard error handling
type DefaultErrorHandler struct {
	// EnableStackTrace includes stack traces in error responses (dev mode)
	EnableStackTrace bool

	// RecoveryStrategies to attempt in order
	RecoveryStrategies []RecoveryStrategy

	// ErrorTransformers modify errors before returning to client
	ErrorTransformers []ErrorTransformer

	// Logger for error logging
	Logger func(format string, args ...any)
}

// ErrorTransformer modifies errors before sending to clients
type ErrorTransformer func(err error) error

// NewDefaultErrorHandler creates a production-ready error handler
func NewDefaultErrorHandler() *DefaultErrorHandler {
	return &DefaultErrorHandler{
		EnableStackTrace:   false,
		RecoveryStrategies: []RecoveryStrategy{},
		ErrorTransformers:  []ErrorTransformer{},
		Logger:             log.Printf,
	}
}

// HandleError processes an error with recovery strategies and transformations
func (h *DefaultErrorHandler) HandleError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// Attempt recovery strategies
	for _, strategy := range h.RecoveryStrategies {
		if strategy.CanRecover(err) {
			if recovered := strategy.Recover(ctx, err); recovered != nil {
				err = recovered
			}
		}
	}

	// Convert to LiftError if needed
	liftErr := h.toLiftError(err)

	// Add context information
	if reqID := ctx.Value("request_id"); reqID != nil {
		liftErr.RequestID = reqID.(string)
	}

	if traceID := ctx.Value("trace_id"); traceID != nil {
		liftErr.TraceID = traceID.(string)
	}

	// Apply transformers
	transformedErr := error(liftErr)
	for _, transformer := range h.ErrorTransformers {
		transformedErr = transformer(transformedErr)
	}

	// Log if needed
	if h.ShouldLog(transformedErr) {
		h.logError(ctx, transformedErr)
	}

	return transformedErr
}

// HandlePanic recovers from panics and converts them to errors
func (h *DefaultErrorHandler) HandlePanic(ctx context.Context, v any) error {
	var err error

	switch x := v.(type) {
	case string:
		err = fmt.Errorf("panic: %s", x)
	case error:
		err = fmt.Errorf("panic: %w", x)
	default:
		err = fmt.Errorf("panic: %v", x)
	}

	// Create panic error with stack trace
	panicErr := &LiftError{
		Code:       "PANIC",
		Message:    "Internal server error",
		StatusCode: 500,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Cause:      err,
		LogError:   true, // Panics should always be logged
	}

	if h.EnableStackTrace {
		panicErr.Details = map[string]any{
			"panic":      v,
			"stacktrace": string(debug.Stack()),
		}
	}

	return h.HandleError(ctx, panicErr)
}

// ShouldLog determines if an error should be logged
func (h *DefaultErrorHandler) ShouldLog(err error) bool {
	if err == nil {
		return false
	}

	// Check if the error has explicit logging preference
	if liftErr, ok := err.(*LiftError); ok {
		if liftErr.LogError {
			return true
		}
		
		// If LogError is false but it's a 5xx error, still log it
		// unless explicitly disabled
		return liftErr.StatusCode >= 500
	}

	// Log unknown errors
	return true
}

// GetStatusCode returns the HTTP status code for an error
func (h *DefaultErrorHandler) GetStatusCode(err error) int {
	if err == nil {
		return 200
	}

	if liftErr, ok := err.(*LiftError); ok {
		return liftErr.StatusCode
	}

	// Default to 500 for unknown errors
	return 500
}

// toLiftError converts any error to a LiftError
func (h *DefaultErrorHandler) toLiftError(err error) *LiftError {
	// Already a LiftError
	if liftErr, ok := err.(*LiftError); ok {
		return liftErr
	}

	// Create generic internal error
	return &LiftError{
		Code:       "SYSTEM_ERROR",
		Message:    "An internal error occurred",
		StatusCode: 500,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Cause:      err,
		Details:    make(map[string]any),
		LogError:   true, // Generic internal errors should be logged
	}
}

// logError logs an error with context
func (h *DefaultErrorHandler) logError(ctx context.Context, err error) {
	liftErr, ok := err.(*LiftError)
	if !ok {
		h.Logger("Error: %v", err)
		return
	}

	logData := map[string]interface{}{
		"code":       liftErr.Code,
		"message":    liftErr.Message,
		"status":     liftErr.StatusCode,
		"request_id": liftErr.RequestID,
		"trace_id":   liftErr.TraceID,
		"timestamp":  liftErr.Timestamp,
	}

	// Add additional context information if available
	if userID := ctx.Value("user_id"); userID != nil {
		logData["user_id"] = userID
	}
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		logData["tenant_id"] = tenantID
	}

	if liftErr.Cause != nil {
		logData["cause"] = liftErr.Cause.Error()
	}

	if h.EnableStackTrace && liftErr.Details != nil {
		logData["details"] = liftErr.Details
	}
	
	if h.EnableStackTrace && liftErr.StackTrace != "" {
		logData["stack_trace"] = liftErr.StackTrace
	}

	jsonData, _ := json.Marshal(logData)
	h.Logger("Error: %s", string(jsonData))
}

// Common Error Transformers

// SanitizeErrorTransformer removes sensitive information from errors
func SanitizeErrorTransformer(err error) error {
	liftErr, ok := err.(*LiftError)
	if !ok {
		return err
	}

	// Remove internal details for client errors
	if liftErr.StatusCode >= 500 {
		sanitized := &LiftError{
			Code:       liftErr.Code,
			Message:    "An internal error occurred",
			StatusCode: liftErr.StatusCode,
			RequestID:  liftErr.RequestID,
			TraceID:    liftErr.TraceID,
			Timestamp:  liftErr.Timestamp,
		}
		return sanitized
	}

	return err
}

// RateLimitErrorTransformer adds rate limit headers to errors
func RateLimitErrorTransformer(err error) error {
	liftErr, ok := err.(*LiftError)
	if !ok {
		return err
	}

	if liftErr.Code == "RATE_LIMITED" {
		if liftErr.Details == nil {
			liftErr.Details = make(map[string]any)
		}
		liftErr.Details["retry_after"] = 60 // seconds
	}

	return err
}
