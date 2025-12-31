package health

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift/resources"
)

type stubConnectionPool struct {
	stats     resources.PoolStats
	healthErr error
}

func (p *stubConnectionPool) Get(context.Context) (any, error) { return nil, nil }
func (p *stubConnectionPool) Put(any) error                    { return nil }
func (p *stubConnectionPool) Close() error                      { return nil }
func (p *stubConnectionPool) Stats() resources.PoolStats        { return p.stats }
func (p *stubConnectionPool) HealthCheck(context.Context) error { return p.healthErr }

func TestPoolHealthChecker_NameAndCheck(t *testing.T) {
	ctx := context.Background()
	pool := &stubConnectionPool{
		stats: resources.PoolStats{
			Active:   1,
			Idle:     1,
			Total:    2,
			Gets:     10,
			Puts:     10,
			Hits:     9,
			Misses:   1,
			Timeouts: 0,
			Errors:   0,
		},
		healthErr: errors.New("unhealthy"),
	}

	checker := NewPoolHealthChecker("pool", pool)
	if checker.Name() != "pool" {
		t.Fatalf("expected name %q, got %q", "pool", checker.Name())
	}

	status := checker.Check(ctx)
	if status.Status != StatusUnhealthy {
		t.Fatalf("expected unhealthy status, got %q", status.Status)
	}
	if status.Error == "" {
		t.Fatal("expected error message to be set")
	}

	pool.healthErr = nil
	pool.stats = resources.PoolStats{
		Idle:   0,
		Gets:   100,
		Errors: 50,
	}
	status = checker.Check(ctx)
	if status.Status != StatusDegraded {
		t.Fatalf("expected degraded status, got %q", status.Status)
	}
	if status.Message == "" {
		t.Fatal("expected degraded status to include a message")
	}

	pool.stats = resources.PoolStats{
		Idle:   2,
		Gets:   100,
		Errors: 0,
	}
	status = checker.Check(ctx)
	if status.Status != StatusHealthy {
		t.Fatalf("expected healthy status, got %q", status.Status)
	}
}

type pingDriver struct {
	pingErr error
}

func (d pingDriver) Open(string) (driver.Conn, error) {
	return &pingConn{pingErr: d.pingErr}, nil
}

type pingConn struct {
	pingErr error
}

func (c *pingConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (c *pingConn) Close() error                        { return nil }
func (c *pingConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }
func (c *pingConn) Ping(context.Context) error          { return c.pingErr }

func TestDatabaseHealthChecker_SuccessAndFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("healthy", func(t *testing.T) {
		driverName := fmt.Sprintf("ping-ok-%d", time.Now().UnixNano())
		sql.Register(driverName, pingDriver{})

		db, err := sql.Open(driverName, "")
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		checker := NewDatabaseHealthChecker("db", db)
		if checker.Name() != "db" {
			t.Fatalf("expected name %q, got %q", "db", checker.Name())
		}

		status := checker.Check(ctx)
		if status.Status != StatusHealthy {
			t.Fatalf("expected healthy status, got %q", status.Status)
		}
		if status.Message != "Database is healthy" {
			t.Fatalf("expected healthy message, got %q", status.Message)
		}
		if status.Details == nil {
			t.Fatal("expected details to be set")
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		driverName := fmt.Sprintf("ping-bad-%d", time.Now().UnixNano())
		sql.Register(driverName, pingDriver{pingErr: errors.New("ping failed")})

		db, err := sql.Open(driverName, "")
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		checker := NewDatabaseHealthChecker("db", db)
		status := checker.Check(ctx)
		if status.Status != StatusUnhealthy {
			t.Fatalf("expected unhealthy status, got %q", status.Status)
		}
		if status.Error == "" {
			t.Fatal("expected error to be set")
		}
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPHealthChecker_CheckPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("healthy response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		checker := NewHTTPHealthChecker("svc", srv.URL)
		if checker.Name() != "svc" {
			t.Fatalf("expected name %q, got %q", "svc", checker.Name())
		}

		status := checker.Check(ctx)
		if status.Status != StatusHealthy {
			t.Fatalf("expected healthy status, got %q", status.Status)
		}
	})

	t.Run("unexpected status code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		checker := NewHTTPHealthChecker("svc", srv.URL)
		status := checker.Check(ctx)
		if status.Status != StatusUnhealthy {
			t.Fatalf("expected unhealthy status, got %q", status.Status)
		}
		if status.Message == "" {
			t.Fatal("expected message for unhealthy status")
		}
	})

	t.Run("request creation failure", func(t *testing.T) {
		checker := NewHTTPHealthChecker("svc", "://bad-url")
		status := checker.Check(ctx)
		if status.Status != StatusUnhealthy {
			t.Fatalf("expected unhealthy status, got %q", status.Status)
		}
		if status.Message != "Failed to create HTTP request" {
			t.Fatalf("expected request creation message, got %q", status.Message)
		}
	})

	t.Run("client do failure", func(t *testing.T) {
		checker := NewHTTPHealthChecker("svc", "http://example.com")
		checker.client = &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			}),
		}

		status := checker.Check(ctx)
		if status.Status != StatusUnhealthy {
			t.Fatalf("expected unhealthy status, got %q", status.Status)
		}
		if status.Message != "HTTP request failed" {
			t.Fatalf("expected request failed message, got %q", status.Message)
		}
	})
}

func TestCheckerNamesAndMemoryThresholdBranches(t *testing.T) {
	ctx := context.Background()

	if NewMemoryHealthChecker("mem").Name() != "mem" {
		t.Fatal("expected memory checker name to be returned")
	}
	if NewCustomHealthChecker("custom", func(context.Context) HealthStatus { return HealthStatus{} }).Name() != "custom" {
		t.Fatal("expected custom checker name to be returned")
	}
	if NewAlwaysUnhealthyChecker("bad").Name() != "bad" {
		t.Fatal("expected always-unhealthy checker name to be returned")
	}

	base := NewMemoryHealthChecker("mem")
	baseStatus := base.Check(ctx)
	alloc, ok := baseStatus.Details["alloc"].(uint64)
	if !ok {
		t.Fatalf("expected alloc detail to be uint64, got %T", baseStatus.Details["alloc"])
	}
	if alloc == 0 {
		t.Skip("unexpected alloc=0; cannot reliably test threshold branches")
	}

	unhealthy := NewMemoryHealthChecker("mem")
	unhealthy.warningThreshold = 0
	unhealthy.criticalThreshold = 0
	if status := unhealthy.Check(ctx); status.Status != StatusUnhealthy {
		t.Fatalf("expected unhealthy status with low thresholds, got %q", status.Status)
	}

	degraded := NewMemoryHealthChecker("mem")
	degraded.warningThreshold = 0
	degraded.criticalThreshold = ^uint64(0)
	if status := degraded.Check(ctx); status.Status != StatusDegraded {
		t.Fatalf("expected degraded status with warning threshold, got %q", status.Status)
	}

	healthy := NewMemoryHealthChecker("mem")
	healthy.warningThreshold = ^uint64(0)
	healthy.criticalThreshold = ^uint64(0)
	if status := healthy.Check(ctx); status.Status != StatusHealthy {
		t.Fatalf("expected healthy status with high thresholds, got %q", status.Status)
	}
}
