package performance

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func newTestPool(cfg *ConnectionPoolConfig, buffer int) *ConnectionPool {
	if cfg == nil {
		cfg = &ConnectionPoolConfig{}
	}

	state := &poolState{
		mu: &sync.RWMutex{},
		resources: &poolResources{
			ctx:     context.Background(),
			config:  cfg,
			clients: make(chan *dynamodb.Client, buffer),
		},
		metrics: &PoolMetrics{},
	}

	return &ConnectionPool{state: state}
}

func TestConnectionPool_MaxConnectionsEnforced(t *testing.T) {
	pool := newTestPool(&ConnectionPoolConfig{MaxConnections: 2}, 0)

	if !pool.reserveConnectionSlot() {
		t.Fatal("expected first slot reservation to succeed")
	}

	if !pool.reserveConnectionSlot() {
		t.Fatal("expected second slot reservation to succeed")
	}

	if pool.reserveConnectionSlot() {
		t.Fatal("expected third reservation to be denied")
	}

	pool.releaseConnectionSlot()
	pool.releaseConnectionSlot()
}

func TestConnectionPool_PoolStatsNoDivisionByZero(t *testing.T) {
	pool := newTestPool(&ConnectionPoolConfig{MaxConnections: 5}, 0)

	stats := pool.PoolStats()
	successRate, ok := stats["success_rate"].(float64)
	if !ok {
		t.Fatalf("expected success_rate to be float64, got %T", stats["success_rate"])
	}

	if math.IsNaN(successRate) || successRate != 0 {
		t.Fatalf("expected success_rate to be 0 without requests, got %v", successRate)
	}

	utilization, ok := stats["pool_utilization"].(float64)
	if !ok {
		t.Fatalf("expected pool_utilization to be float64, got %T", stats["pool_utilization"])
	}

	if math.IsNaN(utilization) || utilization != 0 {
		t.Fatalf("expected pool_utilization to be 0 for empty pool, got %v", utilization)
	}
}

func TestConnectionPool_PoolStatsSaturated(t *testing.T) {
	pool := newTestPool(&ConnectionPoolConfig{MaxConnections: 2}, 0)
	pool.state.totalConnections = 2

	stats := pool.PoolStats()
	utilization, ok := stats["pool_utilization"].(float64)
	if !ok {
		t.Fatalf("expected pool_utilization to be float64, got %T", stats["pool_utilization"])
	}

	if math.Abs(utilization-100) > 1e-9 {
		t.Fatalf("expected utilization at 100%% for saturated pool, got %v", utilization)
	}
}

func TestConnectionPool_CloseDoesNotDeadlock(t *testing.T) {
	pool := newTestPool(&ConnectionPoolConfig{MaxConnections: 2}, 2)
	pool.state.resources.cancel = func() {}

	pool.state.resources.clients <- &dynamodb.Client{}
	pool.state.totalConnections = 1

	done := make(chan struct{})
	go func() {
		if err := pool.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close deadlocked")
	}
}

func TestCalculateSuccessRate(t *testing.T) {
	tests := []struct {
		name           string
		total          int64
		failed         int64
		expectedResult float64
	}{
		{
			name:           "no requests",
			expectedResult: 0,
		},
		{
			name:           "all successful",
			total:          10,
			failed:         0,
			expectedResult: 100,
		},
		{
			name:           "partial failures",
			total:          8,
			failed:         2,
			expectedResult: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSuccessRate(tt.total, tt.failed)
			if math.Abs(result-tt.expectedResult) > 1e-9 {
				t.Fatalf("expected %v got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestConnectionPoolCalculateUtilization(t *testing.T) {
	cfg := &ConnectionPoolConfig{MaxConnections: 10}
	pool := newTestPool(cfg, 0)

	utilization := pool.calculateUtilization()
	if utilization != 0 {
		t.Fatalf("expected zero utilization for empty pool, got %v", utilization)
	}

	pool.state.totalConnections = 5
	utilization = pool.calculateUtilization()
	if math.Abs(utilization-50) > 1e-9 {
		t.Fatalf("expected 50%% utilization, got %v", utilization)
	}

	pool.state.resources.config.MaxConnections = 0
	utilization = pool.calculateUtilization()
	if utilization != 0 {
		t.Fatalf("expected zero utilization when max connections unset, got %v", utilization)
	}
}
