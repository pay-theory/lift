package middleware

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/observability"
)

// LoadSheddingStrategy defines different load shedding strategies
type LoadSheddingStrategy string

const (
	LoadSheddingRandom   LoadSheddingStrategy = "random"   // Random shedding based on probability
	LoadSheddingPriority LoadSheddingStrategy = "priority" // Priority-based shedding
	LoadSheddingAdaptive LoadSheddingStrategy = "adaptive" // Adaptive shedding based on system metrics
	LoadSheddingCircuit  LoadSheddingStrategy = "circuit"  // Circuit breaker style shedding
	LoadSheddingCustom   LoadSheddingStrategy = "custom"   // Custom shedding algorithm

	// Backward compatibility aliases
	LoadSheddingStrategyRandom   = LoadSheddingRandom
	LoadSheddingStrategyPriority = LoadSheddingPriority
	LoadSheddingStrategyAdaptive = LoadSheddingAdaptive
	LoadSheddingStrategyCircuit  = LoadSheddingCircuit
	LoadSheddingStrategyCustom   = LoadSheddingCustom
)

// LoadSheddingConfig holds configuration for load shedding
type LoadSheddingConfig struct {
	// Basic settings
	Strategy LoadSheddingStrategy `json:"strategy"` // Load shedding strategy
	Enabled  bool                 `json:"enabled"`  // Enable/disable load shedding

	// Threshold settings
	CPUThreshold       float64       `json:"cpu_threshold"`        // CPU usage threshold (0.0-1.0)
	MemoryThreshold    float64       `json:"memory_threshold"`     // Memory usage threshold (0.0-1.0)
	LatencyThreshold   time.Duration `json:"latency_threshold"`    // Response time threshold
	ErrorRateThreshold float64       `json:"error_rate_threshold"` // Error rate threshold (0.0-1.0)

	// Adaptive settings
	TargetLatency   time.Duration `json:"target_latency"`    // Target response time
	MaxSheddingRate float64       `json:"max_shedding_rate"` // Maximum shedding rate (0.0-1.0)
	MinSheddingRate float64       `json:"min_shedding_rate"` // Minimum shedding rate (0.0-1.0)
	SheddingRate    float64       `json:"shedding_rate"`     // Fixed shedding rate (for simple strategies)
	AdaptationRate  float64       `json:"adaptation_rate"`   // How quickly to adapt (0.0-1.0)

	// Priority settings
	PriorityExtractor  func(*lift.Context) int `json:"-"`                   // Extract priority from request
	PriorityThresholds map[int]float64         `json:"priority_thresholds"` // Shedding rates by priority

	// Custom algorithm
	CustomShedder func(*lift.Context, *LoadMetrics) bool `json:"-"` // Custom shedding function

	// Monitoring settings
	MetricsWindow time.Duration `json:"metrics_window"` // Window for metrics calculation
	SamplingRate  float64       `json:"sampling_rate"`  // Rate of requests to sample for metrics

	// Response settings
	SheddingHandler    func(*lift.Context) error `json:"-"`                    // Custom shedding response
	SheddingStatusCode int                       `json:"shedding_status_code"` // HTTP status for shed requests
	SheddingMessage    string                    `json:"shedding_message"`     // Message for shed requests

	// Observability
	Logger        observability.StructuredLogger `json:"-"`
	Metrics       observability.MetricsCollector `json:"-"`
	EnableMetrics bool                           `json:"enable_metrics"`

	// Naming
	Name string `json:"name"` // Load shedding name for metrics
}

// LoadMetrics provides real-time system and application metrics
type LoadMetrics struct {
	// System metrics
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`

	// Application metrics
	ActiveRequests int64         `json:"active_requests"`
	RequestRate    float64       `json:"request_rate"`
	AverageLatency time.Duration `json:"average_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
	ErrorRate      float64       `json:"error_rate"`

	// Load shedding metrics
	CurrentSheddingRate float64 `json:"current_shedding_rate"`
	TotalRequests       int64   `json:"total_requests"`
	ShedRequests        int64   `json:"shed_requests"`

	// Timestamps
	LastUpdated time.Time `json:"last_updated"`
	WindowStart time.Time `json:"window_start"`
}

