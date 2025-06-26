package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// BulkheadConfig holds configuration for the bulkhead pattern
type BulkheadConfig struct {
	// Resource limits
	MaxConcurrentRequests int           `json:"max_concurrent_requests"` // Global concurrent request limit
	MaxWaitTime           time.Duration `json:"max_wait_time"`           // Max time to wait for resource

	// Tenant isolation
	PerTenantLimits       map[string]int `json:"per_tenant_limits"`       // Per-tenant concurrent limits
	DefaultTenantLimit    int            `json:"default_tenant_limit"`    // Default limit for unlisted tenants
	EnableTenantIsolation bool           `json:"enable_tenant_isolation"` // Enable per-tenant bulkheads

	// Operation isolation
	PerOperationLimits       map[string]int `json:"per_operation_limits"`       // Per-operation concurrent limits
	DefaultOperationLimit    int            `json:"default_operation_limit"`    // Default limit for unlisted operations
	EnableOperationIsolation bool           `json:"enable_operation_isolation"` // Enable per-operation bulkheads

	// Priority handling
	EnablePriority        bool                    `json:"enable_priority"`         // Enable priority-based queuing
	PriorityExtractor     func(*lift.Context) int `json:"-"`                       // Extract priority from context
	HighPriorityThreshold int                     `json:"high_priority_threshold"` // Threshold for high priority

	// Rejection handling
	RejectionHandler func(*lift.Context, string) error `json:"-"` // Custom rejection handler

	// Observability
	Logger        observability.StructuredLogger `json:"-"`
	Metrics       observability.MetricsCollector `json:"-"`
	EnableMetrics bool                           `json:"enable_metrics"`

	// Naming
	Name string `json:"name"` // Bulkhead name for metrics
}

// BulkheadStats provides statistics about bulkhead performance
type BulkheadStats struct {
	Name                string                    `json:"name"`
	ActiveRequests      int                       `json:"active_requests"`
	QueuedRequests      int                       `json:"queued_requests"`
	TotalRequests       int64                     `json:"total_requests"`
	RejectedRequests    int64                     `json:"rejected_requests"`
	CompletedRequests   int64                     `json:"completed_requests"`
	AverageWaitTime     time.Duration             `json:"average_wait_time"`
	MaxWaitTime         time.Duration             `json:"max_wait_time"`
	ResourceUtilization float64                   `json:"resource_utilization"`
	TenantStats         map[string]*ResourceStats `json:"tenant_stats,omitempty"`
	OperationStats      map[string]*ResourceStats `json:"operation_stats,omitempty"`
}

// ResourceStats provides statistics for a specific resource pool
type ResourceStats struct {
	ActiveRequests   int     `json:"active_requests"`
	QueuedRequests   int     `json:"queued_requests"`
	TotalRequests    int64   `json:"total_requests"`
	RejectedRequests int64   `json:"rejected_requests"`
	Utilization      float64 `json:"utilization"`
	Limit            int     `json:"limit"`
}

// BulkheadMiddleware creates a bulkhead pattern middleware
func BulkheadMiddleware(config BulkheadConfig) lift.Middleware {
	// Set defaults
	if config.MaxConcurrentRequests == 0 {
		config.MaxConcurrentRequests = 100
	}
	if config.MaxWaitTime == 0 {
		config.MaxWaitTime = 30 * time.Second
	}
	if config.DefaultTenantLimit == 0 {
		config.DefaultTenantLimit = 10
	}
	if config.DefaultOperationLimit == 0 {
		config.DefaultOperationLimit = 20
	}
	if config.Name == "" {
		config.Name = "default"
	}
	if config.RejectionHandler == nil {
		config.RejectionHandler = defaultRejectionHandler
	}
	if config.PriorityExtractor == nil {
		config.PriorityExtractor = defaultPriorityExtractor
	}

	manager := &bulkheadManager{
		config:              config,
		globalSemaphore:     newSemaphore(config.MaxConcurrentRequests),
		tenantSemaphores:    make(map[string]*semaphore),
		operationSemaphores: make(map[string]*semaphore),
		stats: &BulkheadStats{
			Name:           config.Name,
			TenantStats:    make(map[string]*ResourceStats),
			OperationStats: make(map[string]*ResourceStats),
		},
	}

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			start := time.Now()

			// Extract context information
			tenantID := ctx.TenantID()
			operation := fmt.Sprintf("%s:%s", ctx.Request.Method, ctx.Request.Path)
			priority := config.PriorityExtractor(ctx)

			// Acquire resources
			acquired, waitTime, err := manager.acquireResources(ctx.Context, tenantID, operation, priority)
			if err != nil {
				// Resource acquisition failed
				if config.Logger != nil {
					config.Logger.Warn("Bulkhead resource acquisition failed", map[string]any{
						"bulkhead_name": config.Name,
						"tenant_id":     tenantID,
						"operation":     operation,
						"priority":      priority,
						"wait_time":     waitTime.String(),
						"error":         "[REDACTED_ERROR_DETAIL]", // Sanitized for security
					})
				}

				// Record metrics
				if config.EnableMetrics && config.Metrics != nil {
					manager.recordRejection(tenantID, operation, waitTime)
				}

				return config.RejectionHandler(ctx, err.Error())
			}

			// Ensure resources are released
			defer func() {
				manager.releaseResources(acquired, tenantID, operation)

				duration := time.Since(start)

				// Record completion metrics
				if config.EnableMetrics && config.Metrics != nil {
					manager.recordCompletion(tenantID, operation, duration, waitTime)
				}

				if config.Logger != nil {
					config.Logger.Debug("Bulkhead request completed", map[string]any{
						"bulkhead_name": config.Name,
						"tenant_id":     tenantID,
						"operation":     operation,
						"duration":      duration.String(),
						"wait_time":     waitTime.String(),
					})
				}
			}()

			// Record successful acquisition
			if config.EnableMetrics && config.Metrics != nil {
				manager.recordAcquisition(tenantID, operation, waitTime)
			}

			// Execute the handler
			return next.Handle(ctx)
		})
	}
}

