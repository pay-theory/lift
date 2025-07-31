package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"    // Normal operation
	CircuitBreakerOpen     CircuitBreakerState = "open"      // Failing fast
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open" // Testing recovery
)

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	Metrics               observability.MetricsCollector                 `json:"-"`
	Logger                observability.StructuredLogger                 `json:"-"`
	ShouldTrip            func(error) bool                               `json:"-"`
	OnStateChange         func(CircuitBreakerState, CircuitBreakerState) `json:"-"`
	FallbackHandler       func(*lift.Context) error                      `json:"-"`
	Name                  string                                         `json:"name"`
	MinRequestThreshold   int                                            `json:"min_request_threshold"`
	RetryBackoff          time.Duration                                  `json:"retry_backoff"`
	MaxRetryAttempts      int                                            `json:"max_retry_attempts"`
	SlidingWindowSize     time.Duration                                  `json:"sliding_window_size"`
	FailureThreshold      int                                            `json:"failure_threshold"`
	ErrorRateThreshold    float64                                        `json:"error_rate_threshold"`
	Timeout               time.Duration                                  `json:"timeout"`
	SuccessThreshold      int                                            `json:"success_threshold"`
	PerTenant             bool                                           `json:"per_tenant"`
	PerOperation          bool                                           `json:"per_operation"`
	EnableTenantIsolation bool                                           `json:"enable_tenant_isolation"`
	EnableMetrics         bool                                           `json:"enable_metrics"`
}

// CircuitBreakerStats provides statistics about circuit breaker performance
type CircuitBreakerStats struct {
	LastFailure          time.Time           `json:"last_failure"`
	LastSuccess          time.Time           `json:"last_success"`
	StateChangedAt       time.Time           `json:"state_changed_at"`
	NextRetryAt          time.Time           `json:"next_retry_at,omitempty"`
	State                CircuitBreakerState `json:"state"`
	FailureCount         int64               `json:"failure_count"`
	SuccessCount         int64               `json:"success_count"`
	TotalRequests        int64               `json:"total_requests"`
	ErrorRate            float64             `json:"error_rate"`
	ConsecutiveFailures  int                 `json:"consecutive_failures"`
	ConsecutiveSuccesses int                 `json:"consecutive_successes"`
}

// CircuitBreakerMiddleware creates a circuit breaker middleware
func CircuitBreakerMiddleware(config CircuitBreakerConfig) lift.Middleware {
	// Set defaults
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 3
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.ErrorRateThreshold == 0 {
		config.ErrorRateThreshold = 0.5 // 50% error rate
	}
	if config.MinRequestThreshold == 0 {
		config.MinRequestThreshold = 10
	}
	if config.SlidingWindowSize == 0 {
		config.SlidingWindowSize = 5 * time.Minute
	}
	if config.MaxRetryAttempts == 0 {
		config.MaxRetryAttempts = 3
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = 5 * time.Second
	}
	if config.Name == "" {
		config.Name = "default"
	}
	if config.ShouldTrip == nil {
		config.ShouldTrip = defaultShouldTrip
	}
	if config.FallbackHandler == nil {
		config.FallbackHandler = defaultFallbackHandler
	}

	manager := &circuitBreakerManager{
		config:   config,
		breakers: make(map[string]*circuitBreaker),
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Get or create circuit breaker for this context
			breaker := manager.getBreakerForContext(ctx)

			// Check if circuit breaker allows the request
			if !breaker.allowRequest() {
				// Circuit is open, execute fallback
				if config.Logger != nil {
					config.Logger.Warn("Circuit breaker open, executing fallback", map[string]any{
						"breaker_name": breaker.name,
						"state":        breaker.getState(),
						"tenant_id":    ctx.TenantID(),
						"operation":    ctx.Request.Path,
					})
				}

				// Record metrics
				if config.EnableMetrics && config.Metrics != nil {
					tags := map[string]string{
						"breaker_name": breaker.name,
						"state":        string(breaker.getState()),
						"action":       "fallback",
					}
					if config.PerTenant || config.EnableTenantIsolation {
						tags["tenant_id"] = ctx.TenantID()
					}

					metrics := config.Metrics.WithTags(tags)
					counter := metrics.Counter("circuit_breaker.fallback.total")
					counter.Inc()
				}

				return config.FallbackHandler(ctx)
			}

			// Execute the request
			start := time.Now()
			err := next.Handle(ctx)
			duration := time.Since(start)

			// Record the result
			if err != nil && config.ShouldTrip(err) {
				breaker.recordFailure()

				if config.Logger != nil {
					config.Logger.Error("Circuit breaker recorded failure", map[string]any{
						"breaker_name": breaker.name,
						"error":        "[REDACTED_ERROR_DETAIL]", // Sanitized for security
						"duration":     duration.String(),
						"tenant_id":    ctx.TenantID(),
					})
				}
			} else {
				breaker.recordSuccess()

				if config.Logger != nil {
					config.Logger.Debug("Circuit breaker recorded success", map[string]any{
						"breaker_name": breaker.name,
						"duration":     duration.String(),
						"tenant_id":    ctx.TenantID(),
					})
				}
			}

			// Record metrics
			if config.EnableMetrics && config.Metrics != nil {
				tags := map[string]string{
					"breaker_name": breaker.name,
					"state":        string(breaker.getState()),
					"result":       map[bool]string{true: "success", false: "failure"}[err == nil],
				}
				if config.PerTenant || config.EnableTenantIsolation {
					tags["tenant_id"] = ctx.TenantID()
				}

				metrics := config.Metrics.WithTags(tags)

				// Record request
				counter := metrics.Counter("circuit_breaker.requests.total")
				counter.Inc()

				// Record duration
				histogram := metrics.Histogram("circuit_breaker.request.duration")
				histogram.Observe(float64(duration.Milliseconds()))

				// Record state
				gauge := metrics.Gauge("circuit_breaker.state")
				stateValue := map[CircuitBreakerState]float64{
					CircuitBreakerClosed:   0,
					CircuitBreakerOpen:     1,
					CircuitBreakerHalfOpen: 0.5,
				}
				gauge.Set(stateValue[breaker.getState()])
			}

			return err
		})
	}
}

