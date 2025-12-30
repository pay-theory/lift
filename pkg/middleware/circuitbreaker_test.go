package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

func TestApplyCircuitBreakerDefaults(t *testing.T) {
	t.Parallel()

	cfg := applyCircuitBreakerDefaults(CircuitBreakerConfig{})

	if cfg.FailureThreshold != 5 {
		t.Fatalf("expected default failure threshold 5, got %d", cfg.FailureThreshold)
	}
	if cfg.SuccessThreshold != 3 {
		t.Fatalf("expected default success threshold 3, got %d", cfg.SuccessThreshold)
	}
	if cfg.Timeout != 60*time.Second {
		t.Fatalf("expected default timeout 60s, got %v", cfg.Timeout)
	}
	if cfg.ErrorRateThreshold != 0.5 {
		t.Fatalf("expected default error rate threshold 0.5, got %f", cfg.ErrorRateThreshold)
	}
	if cfg.MinRequestThreshold != 10 {
		t.Fatalf("expected default min request threshold 10, got %d", cfg.MinRequestThreshold)
	}
	if cfg.ShouldTrip == nil {
		t.Fatal("expected ShouldTrip default to be set")
	}
	if cfg.FallbackHandler == nil {
		t.Fatal("expected FallbackHandler default to be set")
	}
}

func TestCircuitBreakerTripsAndFallbackInvoked(t *testing.T) {
	t.Parallel()

	var shouldTripCalls int
	fallbackErr := errors.New("fallback invoked")
	var fallbackCalled bool

	cfg := applyCircuitBreakerDefaults(CircuitBreakerConfig{
		Name:             "test-cb",
		FailureThreshold: 2,
		ShouldTrip: func(err error) bool {
			if err != nil {
				shouldTripCalls++
			}
			return true
		},
		FallbackHandler: func(_ *lift.Context) error {
			fallbackCalled = true
			return fallbackErr
		},
	})

	manager := newCircuitBreakerManager(cfg)
	ctx := newCircuitBreakerContext()

	failErr := errors.New("boom")
	failingHandler := lift.HandlerFunc(func(*lift.Context) error {
		return failErr
	})

	if err := manager.handleRequest(ctx, failingHandler); !errors.Is(err, failErr) {
		t.Fatalf("expected first failure to return original error, got %v", err)
	}

	if err := manager.handleRequest(ctx, failingHandler); !errors.Is(err, failErr) {
		t.Fatalf("expected second failure to return original error, got %v", err)
	}

	err := manager.handleRequest(ctx, failingHandler)
	if !errors.Is(err, fallbackErr) {
		t.Fatalf("expected fallback error when circuit open, got %v", err)
	}
	if !fallbackCalled {
		t.Fatal("expected fallback handler to be invoked")
	}
	if shouldTripCalls < 2 {
		t.Fatalf("expected ShouldTrip to be called for each failure, got %d", shouldTripCalls)
	}

	breaker := manager.getBreakerForContext(ctx)
	if state := breaker.getState(); state != CircuitBreakerOpen {
		t.Fatalf("expected breaker to be open, got %s", state)
	}
}

func TestCircuitBreakerRecoveryToClosed(t *testing.T) {
	t.Parallel()

	cfg := applyCircuitBreakerDefaults(CircuitBreakerConfig{
		Name:             "recovery-cb",
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          10 * time.Millisecond,
	})

	manager := newCircuitBreakerManager(cfg)
	ctx := newCircuitBreakerContext()

	failErr := errors.New("boom")
	failingHandler := lift.HandlerFunc(func(*lift.Context) error {
		return failErr
	})
	successHandler := lift.HandlerFunc(func(*lift.Context) error {
		return nil
	})

	if err := manager.handleRequest(ctx, failingHandler); !errors.Is(err, failErr) {
		t.Fatalf("expected failure error, got %v", err)
	}

	breaker := manager.getBreakerForContext(ctx)
	if state := breaker.getState(); state != CircuitBreakerOpen {
		t.Fatalf("expected breaker to be open after failure, got %s", state)
	}

	breaker.mutex.Lock()
	breaker.nextRetryAt = time.Now().Add(-time.Second)
	breaker.stateChangedAt = time.Now().Add(-time.Second)
	breaker.mutex.Unlock()

	if err := manager.handleRequest(ctx, successHandler); err != nil {
		t.Fatalf("expected successful execution during half-open, got %v", err)
	}

	if state := breaker.getState(); state != CircuitBreakerClosed {
		t.Fatalf("expected breaker to recover to closed, got %s", state)
	}
}

func TestCircuitBreakerConfigHelpers(t *testing.T) {
	t.Parallel()

	tenant := NewTenantCircuitBreaker("tenant")
	if !tenant.PerTenant {
		t.Fatal("expected NewTenantCircuitBreaker to set PerTenant")
	}

	operation := NewOperationCircuitBreaker("operation")
	if !operation.PerOperation {
		t.Fatal("expected NewOperationCircuitBreaker to set PerOperation")
	}

	shouldTrip := func(error) bool { return false }
	fallback := func(*lift.Context) error { return nil }

	advanced := NewAdvancedCircuitBreaker("advanced", shouldTrip, fallback)
	if advanced.ShouldTrip == nil || advanced.FallbackHandler == nil {
		t.Fatal("expected NewAdvancedCircuitBreaker to set ShouldTrip and FallbackHandler")
	}
}

func TestCircuitBreakerGetStats(t *testing.T) {
	t.Parallel()

	cb := &circuitBreaker{
		state:                CircuitBreakerHalfOpen,
		failureCount:         2,
		successCount:         3,
		lastFailureTime:      time.Unix(1, 0),
		lastSuccessTime:      time.Unix(2, 0),
		stateChangedAt:       time.Unix(3, 0),
		nextRetryAt:          time.Unix(4, 0),
		consecutiveFailures:  1,
		consecutiveSuccesses: 2,
		requestHistory: []requestRecord{
			{success: true},
			{success: false},
		},
	}

	stats := cb.GetStats()
	if stats.TotalRequests != 5 {
		t.Fatalf("expected total requests 5, got %d", stats.TotalRequests)
	}
	if stats.ErrorRate != 0.5 {
		t.Fatalf("expected error rate 0.5, got %f", stats.ErrorRate)
	}
	if stats.State != CircuitBreakerHalfOpen {
		t.Fatalf("expected state %s, got %s", CircuitBreakerHalfOpen, stats.State)
	}
}

func newCircuitBreakerContext() *lift.Context {
	req := lift.NewRequest(nil)
	req.Method = "GET"
	req.Path = "/cb"

	ctx := lift.NewContext(context.Background(), req)
	ctx.Set("tenant_id", "tenant-123")
	return ctx
}
