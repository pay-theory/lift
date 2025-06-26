package errors

import (
	"context"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

// RetryRecoveryStrategy attempts to recover from transient errors with retry logic
type RetryRecoveryStrategy struct {
	MaxRetries    int
	RetryDelay    time.Duration
	RetryableFunc func(ctx context.Context) error
}

// CanRecover determines if this strategy can handle the error
func (r *RetryRecoveryStrategy) CanRecover(err error) bool {
	// Check for network timeout errors (modern replacement for Temporary())
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Check for connection refused
	if opErr, ok := err.(*net.OpError); ok {
		if syscallErr, ok := opErr.Err.(*syscall.Errno); ok {
			return *syscallErr == syscall.ECONNREFUSED
		}
	}

	// Check for context timeout errors
	if err == context.DeadlineExceeded {
		return true
	}

	// Check for timeout errors in error message
	if strings.Contains(err.Error(), "timeout") {
		return true
	}

	// Check for other retryable network conditions
	if strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no route to host") {
		return true
	}

	return false
}

// Recover attempts to recover from the error
func (r *RetryRecoveryStrategy) Recover(ctx context.Context, err error) error {
	if r.RetryableFunc == nil {
		return err
	}

	for i := 0; i < r.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if i > 0 {
			time.Sleep(r.RetryDelay)
		}

		if retryErr := r.RetryableFunc(ctx); retryErr == nil {
			return nil // Success!
		}
	}

	// All retries failed
	return &LiftError{
		Code:       "RETRY_EXHAUSTED",
		Message:    fmt.Sprintf("Failed after %d retries", r.MaxRetries),
		StatusCode: 503,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Cause:      err,
	}
}

// CircuitBreakerRecoveryStrategy implements circuit breaker pattern
type CircuitBreakerRecoveryStrategy struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration

	// Internal state
	failures    int
	lastFailure time.Time
	state       CircuitState
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CanRecover determines if circuit breaker should handle this error
func (c *CircuitBreakerRecoveryStrategy) CanRecover(err error) bool {
	// Circuit breaker handles service unavailable errors
	if liftErr, ok := err.(*LiftError); ok {
		return liftErr.StatusCode >= 500
	}
	return false
}

// Recover implements circuit breaker logic
func (c *CircuitBreakerRecoveryStrategy) Recover(ctx context.Context, err error) error {
	now := time.Now()

	switch c.state {
	case CircuitClosed:
		c.failures++
		c.lastFailure = now

		if c.failures >= c.FailureThreshold {
			c.state = CircuitOpen
			return &LiftError{
				Code:       "CIRCUIT_OPEN",
				Message:    "Service temporarily unavailable",
				StatusCode: 503,
				Timestamp:  now.UTC().Format(time.RFC3339),
				Details: map[string]any{
					"retry_after": int(c.RecoveryTimeout.Seconds()),
				},
			}
		}

	case CircuitOpen:
		if now.Sub(c.lastFailure) > c.RecoveryTimeout {
			c.state = CircuitHalfOpen
		} else {
			return &LiftError{
				Code:       "CIRCUIT_OPEN",
				Message:    "Service temporarily unavailable",
				StatusCode: 503,
				Timestamp:  now.UTC().Format(time.RFC3339),
				Details: map[string]any{
					"retry_after": int(c.RecoveryTimeout.Seconds() - now.Sub(c.lastFailure).Seconds()),
				},
			}
		}

	case CircuitHalfOpen:
		// Allow one request through to test
		c.state = CircuitClosed
		c.failures = 0
	}

	return err
}

// FallbackRecoveryStrategy provides fallback responses for failed operations
type FallbackRecoveryStrategy struct {
	FallbackFunc func(ctx context.Context, err error) error
}

// CanRecover determines if fallback is available
func (f *FallbackRecoveryStrategy) CanRecover(err error) bool {
	return f.FallbackFunc != nil
}

// Recover executes the fallback function
func (f *FallbackRecoveryStrategy) Recover(ctx context.Context, err error) error {
	if fallbackErr := f.FallbackFunc(ctx, err); fallbackErr != nil {
		// Fallback failed, return original error
		return err
	}

	// Fallback succeeded
	return nil
}

// DatabaseRecoveryStrategy handles database-specific errors
type DatabaseRecoveryStrategy struct {
	RetryableErrors []string
	MaxRetries      int
	RetryDelay      time.Duration
}

// CanRecover determines if this is a recoverable database error
func (d *DatabaseRecoveryStrategy) CanRecover(err error) bool {
	errStr := err.Error()
	for _, retryable := range d.RetryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}

// Recover attempts database error recovery
func (d *DatabaseRecoveryStrategy) Recover(ctx context.Context, err error) error {
	// For now, just convert to a structured error
	// In a real implementation, this would attempt reconnection
	return &LiftError{
		Code:       "DATABASE_ERROR",
		Message:    "Database operation failed",
		StatusCode: 503,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Cause:      err,
		Details: map[string]any{
			"retry_after": int(d.RetryDelay.Seconds()),
		},
	}
}

// NewDefaultRecoveryStrategies creates a set of common recovery strategies
func NewDefaultRecoveryStrategies() []RecoveryStrategy {
	return []RecoveryStrategy{
		&CircuitBreakerRecoveryStrategy{
			FailureThreshold: 5,
			RecoveryTimeout:  30 * time.Second,
		},
		&DatabaseRecoveryStrategy{
			RetryableErrors: []string{
				"connection refused",
				"timeout",
				"temporary failure",
			},
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		},
	}
}
