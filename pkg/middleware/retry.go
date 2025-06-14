package middleware

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// RetryStrategy defines different retry strategies
type RetryStrategy string

const (
	RetryStrategyFixed       RetryStrategy = "fixed"       // Fixed delay between retries
	RetryStrategyLinear      RetryStrategy = "linear"      // Linear backoff
	RetryStrategyExponential RetryStrategy = "exponential" // Exponential backoff
	RetryStrategyCustom      RetryStrategy = "custom"      // Custom backoff function
)

// RetryConfig holds configuration for the retry middleware
type RetryConfig struct {
	// Basic retry settings
	MaxAttempts  int           `json:"max_attempts"`  // Maximum number of retry attempts
	InitialDelay time.Duration `json:"initial_delay"` // Initial delay before first retry
	MaxDelay     time.Duration `json:"max_delay"`     // Maximum delay between retries
	Strategy     RetryStrategy `json:"strategy"`      // Retry strategy to use

	// Backoff configuration
	BackoffMultiplier float64 `json:"backoff_multiplier"` // Multiplier for exponential backoff
	Jitter            bool    `json:"jitter"`             // Add random jitter to delays
	JitterRange       float64 `json:"jitter_range"`       // Jitter range (0.0-1.0)

	// Custom strategy
	CustomBackoff func(attempt int, lastDelay time.Duration) time.Duration `json:"-"` // Custom backoff function

	// Retry conditions
	RetryableErrors    []string         `json:"retryable_errors"`     // Specific error types to retry
	RetryCondition     func(error) bool `json:"-"`                    // Custom retry condition
	NonRetryableErrors []string         `json:"non_retryable_errors"` // Errors that should never be retried

	// HTTP-specific settings
	RetryableStatusCodes    []int `json:"retryable_status_codes"`     // HTTP status codes to retry
	NonRetryableStatusCodes []int `json:"non_retryable_status_codes"` // HTTP status codes to never retry

	// Context and timeouts
	PerAttemptTimeout time.Duration `json:"per_attempt_timeout"` // Timeout per individual attempt
	TotalTimeout      time.Duration `json:"total_timeout"`       // Total timeout for all attempts

	// Observability
	Logger        observability.StructuredLogger `json:"-"`
	Metrics       observability.MetricsCollector `json:"-"`
	EnableMetrics bool                           `json:"enable_metrics"`

	// Callbacks
	OnRetry  func(attempt int, err error, delay time.Duration) `json:"-"` // Called before each retry
	OnGiveUp func(attempts int, lastErr error)                 `json:"-"` // Called when giving up

	// Naming
	Name string `json:"name"` // Retry middleware name for metrics
}

// RetryStats provides statistics about retry performance
type RetryStats struct {
	Name              string        `json:"name"`
	TotalRequests     int64         `json:"total_requests"`
	RetriedRequests   int64         `json:"retried_requests"`
	SuccessfulRetries int64         `json:"successful_retries"`
	FailedRetries     int64         `json:"failed_retries"`
	TotalAttempts     int64         `json:"total_attempts"`
	AverageAttempts   float64       `json:"average_attempts"`
	MaxAttempts       int           `json:"max_attempts"`
	AverageDelay      time.Duration `json:"average_delay"`
	TotalDelay        time.Duration `json:"total_delay"`
}

// RetryMiddleware creates a retry middleware
func RetryMiddleware(config RetryConfig) lift.Middleware {
	// Set defaults
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay == 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Strategy == "" {
		config.Strategy = RetryStrategyExponential
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.JitterRange == 0 {
		config.JitterRange = 0.1 // 10% jitter
	}
	if config.PerAttemptTimeout == 0 {
		config.PerAttemptTimeout = 30 * time.Second
	}
	if config.TotalTimeout == 0 {
		config.TotalTimeout = 5 * time.Minute
	}
	if config.Name == "" {
		config.Name = "default"
	}
	if config.RetryCondition == nil {
		config.RetryCondition = defaultRetryCondition
	}

	// Initialize default retryable status codes if not provided
	if len(config.RetryableStatusCodes) == 0 {
		config.RetryableStatusCodes = []int{500, 502, 503, 504, 429}
	}

	// Initialize default non-retryable status codes if not provided
	if len(config.NonRetryableStatusCodes) == 0 {
		config.NonRetryableStatusCodes = []int{400, 401, 403, 404, 422}
	}

	retrier := &retryManager{
		config: config,
		stats: &RetryStats{
			Name:        config.Name,
			MaxAttempts: config.MaxAttempts,
		},
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			return retrier.executeWithRetry(ctx, next)
		})
	}
}