// LoadSheddingStats provides statistics about load shedding performance
type LoadSheddingStats struct {
	Name                string               `json:"name"`
	Strategy            LoadSheddingStrategy `json:"strategy"`
	Enabled             bool                 `json:"enabled"`
	CurrentSheddingRate float64              `json:"current_shedding_rate"`
	TotalRequests       int64                `json:"total_requests"`
	ShedRequests        int64                `json:"shed_requests"`
	SheddingRatio       float64              `json:"shedding_ratio"`
	AverageLatency      time.Duration        `json:"average_latency"`
	SystemMetrics       LoadMetrics          `json:"system_metrics"`
}

// LoadSheddingMiddleware creates a load shedding middleware
func LoadSheddingMiddleware(config LoadSheddingConfig) lift.Middleware {
	// Set defaults
	if config.CPUThreshold == 0 {
		config.CPUThreshold = 0.8 // 80% CPU
	}
	if config.MemoryThreshold == 0 {
		config.MemoryThreshold = 0.85 // 85% Memory
	}
	if config.LatencyThreshold == 0 {
		config.LatencyThreshold = 5 * time.Second
	}
	if config.ErrorRateThreshold == 0 {
		config.ErrorRateThreshold = 0.1 // 10% error rate
	}
	if config.TargetLatency == 0 {
		config.TargetLatency = 100 * time.Millisecond
	}
	if config.MaxSheddingRate == 0 {
		config.MaxSheddingRate = 0.9 // Max 90% shedding
	}
	if config.MinSheddingRate == 0 {
		config.MinSheddingRate = 0.0 // Min 0% shedding
	}
	if config.AdaptationRate == 0 {
		config.AdaptationRate = 0.1 // 10% adaptation rate
	}
	if config.MetricsWindow == 0 {
		config.MetricsWindow = 30 * time.Second
	}
	if config.SamplingRate == 0 {
		config.SamplingRate = 1.0 // Sample all requests by default
	}
	if config.SheddingStatusCode == 0 {
		config.SheddingStatusCode = 503 // Service Unavailable
	}
	if config.SheddingMessage == "" {
		config.SheddingMessage = "Service temporarily overloaded"
	}
	if config.Name == "" {
		config.Name = "default"
	}
	if config.PriorityExtractor == nil {
		config.PriorityExtractor = defaultLoadSheddingPriorityExtractor
	}
	if config.SheddingHandler == nil {
		config.SheddingHandler = defaultSheddingHandler(config.SheddingStatusCode, config.SheddingMessage)
	}

	manager := &loadSheddingManager{
		config:         config,
		metrics:        &LoadMetrics{LastUpdated: time.Now(), WindowStart: time.Now()},
		latencyHistory: make([]time.Duration, 0, 1000),
		requestHistory: make([]loadRequestRecord, 0, 10000),
		stats: &LoadSheddingStats{
			Name:     config.Name,
			Strategy: config.Strategy,
			Enabled:  config.Enabled,
		},
	}

	// Start background metrics collection
	go manager.metricsCollector()

	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			if !config.Enabled {
				return next.Handle(ctx)
			}

			start := time.Now()

			// Update active request count
			atomic.AddInt64(&manager.metrics.ActiveRequests, 1)
			defer atomic.AddInt64(&manager.metrics.ActiveRequests, -1)

			// Check if request should be shed
			shouldShed := manager.shouldShedRequest(ctx)

			if shouldShed {
				// Record shedding metrics
				atomic.AddInt64(&manager.metrics.ShedRequests, 1)
				atomic.AddInt64(&manager.metrics.TotalRequests, 1)

				if config.Logger != nil {
					priority := config.PriorityExtractor(ctx)
					config.Logger.Warn("Request shed due to load", map[string]interface{}{
						"load_shedding_name": config.Name,
						"strategy":           string(config.Strategy),
						"priority":           priority,
						"shedding_rate":      manager.getCurrentSheddingRate(),
						"active_requests":    atomic.LoadInt64(&manager.metrics.ActiveRequests),
					})
				}

				// Record shedding metrics
				if config.EnableMetrics && config.Metrics != nil {
					manager.recordShedding(ctx)
				}

				return config.SheddingHandler(ctx)
			}

			// Execute request
			err := next.Handle(ctx)
			duration := time.Since(start)

			// Record request metrics
			atomic.AddInt64(&manager.metrics.TotalRequests, 1)
			manager.recordLatency(duration)

			if err != nil {
				manager.recordError()
			}

			// Record success metrics
			if config.EnableMetrics && config.Metrics != nil {
				manager.recordSuccess(ctx, duration)
			}

			return err
		})
	}
}

