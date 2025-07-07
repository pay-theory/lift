# Sprint 2: Core Feature Implementation
**Duration**: 2-3 weeks  
**Priority**: HIGH  
**Goal**: Complete missing core functionality for production readiness

## Overview
This sprint focuses on implementing the core features that are currently stubbed out, particularly rate limiting, circuit breaker, service mesh integration, and load shedding. All implementations must integrate with DynamORM where appropriate and follow Lift's middleware patterns.

## Task 1: Complete Rate Limiting Sliding Window

### Current State:
```go
// TODO: Implement sliding window algorithm
// Currently using fixed window which has burst issues
```

### Implementation Requirements:

#### DynamORM Model:
```go
// pkg/models/ratelimit_sliding.go
package models

import (
    "time"
    "github.com/pay-theory/dynamorm"
)

// SlidingWindowEntry represents a single request in the sliding window
type SlidingWindowEntry struct {
    dynamorm.Model
    
    // Composite key: RateLimitKey#Timestamp
    PK string `dynamorm:"pk" json:"-"`
    SK string `dynamorm:"sk" json:"-"`
    
    // Attributes
    RateLimitKey string    `json:"rate_limit_key"`
    Timestamp    time.Time `json:"timestamp"`
    Weight       int       `json:"weight,omitempty"` // For weighted rate limiting
    RequestID    string    `json:"request_id"`
    
    // TTL for automatic cleanup (set to window duration + buffer)
    ExpiresAt int64 `dynamorm:"ttl" json:"-"`
}

// Key structure for efficient queries
func (s *SlidingWindowEntry) Key(rateLimitKey string, timestamp time.Time) {
    s.PK = fmt.Sprintf("RATELIMIT#%s", rateLimitKey)
    s.SK = fmt.Sprintf("TS#%d", timestamp.UnixNano())
}
```

#### Middleware Implementation:
```go
// pkg/middleware/ratelimit_sliding.go
package middleware

import (
    "context"
    "fmt"
    "time"
    
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/models"
    "github.com/pay-theory/lift/pkg/dynamorm"
)

type SlidingWindowRateLimiter struct {
    db            *dynamorm.DynamORMWrapper
    windowSize    time.Duration
    limit         int
    keyExtractor  KeyExtractorFunc
}

func NewSlidingWindowRateLimiter(config RateLimitConfig) (*SlidingWindowRateLimiter, error) {
    // Validate configuration
    if config.WindowSize <= 0 {
        return nil, fmt.Errorf("window size must be positive")
    }
    
    return &SlidingWindowRateLimiter{
        windowSize:   config.WindowSize,
        limit:        config.Limit,
        keyExtractor: config.KeyExtractor,
    }, nil
}

func (r *SlidingWindowRateLimiter) Middleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Extract rate limit key
            key := r.keyExtractor(ctx)
            if key == "" {
                return next.Handle(ctx)
            }
            
            // Get DynamORM instance
            db, err := dynamorm.TenantDB(ctx)
            if err != nil {
                // Log error but don't block request
                ctx.Logger.Error("Rate limiter DB unavailable", map[string]any{
                    "error": err.Error(),
                })
                return next.Handle(ctx)
            }
            
            // Check rate limit
            allowed, remaining, resetAt, err := r.checkRateLimit(ctx.Context, db, key)
            if err != nil {
                // On error, allow request but log
                ctx.Logger.Error("Rate limit check failed", map[string]any{
                    "error": err.Error(),
                    "key": key,
                })
                return next.Handle(ctx)
            }
            
            // Set rate limit headers
            ctx.Response.Header("X-RateLimit-Limit", fmt.Sprintf("%d", r.limit))
            ctx.Response.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
            ctx.Response.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
            
            if !allowed {
                ctx.Response.Header("Retry-After", fmt.Sprintf("%d", int(time.Until(resetAt).Seconds())))
                return ctx.Response.Status(429).JSON(map[string]any{
                    "error": "rate_limit_exceeded",
                    "message": "Too many requests",
                    "retry_after": int(time.Until(resetAt).Seconds()),
                })
            }
            
            // Record this request
            if err := r.recordRequest(ctx.Context, db, key); err != nil {
                ctx.Logger.Warn("Failed to record rate limit entry", map[string]any{
                    "error": err.Error(),
                    "key": key,
                })
            }
            
            return next.Handle(ctx)
        })
    }
}

func (r *SlidingWindowRateLimiter) checkRateLimit(ctx context.Context, db *dynamorm.DynamORMWrapper, key string) (bool, int, time.Time, error) {
    now := time.Now()
    windowStart := now.Add(-r.windowSize)
    
    // Query entries in the sliding window
    entries := []models.SlidingWindowEntry{}
    err := db.QueryPKWithSKRange(
        fmt.Sprintf("RATELIMIT#%s", key),
        fmt.Sprintf("TS#%d", windowStart.UnixNano()),
        fmt.Sprintf("TS#%d", now.UnixNano()),
        &entries,
    )
    if err != nil {
        return false, 0, time.Time{}, err
    }
    
    // Count requests in window
    count := 0
    for _, entry := range entries {
        count += entry.Weight
        if entry.Weight == 0 {
            count++ // Default weight is 1
        }
    }
    
    remaining := r.limit - count
    if remaining < 0 {
        remaining = 0
    }
    
    // Reset time is when the oldest entry expires
    resetAt := now.Add(r.windowSize)
    if len(entries) > 0 {
        resetAt = entries[0].Timestamp.Add(r.windowSize)
    }
    
    return count < r.limit, remaining, resetAt, nil
}

func (r *SlidingWindowRateLimiter) recordRequest(ctx context.Context, db *dynamorm.DynamORMWrapper, key string) error {
    entry := &models.SlidingWindowEntry{
        RequestID: uuid.New().String(),
        Timestamp: time.Now(),
        Weight:    1,
        ExpiresAt: time.Now().Add(r.windowSize + time.Hour).Unix(), // Buffer for cleanup
    }
    entry.Key(key, entry.Timestamp)
    
    return db.Create(entry)
}
```

