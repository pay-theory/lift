package resources

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubPreWarmPool struct {
	getCount   int
	getErrAt   int
	getErr     error
	putErr     error
	putCount   int
	lastPutAny any
}

func (p *stubPreWarmPool) Get(context.Context) (any, error) {
	p.getCount++
	if p.getErrAt > 0 && p.getCount == p.getErrAt {
		return nil, p.getErr
	}
	return p.getCount, nil
}

func (p *stubPreWarmPool) Put(resource any) error {
	p.putCount++
	p.lastPutAny = resource
	return p.putErr
}

func (p *stubPreWarmPool) Close() error               { return nil }
func (p *stubPreWarmPool) Stats() PoolStats           { return PoolStats{} }
func (p *stubPreWarmPool) HealthCheck(context.Context) error { return nil }

func TestDefaultPreWarmer_NameAndErrorPaths(t *testing.T) {
	pw := NewDefaultPreWarmer("pw", 2, time.Second)
	if pw.Name() != "pw" {
		t.Fatalf("expected prewarmer name %q, got %q", "pw", pw.Name())
	}

	t.Run("get error cleans up prior resources", func(t *testing.T) {
		pool := &stubPreWarmPool{
			getErrAt: 2,
			getErr:   errors.New("get failed"),
			putErr:   errors.New("put failed"),
		}
		err := pw.PreWarm(context.Background(), pool)
		if err == nil || !strings.Contains(err.Error(), "failed to pre-warm connection") {
			t.Fatalf("expected pre-warm get error, got %v", err)
		}
		if pool.putCount == 0 {
			t.Fatal("expected prewarmer to attempt Put cleanup for previously acquired resources")
		}
	})

	t.Run("put error when returning resources", func(t *testing.T) {
		pool := &stubPreWarmPool{
			putErr: errors.New("put failed"),
		}
		err := pw.PreWarm(context.Background(), pool)
		if err == nil || !strings.Contains(err.Error(), "failed to return pre-warmed connection") {
			t.Fatalf("expected pre-warm put error, got %v", err)
		}
	})
}

type stubPool struct {
	stats       PoolStats
	healthErr   error
	closeErr    error
	closeDelay  time.Duration
	closeCalled bool
}

func (p *stubPool) Get(context.Context) (any, error) { return nil, nil }
func (p *stubPool) Put(any) error                    { return nil }
func (p *stubPool) Close() error {
	p.closeCalled = true
	if p.closeDelay > 0 {
		time.Sleep(p.closeDelay)
	}
	return p.closeErr
}
func (p *stubPool) Stats() PoolStats                      { return p.stats }
func (p *stubPool) HealthCheck(context.Context) error      { return p.healthErr }

type stubPreWarmer struct {
	err error
}

func (pw stubPreWarmer) PreWarm(context.Context, ConnectionPool) error { return pw.err }
func (pw stubPreWarmer) Name() string                                 { return "pw" }

func TestResourceManager_ErrorPaths(t *testing.T) {
	rm := NewResourceManager(ResourceManagerConfig{ShutdownTimeout: 10 * time.Millisecond})

	pool := &stubPool{}
	if err := rm.RegisterPool("p", pool); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if err := rm.RegisterPool("p", pool); err == nil {
		t.Fatal("expected duplicate register error")
	}
	if _, err := rm.GetPool("missing"); err == nil {
		t.Fatal("expected get missing pool error")
	}

	if err := rm.RegisterPreWarmer("missing", stubPreWarmer{err: errors.New("boom")}); err != nil {
		t.Fatalf("unexpected register prewarmer error: %v", err)
	}

	// Missing pool pre-warmers are ignored.
	if err := rm.PreWarmAll(context.Background()); err != nil {
		t.Fatalf("expected prewarm all to ignore missing pool, got %v", err)
	}

	if err := rm.RegisterPreWarmer("p", stubPreWarmer{err: errors.New("boom")}); err != nil {
		t.Fatalf("unexpected register prewarmer error: %v", err)
	}
	if err := rm.PreWarmAll(context.Background()); err == nil {
		t.Fatal("expected prewarm all to fail due to prewarmer error")
	}

	// Cover manager close error aggregation (timeout + close error).
	slow := &stubPool{closeDelay: 50 * time.Millisecond}
	erring := &stubPool{closeErr: errors.New("close failed")}
	if err := rm.RegisterPool("slow", slow); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if err := rm.RegisterPool("bad", erring); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	if err := rm.Close(); err == nil {
		t.Fatal("expected close to report shutdown errors")
	}

	if err := rm.RegisterPool("new", &stubPool{}); err == nil {
		t.Fatal("expected error registering pool on closed manager")
	}
	if _, err := rm.GetPool("p"); err == nil {
		t.Fatal("expected error getting pool from closed manager")
	}
}
