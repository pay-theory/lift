package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/stretchr/testify/require"
)

func TestBulkheadConfigBuilders(t *testing.T) {
	tenantCfg := NewTenantBulkhead("t", 10, map[string]int{"tenant-1": 1})
	require.True(t, tenantCfg.EnableTenantIsolation)
	require.Equal(t, 1, tenantCfg.PerTenantLimits["tenant-1"])

	opCfg := NewOperationBulkhead("o", 10, map[string]int{"GET:/x": 2})
	require.True(t, opCfg.EnableOperationIsolation)
	require.Equal(t, 2, opCfg.PerOperationLimits["GET:/x"])

	priorityCfg := NewPriorityBulkhead("p", 10, func(_ *lift.Context) int { return 7 })
	require.True(t, priorityCfg.EnablePriority)
	require.Equal(t, 8, priorityCfg.HighPriorityThreshold)
	require.NotNil(t, priorityCfg.PriorityExtractor)
}

func TestDefaultBulkheadHelpers(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test", Headers: map[string]string{"X-Priority": "high"}})
	ctx := lift.NewContext(context.Background(), req)
	require.Equal(t, 10, defaultPriorityExtractor(ctx))

	ctx.Request.Headers["X-Priority"] = "low"
	require.Equal(t, 1, defaultPriorityExtractor(ctx))

	ctx.Request.Headers["X-Priority"] = "other"
	require.Equal(t, 5, defaultPriorityExtractor(ctx))

	ctx.Request.Headers = map[string]string{}
	require.Equal(t, 5, defaultPriorityExtractor(ctx))
}

func TestDefaultRejectionHandler_WritesResponseAndReturnsError(t *testing.T) {
	req := lift.NewRequest(&adapters.Request{Method: "GET", Path: "/test"})
	ctx := lift.NewContext(context.Background(), req)

	err := defaultRejectionHandler(ctx, "busy")
	require.Error(t, err)
	require.Equal(t, 503, ctx.Response.StatusCode)
	_, ok := ctx.Response.Body.(map[string]any)
	require.True(t, ok)

	// Already-written response triggers JSON error path
	ctx2 := lift.NewContext(context.Background(), req)
	require.NoError(t, ctx2.JSON(map[string]any{"ok": true}))
	err = defaultRejectionHandler(ctx2, "busy")
	require.Error(t, err)
}

func TestSemaphore_InsertWaiterAndReleaseBranches(t *testing.T) {
	s := newSemaphore(1)

	wLow := &waiter{priority: 1, ch: make(chan bool, 1), ctx: context.Background()}
	wHigh := &waiter{priority: 10, ch: make(chan bool, 1), ctx: context.Background()}
	s.waitQueue = []*waiter{wLow}

	s.insertWaiter(wHigh)
	require.Equal(t, wHigh, s.waitQueue[0])

	// Normal release path signals waiter
	s.activeCount = 1
	s.release()
	select {
	case acquired := <-wHigh.ch:
		require.True(t, acquired)
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected waiter to be signaled")
	}

	// Default branch: waiter channel is full so send fails
	s2 := newSemaphore(1)
	fullWaiter := &waiter{priority: 1, ch: make(chan bool, 1), ctx: context.Background()}
	fullWaiter.ch <- false
	s2.waitQueue = []*waiter{fullWaiter}
	s2.activeCount = 1
	s2.release()
	require.Equal(t, 0, s2.active())
}
