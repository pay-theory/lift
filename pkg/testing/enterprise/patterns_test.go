package enterprise

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEnterpriseTestPatterns_RunTestSuite_SequentialAndParallel(t *testing.T) {
	patterns := NewEnterpriseTestPatterns()
	ctx := context.Background()

	_, err := patterns.RunTestSuite(ctx, TestSuite{
		Name:  "setup-fails",
		Setup: func() error { return errors.New("nope") },
		Tests: []Test{{Name: "never", Function: func(context.Context) error { return nil }}},
	})
	if err == nil {
		t.Fatalf("expected setup error")
	}

	seqResults, err := patterns.RunTestSuite(ctx, TestSuite{
		Name: "sequential",
		Tests: []Test{
			{Name: "pass", Function: func(context.Context) error { return nil }},
			{Name: "fail", Function: func(context.Context) error { return errors.New("boom") }},
			{
				Name:    "timeout",
				Timeout: 5 * time.Millisecond,
				Function: func(ctx context.Context) error {
					<-ctx.Done()
					return ctx.Err()
				},
			},
		},
		Teardown: func() error { return errors.New("teardown fails") },
	})
	if err != nil {
		t.Fatalf("RunTestSuite sequential error: %v", err)
	}
	if len(seqResults) != 3 {
		t.Fatalf("sequential results len=%d, want 3", len(seqResults))
	}
	if seqResults[0].Status != TestStatusPassed {
		t.Fatalf("seqResults[0].Status=%q, want %q", seqResults[0].Status, TestStatusPassed)
	}
	if seqResults[1].Status != TestStatusFailed {
		t.Fatalf("seqResults[1].Status=%q, want %q", seqResults[1].Status, TestStatusFailed)
	}

	parResults, err := patterns.RunTestSuite(ctx, TestSuite{
		Name:     "parallel",
		Parallel: true,
		Tests: []Test{
			{Name: "p1", Function: func(context.Context) error { return nil }},
			{Name: "p2", Function: func(context.Context) error { return nil }},
		},
	})
	if err != nil {
		t.Fatalf("RunTestSuite parallel error: %v", err)
	}
	if len(parResults) != 2 {
		t.Fatalf("parallel results len=%d, want 2", len(parResults))
	}
}

func TestEnterpriseTestPatterns_EnvironmentsAndHelpers(t *testing.T) {
	patterns := NewEnterpriseTestPatterns()

	if _, err := patterns.GetEnvironment("missing"); err == nil {
		t.Fatalf("GetEnvironment(missing) expected error")
	}

	patterns.AddEnvironment("dev", EnvironmentConfig{DatabaseURL: "db://local"})
	if _, err := patterns.GetEnvironment("dev"); err != nil {
		t.Fatalf("GetEnvironment(dev) error: %v", err)
	}

	if err := patterns.ValidatePerformance(context.Background(), TestCase{Name: "noop"}, "missing"); err == nil {
		t.Fatalf("ValidatePerformance(missing) expected error")
	}
	if err := patterns.ValidatePerformance(context.Background(), TestCase{Name: "noop"}, "dev"); err != nil {
		t.Fatalf("ValidatePerformance(dev) error: %v", err)
	}

	contract, err := patterns.CreateAPIContractTest("api", "v1", []Interaction{
		{ID: "i1", Request: &InteractionRequest{Method: "GET", Path: "/"}, Response: &InteractionResponse{Status: 200}},
	})
	if err != nil {
		t.Fatalf("CreateAPIContractTest error: %v", err)
	}
	if contract == nil || contract.Name == "" {
		t.Fatalf("expected contract to be created")
	}
	if _, ok := patterns.contractSuite.contracts[contract.Name]; !ok {
		t.Fatalf("contract not added to suite")
	}

	if _, err := patterns.RunContractTests(context.Background()); err != nil {
		t.Fatalf("RunContractTests error: %v", err)
	}
	if err := patterns.RunChaosTests(context.Background()); err != nil {
		t.Fatalf("RunChaosTests error: %v", err)
	}

	if err := patterns.CreateChaosScenario("network_latency", map[string]any{
		"latency":   1 * time.Millisecond,
		"duration":  0 * time.Millisecond,
		"ignored_x": "y",
	}); err != nil {
		t.Fatalf("CreateChaosScenario(network_latency) error: %v", err)
	}
	if err := patterns.CreateChaosScenario("network_latency", map[string]any{"latency": "nope"}); err == nil {
		t.Fatalf("CreateChaosScenario(network_latency) expected error for bad latency type")
	}

	if err := patterns.CreateChaosScenario("service_unavailable", map[string]any{
		"service_name": "payments",
		"duration":     0 * time.Millisecond,
	}); err != nil {
		t.Fatalf("CreateChaosScenario(service_unavailable) error: %v", err)
	}
	if err := patterns.CreateChaosScenario("service_unavailable", map[string]any{"service_name": 123}); err == nil {
		t.Fatalf("CreateChaosScenario(service_unavailable) expected error for bad service_name type")
	}

	if err := patterns.CreateChaosScenario("unknown", map[string]any{}); err == nil {
		t.Fatalf("CreateChaosScenario(unknown) expected error")
	}

	called := false
	perfCase := patterns.CreatePerformanceTest("perf", func() error {
		called = true
		return nil
	})
	if err := perfCase.Execute(nil, nil); err != nil {
		t.Fatalf("CreatePerformanceTest.Execute error: %v", err)
	}
	if !called {
		t.Fatalf("expected performance test func to be called")
	}
	if perfCase.Timeout != 30*time.Second || perfCase.Retries != 3 {
		t.Fatalf("unexpected perfCase defaults: timeout=%v retries=%d", perfCase.Timeout, perfCase.Retries)
	}

	if err := patterns.CreateMultiEnvironmentTest("empty", func(*TestEnvironment) error { return nil }); err != nil {
		t.Fatalf("CreateMultiEnvironmentTest(empty envs) error: %v", err)
	}

	patterns.AddEnvironment("staging", EnvironmentConfig{})
	err = patterns.CreateMultiEnvironmentTest("multi", func(env *TestEnvironment) error {
		if env.Name == "dev" {
			return errors.New("fail dev")
		}
		return nil
	})
	if err == nil {
		t.Fatalf("CreateMultiEnvironmentTest expected error")
	}
}

func TestEnterpriseTestPatterns_GenerateReport(t *testing.T) {
	patterns := NewEnterpriseTestPatterns()

	now := time.Now()
	results := []TestResult{
		{Name: "a", Status: TestStatusPassed, StartTime: now.Add(-2 * time.Second), EndTime: now.Add(-1 * time.Second)},
		{Name: "b", Status: TestStatusFailed, StartTime: now.Add(-1 * time.Second), EndTime: now},
		{Name: "c", Status: TestStatusSkipped, StartTime: now.Add(-3 * time.Second), EndTime: now.Add(-2 * time.Second)},
	}

	report := patterns.GenerateReport("suite", results)
	if report.TotalTests != 3 || report.PassedTests != 1 || report.FailedTests != 1 || report.SkippedTests != 1 {
		t.Fatalf("unexpected report counts: total=%d passed=%d failed=%d skipped=%d",
			report.TotalTests, report.PassedTests, report.FailedTests, report.SkippedTests)
	}
	if report.SuiteName != "suite" {
		t.Fatalf("SuiteName=%q, want %q", report.SuiteName, "suite")
	}
	if report.Duration <= 0 {
		t.Fatalf("expected report.Duration > 0, got %v", report.Duration)
	}
}

