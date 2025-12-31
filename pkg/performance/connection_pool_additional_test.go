package performance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/require"
)

func TestConnectionPoolConfigHelpers(t *testing.T) {
	defaultCfg := DefaultConnectionPoolConfig()
	require.NotNil(t, defaultCfg)
	require.Equal(t, "us-east-1", defaultCfg.Region)
	require.Greater(t, defaultCfg.MaxConnections, 0)
	require.GreaterOrEqual(t, defaultCfg.MinConnections, 0)

	prod := ProductionConnectionPoolConfig()
	require.Greater(t, prod.MaxConnections, defaultCfg.MaxConnections)
	require.Greater(t, prod.MinConnections, defaultCfg.MinConnections)

	high := HighThroughputConnectionPoolConfig()
	require.Greater(t, high.MaxConnections, prod.MaxConnections)

	low := LowLatencyConnectionPoolConfig()
	require.Less(t, low.ConnectionTimeout, defaultCfg.ConnectionTimeout)
}

func TestNewConnectionPool_CreatesMinConnectionsAndOverridesEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	cfg := &ConnectionPoolConfig{
		Region:              "us-east-1",
		Endpoint:            "http://localhost:9999",
		MaxConnections:      2,
		MinConnections:      1,
		ConnectionTimeout:   50 * time.Millisecond,
		MaxRetries:          1,
		HealthCheckInterval: 0, // keep tests offline
		HealthCheckTimeout:  50 * time.Millisecond,
	}

	pool, err := NewConnectionPool(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	require.Equal(t, cfg.Endpoint, aws.ToString(pool.state.resources.awsConfig.BaseEndpoint))
	require.Equal(t, 1, len(pool.state.resources.clients))
	require.Equal(t, int64(1), pool.state.metrics.ConnectionsCreated)
	require.Equal(t, int64(1), pool.state.metrics.IdleConnections)
	require.Equal(t, 1, pool.currentTotalConnections())
}

func TestConnectionPool_GetClientAndReturnClient(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	cfg := &ConnectionPoolConfig{
		Region:              "us-east-1",
		MaxConnections:      1,
		MinConnections:      1,
		ConnectionTimeout:   50 * time.Millisecond,
		MaxRetries:          1,
		HealthCheckInterval: 0,
	}

	pool, err := NewConnectionPool(context.Background(), cfg)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	client, err := pool.GetClient(context.Background())
	require.NoError(t, err)
	require.NotNil(t, client)

	metrics := pool.GetMetrics()
	require.Equal(t, int64(1), metrics.ActiveConnections)
	require.Equal(t, int64(0), metrics.IdleConnections)
	require.Equal(t, int64(1), metrics.TotalRequests)

	pool.ReturnClient(client)
	require.Equal(t, 1, len(pool.state.resources.clients))

	metrics = pool.GetMetrics()
	require.Equal(t, int64(0), metrics.ActiveConnections)
	require.Equal(t, int64(1), metrics.IdleConnections)
}