// retryManager manages retry logic and statistics
type retryManager struct {
	config RetryConfig
	stats  *RetryStats
	mu     sync.RWMutex // Protects stats from concurrent access
}

// executeWithRetry executes the handler with retry logic
func (rm *retryManager) executeWithRetry(ctx *lift.Context, handler lift.Handler) error {
	totalStart := time.Now()
	var lastErr error
	var totalDelay time.Duration

	// Create total timeout context
	totalCtx := ctx.Context
	if rm.config.TotalTimeout > 0 {
		var cancel context.CancelFunc
		totalCtx, cancel = context.WithTimeout(ctx.Context, rm.config.TotalTimeout)
		defer cancel()
	}

	for attempt := 1; attempt <= rm.config.MaxAttempts; attempt++ {
		// Create per-attempt timeout context
		attemptCtx := totalCtx
		if rm.config.PerAttemptTimeout > 0 {
			var cancel context.CancelFunc
			attemptCtx, cancel = context.WithTimeout(totalCtx, rm.config.PerAttemptTimeout)
			defer cancel()
		}

		// Update context for this attempt
		originalCtx := ctx.Context
		ctx.Context = attemptCtx

		// Execute the handler
		attemptStart := time.Now()
		err := handler.Handle(ctx)
		attemptDuration := time.Since(attemptStart)

		// Restore original context
		ctx.Context = originalCtx

		// Record attempt metrics
		if rm.config.EnableMetrics && rm.config.Metrics != nil {
			rm.recordAttempt(attempt, err, attemptDuration)
		}

		// Check if request was successful
		if err == nil {
			// Success - record stats and return
			totalDuration := time.Since(totalStart)
			rm.recordSuccess(attempt, totalDuration, totalDelay)

			if rm.config.Logger != nil {
				rm.config.Logger.Info("Request succeeded", map[string]interface{}{
					"retry_name":     rm.config.Name,
					"attempt":        attempt,
					"total_duration": totalDuration.String(),
					"total_delay":    totalDelay.String(),
				})
			}

			return nil
		}

		lastErr = err

		// Check if we should retry this error
		if !rm.shouldRetry(err, attempt) {
			// Don't retry - record failure and return
			totalDuration := time.Since(totalStart)
			rm.recordFailure(attempt, totalDuration, totalDelay, err)

			if rm.config.Logger != nil {
				rm.config.Logger.Error("Request failed (not retryable)", map[string]interface{}{
					"retry_name":     rm.config.Name,
					"attempt":        attempt,
					"error":          err.Error(),
					"total_duration": totalDuration.String(),
				})
			}

			if rm.config.OnGiveUp != nil {
				rm.config.OnGiveUp(attempt, err)
			}

			return err
		}

		// Check if we've reached max attempts
		if attempt >= rm.config.MaxAttempts {
			// Max attempts reached - record failure and return
			totalDuration := time.Since(totalStart)
			rm.recordFailure(attempt, totalDuration, totalDelay, err)

			if rm.config.Logger != nil {
				rm.config.Logger.Error("Request failed after max attempts", map[string]interface{}{
					"retry_name":     rm.config.Name,
					"max_attempts":   rm.config.MaxAttempts,
					"error":          err.Error(),
					"total_duration": totalDuration.String(),
					"total_delay":    totalDelay.String(),
				})
			}

			if rm.config.OnGiveUp != nil {
				rm.config.OnGiveUp(attempt, err)
			}

			return err
		}

		// Calculate delay for next attempt
		delay := rm.calculateDelay(attempt, totalDelay)
		totalDelay += delay

		// Check if total timeout would be exceeded
		if time.Since(totalStart)+delay > rm.config.TotalTimeout {
			totalDuration := time.Since(totalStart)
			rm.recordFailure(attempt, totalDuration, totalDelay-delay, err)

			if rm.config.Logger != nil {
				rm.config.Logger.Error("Request failed due to total timeout", map[string]interface{}{
					"retry_name":     rm.config.Name,
					"attempt":        attempt,
					"total_timeout":  rm.config.TotalTimeout.String(),
					"total_duration": totalDuration.String(),
				})
			}

			if rm.config.OnGiveUp != nil {
				rm.config.OnGiveUp(attempt, err)
			}

			return err
		}

		// Log retry attempt
		if rm.config.Logger != nil {
			rm.config.Logger.Warn("Request failed, retrying", map[string]interface{}{
				"retry_name":   rm.config.Name,
				"attempt":      attempt,
				"next_attempt": attempt + 1,
				"delay":        delay.String(),
				"error":        err.Error(),
			})
		}

		// Call retry callback
		if rm.config.OnRetry != nil {
			rm.config.OnRetry(attempt, err, delay)
		}

		// Wait for delay (with context cancellation support)
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-totalCtx.Done():
			// Context cancelled during delay
			totalDuration := time.Since(totalStart)
			rm.recordFailure(attempt, totalDuration, totalDelay, totalCtx.Err())
			return totalCtx.Err()
		}
	}

	// This should never be reached, but just in case
	return lastErr
}

