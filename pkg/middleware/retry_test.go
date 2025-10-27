package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

type retryableErr struct{}

func (retryableErr) Error() string { return "retryable" }

type nonRetryableErr struct{}

func (nonRetryableErr) Error() string { return "non-retryable" }

func TestRetryManagerShouldRetryRespectsConfiguration(t *testing.T) {
	t.Parallel()

	errNever := errors.New("never retry")

	typeBased := &retryManager{
		config: RetryConfig{
			RetryCondition: func(err error) bool {
				return !errors.Is(err, errNever)
			},
			RetryableErrors:    []string{"*middleware.retryableErr"},
			NonRetryableErrors: []string{"*middleware.nonRetryableErr"},
		},
	}

	if typeBased.shouldRetry(errNever, 1) {
		t.Fatal("expected RetryCondition to prevent retry")
	}

	if typeBased.shouldRetry(&nonRetryableErr{}, 1) {
		t.Fatal("expected non-retryable error type to prevent retry")
	}

	if !typeBased.shouldRetry(&retryableErr{}, 1) {
		t.Fatal("expected retryable error type to allow retry")
	}

	httpBased := &retryManager{
		config: RetryConfig{
			RetryCondition: func(err error) bool {
				return !errors.Is(err, errNever)
			},
			RetryableStatusCodes:    []int{500, 503},
			NonRetryableStatusCodes: []int{400},
		},
	}

	httpRetry := lift.NewLiftError("SERVER_ERROR", "retry me", 503)
	if !httpBased.shouldRetry(httpRetry, 1) {
		t.Fatal("expected retryable HTTP status code to allow retry")
	}

	httpNoRetry := lift.NewLiftError("BAD_REQUEST", "do not retry", 400)
	if httpBased.shouldRetry(httpNoRetry, 1) {
		t.Fatal("expected non-retryable HTTP status code to block retry")
	}

	defaultRM := &retryManager{
		config: RetryConfig{
			RetryCondition: func(error) bool { return true },
		},
	}
	if !defaultRM.shouldRetry(errors.New("other"), 1) {
		t.Fatal("expected default case to retry")
	}
}