// bulkheadManager manages resource allocation and isolation
type bulkheadManager struct {
	config              BulkheadConfig
	globalSemaphore     *semaphore
	tenantSemaphores    map[string]*semaphore
	operationSemaphores map[string]*semaphore
	mutex               sync.RWMutex
	stats               *BulkheadStats
	statsMutex          sync.RWMutex
}

// acquiredResources tracks which resources were acquired for a request
type acquiredResources struct {
	global    bool
	tenant    bool
	operation bool
}

// acquireResources attempts to acquire all necessary resources
func (bm *bulkheadManager) acquireResources(ctx context.Context, tenantID, operation string, priority int) (*acquiredResources, time.Duration, error) {
	start := time.Now()
	acquired := &acquiredResources{}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, bm.config.MaxWaitTime)
	defer cancel()

	// Acquire global semaphore
	if !bm.globalSemaphore.tryAcquire(timeoutCtx, priority) {
		return nil, time.Since(start), fmt.Errorf("global resource limit exceeded")
	}
	acquired.global = true

	// Acquire tenant semaphore if enabled
	if bm.config.EnableTenantIsolation && tenantID != "" {
		tenantSem := bm.getTenantSemaphore(tenantID)
		if !tenantSem.tryAcquire(timeoutCtx, priority) {
			bm.globalSemaphore.release()
			return nil, time.Since(start), fmt.Errorf("tenant resource limit exceeded")
		}
		acquired.tenant = true
	}

	// Acquire operation semaphore if enabled
	if bm.config.EnableOperationIsolation {
		opSem := bm.getOperationSemaphore(operation)
		if !opSem.tryAcquire(timeoutCtx, priority) {
			if acquired.tenant {
				bm.getTenantSemaphore(tenantID).release()
			}
			bm.globalSemaphore.release()
			return nil, time.Since(start), fmt.Errorf("operation resource limit exceeded")
		}
		acquired.operation = true
	}

	return acquired, time.Since(start), nil
}

// releaseResources releases all acquired resources
func (bm *bulkheadManager) releaseResources(acquired *acquiredResources, tenantID, operation string) {
	if acquired.operation {
		bm.getOperationSemaphore(operation).release()
	}
	if acquired.tenant {
		bm.getTenantSemaphore(tenantID).release()
	}
	if acquired.global {
		bm.globalSemaphore.release()
	}
}

// getTenantSemaphore gets or creates a semaphore for a tenant
func (bm *bulkheadManager) getTenantSemaphore(tenantID string) *semaphore {
	bm.mutex.RLock()
	sem, exists := bm.tenantSemaphores[tenantID]
	bm.mutex.RUnlock()

	if exists {
		return sem
	}

	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// Double-check after acquiring write lock
	if sem, exists := bm.tenantSemaphores[tenantID]; exists {
		return sem
	}

	// Determine limit for this tenant
	limit := bm.config.DefaultTenantLimit
	if tenantLimit, exists := bm.config.PerTenantLimits[tenantID]; exists {
		limit = tenantLimit
	}

	sem = newSemaphore(limit)
	bm.tenantSemaphores[tenantID] = sem

	// Initialize stats
	bm.statsMutex.Lock()
	bm.stats.TenantStats[tenantID] = &ResourceStats{
		Limit: limit,
	}
	bm.statsMutex.Unlock()

	return sem
}

