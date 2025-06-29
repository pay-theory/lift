package load

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// LoadTest represents a load testing configuration and execution
type LoadTest struct {
	Name        string
	Description string
	App         *lift.App
	Scenarios   []Scenario
	Config      LoadTestConfig
	results     *Results
	mu          sync.RWMutex
}

// LoadTestConfig holds configuration for load testing
type LoadTestConfig struct {
	Duration     time.Duration // How long to run the test
	Concurrent   int           // Number of concurrent workers
	RampUpTime   time.Duration // Time to ramp up to full concurrency
	RampDownTime time.Duration // Time to ramp down
	MaxRequests  int64         // Maximum number of requests (0 = unlimited)
	ThinkTime    time.Duration // Delay between requests per worker
	Timeout      time.Duration // Request timeout

	// Rate limiting
	RequestsPerSecond float64 // Target RPS (0 = unlimited)

	// Reporting
	ReportInterval time.Duration // How often to report progress
	Percentiles    []float64     // Percentiles to calculate (default: 50, 95, 99)
}

// Scenario represents a test scenario with weight
type Scenario struct {
	Name        string
	Description string
	Weight      int // Relative frequency (higher = more frequent)
	Setup       func() (any, error)
	Execute     func(context.Context, *lift.App, any) (*ScenarioResult, error)
	Validate    func(*ScenarioResult) error
	Cleanup     func(any) error
}

// ScenarioResult contains the result of executing a scenario
type ScenarioResult struct {
	StatusCode   int
	ResponseTime time.Duration
	BytesRead    int64
	BytesWritten int64
	Error        error
	Headers      map[string]string
	Body         []byte
	Metadata     map[string]any
}

// Results contains the aggregated results of a load test
type Results struct {
	// Test metadata
	TestName  string        `json:"test_name"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`

	// Request statistics
	TotalRequests  int64   `json:"total_requests"`
	SuccessCount   int64   `json:"success_count"`
	ErrorCount     int64   `json:"error_count"`
	RequestsPerSec float64 `json:"requests_per_sec"`

	// Response time statistics
	MinLatency    time.Duration `json:"min_latency"`
	MaxLatency    time.Duration `json:"max_latency"`
	MeanLatency   time.Duration `json:"mean_latency"`
	MedianLatency time.Duration `json:"median_latency"`

	// Percentiles
	Percentiles map[string]time.Duration `json:"percentiles"`

	// Error breakdown
	ErrorsByType   map[string]int64 `json:"errors_by_type"`
	ErrorsByStatus map[int]int64    `json:"errors_by_status"`

	// Scenario breakdown
	ScenarioStats map[string]*ScenarioStats `json:"scenario_stats"`

	// Throughput
	BytesRead    int64   `json:"bytes_read"`
	BytesWritten int64   `json:"bytes_written"`
	Throughput   float64 `json:"throughput_mbps"`

	// Raw data for analysis
	Latencies []time.Duration `json:"-"` // Not serialized due to size
}

// ScenarioStats contains statistics for a specific scenario
type ScenarioStats struct {
	Name           string           `json:"name"`
	Count          int64            `json:"count"`
	SuccessCount   int64            `json:"success_count"`
	ErrorCount     int64            `json:"error_count"`
	MeanLatency    time.Duration    `json:"mean_latency"`
	MinLatency     time.Duration    `json:"min_latency"`
	MaxLatency     time.Duration    `json:"max_latency"`
	ErrorsByType   map[string]int64 `json:"errors_by_type"`
	ErrorsByStatus map[int]int64    `json:"errors_by_status"`
}