// loadSheddingManager manages load shedding logic and metrics
type loadSheddingManager struct {
	config         LoadSheddingConfig
	metrics        *LoadMetrics
	latencyHistory []time.Duration
	requestHistory []loadRequestRecord
	errorCount     int64
	mutex          sync.RWMutex
	stats          *LoadSheddingStats
}

// loadRequestRecord tracks individual request metrics for load shedding
type loadRequestRecord struct {
	timestamp time.Time
	duration  time.Duration
	success   bool
}

// shouldShedRequest determines if a request should be shed
func (lsm *loadSheddingManager) shouldShedRequest(ctx *lift.Context) bool {
	switch lsm.config.Strategy {
	case LoadSheddingRandom:
		return lsm.randomShedding()
	case LoadSheddingPriority:
		return lsm.priorityShedding(ctx)
	case LoadSheddingAdaptive:
		return lsm.adaptiveShedding()
	case LoadSheddingCircuit:
		return lsm.circuitShedding()
	case LoadSheddingCustom:
		if lsm.config.CustomShedder != nil {
			return lsm.config.CustomShedder(ctx, lsm.metrics)
		}
		return false
	default:
		return false
	}
}

// randomShedding implements random load shedding based on current load
func (lsm *loadSheddingManager) randomShedding() bool {
	sheddingRate := lsm.calculateSheddingRate()
	return rand.Float64() < sheddingRate
}

// priorityShedding implements priority-based load shedding
func (lsm *loadSheddingManager) priorityShedding(ctx *lift.Context) bool {
	priority := lsm.config.PriorityExtractor(ctx)

	// Get shedding rate for this priority level
	sheddingRate := lsm.calculateSheddingRate()

	// Apply priority-specific adjustments
	if threshold, exists := lsm.config.PriorityThresholds[priority]; exists {
		sheddingRate = math.Min(sheddingRate, threshold)
	}

	// Higher priority requests are less likely to be shed
	priorityMultiplier := 1.0 / (1.0 + float64(priority)*0.1)
	adjustedRate := sheddingRate * priorityMultiplier

	return rand.Float64() < adjustedRate
}

// adaptiveShedding implements adaptive load shedding based on target latency
func (lsm *loadSheddingManager) adaptiveShedding() bool {
	currentLatency := lsm.metrics.AverageLatency
	targetLatency := lsm.config.TargetLatency

	if currentLatency <= targetLatency {
		// Performance is good, reduce shedding
		currentRate := lsm.getCurrentSheddingRate()
		newRate := math.Max(currentRate-lsm.config.AdaptationRate, lsm.config.MinSheddingRate)
		lsm.setCurrentSheddingRate(newRate)
		return rand.Float64() < newRate
	}

	// Performance is poor, increase shedding
	latencyRatio := float64(currentLatency) / float64(targetLatency)
	desiredRate := math.Min((latencyRatio-1.0)*0.5, lsm.config.MaxSheddingRate)

	currentRate := lsm.getCurrentSheddingRate()
	newRate := math.Min(currentRate+lsm.config.AdaptationRate, desiredRate)
	lsm.setCurrentSheddingRate(newRate)

	return rand.Float64() < newRate
}