// shouldRetry determines if an error should be retried
func (rm *retryManager) shouldRetry(err error, attempt int) bool {
	_ = attempt // TODO: Consider attempt-based retry logic (e.g., different rules for different attempts)
	// Check custom retry condition first
	if !rm.config.RetryCondition(err) {
		return false
	}

	// Check non-retryable errors
	for _, nonRetryableError := range rm.config.NonRetryableErrors {
		if fmt.Sprintf("%T", err) == nonRetryableError {
			return false
		}
	}

	// Check retryable errors (if specified)
	if len(rm.config.RetryableErrors) > 0 {
		for _, retryableError := range rm.config.RetryableErrors {
			if fmt.Sprintf("%T", err) == retryableError {
				return true
			}
		}
		return false // Not in retryable list
	}

	// For HTTP errors, check status codes
	if liftErr, ok := err.(*lift.LiftError); ok {
		// Check non-retryable status codes
		for _, code := range rm.config.NonRetryableStatusCodes {
			if liftErr.StatusCode == code {
				return false
			}
		}

		// Check retryable status codes
		for _, code := range rm.config.RetryableStatusCodes {
			if liftErr.StatusCode == code {
				return true
			}
		}

		// Default: don't retry HTTP errors not in retryable list
		return false
	}

	// Default: retry non-HTTP errors
	return true
}

// calculateDelay calculates the delay for the next retry attempt
func (rm *retryManager) calculateDelay(attempt int, totalDelay time.Duration) time.Duration {
	var delay time.Duration

	switch rm.config.Strategy {
	case RetryStrategyFixed:
		delay = rm.config.InitialDelay

	case RetryStrategyLinear:
		delay = time.Duration(attempt) * rm.config.InitialDelay

	case RetryStrategyExponential:
		delay = time.Duration(float64(rm.config.InitialDelay) * math.Pow(rm.config.BackoffMultiplier, float64(attempt-1)))

	case RetryStrategyCustom:
		if rm.config.CustomBackoff != nil {
			delay = rm.config.CustomBackoff(attempt, totalDelay)
		} else {
			delay = rm.config.InitialDelay
		}

	default:
		delay = rm.config.InitialDelay
	}

	// Apply maximum delay limit
	if delay > rm.config.MaxDelay {
		delay = rm.config.MaxDelay
	}

	// Apply jitter if enabled
	if rm.config.Jitter {
		jitterAmount := float64(delay) * rm.config.JitterRange
		jitter := (rand.Float64() - 0.5) * 2 * jitterAmount // Random value between -jitterAmount and +jitterAmount
		delay = time.Duration(float64(delay) + jitter)

		// Ensure delay is not negative
		if delay < 0 {
			delay = time.Millisecond
		}
	}

	return delay
}

// recordAttempt records metrics for an individual attempt
func (rm *retryManager) recordAttempt(attempt int, err error, duration time.Duration) {
	if !rm.config.EnableMetrics || rm.config.Metrics == nil {
		return
	}

	tags := map[string]string{
		"retry_name": rm.config.Name,
		"attempt":    fmt.Sprintf("%d", attempt),
		"result":     map[bool]string{true: "success", false: "failure"}[err == nil],
	}

	metrics := rm.config.Metrics.WithTags(tags)

	// Record attempt
	counter := metrics.Counter("retry.attempts.total")
	counter.Inc()

	// Record attempt duration
	histogram := metrics.Histogram("retry.attempt.duration")
	histogram.Observe(float64(duration.Milliseconds()))
}