// NewLoadTest creates a new load test
func NewLoadTest(name string, app *lift.App, config LoadTestConfig) *LoadTest {
	// Set defaults
	if config.Concurrent == 0 {
		config.Concurrent = 10
	}
	if config.Duration == 0 {
		config.Duration = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.ReportInterval == 0 {
		config.ReportInterval = 5 * time.Second
	}
	if len(config.Percentiles) == 0 {
		config.Percentiles = []float64{50, 95, 99}
	}

	return &LoadTest{
		Name:      name,
		App:       app,
		Config:    config,
		Scenarios: make([]Scenario, 0),
		results: &Results{
			TestName:       name,
			ErrorsByType:   make(map[string]int64),
			ErrorsByStatus: make(map[int]int64),
			ScenarioStats:  make(map[string]*ScenarioStats),
			Percentiles:    make(map[string]time.Duration),
			Latencies:      make([]time.Duration, 0, 10000),
		},
	}
}

// AddScenario adds a scenario to the load test
func (lt *LoadTest) AddScenario(scenario Scenario) *LoadTest {
	lt.Scenarios = append(lt.Scenarios, scenario)

	// Initialize scenario stats
	lt.results.ScenarioStats[scenario.Name] = &ScenarioStats{
		Name:           scenario.Name,
		ErrorsByType:   make(map[string]int64),
		ErrorsByStatus: make(map[int]int64),
		MinLatency:     time.Duration(math.MaxInt64),
	}

	return lt
}

// Run executes the load test
func (lt *LoadTest) Run(ctx context.Context) (*Results, error) {
	if len(lt.Scenarios) == 0 {
		return nil, fmt.Errorf("no scenarios defined")
	}

	lt.results.StartTime = time.Now()

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, lt.Config.Duration)
	defer cancel()

	// Start progress reporting
	progressCtx, progressCancel := context.WithCancel(testCtx)
	defer progressCancel()
	go lt.reportProgress(progressCtx)

	// Create worker pool
	var wg sync.WaitGroup
	workerChan := make(chan struct{}, lt.Config.Concurrent)

	// Rate limiter
	var rateLimiter <-chan time.Time
	if lt.Config.RequestsPerSecond > 0 {
		interval := time.Duration(float64(time.Second) / lt.Config.RequestsPerSecond)
		rateLimiter = time.Tick(interval)
	}

	// Start workers
	for i := 0; i < lt.Config.Concurrent; i++ {
		wg.Add(1)
		go lt.worker(testCtx, &wg, workerChan, rateLimiter)
	}

	// Ramp up
	if lt.Config.RampUpTime > 0 {
		lt.rampUp(testCtx, workerChan)
	} else {
		// Start all workers immediately
		for i := 0; i < lt.Config.Concurrent; i++ {
			select {
			case workerChan <- struct{}{}:
			case <-testCtx.Done():
				break
			}
		}
	}

	// Wait for test completion
	<-testCtx.Done()

	// Ramp down
	if lt.Config.RampDownTime > 0 {
		time.Sleep(lt.Config.RampDownTime)
	}

	// Stop workers
	close(workerChan)
	wg.Wait()

	lt.results.EndTime = time.Now()
	lt.results.Duration = lt.results.EndTime.Sub(lt.results.StartTime)

	// Calculate final statistics
	lt.calculateStats()

	return lt.results, nil
}