// getOperationSemaphore gets or creates a semaphore for an operation
func (bm *bulkheadManager) getOperationSemaphore(operation string) *semaphore {
	bm.mutex.RLock()
	sem, exists := bm.operationSemaphores[operation]
	bm.mutex.RUnlock()

	if exists {
		return sem
	}

	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// Double-check after acquiring write lock
	if sem, exists := bm.operationSemaphores[operation]; exists {
		return sem
	}

	// Determine limit for this operation
	limit := bm.config.DefaultOperationLimit
	if opLimit, exists := bm.config.PerOperationLimits[operation]; exists {
		limit = opLimit
	}

	sem = newSemaphore(limit)
	bm.operationSemaphores[operation] = sem

	// Initialize stats
	bm.statsMutex.Lock()
	bm.stats.OperationStats[operation] = &ResourceStats{
		Limit: limit,
	}
	bm.statsMutex.Unlock()

	return sem
}

// recordAcquisition records successful resource acquisition
func (bm *bulkheadManager) recordAcquisition(tenantID, operation string, waitTime time.Duration) {
	if !bm.config.EnableMetrics || bm.config.Metrics == nil {
		return
	}

	tags := map[string]string{
		"bulkhead_name": bm.config.Name,
		"result":        "acquired",
	}

	if bm.config.EnableTenantIsolation && tenantID != "" {
		tags["tenant_id"] = tenantID
	}

	if bm.config.EnableOperationIsolation {
		tags["operation"] = operation
	}

	metrics := bm.config.Metrics.WithTags(tags)

	// Record acquisition
	counter := metrics.Counter("bulkhead.acquisitions.total")
	counter.Inc()

	// Record wait time
	histogram := metrics.Histogram("bulkhead.wait_time")
	histogram.Observe(float64(waitTime.Milliseconds()))
}

// recordRejection records resource acquisition rejection
func (bm *bulkheadManager) recordRejection(tenantID, operation string, waitTime time.Duration) {
	if !bm.config.EnableMetrics || bm.config.Metrics == nil {
		return
	}

	tags := map[string]string{
		"bulkhead_name": bm.config.Name,
		"result":        "rejected",
	}

	if bm.config.EnableTenantIsolation && tenantID != "" {
		tags["tenant_id"] = tenantID
	}

	if bm.config.EnableOperationIsolation {
		tags["operation"] = operation
	}

	metrics := bm.config.Metrics.WithTags(tags)

	// Record rejection
	counter := metrics.Counter("bulkhead.rejections.total")
	counter.Inc()

	// Record wait time before rejection
	histogram := metrics.Histogram("bulkhead.rejection_wait_time")
	histogram.Observe(float64(waitTime.Milliseconds()))
}

// recordCompletion records request completion
func (bm *bulkheadManager) recordCompletion(tenantID, operation string, duration, waitTime time.Duration) {
	if !bm.config.EnableMetrics || bm.config.Metrics == nil {
		return
	}

	tags := map[string]string{
		"bulkhead_name": bm.config.Name,
	}

	if bm.config.EnableTenantIsolation && tenantID != "" {
		tags["tenant_id"] = tenantID
	}

	if bm.config.EnableOperationIsolation {
		tags["operation"] = operation
	}

	metrics := bm.config.Metrics.WithTags(tags)

	// Record completion
	counter := metrics.Counter("bulkhead.completions.total")
	counter.Inc()

	// Record execution duration
	histogram := metrics.Histogram("bulkhead.execution_duration")
	histogram.Observe(float64(duration.Milliseconds()))

	// Record wait time for completed requests
	waitTimeHist := metrics.Histogram("bulkhead.completion_wait_time")
	waitTimeHist.Observe(float64(waitTime.Milliseconds()))

	// Record resource utilization
	utilization := float64(bm.globalSemaphore.active()) / float64(bm.globalSemaphore.capacity())
	gauge := metrics.Gauge("bulkhead.utilization")
	gauge.Set(utilization)
}

// GetStats returns current bulkhead statistics
func (bm *bulkheadManager) GetStats() BulkheadStats {
	bm.statsMutex.RLock()
	defer bm.statsMutex.RUnlock()

	stats := *bm.stats
	stats.ActiveRequests = bm.globalSemaphore.active()
	stats.ResourceUtilization = float64(stats.ActiveRequests) / float64(bm.config.MaxConcurrentRequests)

	return stats
}

// semaphore implements a priority-aware semaphore
type semaphore struct {
	maxCapacity int
	activeCount int
	waitQueue   []*waiter
	mutex       sync.Mutex
}

// waiter represents a waiting request
type waiter struct {
	priority int
	ch       chan bool
	ctx      context.Context
}

// newSemaphore creates a new semaphore with the given capacity
func newSemaphore(capacity int) *semaphore {
	return &semaphore{
		maxCapacity: capacity,
		waitQueue:   make([]*waiter, 0),
	}
}

