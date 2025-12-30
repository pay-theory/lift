package enterprise

import (
	"context"
	"testing"
	"time"
)

func TestMultiRegionChaosOrchestrator_CreateDistributedExperiment(t *testing.T) {
	if _, err := NewMultiRegionChaosOrchestrator(nil); err == nil {
		t.Fatalf("NewMultiRegionChaosOrchestrator(nil) expected error")
	}

	orchestrator, err := NewMultiRegionChaosOrchestrator(&DistributedConfig{
		Regions: []*RegionConfig{
			{Name: "us-east-1"},
		},
	})
	if err != nil {
		t.Fatalf("NewMultiRegionChaosOrchestrator error: %v", err)
	}

	_, err = orchestrator.CreateDistributedExperiment(context.Background(), &DistributedExperimentSpec{})
	if err == nil {
		t.Fatalf("CreateDistributedExperiment expected validation error")
	}

	_, err = orchestrator.CreateDistributedExperiment(context.Background(), &DistributedExperimentSpec{
		Name:    "exp",
		Regions: []string{"unknown"},
		Phases:  []*ExperimentPhase{{Name: "p1", Duration: 1 * time.Second}},
	})
	if err == nil {
		t.Fatalf("CreateDistributedExperiment expected region not found error")
	}

	exp, err := orchestrator.CreateDistributedExperiment(context.Background(), &DistributedExperimentSpec{
		Name:    "exp",
		Regions: []string{"us-east-1"},
		Phases:  []*ExperimentPhase{{Name: "p1", Duration: 1 * time.Second}},
	})
	if err != nil || exp == nil || exp.ID == "" {
		t.Fatalf("CreateDistributedExperiment expected success, got exp=%#v err=%v", exp, err)
	}

	if err := orchestrator.coordinator.ExecuteExperiment(context.Background(), exp); err != nil {
		t.Fatalf("ExecuteExperiment error: %v", err)
	}
}