// circuitShedding implements circuit breaker style load shedding
func (lsm *loadSheddingManager) circuitShedding() bool {
	// Check multiple thresholds
	cpuOverload := lsm.metrics.CPUUsage > lsm.config.CPUThreshold
	memoryOverload := lsm.metrics.MemoryUsage > lsm.config.MemoryThreshold
	latencyOverload := lsm.metrics.AverageLatency > lsm.config.LatencyThreshold
	errorOverload := lsm.metrics.ErrorRate > lsm.config.ErrorRateThreshold

	overloadCount := 0
	if cpuOverload {
		overloadCount++
	}
	if memoryOverload {
		overloadCount++
	}
	if latencyOverload {
		overloadCount++
	}
	if errorOverload {
		overloadCount++
	}

	// Shed based on number of overloaded metrics
	sheddingRate := float64(overloadCount) * 0.25 // 25% per overloaded metric
	sheddingRate = math.Min(sheddingRate, lsm.config.MaxSheddingRate)

	lsm.setCurrentSheddingRate(sheddingRate)
	return rand.Float64() < sheddingRate
}

// calculateSheddingRate calculates the current shedding rate based on system metrics
func (lsm *loadSheddingManager) calculateSheddingRate() float64 {
	lsm.mutex.RLock()
	defer lsm.mutex.RUnlock()

	// Base shedding rate on multiple factors
	cpuFactor := math.Max(0, (lsm.metrics.CPUUsage-lsm.config.CPUThreshold)/(1.0-lsm.config.CPUThreshold))
	memoryFactor := math.Max(0, (lsm.metrics.MemoryUsage-lsm.config.MemoryThreshold)/(1.0-lsm.config.MemoryThreshold))

	latencyFactor := 0.0
	if lsm.config.LatencyThreshold > 0 {
		latencyFactor = math.Max(0, (float64(lsm.metrics.AverageLatency)-float64(lsm.config.LatencyThreshold))/float64(lsm.config.LatencyThreshold))
	}

	errorFactor := math.Max(0, (lsm.metrics.ErrorRate-lsm.config.ErrorRateThreshold)/(1.0-lsm.config.ErrorRateThreshold))

	// Combine factors (weighted average)
	combinedFactor := (cpuFactor*0.3 + memoryFactor*0.3 + latencyFactor*0.3 + errorFactor*0.1)

	// Apply bounds
	sheddingRate := math.Min(combinedFactor, lsm.config.MaxSheddingRate)
	sheddingRate = math.Max(sheddingRate, lsm.config.MinSheddingRate)

	return sheddingRate
}

// getCurrentSheddingRate returns the current shedding rate
func (lsm *loadSheddingManager) getCurrentSheddingRate() float64 {
	lsm.mutex.RLock()
	defer lsm.mutex.RUnlock()
	return lsm.metrics.CurrentSheddingRate
}

// setCurrentSheddingRate sets the current shedding rate
func (lsm *loadSheddingManager) setCurrentSheddingRate(rate float64) {
	lsm.mutex.Lock()
	defer lsm.mutex.Unlock()
	lsm.metrics.CurrentSheddingRate = rate
}

// recordLatency records request latency
func (lsm *loadSheddingManager) recordLatency(duration time.Duration) {
	lsm.mutex.Lock()
	defer lsm.mutex.Unlock()

	// Add to history
	lsm.latencyHistory = append(lsm.latencyHistory, duration)

	// Keep only recent history
	if len(lsm.latencyHistory) > 1000 {
		lsm.latencyHistory = lsm.latencyHistory[len(lsm.latencyHistory)-1000:]
	}

	// Update metrics
	lsm.updateLatencyMetrics()
}

// recordError records request error
func (lsm *loadSheddingManager) recordError() {
	atomic.AddInt64(&lsm.errorCount, 1)
}

