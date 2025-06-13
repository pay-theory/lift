package resources

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ConnectionPool manages a pool of reusable resources
type ConnectionPool interface {
	// Get retrieves a resource from the pool
	Get(ctx context.Context) (interface{}, error)

	// Put returns a resource to the pool
	Put(resource interface{}) error

	// Close shuts down the pool and cleans up all resources
	Close() error

	// Stats returns current pool statistics
	Stats() PoolStats

	// HealthCheck verifies pool health
	HealthCheck(ctx context.Context) error
}

// Resource defines the interface for pooled resources
type Resource interface {
	// Initialize sets up the resource
	Initialize(ctx context.Context) error

	// HealthCheck verifies resource health
	HealthCheck(ctx context.Context) error

	// Cleanup releases resource-specific resources
	Cleanup() error

	// IsValid checks if the resource is still usable
	IsValid() bool

	// LastUsed returns when the resource was last used
	LastUsed() time.Time

	// MarkUsed updates the last used timestamp
	MarkUsed()
}

// ResourceFactory creates new resources
type ResourceFactory interface {
	// Create creates a new resource instance
	Create(ctx context.Context) (Resource, error)

	// Validate checks if a resource is still valid
	Validate(resource Resource) bool
}

// PoolConfig configures connection pool behavior
type PoolConfig struct {
	// MinIdle minimum number of idle connections
	MinIdle int

	// MaxActive maximum number of active connections
	MaxActive int

	// MaxIdle maximum number of idle connections
	MaxIdle int

	// IdleTimeout how long to keep idle connections
	IdleTimeout time.Duration

	// MaxLifetime maximum lifetime of a connection
	MaxLifetime time.Duration

	// GetTimeout timeout for getting a connection
	GetTimeout time.Duration

	// HealthCheckInterval how often to health check idle connections
	HealthCheckInterval time.Duration

	// PreWarm whether to pre-warm the pool on startup
	PreWarm bool
}

// PoolStats provides pool statistics
type PoolStats struct {
	// Active number of active connections
	Active int `json:"active"`

	// Idle number of idle connections
	Idle int `json:"idle"`

	// Total total connections created
	Total int `json:"total"`

	// Gets total number of get requests
	Gets int64 `json:"gets"`

	// Puts total number of put requests
	Puts int64 `json:"puts"`

	// Hits successful gets from pool
	Hits int64 `json:"hits"`

	// Misses gets that required new connection
	Misses int64 `json:"misses"`

	// Timeouts gets that timed out
	Timeouts int64 `json:"timeouts"`

	// Errors connection errors
	Errors int64 `json:"errors"`
}

// DefaultConnectionPool implements ConnectionPool
type DefaultConnectionPool struct {
	config  PoolConfig
	factory ResourceFactory

	// Pool state
	idle   []Resource
	active map[Resource]bool
	stats  PoolStats
	closed bool

	// Synchronization
	mu   sync.RWMutex
	cond *sync.Cond

	// Background tasks
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config PoolConfig, factory ResourceFactory) *DefaultConnectionPool {
	pool := &DefaultConnectionPool{
		config:      config,
		factory:     factory,
		idle:        make([]Resource, 0, config.MaxIdle),
		active:      make(map[Resource]bool),
		stopCleanup: make(chan struct{}),
	}

	pool.cond = sync.NewCond(&pool.mu)

	// Start background cleanup if health check interval is set
	if config.HealthCheckInterval > 0 {
		pool.startCleanup()
	}

	return pool
}