// tryAcquire attempts to acquire the semaphore with priority support
func (s *semaphore) tryAcquire(ctx context.Context, priority int) bool {
	s.mutex.Lock()

	// Try immediate acquisition
	if s.activeCount < s.maxCapacity {
		s.activeCount++
		s.mutex.Unlock()
		return true
	}

	// Need to wait - create waiter
	waiter := &waiter{
		priority: priority,
		ch:       make(chan bool, 1),
		ctx:      ctx,
	}

	// Insert waiter in priority order
	s.insertWaiter(waiter)
	s.mutex.Unlock()

	// Wait for acquisition or timeout
	select {
	case acquired := <-waiter.ch:
		return acquired
	case <-ctx.Done():
		// Remove from queue on timeout
		s.removeWaiter(waiter)
		return false
	}
}

// release releases the semaphore and notifies waiting requests
func (s *semaphore) release() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.activeCount > 0 {
		s.activeCount--
	}

	// Notify next waiter
	if len(s.waitQueue) > 0 {
		waiter := s.waitQueue[0]
		s.waitQueue = s.waitQueue[1:]
		s.activeCount++

		select {
		case waiter.ch <- true:
		default:
			// Waiter already timed out, try next
			if s.activeCount > 0 {
				s.activeCount--
			}
			if len(s.waitQueue) > 0 {
				s.release() // Recursive call to try next waiter
			}
		}
	}
}

// insertWaiter inserts a waiter in priority order (higher priority first)
func (s *semaphore) insertWaiter(w *waiter) {
	for i, existing := range s.waitQueue {
		if w.priority > existing.priority {
			// Insert at position i
			s.waitQueue = append(s.waitQueue[:i], append([]*waiter{w}, s.waitQueue[i:]...)...)
			return
		}
	}
	// Append at end
	s.waitQueue = append(s.waitQueue, w)
}

// removeWaiter removes a waiter from the queue
func (s *semaphore) removeWaiter(target *waiter) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, w := range s.waitQueue {
		if w == target {
			s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
			break
		}
	}
}

// active returns the number of active acquisitions
func (s *semaphore) active() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.activeCount
}

// capacity returns the semaphore capacity
func (s *semaphore) capacity() int {
	return s.maxCapacity
}

// Default implementations

// defaultRejectionHandler provides a default rejection response
func defaultRejectionHandler(ctx *lift.Context, reason string) error {
	ctx.Status(503).JSON(map[string]any{
		"error":   "Service temporarily unavailable",
		"message": "Resource limit exceeded",
		"reason":  reason,
		"code":    "BULKHEAD_LIMIT_EXCEEDED",
	})
	// Return an error to indicate rejection
	return fmt.Errorf("bulkhead limit exceeded: %s", reason)
}

// defaultPriorityExtractor extracts priority from context (default: normal priority)
func defaultPriorityExtractor(ctx *lift.Context) int {
	// Check for priority header
	if priority := ctx.Request.Headers["X-Priority"]; priority != "" {
		switch priority {
		case "high":
			return 10
		case "low":
			return 1
		default:
			return 5
		}
	}
	return 5 // Normal priority
}

// Utility functions for common bulkhead configurations

// NewBasicBulkhead creates a basic bulkhead with sensible defaults
func NewBasicBulkhead(name string, maxConcurrent int) BulkheadConfig {
	return BulkheadConfig{
		Name:                  name,
		MaxConcurrentRequests: maxConcurrent,
		MaxWaitTime:           30 * time.Second,
		DefaultTenantLimit:    maxConcurrent / 10,
		DefaultOperationLimit: maxConcurrent / 5,
		EnableMetrics:         true,
	}
}

// NewTenantBulkhead creates a tenant-isolated bulkhead
func NewTenantBulkhead(name string, maxConcurrent int, tenantLimits map[string]int) BulkheadConfig {
	config := NewBasicBulkhead(name, maxConcurrent)
	config.EnableTenantIsolation = true
	config.PerTenantLimits = tenantLimits
	return config
}

// NewOperationBulkhead creates an operation-isolated bulkhead
func NewOperationBulkhead(name string, maxConcurrent int, operationLimits map[string]int) BulkheadConfig {
	config := NewBasicBulkhead(name, maxConcurrent)
	config.EnableOperationIsolation = true
	config.PerOperationLimits = operationLimits
	return config
}

// NewPriorityBulkhead creates a priority-aware bulkhead
func NewPriorityBulkhead(name string, maxConcurrent int, priorityExtractor func(*lift.Context) int) BulkheadConfig {
	config := NewBasicBulkhead(name, maxConcurrent)
	config.EnablePriority = true
	config.PriorityExtractor = priorityExtractor
	config.HighPriorityThreshold = 8
	return config
}