func TestConnectionPool_GetClient_ContextDone(t *testing.T) {
	pool := newTestPool(&ConnectionPoolConfig{
		MaxConnections:    1,
		ConnectionTimeout: 50 * time.Millisecond,
	}, 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.GetClient(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestConnectionPool_GetClient_WaitForClientAndTimeout(t *testing.T) {
	t.Run("wait path receives client", func(t *testing.T) {
		cfg := &ConnectionPoolConfig{
			MaxConnections:    1,
			ConnectionTimeout: 250 * time.Millisecond,
		}
		pool := newTestPool(cfg, 0)
		pool.state.mu.Lock()
		pool.state.totalConnections = 1 // saturate so reserveConnectionSlot fails
		pool.state.mu.Unlock()
		pool.state.metrics.mu.Lock()
		pool.state.metrics.IdleConnections = 1
		pool.state.metrics.mu.Unlock()

		go func() {
			time.Sleep(25 * time.Millisecond)
			pool.state.resources.clients <- &dynamodb.Client{}
		}()

		client, err := pool.GetClient(context.Background())
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("wait path times out", func(t *testing.T) {
		cfg := &ConnectionPoolConfig{
			MaxConnections:    1,
			ConnectionTimeout: 25 * time.Millisecond,
		}
		pool := newTestPool(cfg, 0)
		pool.state.mu.Lock()
		pool.state.totalConnections = 1 // saturate so reserveConnectionSlot fails
		pool.state.mu.Unlock()

		_, err := pool.GetClient(context.Background())
		require.Error(t, err)
	})

	t.Run("first select timeout when ConnectionTimeout is zero", func(t *testing.T) {
		pool := newTestPool(&ConnectionPoolConfig{
			MaxConnections:    1,
			ConnectionTimeout: 0,
		}, 0)

		_, err := pool.GetClient(context.Background())
		require.Error(t, err)
	})
}

func TestConnectionPool_ReturnClient_DropsWhenChannelFull(t *testing.T) {
	cfg := &ConnectionPoolConfig{MaxConnections: 2}
	pool := newTestPool(cfg, 1)

	// Fill the pool channel.
	pool.state.resources.clients <- &dynamodb.Client{}

	pool.state.metrics.mu.Lock()
	pool.state.metrics.ActiveConnections = 1
	pool.state.metrics.mu.Unlock()

	pool.state.mu.Lock()
	pool.state.totalConnections = 1
	pool.state.mu.Unlock()

	pool.ReturnClient(&dynamodb.Client{})
	require.Equal(t, int64(1), pool.state.metrics.ConnectionsDestroyed)
	require.Equal(t, 0, pool.currentTotalConnections())
}

func TestConnectionPool_PerformHealthCheck_ReplacesUnhealthyClients(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if call == 2 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"__type":"InternalServerError","message":"fail"}`))
			return
		}

		if call >= 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Endpoints":[{"Address":"127.0.0.1","CachePeriodInMinutes":1}]}`))
			return
		}
	}))
	defer server.Close()

	awsCfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  server.Client(),
		Retryer: func() aws.Retryer {
			return aws.NopRetryer{}
		},
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               server.URL,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		}),
	}

	pool := newTestPool(&ConnectionPoolConfig{
		MaxConnections:     2,
		ConnectionTimeout:  50 * time.Millisecond,
		HealthCheckTimeout: 250 * time.Millisecond,
	}, 2)
	pool.state.resources.awsConfig = &awsCfg

	pool.state.resources.clients <- dynamodb.NewFromConfig(awsCfg)
	pool.state.resources.clients <- dynamodb.NewFromConfig(awsCfg)

	pool.performHealthCheck()

	metrics := pool.GetMetrics()
	// performHealthCheck checks up to 5 connections and may re-check the same one.
	require.Equal(t, int64(4), metrics.HealthChecksPassed)
	require.Equal(t, int64(1), metrics.HealthChecksFailed)
	require.False(t, metrics.LastHealthCheck.IsZero())
	require.Equal(t, int64(1), metrics.ConnectionsCreated)
	require.Equal(t, int64(1), metrics.ConnectionsDestroyed)
}

func TestConnectionPool_OptimizeForWorkload(t *testing.T) {
	pool := newTestPool(&ConnectionPoolConfig{Region: "us-east-1", Endpoint: "http://local", MaxConnections: 10}, 0)

	require.NoError(t, pool.OptimizeForWorkload("high-throughput"))
	require.Equal(t, "us-east-1", pool.state.resources.config.Region)
	require.Equal(t, "http://local", pool.state.resources.config.Endpoint)

	require.NoError(t, pool.OptimizeForWorkload("low-latency"))
	require.NoError(t, pool.OptimizeForWorkload("production"))
	require.Error(t, pool.OptimizeForWorkload("unknown"))
}

func TestConnectionPool_GetMetrics_ReturnsCopy(t *testing.T) {
	pool := newTestPool(&ConnectionPoolConfig{MaxConnections: 1}, 0)
	pool.state.metrics.mu.Lock()
	pool.state.metrics.TotalRequests = 10
	pool.state.metrics.mu.Unlock()

	m := pool.GetMetrics()
	require.Equal(t, int64(10), m.TotalRequests)
	m.TotalRequests = 0
	require.Equal(t, int64(10), pool.GetMetrics().TotalRequests)
}
