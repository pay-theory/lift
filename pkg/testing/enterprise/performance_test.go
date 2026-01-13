package enterprise

import (
	"context"
	"testing"
	"time"
)

func TestPerformanceTester_ExecuteTest_CoversTypes(t *testing.T) {
	tester := NewPerformanceTester(nil)
	ctx := context.Background()

	respOK := PerformanceTestCase{
		Name: "resp_ok",
		Type: PerformanceTypeResponseTime,
		Config: PerformanceConfig{
			TestDuration:    1 * time.Second,
			ConcurrentUsers: 1,
			ExpectedP95:     200 * time.Millisecond,
			ExpectedP99:     300 * time.Millisecond,
			ExpectedTPS:     1,
			MaxMemoryMB:     1024,
			Endpoint:        "/",
			Method:          "GET",
			QueryType:       "n/a",
		},
	}
	if result, err := tester.ExecuteTest(ctx, respOK); err != nil || result == nil || !result.Success {
		t.Fatalf("response time expected success, got result=%#v err=%v", result, err)
	}

	respFail := respOK
	respFail.Name = "resp_fail"
	respFail.Config.ExpectedP95 = 1 * time.Millisecond
	if result, err := tester.ExecuteTest(ctx, respFail); err != nil || result == nil || result.Success {
		t.Fatalf("response time expected failure, got result=%#v err=%v", result, err)
	}

	throughput := PerformanceTestCase{
		Name: "tps_ok",
		Type: PerformanceTypeThroughput,
		Config: PerformanceConfig{
			TestDuration: 1 * time.Second,
			ExpectedTPS:  100,
		},
	}
	if result, err := tester.ExecuteTest(ctx, throughput); err != nil || result == nil || !result.Success {
		t.Fatalf("throughput expected success, got result=%#v err=%v", result, err)
	}

	dbFail := PerformanceTestCase{
		Name: "db_fail",
		Type: PerformanceTypeDatabase,
		Config: PerformanceConfig{
			TestDuration:    1 * time.Second,
			ConcurrentUsers: 1,
			ExpectedP95:     1 * time.Millisecond,
		},
	}
	if result, err := tester.ExecuteTest(ctx, dbFail); err != nil || result == nil || result.Success {
		t.Fatalf("database expected failure, got result=%#v err=%v", result, err)
	}

	memFail := PerformanceTestCase{
		Name: "mem_fail",
		Type: PerformanceTypeMemory,
		Config: PerformanceConfig{
			ConcurrentUsers: 100,
			MaxMemoryMB:     10,
		},
	}
	if result, err := tester.ExecuteTest(ctx, memFail); err != nil || result == nil || result.Success {
		t.Fatalf("memory expected failure, got result=%#v err=%v", result, err)
	}

	unknown := PerformanceTestCase{
		Name: "unknown",
		Type: PerformanceType("nope"),
	}
	if result, err := tester.ExecuteTest(ctx, unknown); err != nil || result == nil || result.Success {
		t.Fatalf("unknown expected failure, got result=%#v err=%v", result, err)
	}
}

func TestPerformanceReporter_GenerateReport(t *testing.T) {
	reporter := NewPerformanceReporter()

	report, err := reporter.GenerateReport([]PerformanceTestResult{
		{Success: true},
		{Success: false},
	})
	if err != nil {
		t.Fatalf("GenerateReport error: %v", err)
	}
	if report.TotalTests != 2 || report.PassedTests != 1 || report.FailedTests != 1 {
		t.Fatalf("unexpected counts: %#v", report)
	}
	if report.Summary == "" {
		t.Fatalf("expected non-empty summary")
	}

	report, err = reporter.GenerateReport([]PerformanceTestResult{{Success: true}})
	if err != nil {
		t.Fatalf("GenerateReport(all pass) error: %v", err)
	}
	if report.FailedTests != 0 || !report.AllTestsPassed {
		t.Fatalf("expected AllTestsPassed, got %#v", report)
	}
}

func TestRegressionDetector_DetectRegressions(t *testing.T) {
	detector := NewRegressionDetector()

	base := PerformanceTestResult{
		TestCase: PerformanceTestCase{Name: "t"},
		Metrics: PerformanceTestMetrics{
			P95Latency:    100 * time.Millisecond,
			ThroughputTPS: 100,
			MaxMemoryMB:   100,
		},
	}
	regressions, err := detector.DetectRegressions([]PerformanceTestResult{base})
	if err != nil {
		t.Fatalf("DetectRegressions error: %v", err)
	}
	if len(regressions) != 0 {
		t.Fatalf("expected 0 regressions, got %d", len(regressions))
	}

	worse := base
	worse.Metrics.P95Latency = 250 * time.Millisecond
	worse.Metrics.ThroughputTPS = 50
	worse.Metrics.MaxMemoryMB = 200

	regressions, err = detector.DetectRegressions([]PerformanceTestResult{worse})
	if err != nil {
		t.Fatalf("DetectRegressions error: %v", err)
	}
	if len(regressions) == 0 {
		t.Fatalf("expected regressions")
	}
}

func TestPerformanceValidator_ThresholdsBaselinesAndReports(t *testing.T) {
	validator := NewPerformanceValidator()
	env := &TestEnvironment{Name: "dev"}

	// Configure thresholds so baseline comparisons do not default to RegressionFactor=0.
	validator.SetThreshold("dev", "execution_time_ms", 10000, 10)
	validator.SetThreshold("dev", "memory_usage_mb", 10000, 10)
	validator.SetThreshold("dev", "cpu_usage_percent", 10000, 10)

	if err := validator.ValidatePerformance(TestCase{Name: "noop"}, env); err != nil {
		t.Fatalf("ValidatePerformance first run error: %v", err)
	}
	if err := validator.ValidatePerformance(TestCase{Name: "noop"}, env); err != nil {
		t.Fatalf("ValidatePerformance second run error: %v", err)
	}

	if _, ok := validator.GetBaseline("dev", "execution_time_ms"); !ok {
		t.Fatalf("expected baseline to be set")
	}
	reports := validator.reporter.GetReports("dev")
	if len(reports) == 0 {
		t.Fatalf("expected at least one report")
	}

	validator.SetThreshold("dev", "execution_time_ms", 0, 10)
	if err := validator.ValidatePerformance(TestCase{Name: "noop"}, env); err == nil {
		t.Fatalf("expected absolute threshold exceeded error")
	}
}
