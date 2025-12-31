package resources

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

type stubResource struct {
	lastUsed     time.Time
	healthErr    error
	cleanupErr   error
	cleanupCalls int
	valid        bool
}

func (r *stubResource) Initialize(context.Context) error { return nil }
func (r *stubResource) HealthCheck(context.Context) error { return r.healthErr }
func (r *stubResource) Cleanup() error {
	r.cleanupCalls++
	r.valid = false
	return r.cleanupErr
}
func (r *stubResource) IsValid() bool { return r.valid }
func (r *stubResource) LastUsed() time.Time {
	return r.lastUsed
}
func (r *stubResource) MarkUsed() { r.lastUsed = time.Now() }

type failingFactory struct{}

func (failingFactory) Create(context.Context) (Resource, error) { return nil, errors.New("create failed") }
func (failingFactory) Validate(Resource) bool                   { return true }

type initFailResource struct {
	initErr    error
	cleanupErr error
}

func (r *initFailResource) Initialize(context.Context) error  { return r.initErr }
func (r *initFailResource) HealthCheck(context.Context) error { return nil }
func (r *initFailResource) Cleanup() error                    { return r.cleanupErr }
func (r *initFailResource) IsValid() bool                     { return true }
func (r *initFailResource) LastUsed() time.Time               { return time.Now() }
func (r *initFailResource) MarkUsed()                         {}

type initFailFactory struct {
	res Resource
}

func (f initFailFactory) Create(context.Context) (Resource, error) { return f.res, nil }
func (f initFailFactory) Validate(Resource) bool                   { return true }

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()
	if cfg.MinIdle <= 0 || cfg.MaxActive <= 0 || cfg.MaxIdle <= 0 {
		t.Fatalf("expected positive pool sizing defaults, got %+v", cfg)
	}
	if cfg.HealthCheckInterval <= 0 || cfg.IdleTimeout <= 0 || cfg.MaxLifetime <= 0 {
		t.Fatalf("expected positive duration defaults, got %+v", cfg)
	}
	if !cfg.PreWarm {
		t.Fatal("expected default PreWarm to be true")
	}
}

func TestConnectionPool_CleanupFiltersStaleAndUnhealthyResources(t *testing.T) {
	now := time.Now()

	cfg := PoolConfig{
		Logger:      &lift.NoOpLogger{},
		MinIdle:     0,
		MaxActive:   10,
		MaxIdle:     10,
		MaxLifetime: 2 * time.Hour,
		IdleTimeout: 30 * time.Minute,
	}
	pool := NewConnectionPool(cfg, failingFactory{})
	t.Cleanup(func() { _ = pool.Close() })

	lifetimeExpired := &stubResource{lastUsed: now.Add(-3 * time.Hour), valid: true}
	idleExpired := &stubResource{lastUsed: now.Add(-45 * time.Minute), valid: true}
	healthFail := &stubResource{lastUsed: now, valid: true, healthErr: errors.New("unhealthy"), cleanupErr: errors.New("cleanup failed")}
	healthy := &stubResource{lastUsed: now, valid: true}

	pool.mu.Lock()
	pool.idle = []Resource{lifetimeExpired, idleExpired, healthFail, healthy}
	pool.mu.Unlock()

	pool.cleanup()

	pool.mu.RLock()
	idle := append([]Resource(nil), pool.idle...)
	pool.mu.RUnlock()

	if len(idle) != 1 {
		t.Fatalf("expected 1 idle resource after cleanup, got %d", len(idle))
	}
	if idle[0] != healthy {
		t.Fatalf("expected only healthy resource to remain idle, got %#v", idle[0])
	}
	if lifetimeExpired.cleanupCalls == 0 || idleExpired.cleanupCalls == 0 || healthFail.cleanupCalls == 0 {
		t.Fatalf("expected all removed resources to be cleaned up (calls: lifetime=%d idle=%d health=%d)",
			lifetimeExpired.cleanupCalls, idleExpired.cleanupCalls, healthFail.cleanupCalls)
	}
}

func TestConnectionPool_StartCleanupAndCloseStopsTicker(t *testing.T) {
	cfg := PoolConfig{
		Logger:              &lift.NoOpLogger{},
		MinIdle:             0,
		MaxActive:           1,
		MaxIdle:             1,
		HealthCheckInterval: time.Millisecond,
	}

	pool := NewConnectionPool(cfg, failingFactory{})
	if pool.cleanupTicker == nil {
		t.Fatal("expected cleanup ticker to be initialized when HealthCheckInterval is set")
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestConnectionPool_Get_CreateErrorAndInitError(t *testing.T) {
	t.Run("create error", func(t *testing.T) {
		pool := NewConnectionPool(PoolConfig{
			Logger:    &lift.NoOpLogger{},
			MinIdle:   0,
			MaxActive: 1,
			MaxIdle:   0,
		}, failingFactory{})

		_, err := pool.Get(context.Background())
		if err == nil {
			t.Fatal("expected create error, got nil")
		}
	})

	t.Run("init error triggers cleanup", func(t *testing.T) {
		pool := NewConnectionPool(PoolConfig{
			Logger:    &lift.NoOpLogger{},
			MinIdle:   0,
			MaxActive: 1,
			MaxIdle:   0,
		}, initFailFactory{
			res: &initFailResource{
				initErr:    errors.New("init failed"),
				cleanupErr: errors.New("cleanup failed"),
			},
		})

		_, err := pool.Get(context.Background())
		if err == nil {
			t.Fatal("expected init error, got nil")
		}
	})
}