// circuitBreakerManager manages multiple circuit breakers
type circuitBreakerManager struct {
	breakers map[string]*circuitBreaker
	config   CircuitBreakerConfig
	mutex    sync.RWMutex
}

// getBreakerForContext returns the appropriate circuit breaker for the context
func (m *circuitBreakerManager) getBreakerForContext(ctx *lift.Context) *circuitBreaker {
	key := m.generateBreakerKey(ctx)

	m.mutex.RLock()
	breaker, exists := m.breakers[key]
	m.mutex.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Double-check after acquiring write lock
	if existingBreaker, exists := m.breakers[key]; exists {
		return existingBreaker
	}

	breaker = newCircuitBreaker(key, m.config)
	m.breakers[key] = breaker
	return breaker
}

// generateBreakerKey creates a unique key for the circuit breaker
func (m *circuitBreakerManager) generateBreakerKey(ctx *lift.Context) string {
	parts := []string{m.config.Name}

	// Use EnableTenantIsolation as an alias for PerTenant
	if m.config.PerTenant || m.config.EnableTenantIsolation {
		parts = append(parts, "tenant", ctx.TenantID())
	}

	if m.config.PerOperation {
		parts = append(parts, "op", fmt.Sprintf("%s:%s", ctx.Request.Method, ctx.Request.Path))
	}

	key := ""
	for i, part := range parts {
		if i > 0 {
			key += ":"
		}
		key += part
	}

	return key
}

// circuitBreaker implements the circuit breaker logic
type circuitBreaker struct {
	// Largest fields first
	mutex          sync.RWMutex         // 24 bytes
	requestHistory []requestRecord      // 24 bytes (slice)
	config         CircuitBreakerConfig // struct

	// 8-byte fields (time.Time is struct with int64 + ptr)
	lastSuccessTime time.Time
	nextRetryAt     time.Time
	stateChangedAt  time.Time
	lastFailureTime time.Time
	failureCount    int64
	successCount    int64

	// String field (16 bytes)
	name string

	// 4-byte fields
	consecutiveSuccesses int
	consecutiveFailures  int

	// Smaller field last
	state CircuitBreakerState
}

// requestRecord tracks individual request results for sliding window analysis
type requestRecord struct {
	timestamp time.Time
	success   bool
}

// newCircuitBreaker creates a new circuit breaker instance
func newCircuitBreaker(name string, config CircuitBreakerConfig) *circuitBreaker {
	return &circuitBreaker{
		name:           name,
		config:         config,
		state:          CircuitBreakerClosed,
		stateChangedAt: time.Now(),
		requestHistory: make([]requestRecord, 0),
	}
}

// allowRequest determines if a request should be allowed through
func (cb *circuitBreaker) allowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if timeout has passed
		if time.Now().After(cb.nextRetryAt) {
			// Transition to half-open
			cb.transitionToHalfOpen()
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		// Allow limited requests in half-open state
		return cb.consecutiveSuccesses < cb.config.MaxRetryAttempts
	default:
		return false
	}
}

// recordSuccess records a successful request
func (cb *circuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.successCount++
	cb.consecutiveSuccesses++
	cb.consecutiveFailures = 0
	cb.lastSuccessTime = time.Now()

	// Add to history
	cb.addToHistory(true)

	// Check for state transitions
	if cb.state == CircuitBreakerHalfOpen && cb.consecutiveSuccesses >= cb.config.SuccessThreshold {
		cb.transitionToClosed()
	}
}

// recordFailure records a failed request
func (cb *circuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.consecutiveFailures++
	cb.consecutiveSuccesses = 0
	cb.lastFailureTime = time.Now()

	// Add to history
	cb.addToHistory(false)

	// Check for state transitions
	switch cb.state {
	case CircuitBreakerClosed:
		// Check failure threshold
		if cb.consecutiveFailures >= cb.config.FailureThreshold {
			cb.transitionToOpen()
		} else {
			// Check error rate threshold
			errorRate := cb.calculateErrorRate()
			totalRequests := cb.successCount + cb.failureCount
			if totalRequests >= int64(cb.config.MinRequestThreshold) && errorRate >= cb.config.ErrorRateThreshold {
				cb.transitionToOpen()
			}
		}
	case CircuitBreakerHalfOpen:
		// Any failure in half-open state transitions back to open
		cb.transitionToOpen()
	}
}