### Integration with Lift App:
```go
// Example usage
rateLimiter, err := middleware.NewSlidingWindowRateLimiter(middleware.RateLimitConfig{
    WindowSize: 15 * time.Minute,
    Limit:      100,
    KeyExtractor: middleware.IPKeyExtractor, // or UserKeyExtractor, TenantKeyExtractor
})

app.Use(rateLimiter.Middleware())
```

## Task 2: Implement Circuit Breaker

### Implementation Requirements:

#### DynamORM Model:
```go
// pkg/models/circuit_breaker.go
package models

type CircuitBreakerState struct {
    dynamorm.Model
    
    PK string `dynamorm:"pk" json:"-"` // CIRCUIT#ServiceName
    SK string `dynamorm:"sk" json:"-"` // STATE
    
    ServiceName      string    `json:"service_name"`
    State           string    `json:"state"` // "closed", "open", "half-open"
    FailureCount    int       `json:"failure_count"`
    SuccessCount    int       `json:"success_count"`
    LastFailureTime time.Time `json:"last_failure_time,omitempty"`
    LastSuccessTime time.Time `json:"last_success_time,omitempty"`
    NextRetryTime   time.Time `json:"next_retry_time,omitempty"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

#### Circuit Breaker Implementation:
```go
// pkg/middleware/circuitbreaker.go
package middleware

type CircuitBreaker struct {
    db                *dynamorm.DynamORMWrapper
    serviceName       string
    failureThreshold  int
    successThreshold  int
    timeout          time.Duration
    halfOpenRequests int
}

func NewCircuitBreaker(config CircuitBreakerConfig) (*CircuitBreaker, error) {
    if config.FailureThreshold <= 0 {
        config.FailureThreshold = 5 // Default
    }
    if config.Timeout <= 0 {
        config.Timeout = 60 * time.Second // Default
    }
    
    return &CircuitBreaker{
        serviceName:      config.ServiceName,
        failureThreshold: config.FailureThreshold,
        successThreshold: config.SuccessThreshold,
        timeout:         config.Timeout,
        halfOpenRequests: config.HalfOpenRequests,
    }, nil
}

func (cb *CircuitBreaker) Middleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Get current state
            state, err := cb.getState(ctx)
            if err != nil {
                // On error, allow request
                return next.Handle(ctx)
            }
            
            // Check if circuit is open
            if state.State == "open" {
                if time.Now().Before(state.NextRetryTime) {
                    return lift.NewError(http.StatusServiceUnavailable,
                        "Service temporarily unavailable",
                        map[string]interface{}{
                            "service": cb.serviceName,
                            "retry_after": int(time.Until(state.NextRetryTime).Seconds()),
                        })
                }
                // Move to half-open
                state.State = "half-open"
                state.SuccessCount = 0
                cb.updateState(ctx, state)
            }
            
            // Execute request
            err = next.Handle(ctx)
            
            // Update circuit breaker based on result
            if err != nil {
                cb.recordFailure(ctx, state)
            } else {
                cb.recordSuccess(ctx, state)
            }
            
            return err
        })
    }
}

func (cb *CircuitBreaker) recordFailure(ctx *lift.Context, state *models.CircuitBreakerState) {
    state.FailureCount++
    state.LastFailureTime = time.Now()
    
    // Check if we should open the circuit
    if state.State == "closed" && state.FailureCount >= cb.failureThreshold {
        state.State = "open"
        state.NextRetryTime = time.Now().Add(cb.timeout)
        ctx.Logger.Warn("Circuit breaker opened", map[string]any{
            "service": cb.serviceName,
            "failures": state.FailureCount,
        })
    } else if state.State == "half-open" {
        // Failed in half-open, go back to open
        state.State = "open"
        state.NextRetryTime = time.Now().Add(cb.timeout)
        state.FailureCount = 0
    }
    
    cb.updateState(ctx, state)
}

func (cb *CircuitBreaker) recordSuccess(ctx *lift.Context, state *models.CircuitBreakerState) {
    state.SuccessCount++
    state.LastSuccessTime = time.Now()
    
    if state.State == "half-open" && state.SuccessCount >= cb.successThreshold {
        // Enough successes, close the circuit
        state.State = "closed"
        state.FailureCount = 0
        ctx.Logger.Info("Circuit breaker closed", map[string]any{
            "service": cb.serviceName,
        })
    }
    
    // Reset failure count on success in closed state
    if state.State == "closed" {
        state.FailureCount = 0
    }
    
    cb.updateState(ctx, state)
}
```