// recordSuccess records metrics for a successful request
func (rm *retryManager) recordSuccess(attempts int, totalDuration, totalDelay time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.stats.TotalRequests++
	rm.stats.TotalAttempts += int64(attempts)
	rm.stats.TotalDelay += totalDelay

	if attempts > 1 {
		rm.stats.RetriedRequests++
		rm.stats.SuccessfulRetries++
	}

	rm.stats.AverageAttempts = float64(rm.stats.TotalAttempts) / float64(rm.stats.TotalRequests)
	if rm.stats.TotalRequests > 0 {
		rm.stats.AverageDelay = time.Duration(int64(rm.stats.TotalDelay) / rm.stats.TotalRequests)
	}

	if !rm.config.EnableMetrics || rm.config.Metrics == nil {
		return
	}

	tags := map[string]string{
		"retry_name": rm.config.Name,
		"result":     "success",
	}

	metrics := rm.config.Metrics.WithTags(tags)

	// Record success
	counter := metrics.Counter("retry.requests.total")
	counter.Inc()

	// Record total duration
	histogram := metrics.Histogram("retry.total.duration")
	histogram.Observe(float64(totalDuration.Milliseconds()))

	// Record total delay
	delayHistogram := metrics.Histogram("retry.total.delay")
	delayHistogram.Observe(float64(totalDelay.Milliseconds()))

	// Record attempt count
	attemptHistogram := metrics.Histogram("retry.attempts.count")
	attemptHistogram.Observe(float64(attempts))
}

// recordFailure records metrics for a failed request
func (rm *retryManager) recordFailure(attempts int, totalDuration, totalDelay time.Duration, err error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.stats.TotalRequests++
	rm.stats.TotalAttempts += int64(attempts)
	rm.stats.TotalDelay += totalDelay

	if attempts > 1 {
		rm.stats.RetriedRequests++
		rm.stats.FailedRetries++
	}

	rm.stats.AverageAttempts = float64(rm.stats.TotalAttempts) / float64(rm.stats.TotalRequests)
	if rm.stats.TotalRequests > 0 {
		rm.stats.AverageDelay = time.Duration(int64(rm.stats.TotalDelay) / rm.stats.TotalRequests)
	}

	if !rm.config.EnableMetrics || rm.config.Metrics == nil {
		return
	}

	tags := map[string]string{
		"retry_name": rm.config.Name,
		"result":     "failure",
		"error_type": fmt.Sprintf("%T", err),
	}

	metrics := rm.config.Metrics.WithTags(tags)

	// Record failure
	counter := metrics.Counter("retry.requests.total")
	counter.Inc()

	// Record total duration
	histogram := metrics.Histogram("retry.total.duration")
	histogram.Observe(float64(totalDuration.Milliseconds()))

	// Record total delay
	delayHistogram := metrics.Histogram("retry.total.delay")
	delayHistogram.Observe(float64(totalDelay.Milliseconds()))

	// Record attempt count
	attemptHistogram := metrics.Histogram("retry.attempts.count")
	attemptHistogram.Observe(float64(attempts))
}

// GetStats returns current retry statistics
func (rm *retryManager) GetStats() RetryStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return *rm.stats
}

// Default implementations

// defaultRetryCondition determines if an error should be retried by default
func defaultRetryCondition(err error) bool {
	// Don't retry context cancellation or timeout errors
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Don't retry validation errors
	if liftErr, ok := err.(*lift.LiftError); ok && liftErr.Code == "VALIDATION_ERROR" {
		return false
	}

	// Retry other errors by default
	return true
}

// Utility functions for common retry configurations

// NewBasicRetry creates a basic retry configuration with exponential backoff
func NewBasicRetry(name string, maxAttempts int) RetryConfig {
	return RetryConfig{
		Name:              name,
		MaxAttempts:       maxAttempts,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		Strategy:          RetryStrategyExponential,
		BackoffMultiplier: 2.0,
		Jitter:            true,
		JitterRange:       0.1,
		PerAttemptTimeout: 30 * time.Second,
		TotalTimeout:      5 * time.Minute,
		EnableMetrics:     true,
	}
}

// NewHTTPRetry creates a retry configuration optimized for HTTP requests
func NewHTTPRetry(name string, maxAttempts int) RetryConfig {
	config := NewBasicRetry(name, maxAttempts)
	config.RetryableStatusCodes = []int{500, 502, 503, 504, 429}
	config.NonRetryableStatusCodes = []int{400, 401, 403, 404, 422}
	return config
}

// NewDatabaseRetry creates a retry configuration optimized for database operations
func NewDatabaseRetry(name string, maxAttempts int) RetryConfig {
	config := NewBasicRetry(name, maxAttempts)
	config.InitialDelay = 50 * time.Millisecond
	config.MaxDelay = 5 * time.Second
	config.BackoffMultiplier = 1.5
	return config
}

// NewCustomRetry creates a retry configuration with custom backoff
func NewCustomRetry(name string, maxAttempts int, backoffFunc func(int, time.Duration) time.Duration) RetryConfig {
	config := NewBasicRetry(name, maxAttempts)
	config.Strategy = RetryStrategyCustom
	config.CustomBackoff = backoffFunc
	return config
}