// updateLatencyMetrics calculates latency percentiles
func (lsm *loadSheddingManager) updateLatencyMetrics() {
	if len(lsm.latencyHistory) == 0 {
		return
	}

	// Calculate average
	var total time.Duration
	for _, duration := range lsm.latencyHistory {
		total += duration
	}
	lsm.metrics.AverageLatency = total / time.Duration(len(lsm.latencyHistory))

	// Calculate percentiles (simplified)
	if len(lsm.latencyHistory) >= 20 {
		sorted := make([]time.Duration, len(lsm.latencyHistory))
		copy(sorted, lsm.latencyHistory)

		// Simple sort for percentiles
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		p95Index := int(float64(len(sorted)) * 0.95)
		p99Index := int(float64(len(sorted)) * 0.99)

		lsm.metrics.P95Latency = sorted[p95Index]
		lsm.metrics.P99Latency = sorted[p99Index]
	}
}

// metricsCollector runs in background to collect system metrics
func (lsm *loadSheddingManager) metricsCollector() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		lsm.updateMetrics()
	}
}

// updateMetrics updates system and application metrics
func (lsm *loadSheddingManager) updateMetrics() {
	lsm.mutex.Lock()
	defer lsm.mutex.Unlock()

	now := time.Now()

	// Update request rate
	windowDuration := now.Sub(lsm.metrics.WindowStart)
	totalRequests := atomic.LoadInt64(&lsm.metrics.TotalRequests)
	if windowDuration > 0 {
		lsm.metrics.RequestRate = float64(totalRequests) / windowDuration.Seconds()
	}

	// Update error rate
	if totalRequests > 0 {
		lsm.metrics.ErrorRate = float64(atomic.LoadInt64(&lsm.errorCount)) / float64(totalRequests)
	}

	// Reset window if needed
	if windowDuration > lsm.config.MetricsWindow {
		lsm.metrics.WindowStart = now
		atomic.StoreInt64(&lsm.metrics.TotalRequests, 0)
		atomic.StoreInt64(&lsm.metrics.ShedRequests, 0)
		atomic.StoreInt64(&lsm.errorCount, 0)
	}

	lsm.metrics.LastUpdated = now

	// Update stats
	lsm.stats.CurrentSheddingRate = lsm.metrics.CurrentSheddingRate
	lsm.stats.TotalRequests = totalRequests
	shedRequests := atomic.LoadInt64(&lsm.metrics.ShedRequests)
	lsm.stats.ShedRequests = shedRequests
	if totalRequests > 0 {
		lsm.stats.SheddingRatio = float64(shedRequests) / float64(totalRequests)
	}
	lsm.stats.AverageLatency = lsm.metrics.AverageLatency

	// Create a safe copy of metrics with atomic values
	lsm.stats.SystemMetrics = LoadMetrics{
		CPUUsage:            lsm.metrics.CPUUsage,
		MemoryUsage:         lsm.metrics.MemoryUsage,
		ActiveRequests:      atomic.LoadInt64(&lsm.metrics.ActiveRequests),
		RequestRate:         lsm.metrics.RequestRate,
		AverageLatency:      lsm.metrics.AverageLatency,
		P95Latency:          lsm.metrics.P95Latency,
		P99Latency:          lsm.metrics.P99Latency,
		ErrorRate:           lsm.metrics.ErrorRate,
		CurrentSheddingRate: lsm.metrics.CurrentSheddingRate,
		TotalRequests:       totalRequests,
		ShedRequests:        shedRequests,
		LastUpdated:         lsm.metrics.LastUpdated,
		WindowStart:         lsm.metrics.WindowStart,
	}
}

// recordShedding records metrics for shed requests
func (lsm *loadSheddingManager) recordShedding(ctx *lift.Context) {
	if !lsm.config.EnableMetrics || lsm.config.Metrics == nil {
		return
	}

	priority := lsm.config.PriorityExtractor(ctx)

	tags := map[string]string{
		"load_shedding_name": lsm.config.Name,
		"strategy":           string(lsm.config.Strategy),
		"result":             "shed",
		"priority":           fmt.Sprintf("%d", priority),
	}

	metrics := lsm.config.Metrics.WithTags(tags)

	// Record shedding
	counter := metrics.Counter("load_shedding.requests.total")
	counter.Inc()

	// Record current shedding rate
	gauge := metrics.Gauge("load_shedding.rate")
	gauge.Set(lsm.getCurrentSheddingRate())
}