// addToHistory adds a request record to the sliding window history
func (cb *circuitBreaker) addToHistory(success bool) {
	now := time.Now()
	record := requestRecord{timestamp: now, success: success}

	// Add new record
	cb.requestHistory = append(cb.requestHistory, record)

	// Remove old records outside the sliding window
	cutoff := now.Add(-cb.config.SlidingWindowSize)
	for i, r := range cb.requestHistory {
		if r.timestamp.After(cutoff) {
			cb.requestHistory = cb.requestHistory[i:]
			break
		}
	}
}

// calculateErrorRate calculates the error rate within the sliding window
func (cb *circuitBreaker) calculateErrorRate() float64 {
	if len(cb.requestHistory) == 0 {
		return 0.0
	}

	failures := 0
	for _, record := range cb.requestHistory {
		if !record.success {
			failures++
		}
	}

	return float64(failures) / float64(len(cb.requestHistory))
}

// transitionToOpen transitions the circuit breaker to open state
func (cb *circuitBreaker) transitionToOpen() {
	oldState := cb.state
	cb.state = CircuitBreakerOpen
	cb.stateChangedAt = time.Now()
	cb.nextRetryAt = time.Now().Add(cb.config.Timeout)

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, cb.state)
	}
}

// transitionToHalfOpen transitions the circuit breaker to half-open state
func (cb *circuitBreaker) transitionToHalfOpen() {
	oldState := cb.state
	cb.state = CircuitBreakerHalfOpen
	cb.stateChangedAt = time.Now()
	cb.consecutiveSuccesses = 0
	cb.consecutiveFailures = 0

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, cb.state)
	}
}

// transitionToClosed transitions the circuit breaker to closed state
func (cb *circuitBreaker) transitionToClosed() {
	oldState := cb.state
	cb.state = CircuitBreakerClosed
	cb.stateChangedAt = time.Now()
	cb.consecutiveFailures = 0

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, cb.state)
	}
}

// getState returns the current state of the circuit breaker
func (cb *circuitBreaker) getState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *circuitBreaker) GetStats() CircuitBreakerStats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return CircuitBreakerStats{
		State:                cb.state,
		FailureCount:         cb.failureCount,
		SuccessCount:         cb.successCount,
		TotalRequests:        cb.successCount + cb.failureCount,
		ErrorRate:            cb.calculateErrorRate(),
		LastFailure:          cb.lastFailureTime,
		LastSuccess:          cb.lastSuccessTime,
		StateChangedAt:       cb.stateChangedAt,
		NextRetryAt:          cb.nextRetryAt,
		ConsecutiveFailures:  cb.consecutiveFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
	}
}

// Default implementations

// defaultShouldTrip determines if an error should trip the circuit breaker
func defaultShouldTrip(err error) bool {
	// Trip on any error by default
	// In practice, you might want to exclude certain errors like validation errors
	return err != nil
}

// defaultFallbackHandler provides a default fallback response
func defaultFallbackHandler(ctx *lift.Context) error {
	return ctx.Status(503).JSON(map[string]any{
		"error":   "Service temporarily unavailable",
		"message": "Circuit breaker is open",
		"code":    "CIRCUIT_BREAKER_OPEN",
	})
}

// Utility functions for common circuit breaker configurations

// NewBasicCircuitBreaker creates a basic circuit breaker with sensible defaults
func NewBasicCircuitBreaker(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:                name,
		FailureThreshold:    5,
		SuccessThreshold:    3,
		Timeout:             60 * time.Second,
		ErrorRateThreshold:  0.5,
		MinRequestThreshold: 10,
		SlidingWindowSize:   5 * time.Minute,
		MaxRetryAttempts:    3,
		RetryBackoff:        5 * time.Second,
		EnableMetrics:       true,
	}
}

// NewTenantCircuitBreaker creates a per-tenant circuit breaker
func NewTenantCircuitBreaker(name string) CircuitBreakerConfig {
	config := NewBasicCircuitBreaker(name)
	config.PerTenant = true
	return config
}

// NewOperationCircuitBreaker creates a per-operation circuit breaker
func NewOperationCircuitBreaker(name string) CircuitBreakerConfig {
	config := NewBasicCircuitBreaker(name)
	config.PerOperation = true
	return config
}

// NewAdvancedCircuitBreaker creates a circuit breaker with custom failure detection
func NewAdvancedCircuitBreaker(name string, shouldTrip func(error) bool, fallback func(*lift.Context) error) CircuitBreakerConfig {
	config := NewBasicCircuitBreaker(name)
	config.ShouldTrip = shouldTrip
	config.FallbackHandler = fallback
	return config
}
