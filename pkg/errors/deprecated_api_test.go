package errors

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestModernNetworkErrorHandling(t *testing.T) {
	strategy := &RetryRecoveryStrategy{
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
	}

	t.Run("Handles timeout errors correctly", func(t *testing.T) {
		// Create a mock network timeout error
		timeoutErr := &mockNetError{timeout: true, temporary: false}

		canRecover := strategy.CanRecover(timeoutErr)
		assert.True(t, canRecover, "Should be able to recover from timeout errors")
	})

	t.Run("Handles context deadline exceeded", func(t *testing.T) {
		err := context.DeadlineExceeded

		canRecover := strategy.CanRecover(err)
		assert.True(t, canRecover, "Should be able to recover from context deadline exceeded")
	})

	t.Run("Handles connection reset errors", func(t *testing.T) {
		err := errors.New("connection reset by peer")

		canRecover := strategy.CanRecover(err)
		assert.True(t, canRecover, "Should be able to recover from connection reset errors")
	})

	t.Run("Handles connection refused errors", func(t *testing.T) {
		err := errors.New("connection refused")

		canRecover := strategy.CanRecover(err)
		assert.True(t, canRecover, "Should be able to recover from connection refused errors")
	})

	t.Run("Handles no route to host errors", func(t *testing.T) {
		err := errors.New("no route to host")

		canRecover := strategy.CanRecover(err)
		assert.True(t, canRecover, "Should be able to recover from no route to host errors")
	})

	t.Run("Correctly identifies non-recoverable errors", func(t *testing.T) {
		err := errors.New("authentication failed")

		canRecover := strategy.CanRecover(err)
		assert.False(t, canRecover, "Should not recover from non-network errors")
	})

	t.Run("Does not rely on deprecated Temporary() method", func(t *testing.T) {
		// Create a mock error that would return true for Temporary() but false for Timeout()
		// This ensures we're using the new method
		mockErr := &mockNetError{timeout: false, temporary: true}

		canRecover := strategy.CanRecover(mockErr)
		assert.False(t, canRecover, "Should not recover based on deprecated Temporary() method")
	})
}

func TestRecoveryWithModernErrorHandling(t *testing.T) {
	attempts := 0
	strategy := &RetryRecoveryStrategy{
		MaxRetries: 2,
		RetryDelay: 1 * time.Millisecond,
		RetryableFunc: func(ctx context.Context) error {
			attempts++
			if attempts < 2 {
				return &mockNetError{timeout: true}
			}
			return nil // Success on second attempt
		},
	}

	t.Run("Successfully recovers from timeout error", func(t *testing.T) {
		attempts = 0
		timeoutErr := &mockNetError{timeout: true}

		ctx := context.Background()
		err := strategy.Recover(ctx, timeoutErr)

		assert.NoError(t, err, "Should successfully recover from timeout error")
		assert.Equal(t, 2, attempts, "Should have made 2 attempts")
	})
}

// mockNetError implements net.Error interface for testing
type mockNetError struct {
	timeout   bool
	temporary bool
	message   string
}

func (e *mockNetError) Error() string {
	if e.message != "" {
		return e.message
	}
	return "mock network error"
}

func (e *mockNetError) Timeout() bool {
	return e.timeout
}

func (e *mockNetError) Temporary() bool {
	return e.temporary
}

// Ensure mockNetError implements net.Error
var _ net.Error = (*mockNetError)(nil)