func TestRetryManagerCalculateDelayStrategies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      RetryConfig
		attempt     int
		totalDelay  time.Duration
		want        time.Duration
		description string
	}{
		{
			name: "fixed backoff",
			config: RetryConfig{
				Strategy:     RetryStrategyFixed,
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     time.Second,
				Jitter:       false,
			},
			attempt: 3,
			want:    100 * time.Millisecond,
		},
		{
			name: "linear backoff",
			config: RetryConfig{
				Strategy:     RetryStrategyLinear,
				InitialDelay: 50 * time.Millisecond,
				MaxDelay:     time.Second,
				Jitter:       false,
			},
			attempt: 4,
			want:    200 * time.Millisecond,
		},
		{
			name: "exponential backoff",
			config: RetryConfig{
				Strategy:          RetryStrategyExponential,
				InitialDelay:      25 * time.Millisecond,
				BackoffMultiplier: 2,
				MaxDelay:          time.Second,
				Jitter:            false,
			},
			attempt: 3,
			want:    100 * time.Millisecond,
		},
		{
			name: "max delay clamp",
			config: RetryConfig{
				Strategy:          RetryStrategyExponential,
				InitialDelay:      200 * time.Millisecond,
				BackoffMultiplier: 3,
				MaxDelay:          400 * time.Millisecond,
				Jitter:            false,
			},
			attempt: 3,
			want:    400 * time.Millisecond,
		},
		{
			name: "custom backoff",
			config: RetryConfig{
				Strategy:     RetryStrategyCustom,
				InitialDelay: 10 * time.Millisecond,
				MaxDelay:     time.Second,
				Jitter:       false,
				CustomBackoff: func(attempt int, totalDelay time.Duration) time.Duration {
					return time.Duration(attempt) * 15 * time.Millisecond
				},
			},
			attempt:    3,
			totalDelay: 30 * time.Millisecond,
			want:       45 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rm := &retryManager{config: tt.config}
			got := rm.calculateDelay(tt.attempt, tt.totalDelay)
			if got != tt.want {
				t.Fatalf("calculateDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestRetryExecutionSuccessAfterRetries(t *testing.T) {
	t.Parallel()

	var attempts int
	var onRetryCalls []int
	var onRetryDelays []time.Duration
	var giveUpCalled bool

	cfg := RetryConfig{
		Name:         "retry-success",
		MaxAttempts:  3,
		Strategy:     RetryStrategyFixed,
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Jitter:       false,
		RetryCondition: func(error) bool {
			return true
		},
		OnRetry: func(attempt int, _ error, delay time.Duration) {
			onRetryCalls = append(onRetryCalls, attempt)
			onRetryDelays = append(onRetryDelays, delay)
		},
		OnGiveUp: func(_ int, _ error) {
			giveUpCalled = true
		},
	}

	rm := &retryManager{
		config: cfg,
		stats: &RetryStats{
			Name:        cfg.Name,
			MaxAttempts: cfg.MaxAttempts,
		},
	}

	ctx := newRetryContext()
	handler := lift.HandlerFunc(func(*lift.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	execution := newRetryExecution(rm, ctx, handler)
	if err := execution.execute(); err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}

	if len(onRetryCalls) != 2 || onRetryCalls[0] != 1 || onRetryCalls[1] != 2 {
		t.Fatalf("unexpected OnRetry calls: %v", onRetryCalls)
	}
	for _, d := range onRetryDelays {
		if d != cfg.InitialDelay {
			t.Fatalf("expected OnRetry delay %v, got %v", cfg.InitialDelay, d)
		}
	}

	if giveUpCalled {
		t.Fatal("expected OnGiveUp not to be called on eventual success")
	}

	stats := rm.GetStats()
	if stats.TotalRequests != 1 {
		t.Fatalf("expected total requests 1, got %d", stats.TotalRequests)
	}
	if stats.TotalAttempts != 3 {
		t.Fatalf("expected total attempts 3, got %d", stats.TotalAttempts)
	}
	if stats.SuccessfulRetries != 1 {
		t.Fatalf("expected successful retries 1, got %d", stats.SuccessfulRetries)
	}
	if stats.FailedRetries != 0 {
		t.Fatalf("expected failed retries 0, got %d", stats.FailedRetries)
	}
	expectedDelay := 2 * cfg.InitialDelay
	if stats.TotalDelay != expectedDelay {
		t.Fatalf("expected total delay %v, got %v", expectedDelay, stats.TotalDelay)
	}
}

func TestRetryExecutionGiveUp(t *testing.T) {
	t.Parallel()

	var attempts int
	var onRetryCalls []int
	var giveUpCalled bool

	maxAttempts := 2
	cfg := RetryConfig{
		Name:         "retry-giveup",
		MaxAttempts:  maxAttempts,
		Strategy:     RetryStrategyFixed,
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Jitter:       false,
		RetryCondition: func(error) bool {
			return true
		},
		OnRetry: func(attempt int, _ error, _ time.Duration) {
			onRetryCalls = append(onRetryCalls, attempt)
		},
	}
	cfg.OnGiveUp = func(a int, _ error) {
		giveUpCalled = true
		if a != maxAttempts {
			t.Fatalf("expected OnGiveUp attempts %d, got %d", maxAttempts, a)
		}
	}

	rm := &retryManager{
		config: cfg,
		stats: &RetryStats{
			Name:        cfg.Name,
			MaxAttempts: cfg.MaxAttempts,
		},
	}

	ctx := newRetryContext()
	handler := lift.HandlerFunc(func(*lift.Context) error {
		attempts++
		return errors.New("persistent failure")
	})

	execution := newRetryExecution(rm, ctx, handler)
	if err := execution.execute(); err == nil {
		t.Fatal("expected execute() to return error after exhausting retries")
	}

	if attempts != cfg.MaxAttempts {
		t.Fatalf("expected attempts %d, got %d", cfg.MaxAttempts, attempts)
	}

	if len(onRetryCalls) != 1 || onRetryCalls[0] != 1 {
		t.Fatalf("unexpected OnRetry calls: %v", onRetryCalls)
	}

	if !giveUpCalled {
		t.Fatal("expected OnGiveUp to be invoked")
	}

	stats := rm.GetStats()
	if stats.TotalRequests != 1 {
		t.Fatalf("expected total requests 1, got %d", stats.TotalRequests)
	}
	if stats.TotalAttempts != int64(cfg.MaxAttempts) {
		t.Fatalf("expected total attempts %d, got %d", cfg.MaxAttempts, stats.TotalAttempts)
	}
	if stats.FailedRetries != 1 {
		t.Fatalf("expected failed retries 1, got %d", stats.FailedRetries)
	}
	if stats.SuccessfulRetries != 0 {
		t.Fatalf("expected successful retries 0, got %d", stats.SuccessfulRetries)
	}
}

func newRetryContext() *lift.Context {
	req := lift.NewRequest(nil)
	req.Method = "GET"
	req.Path = "/retry"

	return lift.NewContext(context.Background(), req)
}
