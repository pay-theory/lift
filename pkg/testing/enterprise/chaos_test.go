package enterprise

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubFailure struct {
	duration time.Duration
	cleanErr error
	active   bool
}

func (f *stubFailure) Type() string            { return "stub" }
func (f *stubFailure) Severity() ChaosSeverity { return ChaosSeverityLow }
func (f *stubFailure) Duration() time.Duration { return f.duration }
func (f *stubFailure) IsActive() bool          { return f.active }
func (f *stubFailure) Cleanup() error {
	f.active = false
	return f.cleanErr
}

type stubScenario struct {
	failure ChaosFailure
}

func (s stubScenario) Name() string        { return "stub_scenario" }
func (s stubScenario) Description() string { return "stub" }
func (s stubScenario) InjectFailure(_ context.Context) (ChaosFailure, error) {
	return s.failure, nil
}
func (s stubScenario) ExecuteOperations(_ context.Context) ([]OperationResult, error) {
	return []OperationResult{
		{Operation: "op1", Success: true, Duration: 1 * time.Millisecond, Timestamp: time.Now()},
		{Operation: "op2", Success: true, Duration: 1 * time.Millisecond, Timestamp: time.Now()},
	}, nil
}
func (s stubScenario) ValidateRecovery(_ context.Context, _ *ChaosMetrics) error { return nil }

func TestChaosTest_ExecuteChaos_Disabled(t *testing.T) {
	ct := NewChaosTest(ChaosConfig{Enabled: false})
	if err := ct.ExecuteChaos(context.Background()); err == nil {
		t.Fatalf("ExecuteChaos expected error when disabled")
	}
}

func TestChaosTest_ExecuteChaos_Success(t *testing.T) {
	ct := NewChaosTest(ChaosConfig{Enabled: true})
	ct.recovery.thresholds.MaxRecoveryTime = 3 * time.Second

	ct.AddScenario(stubScenario{
		failure: &stubFailure{duration: 0, cleanErr: errors.New("cleanup failed"), active: true},
	})

	if err := ct.ExecuteChaos(context.Background()); err != nil {
		t.Fatalf("ExecuteChaos error: %v", err)
	}

	if ct.metrics.TotalOperations == 0 {
		t.Fatalf("expected metrics.TotalOperations to be updated")
	}
	if ct.metrics.ErrorRate != 0 {
		t.Fatalf("expected ErrorRate=0, got %v", ct.metrics.ErrorRate)
	}
}

func TestRecoveryValidator_validateMetrics(t *testing.T) {
	r := NewRecoveryValidator()

	if err := r.validateMetrics(&ChaosMetrics{TotalOperations: 10, SuccessfulOps: 10, FailedOps: 0, ErrorRate: 0.2}); err == nil {
		t.Fatalf("expected error due to high error rate")
	}

	if err := r.validateMetrics(&ChaosMetrics{TotalOperations: 10, SuccessfulOps: 1, FailedOps: 9, ErrorRate: 0.0}); err == nil {
		t.Fatalf("expected error due to low success rate")
	}
}

func TestConcreteChaosScenarios(t *testing.T) {
	lat := NewNetworkLatencyScenario(0, 0)
	if lat.Name() == "" || lat.Description() == "" {
		t.Fatalf("expected name and description")
	}
	failure, err := lat.InjectFailure(context.Background())
	if err != nil || failure == nil {
		t.Fatalf("InjectFailure error=%v failure=%#v", err, failure)
	}
	_ = failure.Type()
	_ = failure.Severity()
	_ = failure.Duration()
	_ = failure.IsActive()
	_ = failure.Cleanup()
	results, err := lat.ExecuteOperations(context.Background())
	if err != nil || len(results) == 0 {
		t.Fatalf("ExecuteOperations error=%v results=%d", err, len(results))
	}
	_ = lat.ValidateRecovery(context.Background(), &ChaosMetrics{AverageLatency: 0})

	svc := NewServiceUnavailableScenario("svc", 0)
	if svc.Name() == "" || svc.Description() == "" {
		t.Fatalf("expected name and description")
	}
	failure, err = svc.InjectFailure(context.Background())
	if err != nil || failure == nil {
		t.Fatalf("InjectFailure error=%v failure=%#v", err, failure)
	}
	_ = failure.Type()
	_ = failure.Severity()
	_ = failure.Duration()
	_ = failure.IsActive()
	_ = failure.Cleanup()
	results, err = svc.ExecuteOperations(context.Background())
	if err != nil || len(results) == 0 {
		t.Fatalf("ExecuteOperations error=%v results=%d", err, len(results))
	}
	if err := svc.ValidateRecovery(context.Background(), &ChaosMetrics{ErrorRate: 0.2}); err == nil {
		t.Fatalf("ValidateRecovery expected error for high error rate")
	}
}

func TestRecoveryValidator_HealthChecks(t *testing.T) {
	r := NewRecoveryValidator()

	r.AddHealthCheck(func(context.Context) error { return nil })
	if !r.isSystemHealthy(context.Background()) {
		t.Fatalf("expected system to be healthy")
	}

	r.AddHealthCheck(func(context.Context) error { return errors.New("unhealthy") })
	if r.isSystemHealthy(context.Background()) {
		t.Fatalf("expected system to be unhealthy")
	}
}
