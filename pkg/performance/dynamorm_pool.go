package performance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// DynamORMPool wraps DynamORM with connection pooling for improved performance
type DynamORMPool struct {
	pool     *ConnectionPool
	config   *PooledDynamORMConfig
	mu       sync.RWMutex
	closed   bool
	sessions map[string]*PooledSession
}

// PooledDynamORMConfig holds configuration for pooled DynamORM operations
type PooledDynamORMConfig struct {
	ConnectionPoolConfig *ConnectionPoolConfig `json:"connection_pool_config"`

	// DynamORM-specific settings
	DefaultTableName   string        `json:"default_table_name"`
	DefaultTimeout     time.Duration `json:"default_timeout"`
	EnableBatching     bool          `json:"enable_batching"`
	BatchSize          int           `json:"batch_size"`
	BatchFlushInterval time.Duration `json:"batch_flush_interval"`

	// Caching settings
	EnableCaching bool          `json:"enable_caching"`
	CacheTTL      time.Duration `json:"cache_ttl"`
	CacheSize     int           `json:"cache_size"`

	// Monitoring
	EnableMetrics bool   `json:"enable_metrics"`
	MetricsPrefix string `json:"metrics_prefix"`
}

// DefaultPooledDynamORMConfig returns a default configuration for pooled DynamORM
func DefaultPooledDynamORMConfig() *PooledDynamORMConfig {
	return &PooledDynamORMConfig{
		ConnectionPoolConfig: DefaultConnectionPoolConfig(),
		DefaultTimeout:       30 * time.Second,
		EnableBatching:       true,
		BatchSize:            25,
		BatchFlushInterval:   100 * time.Millisecond,
		EnableCaching:        true,
		CacheTTL:             5 * time.Minute,
		CacheSize:            1000,
		EnableMetrics:        true,
		MetricsPrefix:        "dynamorm_pool",
	}
}

// PooledSession represents a DynamORM session with pooled connections
type PooledSession struct {
	client   *dynamodb.Client
	pool     *ConnectionPool
	lastUsed time.Time
	mu       sync.RWMutex
}

// NewDynamORMPool creates a new pooled DynamORM instance
func NewDynamORMPool(ctx context.Context, config *PooledDynamORMConfig) (*DynamORMPool, error) {
	if config == nil {
		config = DefaultPooledDynamORMConfig()
	}

	// Create connection pool
	pool, err := NewConnectionPool(ctx, config.ConnectionPoolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &DynamORMPool{
		pool:     pool,
		config:   config,
		sessions: make(map[string]*PooledSession),
	}, nil
}

// GetSession returns a DynamORM session, creating one if necessary
func (p *DynamORMPool) GetSession(ctx context.Context, tableName string) (*PooledSession, error) {
	if p.closed {
		return nil, fmt.Errorf("DynamORM pool is closed")
	}

	// Use default table name if not specified
	if tableName == "" {
		tableName = p.config.DefaultTableName
	}

	sessionKey := tableName

	p.mu.RLock()
	session, exists := p.sessions[sessionKey]
	p.mu.RUnlock()

	if exists {
		session.mu.Lock()
		session.lastUsed = time.Now()
		session.mu.Unlock()
		return session, nil
	}

	// Create new session
	client, err := p.pool.GetClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get client from pool: %w", err)
	}

	session = &PooledSession{
		client:   client,
		pool:     p.pool,
		lastUsed: time.Now(),
	}

	p.mu.Lock()
	p.sessions[sessionKey] = session
	p.mu.Unlock()

	return session, nil
}

// ReturnSession returns a session to the pool
func (p *DynamORMPool) ReturnSession(session *PooledSession) {
	if session != nil && session.client != nil {
		p.pool.ReturnClient(session.client)
	}
}

// Close closes the DynamORM pool and all sessions
func (p *DynamORMPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Return all session clients to the pool
	for _, session := range p.sessions {
		if session.client != nil {
			p.pool.ReturnClient(session.client)
		}
	}

	// Close the connection pool
	return p.pool.Close()
}

// ExecuteWithClient executes a function with a pooled DynamoDB client
func (p *DynamORMPool) ExecuteWithClient(ctx context.Context, tableName string, fn func(*dynamodb.Client) error) error {
	session, err := p.GetSession(ctx, tableName)
	if err != nil {
		return err
	}
	defer p.ReturnSession(session)

	// Execute the function with the client
	// Note: DefaultTimeout is not used here as the function doesn't accept context
	// Timeout should be handled by the caller or the function signature should be updated
	return fn(session.client)
}

// GetClient returns a DynamoDB client for direct operations
func (p *DynamORMPool) GetClient(ctx context.Context, tableName string) (*dynamodb.Client, func(), error) {
	session, err := p.GetSession(ctx, tableName)
	if err != nil {
		return nil, nil, err
	}

	// Return client and cleanup function
	cleanup := func() {
		p.ReturnSession(session)
	}

	return session.client, cleanup, nil
}

// GetPoolStats returns statistics about the connection pool
func (p *DynamORMPool) GetPoolStats() map[string]interface{} {
	return p.pool.PoolStats()
}

// OptimizeForWorkload optimizes the pool for a specific workload type
func (p *DynamORMPool) OptimizeForWorkload(workloadType string) error {
	return p.pool.OptimizeForWorkload(workloadType)
}

// SessionStats returns statistics about active sessions
func (p *DynamORMPool) SessionStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"total_sessions": len(p.sessions),
		"sessions":       make(map[string]interface{}),
	}

	for key, session := range p.sessions {
		session.mu.RLock()
		sessionStats := map[string]interface{}{
			"last_used":  session.lastUsed,
			"idle_time":  time.Since(session.lastUsed),
			"table_name": key,
		}
		session.mu.RUnlock()

		stats["sessions"].(map[string]interface{})[key] = sessionStats
	}

	return stats
}

// CleanupIdleSessions removes sessions that have been idle for too long
func (p *DynamORMPool) CleanupIdleSessions(maxIdleTime time.Duration) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for key, session := range p.sessions {
		session.mu.RLock()
		idleTime := now.Sub(session.lastUsed)
		session.mu.RUnlock()

		if idleTime > maxIdleTime {
			if session.client != nil {
				p.pool.ReturnClient(session.client)
			}
			delete(p.sessions, key)
			cleaned++
		}
	}

	return cleaned
}

// StartMaintenanceRoutine starts a background routine for pool maintenance
func (p *DynamORMPool) StartMaintenanceRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Cleanup idle sessions
				maxIdle := p.config.ConnectionPoolConfig.MaxIdleTime
				if maxIdle <= 0 {
					maxIdle = 10 * time.Minute
				}
				cleaned := p.CleanupIdleSessions(maxIdle)

				if p.config.EnableMetrics && cleaned > 0 {
					// TODO: Send cleanup metrics to metrics system
					// For now, track internally without logging
					_ = cleaned
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}
