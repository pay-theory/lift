package performance

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/require"
)

func TestDynamORMPool_BasicSessionLifecycle(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	cfg := &PooledDynamORMConfig{
		ConnectionPoolConfig: &ConnectionPoolConfig{
			Region:              "us-east-1",
			MaxConnections:      2,
			MinConnections:      0,
			ConnectionTimeout:   50 * time.Millisecond,
			MaxRetries:          1,
			HealthCheckInterval: 0,
			MaxIdleTime:         10 * time.Millisecond,
		},
		DefaultTableName: "default-table",
		EnableMetrics:    true,
		MetricsPrefix:    "dynamorm_pool",
	}

	pool, err := NewDynamORMPool(context.Background(), cfg)
	require.NoError(t, err)
	pool.WithMetrics(&lift.NoOpMetrics{})
	defer func() { _ = pool.Close() }()

	require.NotNil(t, pool.GetPoolStats())
	require.NoError(t, pool.OptimizeForWorkload("production"))

	session1, err := pool.GetSession(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, session1)

	lastUsed1 := session1.lastUsed
	time.Sleep(1 * time.Millisecond)

	session2, err := pool.GetSession(context.Background(), "default-table")
	require.NoError(t, err)
	require.Same(t, session1, session2)
	require.True(t, session2.lastUsed.After(lastUsed1))

	require.NoError(t, pool.ExecuteWithClient(context.Background(), "default-table", func(_ *dynamodb.Client) error {
		return nil
	}))

	client, cleanup, err := pool.GetClient(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, cleanup)
	cleanup()

	stats := pool.SessionStats()
	require.Equal(t, 1, stats["total_sessions"])

	// Force the session to be old, then cleanup should remove it.
	session1.mu.Lock()
	session1.lastUsed = time.Now().Add(-time.Minute)
	session1.mu.Unlock()

	cleaned := pool.CleanupIdleSessions(10 * time.Millisecond)
	require.Equal(t, 1, cleaned)

	stats = pool.SessionStats()
	require.Equal(t, 0, stats["total_sessions"])

	require.NoError(t, pool.Close())
	require.NoError(t, pool.Close())
	require.Error(t, func() error {
		_, err := pool.GetSession(context.Background(), "x")
		return err
	}())
}

func TestDynamORMPool_MaintenanceRoutineCleansIdleSessions(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	cfg := &PooledDynamORMConfig{
		ConnectionPoolConfig: &ConnectionPoolConfig{
			Region:              "us-east-1",
			MaxConnections:      2,
			MinConnections:      0,
			ConnectionTimeout:   50 * time.Millisecond,
			MaxRetries:          1,
			HealthCheckInterval: 0,
			MaxIdleTime:         1 * time.Millisecond,
		},
		DefaultTableName: "default-table",
		EnableMetrics:    true,
		MetricsPrefix:    "dynamorm_pool",
	}

	pool, err := NewDynamORMPool(context.Background(), cfg)
	require.NoError(t, err)
	pool.WithMetrics(&lift.NoOpMetrics{})
	defer func() { _ = pool.Close() }()

	session, err := pool.GetSession(context.Background(), "default-table")
	require.NoError(t, err)

	session.mu.Lock()
	session.lastUsed = time.Now().Add(-time.Minute)
	session.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.StartMaintenanceRoutine(ctx, 5*time.Millisecond)

	require.Eventually(t, func() bool {
		stats := pool.SessionStats()
		return stats["total_sessions"] == 0
	}, time.Second, 10*time.Millisecond)

	cancel()
}

func TestDefaultPooledDynamORMConfig(t *testing.T) {
	cfg := DefaultPooledDynamORMConfig()
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.ConnectionPoolConfig)
	require.True(t, cfg.EnableMetrics)
	require.NotEmpty(t, cfg.MetricsPrefix)
}