// Get retrieves a resource from the pool
func (p *DefaultConnectionPool) Get(ctx context.Context) (interface{}, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, errors.New("pool is closed")
	}

	p.stats.Gets++

	// Try to get from idle pool first
	if len(p.idle) > 0 {
		resource := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		// Validate the resource
		if p.factory.Validate(resource) && resource.IsValid() {
			p.active[resource] = true
			resource.MarkUsed()
			p.stats.Hits++
			p.mu.Unlock()
			return resource, nil
		}

		// Resource is invalid, clean it up
		resource.Cleanup()
	}

	// Check if we can create a new connection
	if len(p.active) >= p.config.MaxActive {
		p.stats.Timeouts++
		p.mu.Unlock()
		return nil, errors.New("connection pool exhausted")
	}

	// Create new resource (unlock while creating)
	p.mu.Unlock()

	resource, err := p.factory.Create(ctx)
	if err != nil {
		p.mu.Lock()
		p.stats.Errors++
		p.mu.Unlock()
		return nil, err
	}

	if err := resource.Initialize(ctx); err != nil {
		p.mu.Lock()
		p.stats.Errors++
		p.mu.Unlock()
		resource.Cleanup()
		return nil, err
	}

	p.mu.Lock()
	p.active[resource] = true
	p.stats.Total++
	p.stats.Misses++
	resource.MarkUsed()
	p.mu.Unlock()

	return resource, nil
}

// Put returns a resource to the pool
func (p *DefaultConnectionPool) Put(resource interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return errors.New("pool is closed")
	}

	res, ok := resource.(Resource)
	if !ok {
		return errors.New("resource does not implement Resource interface")
	}

	p.stats.Puts++

	// Remove from active
	delete(p.active, res)

	// Check if resource is still valid
	if !p.factory.Validate(res) || !res.IsValid() {
		res.Cleanup()
		p.cond.Signal()
		return nil
	}

	// Add to idle pool if there's space
	if len(p.idle) < p.config.MaxIdle {
		p.idle = append(p.idle, res)
	} else {
		// Pool is full, cleanup the resource
		res.Cleanup()
	}

	p.cond.Signal()
	return nil
}

// Close shuts down the pool
func (p *DefaultConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Stop cleanup goroutine
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
		close(p.stopCleanup)
	}

	// Cleanup all idle resources
	for _, resource := range p.idle {
		resource.Cleanup()
	}
	p.idle = nil

	// Cleanup all active resources
	for resource := range p.active {
		resource.Cleanup()
	}
	p.active = nil

	p.cond.Broadcast()
	return nil
}

// Stats returns current pool statistics
func (p *DefaultConnectionPool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := p.stats
	stats.Active = len(p.active)
	stats.Idle = len(p.idle)

	return stats
}

// HealthCheck verifies pool health
func (p *DefaultConnectionPool) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return errors.New("pool is closed")
	}

	// Check if we have minimum idle connections
	if len(p.idle) < p.config.MinIdle {
		return errors.New("insufficient idle connections")
	}

	return nil
}

// startCleanup starts the background cleanup goroutine
func (p *DefaultConnectionPool) startCleanup() {
	p.cleanupTicker = time.NewTicker(p.config.HealthCheckInterval)

	go func() {
		for {
			select {
			case <-p.cleanupTicker.C:
				p.cleanup()
			case <-p.stopCleanup:
				return
			}
		}
	}()
}

// cleanup removes stale connections from the idle pool
func (p *DefaultConnectionPool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	now := time.Now()
	validIdle := make([]Resource, 0, len(p.idle))

	for _, resource := range p.idle {
		// Check if resource has exceeded max lifetime
		if p.config.MaxLifetime > 0 && now.Sub(resource.LastUsed()) > p.config.MaxLifetime {
			resource.Cleanup()
			continue
		}

		// Check if resource has been idle too long
		if p.config.IdleTimeout > 0 && now.Sub(resource.LastUsed()) > p.config.IdleTimeout {
			resource.Cleanup()
			continue
		}

		// Health check the resource
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := resource.HealthCheck(ctx); err != nil {
			cancel()
			resource.Cleanup()
			continue
		}
		cancel()

		validIdle = append(validIdle, resource)
	}

	p.idle = validIdle
}

// DefaultPoolConfig returns a sensible default configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinIdle:             2,
		MaxActive:           10,
		MaxIdle:             5,
		IdleTimeout:         5 * time.Minute,
		MaxLifetime:         30 * time.Minute,
		GetTimeout:          30 * time.Second,
		HealthCheckInterval: 1 * time.Minute,
		PreWarm:             true,
	}
}
