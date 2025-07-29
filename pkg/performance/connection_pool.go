package performance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// ConnectionPoolConfig holds configuration for DynamoDB connection pooling
type ConnectionPoolConfig struct {
	Region              string        `json:"region"`
	Endpoint            string        `json:"endpoint,omitempty"`
	MaxConnections      int           `json:"max_connections"`
	MinConnections      int           `json:"min_connections"`
	MaxIdleTime         time.Duration `json:"max_idle_time"`
	ConnectionTimeout   time.Duration `json:"connection_timeout"`
	MaxRetries          int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`
	BackoffMultiplier   float64       `json:"backoff_multiplier"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`
	EnableMetrics       bool          `json:"enable_metrics"`
}

// DefaultConnectionPoolConfig returns a default configuration optimized for DynamORM
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxConnections:      50,
		MinConnections:      5,
		MaxIdleTime:         10 * time.Minute,
		ConnectionTimeout:   30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          100 * time.Millisecond,
		BackoffMultiplier:   2.0,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		Region:              "us-east-1",
		EnableMetrics:       true,
	}
}

// ProductionConnectionPoolConfig returns configuration optimized for production workloads
func ProductionConnectionPoolConfig() *ConnectionPoolConfig {
	config := DefaultConnectionPoolConfig()
	config.MaxConnections = 100
	config.MinConnections = 10
	config.MaxIdleTime = 5 * time.Minute
	config.ConnectionTimeout = 10 * time.Second
	config.HealthCheckInterval = 15 * time.Second
	return config
}

// HighThroughputConnectionPoolConfig returns configuration for high-throughput scenarios
func HighThroughputConnectionPoolConfig() *ConnectionPoolConfig {
	config := DefaultConnectionPoolConfig()
	config.MaxConnections = 200
	config.MinConnections = 20
	config.MaxIdleTime = 2 * time.Minute
	config.ConnectionTimeout = 5 * time.Second
	config.MaxRetries = 5
	config.HealthCheckInterval = 10 * time.Second
	return config
}

// LowLatencyConnectionPoolConfig returns configuration optimized for low latency
func LowLatencyConnectionPoolConfig() *ConnectionPoolConfig {
	config := DefaultConnectionPoolConfig()
	config.MaxConnections = 150
	config.MinConnections = 25
	config.MaxIdleTime = 1 * time.Minute
	config.ConnectionTimeout = 2 * time.Second
	config.RetryDelay = 50 * time.Millisecond
	config.HealthCheckInterval = 5 * time.Second
	return config
}

// ConnectionPool manages a pool of DynamoDB connections
type ConnectionPool struct {
	ctx       context.Context
	config    *ConnectionPoolConfig
	clients   chan *dynamodb.Client
	metrics   *PoolMetrics
	cancel    context.CancelFunc
	awsConfig aws.Config
	mu        sync.RWMutex
	closed    bool
}

// PoolMetrics tracks connection pool performance
type PoolMetrics struct {
	LastHealthCheck      time.Time     `json:"last_health_check"`
	ActiveConnections    int64         `json:"active_connections"`
	IdleConnections      int64         `json:"idle_connections"`
	TotalRequests        int64         `json:"total_requests"`
	FailedRequests       int64         `json:"failed_requests"`
	AverageWaitTime      time.Duration `json:"average_wait_time"`
	ConnectionsCreated   int64         `json:"connections_created"`
	ConnectionsDestroyed int64         `json:"connections_destroyed"`
	HealthChecksPassed   int64         `json:"health_checks_passed"`
	HealthChecksFailed   int64         `json:"health_checks_failed"`
	mu                   sync.RWMutex
}

// NewConnectionPool creates a new connection pool with the given configuration
func NewConnectionPool(ctx context.Context, cfg *ConnectionPoolConfig) (*ConnectionPool, error) {
	if cfg == nil {
		cfg = DefaultConnectionPoolConfig()
	}

	// Load AWS configuration
	awsConfig, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithRetryMaxAttempts(cfg.MaxRetries),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override endpoint if specified
	if cfg.Endpoint != "" {
		awsConfig.BaseEndpoint = aws.String(cfg.Endpoint)
	}

	poolCtx, cancel := context.WithCancel(ctx)

	pool := &ConnectionPool{
		config:    cfg,
		clients:   make(chan *dynamodb.Client, cfg.MaxConnections),
		metrics:   &PoolMetrics{},
		ctx:       poolCtx,
		cancel:    cancel,
		awsConfig: awsConfig,
	}

	// Create minimum number of connections
	for i := 0; i < cfg.MinConnections; i++ {
		client := pool.createClient()
		pool.clients <- client
		pool.metrics.ConnectionsCreated++
		pool.metrics.IdleConnections++
	}

	// Start health check routine
	if cfg.HealthCheckInterval > 0 {
		go pool.healthCheckRoutine()
	}

	return pool, nil
}

// GetClient retrieves a client from the pool
func (p *ConnectionPool) GetClient(ctx context.Context) (*dynamodb.Client, error) {
	if p.closed {
		return nil, fmt.Errorf("connection pool is closed")
	}

	start := time.Now()
	defer func() {
		waitTime := time.Since(start)
		p.metrics.mu.Lock()
		p.metrics.TotalRequests++
		p.metrics.AverageWaitTime = (p.metrics.AverageWaitTime + waitTime) / 2
		p.metrics.mu.Unlock()
	}()

	// Try to get an existing client
	select {
	case client := <-p.clients:
		p.metrics.mu.Lock()
		p.metrics.ActiveConnections++
		p.metrics.IdleConnections--
		p.metrics.mu.Unlock()
		return client, nil
	case <-time.After(p.config.ConnectionTimeout):
		p.metrics.mu.Lock()
		p.metrics.FailedRequests++
		p.metrics.mu.Unlock()
		return nil, fmt.Errorf("connection timeout after %v", p.config.ConnectionTimeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Create new client if pool not at max capacity
		if len(p.clients) < p.config.MaxConnections {
			client := p.createClient()
			p.metrics.mu.Lock()
			p.metrics.ConnectionsCreated++
			p.metrics.ActiveConnections++
			p.metrics.mu.Unlock()
			return client, nil
		}

		// Wait for available client
		select {
		case client := <-p.clients:
			p.metrics.mu.Lock()
			p.metrics.ActiveConnections++
			p.metrics.IdleConnections--
			p.metrics.mu.Unlock()
			return client, nil
		case <-time.After(p.config.ConnectionTimeout):
			p.metrics.mu.Lock()
			p.metrics.FailedRequests++
			p.metrics.mu.Unlock()
			return nil, fmt.Errorf("connection timeout after %v", p.config.ConnectionTimeout)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ReturnClient returns a client to the pool
func (p *ConnectionPool) ReturnClient(client *dynamodb.Client) {
	if p.closed || client == nil {
		return
	}

	// Return client to pool if there's space
	select {
	case p.clients <- client:
		p.metrics.mu.Lock()
		p.metrics.ActiveConnections--
		p.metrics.IdleConnections++
		p.metrics.mu.Unlock()
	default:
		// Pool is full, client will be garbage collected
		p.metrics.mu.Lock()
		p.metrics.ConnectionsDestroyed++
		p.metrics.ActiveConnections--
		p.metrics.mu.Unlock()
	}
}

// Close closes the connection pool and all clients
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.cancel()

	// Close all clients in the pool
	close(p.clients)
	for client := range p.clients {
		// DynamoDB clients don't need explicit closing
		_ = client
		p.metrics.ConnectionsDestroyed++
	}

	return nil
}

// GetMetrics returns current pool metrics
func (p *ConnectionPool) GetMetrics() *PoolMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	// Return a copy of metrics without the mutex
	return &PoolMetrics{
		ActiveConnections:    p.metrics.ActiveConnections,
		IdleConnections:      p.metrics.IdleConnections,
		TotalRequests:        p.metrics.TotalRequests,
		FailedRequests:       p.metrics.FailedRequests,
		AverageWaitTime:      p.metrics.AverageWaitTime,
		ConnectionsCreated:   p.metrics.ConnectionsCreated,
		ConnectionsDestroyed: p.metrics.ConnectionsDestroyed,
		HealthChecksPassed:   p.metrics.HealthChecksPassed,
		HealthChecksFailed:   p.metrics.HealthChecksFailed,
		LastHealthCheck:      p.metrics.LastHealthCheck,
	}
}

// createClient creates a new DynamoDB client
func (p *ConnectionPool) createClient() *dynamodb.Client {
	return dynamodb.NewFromConfig(p.awsConfig)
}

// healthCheckRoutine performs periodic health checks on pool connections
func (p *ConnectionPool) healthCheckRoutine() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performHealthCheck()
		case <-p.ctx.Done():
			return
		}
	}
}

// performHealthCheck checks the health of connections in the pool
func (p *ConnectionPool) performHealthCheck() {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.HealthCheckTimeout)
	defer cancel()

	// Sample a few connections for health check
	healthyCount := 0
	totalChecked := 0
	maxCheck := 5 // Check up to 5 connections

healthLoop:
	for i := 0; i < maxCheck && len(p.clients) > 0; i++ {
		select {
		case client := <-p.clients:
			totalChecked++

			// Perform a simple operation to check health
			_, err := client.DescribeEndpoints(ctx, &dynamodb.DescribeEndpointsInput{})
			if err == nil {
				healthyCount++
				p.clients <- client // Return healthy client
			} else {
				// Create a new client to replace the unhealthy one
				newClient := p.createClient()
				p.clients <- newClient
				p.metrics.mu.Lock()
				p.metrics.ConnectionsDestroyed++
				p.metrics.ConnectionsCreated++
				p.metrics.mu.Unlock()
			}
		default:
			break healthLoop
		}
	}

	// Update health check metrics
	p.metrics.mu.Lock()
	p.metrics.HealthChecksPassed += int64(healthyCount)
	p.metrics.HealthChecksFailed += int64(totalChecked - healthyCount)
	p.metrics.LastHealthCheck = time.Now()
	p.metrics.mu.Unlock()
}

// PoolStats returns formatted statistics about the pool
func (p *ConnectionPool) PoolStats() map[string]interface{} {
	metrics := p.GetMetrics()

	return map[string]interface{}{
		"active_connections":    metrics.ActiveConnections,
		"idle_connections":      metrics.IdleConnections,
		"total_requests":        metrics.TotalRequests,
		"failed_requests":       metrics.FailedRequests,
		"success_rate":          float64(metrics.TotalRequests-metrics.FailedRequests) / float64(metrics.TotalRequests) * 100,
		"average_wait_time_ms":  metrics.AverageWaitTime.Milliseconds(),
		"connections_created":   metrics.ConnectionsCreated,
		"connections_destroyed": metrics.ConnectionsDestroyed,
		"health_checks_passed":  metrics.HealthChecksPassed,
		"health_checks_failed":  metrics.HealthChecksFailed,
		"last_health_check":     metrics.LastHealthCheck,
		"pool_utilization":      float64(metrics.ActiveConnections) / float64(p.config.MaxConnections) * 100,
	}
}

// OptimizeForWorkload adjusts pool configuration based on workload characteristics
func (p *ConnectionPool) OptimizeForWorkload(workloadType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var newConfig *ConnectionPoolConfig

	switch workloadType {
	case "high-throughput":
		newConfig = HighThroughputConnectionPoolConfig()
	case "low-latency":
		newConfig = LowLatencyConnectionPoolConfig()
	case "production":
		newConfig = ProductionConnectionPoolConfig()
	default:
		return fmt.Errorf("unknown workload type: %s", workloadType)
	}

	// Apply new configuration (simplified - in practice would need gradual transition)
	newConfig.Region = p.config.Region
	newConfig.Endpoint = p.config.Endpoint
	p.config = newConfig

	return nil
}
