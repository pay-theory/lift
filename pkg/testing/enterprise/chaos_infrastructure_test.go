package enterprise

import (
	"context"
	"testing"
	"time"
)

func TestFaultInjectors_StatusLifecycle(t *testing.T) {
	target := ExperimentTarget{Name: "svc"}

	nfi := NewNetworkFaultInjector(&NetworkFaultConfig{DefaultDelay: 10 * time.Millisecond})
	latencyFault := FaultDefinition{
		ID:         "f1",
		Type:       LatencyFault,
		Duration:   0,
		Parameters: map[string]any{},
	}
	if err := nfi.Inject(context.Background(), latencyFault, target); err != nil {
		t.Fatalf("NetworkFaultInjector.Inject latency error: %v", err)
	}
	if status, err := nfi.Status(context.Background(), latencyFault, target); err != nil || !status.Active {
		t.Fatalf("NetworkFaultInjector.Status active=%v err=%v", status.Active, err)
	}
	if err := nfi.Remove(context.Background(), latencyFault, target); err != nil {
		t.Fatalf("NetworkFaultInjector.Remove error: %v", err)
	}
	if status, err := nfi.Status(context.Background(), latencyFault, target); err != nil || status.Active {
		t.Fatalf("NetworkFaultInjector.Status expected inactive, got active=%v err=%v", status.Active, err)
	}

	if err := nfi.Inject(context.Background(), FaultDefinition{
		ID:         "f2",
		Type:       NetworkPartition,
		Duration:   0,
		Parameters: map[string]any{"partition": 123},
	}, target); err == nil {
		t.Fatalf("NetworkFaultInjector.Inject expected error for bad partition type")
	}

	sfi := NewServiceFaultInjector(&ServiceFaultConfig{DefaultTimeout: 5 * time.Second})
	svcFault := FaultDefinition{
		ID:         "s1",
		Type:       ServiceUnavailable,
		Duration:   0,
		Parameters: map[string]any{},
	}
	if err := sfi.Inject(context.Background(), svcFault, target); err != nil {
		t.Fatalf("ServiceFaultInjector.Inject error: %v", err)
	}
	if status, err := sfi.Status(context.Background(), svcFault, target); err != nil || !status.Active {
		t.Fatalf("ServiceFaultInjector.Status active=%v err=%v", status.Active, err)
	}
	if err := sfi.Remove(context.Background(), svcFault, target); err != nil {
		t.Fatalf("ServiceFaultInjector.Remove error: %v", err)
	}

	rfi := NewResourceFaultInjector(&ResourceFaultConfig{})
	resourceFault := FaultDefinition{
		ID:         "r1",
		Type:       ResourceExhaustion,
		Duration:   0,
		Parameters: map[string]any{"resource_type": "cpu", "percentage": 0.5},
	}
	if err := rfi.Inject(context.Background(), resourceFault, target); err != nil {
		t.Fatalf("ResourceFaultInjector.Inject error: %v", err)
	}
	if status, err := rfi.Status(context.Background(), resourceFault, target); err != nil || !status.Active {
		t.Fatalf("ResourceFaultInjector.Status active=%v err=%v", status.Active, err)
	}
	if err := rfi.Remove(context.Background(), resourceFault, target); err != nil {
		t.Fatalf("ResourceFaultInjector.Remove error: %v", err)
	}
	if status, err := rfi.Status(context.Background(), resourceFault, target); err != nil || status.Active {
		t.Fatalf("ResourceFaultInjector.Status expected inactive, got active=%v err=%v", status.Active, err)
	}
	if _, err := rfi.Status(context.Background(), FaultDefinition{ID: "missing"}, target); err != nil {
		t.Fatalf("ResourceFaultInjector.Status missing error: %v", err)
	}
}

func TestValidateExperimentSafety(t *testing.T) {
	experiment := &ChaosExperiment{
		ID:       "exp1",
		Name:     "exp",
		Duration: 10 * time.Second,
		Target: ExperimentTarget{
			Identifier: "svc",
			Scope:      ClusterScope,
		},
		Faults: []FaultDefinition{
			{ID: "f1", Severity: CriticalSeverity},
		},
	}

	policy := &ChaosPolicy{
		Rules: []PolicyRule{
			{Type: BlastRadiusRule, Enabled: true, Parameters: map[string]any{"max_percentage": 20.0}},
			{Type: TimeWindowRule, Enabled: true, Parameters: map[string]any{"max_duration": 1 * time.Second}},
			{Type: ApprovalRule, Enabled: true, Parameters: map[string]any{}},
			{Type: ApprovalRule, Enabled: false, Parameters: map[string]any{}},
		},
	}

	violations := ValidateExperimentSafety(experiment, policy)
	if len(violations) < 2 {
		t.Fatalf("expected violations, got %d: %#v", len(violations), violations)
	}
}
