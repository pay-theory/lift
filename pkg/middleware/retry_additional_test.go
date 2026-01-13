package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestRetryMiddleware_TotalTimeoutExceeded_UsesGiveUpHandler(t *testing.T) {
	logger := &mockLogger{}
	gaveUp := false

	mw := RetryMiddleware(RetryConfig{
		Name:         "retry",
		MaxAttempts:  3,
		Strategy:     RetryStrategyFixed,
		InitialDelay: 50 * time.Millisecond,
		TotalTimeout: 1 * time.Millisecond,
		Logger:       logger,
		RetryCondition: func(err error) bool {
			return err != nil
		},
		OnGiveUp: func(_ int, _ error) { gaveUp = true },
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		return errors.New("fail")
	})).Handle(ctx)

	require.Error(t, err)
	require.True(t, gaveUp)
	require.Greater(t, len(logger.logs), 0)
}

func TestRetryMiddleware_ContextCanceledDuringDelay_ReturnsContextCanceled(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{}, 1)

	mw := RetryMiddleware(RetryConfig{
		Name:         "retry",
		MaxAttempts:  3,
		Strategy:     RetryStrategyFixed,
		InitialDelay: 200 * time.Millisecond,
		TotalTimeout: time.Second,
	})

	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(cancelCtx, req)

	go func() {
		<-ready
		cancel()
	}()

	err := mw(lift.HandlerFunc(func(_ *lift.Context) error {
		ready <- struct{}{}
		return errors.New("fail")
	})).Handle(ctx)

	require.ErrorIs(t, err, context.Canceled)
}

func TestRetryConfigBuilders(t *testing.T) {
	httpCfg := NewHTTPRetry("http", 3)
	require.Equal(t, RetryStrategyExponential, httpCfg.Strategy)
	require.NotEmpty(t, httpCfg.RetryableStatusCodes)
	require.NotEmpty(t, httpCfg.NonRetryableStatusCodes)

	dbCfg := NewDatabaseRetry("db", 3)
	require.Equal(t, 50*time.Millisecond, dbCfg.InitialDelay)
	require.Equal(t, 5*time.Second, dbCfg.MaxDelay)
	require.Equal(t, 1.5, dbCfg.BackoffMultiplier)

	customCfg := NewCustomRetry("custom", 3, func(_ int, _ time.Duration) time.Duration { return time.Millisecond })
	require.Equal(t, RetryStrategyCustom, customCfg.Strategy)
	require.NotNil(t, customCfg.CustomBackoff)
}
