package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

func TestApplyBulkheadDefaults(t *testing.T) {
	t.Parallel()

	cfg := applyBulkheadDefaults(BulkheadConfig{})

	if cfg.MaxConcurrentRequests != 100 {
		t.Fatalf("expected default MaxConcurrentRequests 100, got %d", cfg.MaxConcurrentRequests)
	}
	if cfg.MaxWaitTime != 30*time.Second {
		t.Fatalf("expected default MaxWaitTime 30s, got %v", cfg.MaxWaitTime)
	}
	if cfg.DefaultTenantLimit != 10 {
		t.Fatalf("expected default tenant limit 10, got %d", cfg.DefaultTenantLimit)
	}
	if cfg.DefaultOperationLimit != 20 {
		t.Fatalf("expected default operation limit 20, got %d", cfg.DefaultOperationLimit)
	}
	if cfg.RejectionHandler == nil {
		t.Fatal("expected default rejection handler to be set")
	}
	if cfg.PriorityExtractor == nil {
		t.Fatal("expected default priority extractor to be set")
	}
}

func TestBulkheadRequestHandlerSuccessReleasesResources(t *testing.T) {
	t.Parallel()

	cfg := applyBulkheadDefaults(BulkheadConfig{
		MaxConcurrentRequests:    2,
		EnableTenantIsolation:    true,
		EnableOperationIsolation: true,
	})

	manager := newBulkheadManager(cfg)
	ctx := newBulkheadContext(5, "tenant-1")

	handler := newBulkheadRequestHandler(manager, ctx)

	var executed bool
	next := lift.HandlerFunc(func(ctx *lift.Context) error {
		executed = true
		return nil
	})

	if err := handler.handle(next); err != nil {
		t.Fatalf("handle() error = %v", err)
	}

	if !executed {
		t.Fatal("expected downstream handler to execute")
	}

	if active := manager.globalSemaphore.active(); active != 0 {
		t.Fatalf("expected global semaphore to release, active=%d", active)
	}

	manager.mutex.RLock()
	tenantSem := manager.tenantSemaphores["tenant-1"]
	opSem := manager.operationSemaphores["GET:/resource"]
	manager.mutex.RUnlock()

	if tenantSem == nil || tenantSem.active() != 0 {
		t.Fatal("expected tenant semaphore to be released")
	}
	if opSem == nil || opSem.active() != 0 {
		t.Fatal("expected operation semaphore to be released")
	}

	stats := manager.GetStats()
	if stats.ActiveRequests != 0 {
		t.Fatalf("expected no active requests after completion, got %d", stats.ActiveRequests)
	}
}

func TestBulkheadRejectionHandlerInvokedOnExhaustion(t *testing.T) {
	t.Parallel()

	expected := errors.New("rejected")
	cfg := applyBulkheadDefaults(BulkheadConfig{
		MaxConcurrentRequests: 1,
		MaxWaitTime:           10 * time.Millisecond,
		RejectionHandler: func(_ *lift.Context, _ string) error {
			return expected
		},
	})

	manager := newBulkheadManager(cfg)

	if !manager.globalSemaphore.tryAcquire(context.Background(), 1) {
		t.Fatal("expected to acquire semaphore for setup")
	}
	defer manager.globalSemaphore.release()

	ctx := newBulkheadContext(1, "")

	err := newBulkheadRequestHandler(manager, ctx).handle(lift.HandlerFunc(func(*lift.Context) error {
		t.Fatal("handler should not execute when semaphore exhausted")
		return nil
	}))

	if !errors.Is(err, expected) {
		t.Fatalf("expected rejection error %v, got %v", expected, err)
	}
}

func TestBulkheadPriorityQueueHighPriorityWins(t *testing.T) {
	t.Parallel()

	cfg := applyBulkheadDefaults(BulkheadConfig{
		MaxConcurrentRequests: 1,
		MaxWaitTime:           200 * time.Millisecond,
		EnablePriority:        true,
		PriorityExtractor: func(ctx *lift.Context) int {
			if priority, ok := ctx.Get("priority").(int); ok {
				return priority
			}
			return 1
		},
	})

	manager := newBulkheadManager(cfg)

	if !manager.globalSemaphore.tryAcquire(context.Background(), 1) {
		t.Fatal("failed to reserve semaphore for setup")
	}

	lowCtx := newBulkheadContext(1, "")
	highCtx := newBulkheadContext(10, "")
	operation := "GET:/resource"

	lowAcquired := make(chan struct{}, 1)
	highAcquired := make(chan struct{}, 1)
	lowErr := make(chan error, 1)
	highErr := make(chan error, 1)

	go func() {
		acquired, _, err := manager.acquireResources(lowCtx.Context, lowCtx.TenantID(), operation, cfg.PriorityExtractor(lowCtx))
		if err == nil {
			lowAcquired <- struct{}{}
			manager.releaseResources(acquired, lowCtx.TenantID(), operation)
		}
		lowErr <- err
	}()

	go func() {
		acquired, _, err := manager.acquireResources(highCtx.Context, highCtx.TenantID(), operation, cfg.PriorityExtractor(highCtx))
		if err == nil {
			highAcquired <- struct{}{}
			manager.releaseResources(acquired, highCtx.TenantID(), operation)
		}
		highErr <- err
	}()

	time.Sleep(5 * time.Millisecond)

	select {
	case <-lowAcquired:
		t.Fatal("low priority should not acquire before release")
	case <-highAcquired:
		t.Fatal("high priority should not acquire before release")
	default:
	}

	manager.globalSemaphore.release()

	select {
	case <-highAcquired:
	case <-lowAcquired:
		t.Fatal("expected high priority to acquire before low priority")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for high priority acquisition")
	}

	select {
	case <-lowAcquired:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("low priority request did not acquire after high priority")
	}

	if err := <-highErr; err != nil {
		t.Fatalf("high priority acquisition returned error: %v", err)
	}
	if err := <-lowErr; err != nil {
		t.Fatalf("low priority acquisition returned error: %v", err)
	}
}

func newBulkheadContext(priority int, tenant string) *lift.Context {
	req := lift.NewRequest(nil)
	req.Method = "GET"
	req.Path = "/resource"

	ctx := lift.NewContext(context.Background(), req)
	ctx.Set("priority", priority)
	if tenant != "" {
		ctx.Set("tenant_id", tenant)
	}
	return ctx
}