// recordSuccess records metrics for successful requests
func (lsm *loadSheddingManager) recordSuccess(ctx *lift.Context, duration time.Duration) {
	if !lsm.config.EnableMetrics || lsm.config.Metrics == nil {
		return
	}

	priority := lsm.config.PriorityExtractor(ctx)

	tags := map[string]string{
		"load_shedding_name": lsm.config.Name,
		"strategy":           string(lsm.config.Strategy),
		"result":             "success",
		"priority":           fmt.Sprintf("%d", priority),
	}

	metrics := lsm.config.Metrics.WithTags(tags)

	// Record success
	counter := metrics.Counter("load_shedding.requests.total")
	counter.Inc()

	// Record latency
	histogram := metrics.Histogram("load_shedding.latency")
	histogram.Observe(float64(duration.Milliseconds()))

	// Record active requests
	gauge := metrics.Gauge("load_shedding.active_requests")
	gauge.Set(float64(atomic.LoadInt64(&lsm.metrics.ActiveRequests)))
}

// GetStats returns current load shedding statistics
func (lsm *loadSheddingManager) GetStats() LoadSheddingStats {
	lsm.mutex.RLock()
	defer lsm.mutex.RUnlock()
	return *lsm.stats
}

// Default implementations

// defaultLoadSheddingPriorityExtractor extracts priority from context
func defaultLoadSheddingPriorityExtractor(ctx *lift.Context) int {
	// Check for priority header
	if priority := ctx.Request.Headers["X-Priority"]; priority != "" {
		switch priority {
		case "critical":
			return 10
		case "high":
			return 8
		case "normal":
			return 5
		case "low":
			return 2
		case "background":
			return 1
		default:
			return 5
		}
	}
	return 5 // Normal priority
}

// defaultSheddingHandler creates a default shedding response handler
func defaultSheddingHandler(statusCode int, message string) func(*lift.Context) error {
	return func(ctx *lift.Context) error {
		return ctx.Status(statusCode).JSON(map[string]interface{}{
			"error":       "Service Overloaded",
			"message":     message,
			"code":        "LOAD_SHED",
			"retry_after": "5",
		})
	}
}

// Utility functions for common load shedding configurations

// NewBasicLoadShedding creates a basic load shedding configuration
func NewBasicLoadShedding(name string) LoadSheddingConfig {
	return LoadSheddingConfig{
		Name:               name,
		Strategy:           LoadSheddingAdaptive,
		Enabled:            true,
		CPUThreshold:       0.8,
		MemoryThreshold:    0.85,
		LatencyThreshold:   5 * time.Second,
		ErrorRateThreshold: 0.1,
		TargetLatency:      100 * time.Millisecond,
		MaxSheddingRate:    0.9,
		MinSheddingRate:    0.0,
		AdaptationRate:     0.1,
		MetricsWindow:      30 * time.Second,
		SamplingRate:       1.0,
		EnableMetrics:      true,
	}
}

// NewPriorityLoadShedding creates a priority-based load shedding configuration
func NewPriorityLoadShedding(name string, priorityThresholds map[int]float64) LoadSheddingConfig {
	config := NewBasicLoadShedding(name)
	config.Strategy = LoadSheddingPriority
	config.PriorityThresholds = priorityThresholds
	return config
}

// NewAdaptiveLoadShedding creates an adaptive load shedding configuration
func NewAdaptiveLoadShedding(name string, targetLatency time.Duration) LoadSheddingConfig {
	config := NewBasicLoadShedding(name)
	config.Strategy = LoadSheddingAdaptive
	config.TargetLatency = targetLatency
	return config
}

// NewCustomLoadShedding creates a custom load shedding configuration
func NewCustomLoadShedding(name string, customShedder func(*lift.Context, *LoadMetrics) bool) LoadSheddingConfig {
	config := NewBasicLoadShedding(name)
	config.Strategy = LoadSheddingCustom
	config.CustomShedder = customShedder
	return config
}
