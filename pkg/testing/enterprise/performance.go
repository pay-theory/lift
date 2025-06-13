package enterprise

import (
	"fmt"
	"sync"
	"time"
)

// PerformanceValidator validates performance across environments
type PerformanceValidator struct {
	baselines  map[string]PerformanceBaseline
	thresholds map[string]PerformanceThreshold
	profiler   *PerformanceProfiler
	reporter   *PerformanceReporter
	mutex      sync.RWMutex
}

// PerformanceBaseline represents a performance baseline
type PerformanceBaseline struct {
	Metric      string
	Value       float64
	Environment string
	Timestamp   time.Time
}

// PerformanceThreshold represents performance thresholds
type PerformanceThreshold struct {
	Metric           string
	AbsoluteMax      float64
	RegressionFactor float64
	Environment      string
}

// PerformanceProfiler profiles performance metrics
type PerformanceProfiler struct {
	metrics map[string][]float64
	mutex   sync.RWMutex
}

// PerformanceReporter reports performance results
type PerformanceReporter struct {
	reports []PerformanceReport
	mutex   sync.RWMutex
}

// PerformanceReport represents a performance report
type PerformanceReport struct {
	Environment string
	Metrics     map[string]float64
	Timestamp   time.Time
	Status      string
}

// NewPerformanceValidator creates a new performance validator
func NewPerformanceValidator() *PerformanceValidator {
	return &PerformanceValidator{
		baselines:  make(map[string]PerformanceBaseline),
		thresholds: make(map[string]PerformanceThreshold),
		profiler:   NewPerformanceProfiler(),
		reporter:   NewPerformanceReporter(),
	}
}

// NewPerformanceProfiler creates a new performance profiler
func NewPerformanceProfiler() *PerformanceProfiler {
	return &PerformanceProfiler{
		metrics: make(map[string][]float64),
	}
}

// NewPerformanceReporter creates a new performance reporter
func NewPerformanceReporter() *PerformanceReporter {
	return &PerformanceReporter{
		reports: make([]PerformanceReport, 0),
	}
}

// ValidatePerformance validates performance for a test case
func (p *PerformanceValidator) ValidatePerformance(testCase TestCase, env *TestEnvironment) error {
	// Profile the test case
	metrics, err := p.profiler.Profile(testCase, env)
	if err != nil {
		return fmt.Errorf("profiling failed: %w", err)
	}

	// Compare against baselines and thresholds
	for metric, value := range metrics {
		baselineKey := fmt.Sprintf("%s_%s", env.Name, metric)
		thresholdKey := fmt.Sprintf("%s_%s", env.Name, metric)

		// Check against baseline
		if baseline, exists := p.baselines[baselineKey]; exists {
			threshold := p.thresholds[thresholdKey]

			if value > baseline.Value*threshold.RegressionFactor {
				return fmt.Errorf("performance regression detected: %s %.2f > %.2f",
					metric, value, baseline.Value*threshold.RegressionFactor)
			}
		}

		// Check against absolute threshold
		if threshold, exists := p.thresholds[thresholdKey]; exists {
			if value > threshold.AbsoluteMax {
				return fmt.Errorf("performance threshold exceeded: %s %.2f > %.2f",
					metric, value, threshold.AbsoluteMax)
			}
		}
	}

	// Update baselines if performance improved
	p.updateBaselines(env.Name, metrics)

	// Generate report
	report := PerformanceReport{
		Environment: env.Name,
		Metrics:     metrics,
		Timestamp:   time.Now(),
		Status:      "passed",
	}
	p.reporter.AddReport(report)

	return nil
}

// Profile profiles a test case and returns metrics
func (p *PerformanceProfiler) Profile(testCase TestCase, env *TestEnvironment) (map[string]float64, error) {
	metrics := make(map[string]float64)

	// Measure execution time
	start := time.Now()

	// This would normally execute the test case and measure various metrics
	// For now, we'll simulate some metrics
	time.Sleep(1 * time.Millisecond) // Simulate work

	duration := time.Since(start)
	metrics["execution_time_ms"] = float64(duration.Nanoseconds()) / 1e6
	metrics["memory_usage_mb"] = 10.5   // Simulated
	metrics["cpu_usage_percent"] = 15.2 // Simulated

	// Store metrics for trend analysis
	p.mutex.Lock()
	for metric, value := range metrics {
		key := fmt.Sprintf("%s_%s", env.Name, metric)
		p.metrics[key] = append(p.metrics[key], value)

		// Keep only last 100 measurements
		if len(p.metrics[key]) > 100 {
			p.metrics[key] = p.metrics[key][1:]
		}
	}
	p.mutex.Unlock()

	return metrics, nil
}

// updateBaselines updates performance baselines
func (p *PerformanceValidator) updateBaselines(envName string, metrics map[string]float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for metric, value := range metrics {
		key := fmt.Sprintf("%s_%s", envName, metric)

		// Update baseline if this is better performance
		if baseline, exists := p.baselines[key]; !exists || value < baseline.Value {
			p.baselines[key] = PerformanceBaseline{
				Metric:      metric,
				Value:       value,
				Environment: envName,
				Timestamp:   time.Now(),
			}
		}
	}
}

// SetThreshold sets a performance threshold
func (p *PerformanceValidator) SetThreshold(envName, metric string, absoluteMax, regressionFactor float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	key := fmt.Sprintf("%s_%s", envName, metric)
	p.thresholds[key] = PerformanceThreshold{
		Metric:           metric,
		AbsoluteMax:      absoluteMax,
		RegressionFactor: regressionFactor,
		Environment:      envName,
	}
}

// GetBaseline gets a performance baseline
func (p *PerformanceValidator) GetBaseline(envName, metric string) (PerformanceBaseline, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	key := fmt.Sprintf("%s_%s", envName, metric)
	baseline, exists := p.baselines[key]
	return baseline, exists
}

// AddReport adds a performance report
func (p *PerformanceReporter) AddReport(report PerformanceReport) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.reports = append(p.reports, report)

	// Keep only last 1000 reports
	if len(p.reports) > 1000 {
		p.reports = p.reports[1:]
	}
}

// GetReports gets performance reports for an environment
func (p *PerformanceReporter) GetReports(envName string) []PerformanceReport {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var reports []PerformanceReport
	for _, report := range p.reports {
		if report.Environment == envName {
			reports = append(reports, report)
		}
	}

	return reports
}