// worker executes scenarios in a loop
func (lt *LoadTest) worker(ctx context.Context, wg *sync.WaitGroup, workerChan <-chan struct{}, rateLimiter <-chan time.Time) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-workerChan:
			// Rate limiting
			if rateLimiter != nil {
				select {
				case <-rateLimiter:
				case <-ctx.Done():
					return
				}
			}

			// Check max requests limit
			if lt.Config.MaxRequests > 0 && atomic.LoadInt64(&lt.results.TotalRequests) >= lt.Config.MaxRequests {
				return
			}

			// Execute scenario
			scenario := lt.selectScenario()
			lt.executeScenario(ctx, scenario)

			// Think time
			if lt.Config.ThinkTime > 0 {
				select {
				case <-time.After(lt.Config.ThinkTime):
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// selectScenario selects a scenario based on weights
func (lt *LoadTest) selectScenario() Scenario {
	if len(lt.Scenarios) == 1 {
		return lt.Scenarios[0]
	}

	// Calculate total weight
	totalWeight := 0
	for _, scenario := range lt.Scenarios {
		totalWeight += scenario.Weight
	}

	// Select random scenario based on weight
	// This is a simplified implementation - in production you'd use proper weighted random selection
	target := int(atomic.LoadInt64(&lt.results.TotalRequests)) % totalWeight
	current := 0

	for _, scenario := range lt.Scenarios {
		current += scenario.Weight
		if target < current {
			return scenario
		}
	}

	// Fallback to first scenario
	return lt.Scenarios[0]
}

// executeScenario executes a single scenario
func (lt *LoadTest) executeScenario(ctx context.Context, scenario Scenario) {
	atomic.AddInt64(&lt.results.TotalRequests, 1)

	// Setup
	var setupData any
	var err error
	if scenario.Setup != nil {
		setupData, err = scenario.Setup()
		if err != nil {
			lt.recordError("setup", 0, err)
			return
		}
	}

	// Execute with timeout
	execCtx, cancel := context.WithTimeout(ctx, lt.Config.Timeout)
	defer cancel()

	start := time.Now()
	result, err := scenario.Execute(execCtx, lt.App, setupData)
	duration := time.Since(start)

	// Record result
	if err != nil {
		lt.recordError("execution", 0, err)
	} else if result != nil {
		lt.recordResult(scenario.Name, result, duration)

		// Validate
		if scenario.Validate != nil {
			if err := scenario.Validate(result); err != nil {
				lt.recordError("validation", result.StatusCode, err)
			}
		}
	}

	// Cleanup
	if scenario.Cleanup != nil && setupData != nil {
		scenario.Cleanup(setupData)
	}
}

// recordResult records a successful result
func (lt *LoadTest) recordResult(scenarioName string, result *ScenarioResult, duration time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	atomic.AddInt64(&lt.results.SuccessCount, 1)

	// Record latency
	lt.results.Latencies = append(lt.results.Latencies, duration)

	// Update scenario stats
	stats := lt.results.ScenarioStats[scenarioName]
	stats.Count++
	stats.SuccessCount++

	if duration < stats.MinLatency {
		stats.MinLatency = duration
	}
	if duration > stats.MaxLatency {
		stats.MaxLatency = duration
	}

	// Update throughput
	if result != nil {
		atomic.AddInt64(&lt.results.BytesRead, result.BytesRead)
		atomic.AddInt64(&lt.results.BytesWritten, result.BytesWritten)
	}
}

// recordError records an error
func (lt *LoadTest) recordError(errorType string, statusCode int, err error) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	atomic.AddInt64(&lt.results.ErrorCount, 1)

	// Record by type
	lt.results.ErrorsByType[errorType]++

	// Record by status code
	if statusCode > 0 {
		lt.results.ErrorsByStatus[statusCode]++
	}
}

// rampUp gradually increases the number of active workers
func (lt *LoadTest) rampUp(ctx context.Context, workerChan chan<- struct{}) {
	interval := lt.Config.RampUpTime / time.Duration(lt.Config.Concurrent)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for i := 0; i < lt.Config.Concurrent; i++ {
		select {
		case <-ticker.C:
			select {
			case workerChan <- struct{}{}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// reportProgress reports progress at regular intervals
func (lt *LoadTest) reportProgress(ctx context.Context) {
	ticker := time.NewTicker(lt.Config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lt.printProgress()
		case <-ctx.Done():
			return
		}
	}
}

// printProgress prints current progress
func (lt *LoadTest) printProgress() {
	elapsed := time.Since(lt.results.StartTime)
	total := atomic.LoadInt64(&lt.results.TotalRequests)
	success := atomic.LoadInt64(&lt.results.SuccessCount)
	errors := atomic.LoadInt64(&lt.results.ErrorCount)

	rps := float64(total) / elapsed.Seconds()

	fmt.Printf("[%v] Requests: %d, Success: %d, Errors: %d, RPS: %.2f\n",
		elapsed.Truncate(time.Second), total, success, errors, rps)
}

// calculateStats calculates final statistics
func (lt *LoadTest) calculateStats() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	// Calculate RPS
	if lt.results.Duration > 0 {
		lt.results.RequestsPerSec = float64(lt.results.TotalRequests) / lt.results.Duration.Seconds()
	}

	// Calculate throughput
	if lt.results.Duration > 0 {
		totalBytes := float64(lt.results.BytesRead + lt.results.BytesWritten)
		lt.results.Throughput = (totalBytes / 1024 / 1024) / lt.results.Duration.Seconds() // MB/s
	}

	// Calculate latency statistics
	if len(lt.results.Latencies) > 0 {
		sort.Slice(lt.results.Latencies, func(i, j int) bool {
			return lt.results.Latencies[i] < lt.results.Latencies[j]
		})

		lt.results.MinLatency = lt.results.Latencies[0]
		lt.results.MaxLatency = lt.results.Latencies[len(lt.results.Latencies)-1]

		// Calculate mean
		var total time.Duration
		for _, latency := range lt.results.Latencies {
			total += latency
		}
		lt.results.MeanLatency = total / time.Duration(len(lt.results.Latencies))

		// Calculate median
		medianIndex := len(lt.results.Latencies) / 2
		lt.results.MedianLatency = lt.results.Latencies[medianIndex]

		// Calculate percentiles
		for _, p := range lt.Config.Percentiles {
			index := int(float64(len(lt.results.Latencies)) * p / 100)
			if index >= len(lt.results.Latencies) {
				index = len(lt.results.Latencies) - 1
			}
			lt.results.Percentiles[fmt.Sprintf("P%.0f", p)] = lt.results.Latencies[index]
		}
	}

	// Calculate scenario statistics
	for _, stats := range lt.results.ScenarioStats {
		if stats.Count > 0 {
			// This would require tracking latencies per scenario
			// For now, we'll leave mean latency as zero
		}
	}
}

// PrintResults prints a summary of the results
func (lt *LoadTest) PrintResults() {
	results := lt.results

	fmt.Printf("\n=== Load Test Results: %s ===\n", results.TestName)
	fmt.Printf("Duration: %v\n", results.Duration)
	fmt.Printf("Total Requests: %d\n", results.TotalRequests)
	fmt.Printf("Successful Requests: %d (%.2f%%)\n",
		results.SuccessCount,
		float64(results.SuccessCount)/float64(results.TotalRequests)*100)
	fmt.Printf("Failed Requests: %d (%.2f%%)\n",
		results.ErrorCount,
		float64(results.ErrorCount)/float64(results.TotalRequests)*100)
	fmt.Printf("Requests/sec: %.2f\n", results.RequestsPerSec)
	fmt.Printf("Throughput: %.2f MB/s\n", results.Throughput)

	if len(results.Latencies) > 0 {
		fmt.Printf("\nLatency Statistics:\n")
		fmt.Printf("  Min: %v\n", results.MinLatency)
		fmt.Printf("  Max: %v\n", results.MaxLatency)
		fmt.Printf("  Mean: %v\n", results.MeanLatency)
		fmt.Printf("  Median: %v\n", results.MedianLatency)

		for _, p := range lt.Config.Percentiles {
			key := fmt.Sprintf("P%.0f", p)
			if latency, exists := results.Percentiles[key]; exists {
				fmt.Printf("  %s: %v\n", key, latency)
			}
		}
	}

	if len(results.ErrorsByType) > 0 {
		fmt.Printf("\nErrors by Type:\n")
		for errorType, count := range results.ErrorsByType {
			fmt.Printf("  %s: %d\n", errorType, count)
		}
	}

	if len(results.ErrorsByStatus) > 0 {
		fmt.Printf("\nErrors by Status Code:\n")
		for status, count := range results.ErrorsByStatus {
			fmt.Printf("  %d: %d\n", status, count)
		}
	}

	fmt.Printf("\nScenario Breakdown:\n")
	for name, stats := range results.ScenarioStats {
		fmt.Printf("  %s: %d requests (%.2f%% success)\n",
			name, stats.Count,
			float64(stats.SuccessCount)/float64(stats.Count)*100)
	}
}

// Example scenarios for common use cases

// HTTPGetScenario creates a simple HTTP GET scenario
func HTTPGetScenario(name, path string) Scenario {
	return Scenario{
		Name:   name,
		Weight: 1,
		Execute: func(ctx context.Context, app *lift.App, data any) (*ScenarioResult, error) {
			start := time.Now()

			// This would need to be implemented to actually call the app
			// For now, return a mock result
			return &ScenarioResult{
				StatusCode:   200,
				ResponseTime: time.Since(start),
				BytesRead:    1024,
				Headers:      map[string]string{"Content-Type": "application/json"},
				Body:         []byte(`{"status": "ok"}`),
			}, nil
		},
	}
}

// HTTPPostScenario creates a simple HTTP POST scenario
func HTTPPostScenario(name, path string, payload any) Scenario {
	return Scenario{
		Name:   name,
		Weight: 1,
		Setup: func() (any, error) {
			return payload, nil
		},
		Execute: func(ctx context.Context, app *lift.App, data any) (*ScenarioResult, error) {
			start := time.Now()

			// This would need to be implemented to actually call the app
			// For now, return a mock result
			return &ScenarioResult{
				StatusCode:   201,
				ResponseTime: time.Since(start),
				BytesRead:    512,
				BytesWritten: 256,
				Headers:      map[string]string{"Content-Type": "application/json"},
				Body:         []byte(`{"id": "123", "status": "created"}`),
			}, nil
		},
	}
}

// RateLimitTestScenario creates a scenario for testing rate limits
func RateLimitTestScenario(name, path string, expectedLimit int) Scenario {
	return Scenario{
		Name:   name,
		Weight: 1,
		Execute: func(ctx context.Context, app *lift.App, data any) (*ScenarioResult, error) {
			start := time.Now()

			// This would make rapid requests to test rate limiting
			// For now, return a mock result
			return &ScenarioResult{
				StatusCode:   200, // or 429 if rate limited
				ResponseTime: time.Since(start),
				BytesRead:    256,
				Headers: map[string]string{
					"X-RateLimit-Limit":     fmt.Sprintf("%d", expectedLimit),
					"X-RateLimit-Remaining": "5",
				},
				Body: []byte(`{"status": "ok"}`),
			}, nil
		},
		Validate: func(result *ScenarioResult) error {
			// Validate rate limit headers are present
			if result.Headers["X-RateLimit-Limit"] == "" {
				return fmt.Errorf("missing rate limit headers")
			}
			return nil
		},
	}
}