## Task 3: Complete Service Mesh Integration

### Implementation Requirements:

```go
// pkg/adapters/servicemesh.go
package adapters

import (
    "github.com/aws/aws-sdk-go-v2/service/appmesh"
    "github.com/aws/aws-sdk-go-v2/service/servicediscovery"
)

type ServiceMeshAdapter struct {
    meshName        string
    virtualNode     string
    serviceName     string
    namespace       string
    appMeshClient   *appmesh.Client
    sdClient        *servicediscovery.Client
}

func NewServiceMeshAdapter(config ServiceMeshConfig) (*ServiceMeshAdapter, error) {
    // Initialize AWS clients
    cfg, err := awsconfig.LoadDefaultConfig(context.Background())
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }
    
    return &ServiceMeshAdapter{
        meshName:      config.MeshName,
        virtualNode:   config.VirtualNode,
        serviceName:   config.ServiceName,
        namespace:     config.Namespace,
        appMeshClient: appmesh.NewFromConfig(cfg),
        sdClient:      servicediscovery.NewFromConfig(cfg),
    }, nil
}

func (s *ServiceMeshAdapter) RegisterService(ctx context.Context) error {
    // Register with AWS Cloud Map
    instanceId := fmt.Sprintf("%s-%s", s.serviceName, uuid.New().String())
    
    _, err := s.sdClient.RegisterInstance(ctx, &servicediscovery.RegisterInstanceInput{
        ServiceId: aws.String(s.getServiceId()),
        InstanceId: aws.String(instanceId),
        Attributes: map[string]string{
            "AWS_INSTANCE_IPV4": s.getPrivateIP(),
            "AWS_INSTANCE_PORT": s.getPort(),
            "VIRTUAL_NODE":      s.virtualNode,
        },
    })
    
    if err != nil {
        return fmt.Errorf("failed to register service: %w", err)
    }
    
    // Set up health check endpoint
    s.setupHealthCheck()
    
    // Configure Envoy proxy settings
    s.configureEnvoy()
    
    return nil
}

func (s *ServiceMeshAdapter) Middleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Add service mesh headers
            ctx.Response.Header("X-Service-Name", s.serviceName)
            ctx.Response.Header("X-Virtual-Node", s.virtualNode)
            
            // Extract trace headers for distributed tracing
            traceHeaders := s.extractTraceHeaders(ctx)
            ctx.Set("trace_headers", traceHeaders)
            
            // Add service mesh metadata to context
            ctx.Set("mesh_name", s.meshName)
            ctx.Set("virtual_node", s.virtualNode)
            
            return next.Handle(ctx)
        })
    }
}

// Health check for service mesh
func (s *ServiceMeshAdapter) HealthCheckHandler() lift.Handler {
    return lift.HandlerFunc(func(ctx *lift.Context) error {
        // Check downstream dependencies
        health := s.checkDependencies(ctx)
        
        if !health.Healthy {
            return ctx.Response.Status(503).JSON(health)
        }
        
        return ctx.Response.JSON(health)
    })
}
```

## Task 4: Implement Load Shedding

### Implementation Requirements:

```go
// pkg/middleware/loadshedding.go
package middleware

import (
    "sync/atomic"
    "github.com/pay-theory/lift/pkg/metrics"
)

type LoadShedder struct {
    // Configuration
    cpuThreshold      float64
    memoryThreshold   float64
    latencyThreshold  time.Duration
    queueSizeLimit    int
    
    // Metrics
    currentCPU        atomic.Value // float64
    currentMemory     atomic.Value // float64
    avgLatency        atomic.Value // time.Duration
    queueSize         atomic.Int64
    
    // Shedding algorithms
    algorithm         SheddingAlgorithm
    priorityExtractor PriorityExtractorFunc
}

type SheddingAlgorithm interface {
    ShouldShed(ctx *lift.Context, metrics LoadMetrics) bool
}

// Adaptive Load Shedding Algorithm
type AdaptiveLoadShedding struct {
    // Uses Little's Law and response time targets
    targetLatency    time.Duration
    minShedProbability float64
    maxShedProbability float64
}

func (a *AdaptiveLoadShedding) ShouldShed(ctx *lift.Context, metrics LoadMetrics) bool {
    // Calculate shedding probability based on current load
    shedProbability := a.calculateShedProbability(metrics)
    
    // Extract request priority
    priority := a.extractPriority(ctx)
    
    // Adjust probability based on priority
    adjustedProbability := shedProbability * (1.0 - priority)
    
    // Random shed decision
    return rand.Float64() < adjustedProbability
}

func (a *AdaptiveLoadShedding) calculateShedProbability(metrics LoadMetrics) float64 {
    // Use multiple signals
    latencyRatio := float64(metrics.AvgLatency) / float64(a.targetLatency)
    cpuRatio := metrics.CPU / 0.8 // Target 80% CPU
    memoryRatio := metrics.Memory / 0.8
    
    // Combine signals
    loadFactor := math.Max(latencyRatio, math.Max(cpuRatio, memoryRatio))
    
    // Map to probability with bounds
    probability := (loadFactor - 1.0) / 2.0
    probability = math.Max(a.minShedProbability, math.Min(a.maxShedProbability, probability))
    
    return probability
}

func NewLoadShedder(config LoadShedderConfig) (*LoadShedder, error) {
    ls := &LoadShedder{
        cpuThreshold:     config.CPUThreshold,
        memoryThreshold:  config.MemoryThreshold,
        latencyThreshold: config.LatencyThreshold,
        queueSizeLimit:   config.QueueSizeLimit,
        algorithm:        config.Algorithm,
    }
    
    // Start metrics collection
    go ls.collectMetrics()
    
    return ls, nil
}

func (ls *LoadShedder) Middleware() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Increment queue size
            queueSize := ls.queueSize.Add(1)
            defer ls.queueSize.Add(-1)
            
            // Check queue limit first (hard limit)
            if queueSize > int64(ls.queueSizeLimit) {
                ls.recordShed(ctx, "queue_limit")
                return ctx.Response.Status(503).JSON(map[string]any{
                    "error": "service_overloaded",
                    "message": "Server is currently overloaded",
                    "retry_after": 5,
                })
            }
            
            // Get current metrics
            metrics := ls.getCurrentMetrics()
            
            // Apply shedding algorithm
            if ls.algorithm.ShouldShed(ctx, metrics) {
                ls.recordShed(ctx, "adaptive")
                return ctx.Response.Status(503).JSON(map[string]any{
                    "error": "service_overloaded",
                    "message": "Server is currently overloaded",
                    "retry_after": 5,
                })
            }
            
            // Track request latency
            start := time.Now()
            err := next.Handle(ctx)
            latency := time.Since(start)
            
            // Update latency metrics
            ls.updateLatency(latency)
            
            return err
        })
    }
}

// Priority-based load shedding
func (ls *LoadShedder) extractPriority(ctx *lift.Context) float64 {
    // Check for priority header
    if priority := ctx.Header("X-Priority"); priority != "" {
        switch priority {
        case "critical":
            return 1.0
        case "high":
            return 0.8
        case "normal":
            return 0.5
        case "low":
            return 0.2
        }
    }
    
    // Check user tier for multi-tenant
    if ctx.TenantID() != "" {
        tenant := ls.getTenantTier(ctx.TenantID())
        switch tenant.Tier {
        case "enterprise":
            return 0.9
        case "professional":
            return 0.7
        case "starter":
            return 0.4
        }
    }
    
    return 0.5 // Default priority
}
```

## Task 5: Complete Memory Cache TTL

### Implementation Requirements:

```go
// pkg/cache/memory_ttl.go
package cache

import (
    "sync"
    "time"
    "container/heap"
)

type TTLCache struct {
    mu          sync.RWMutex
    items       map[string]*cacheItem
    expHeap     *expirationHeap
    maxSize     int
    evictPolicy EvictionPolicy
    stopCleanup chan struct{}
}

type cacheItem struct {
    key       string
    value     interface{}
    expiresAt time.Time
    size      int
    hits      int
    lastAccess time.Time
    index     int // heap index
}

type expirationHeap []*cacheItem

func NewTTLCache(config CacheConfig) *TTLCache {
    cache := &TTLCache{
        items:       make(map[string]*cacheItem),
        expHeap:     &expirationHeap{},
        maxSize:     config.MaxSize,
        evictPolicy: config.EvictionPolicy,
        stopCleanup: make(chan struct{}),
    }
    
    heap.Init(cache.expHeap)
    
    // Start cleanup goroutine
    go cache.cleanupLoop()
    
    return cache
}

func (c *TTLCache) Set(key string, value interface{}, ttl time.Duration) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    expiresAt := time.Now().Add(ttl)
    size := c.calculateSize(value)
    
    // Check if we need to evict
    if c.currentSize()+size > c.maxSize {
        c.evict(size)
    }
    
    item := &cacheItem{
        key:        key,
        value:      value,
        expiresAt:  expiresAt,
        size:       size,
        lastAccess: time.Now(),
    }
    
    // Remove old item if exists
    if old, exists := c.items[key]; exists {
        heap.Remove(c.expHeap, old.index)
    }
    
    // Add to cache and heap
    c.items[key] = item
    heap.Push(c.expHeap, item)
    
    return nil
}

func (c *TTLCache) Get(key string) (interface{}, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    item, exists := c.items[key]
    if !exists {
        return nil, false
    }
    
    // Check if expired
    if time.Now().After(item.expiresAt) {
        c.removeItem(item)
        return nil, false
    }
    
    // Update access stats
    item.hits++
    item.lastAccess = time.Now()
    
    return item.value, true
}

func (c *TTLCache) cleanupLoop() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            c.removeExpired()
        case <-c.stopCleanup:
            return
        }
    }
}

func (c *TTLCache) removeExpired() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    for c.expHeap.Len() > 0 {
        item := (*c.expHeap)[0]
        if now.Before(item.expiresAt) {
            break
        }
        
        heap.Pop(c.expHeap)
        delete(c.items, item.key)
    }
}

// Eviction policies
func (c *TTLCache) evict(needed int) {
    switch c.evictPolicy {
    case LRU:
        c.evictLRU(needed)
    case LFU:
        c.evictLFU(needed)
    case FIFO:
        c.evictFIFO(needed)
    }
}

// Integration with Lift middleware
func CacheMiddleware(cache *TTLCache) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Generate cache key
            key := generateCacheKey(ctx)
            
            // Check cache
            if cached, found := cache.Get(key); found {
                ctx.Logger.Debug("Cache hit", map[string]any{
                    "key": key,
                })
                return ctx.Response.JSON(cached)
            }
            
            // Create response recorder
            rec := NewResponseRecorder()
            ctx.Response = rec
            
            // Execute handler
            err := next.Handle(ctx)
            if err != nil {
                return err
            }
            
            // Cache successful responses
            if rec.StatusCode >= 200 && rec.StatusCode < 300 {
                ttl := getCacheTTL(ctx)
                cache.Set(key, rec.Body, ttl)
            }
            
            // Write actual response
            return ctx.Response.Write(rec.Body)
        })
    }
}
```

## Testing Strategy

### Unit Tests:
- Test each algorithm independently
- Verify TTL expiration
- Test concurrent access patterns
- Validate eviction policies

### Integration Tests:
- Test with DynamORM backend
- Verify distributed rate limiting
- Test circuit breaker state transitions
- Validate service mesh registration

### Load Tests:
- Verify load shedding effectiveness
- Test rate limit accuracy under load
- Measure cache hit rates
- Validate circuit breaker performance

## Rollout Plan

### Week 1:
- Day 1-3: Rate limiting sliding window
- Day 4-5: Circuit breaker implementation

### Week 2:
- Day 1-3: Service mesh integration
- Day 4-5: Load shedding algorithms

### Week 3:
- Day 1-2: Memory cache TTL
- Day 3-4: Integration testing
- Day 5: Performance testing and optimization

## Success Criteria

1. **Rate limiting** accurately enforces limits with <5% error margin
2. **Circuit breaker** prevents cascade failures
3. **Service mesh** successfully registers and handles traffic
4. **Load shedding** maintains p99 latency under load
5. **Cache** reduces backend load by >50%
6. **All features** integrate seamlessly with DynamORM