package enterprise

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestChaosEngineeringFramework_RunExperiment_Validation(t *testing.T) {
	framework := NewChaosEngineeringFramework(&ChaosEngineeringConfig{
		Security: SecurityConfig{
			ForbiddenTargets: []string{"forbidden"},
		},
	})

	if _, err := framework.RunExperiment(context.Background(), &ChaosExperiment{}); err == nil {
		t.Fatalf("RunExperiment expected validation error")
	}

	exp := &ChaosExperiment{
		ID:       "exp1",
		Name:     "experiment",
		Duration: 1 * time.Second,
		Target: ExperimentTarget{
			Identifier: "ok",
			Name:       "svc",
			Namespace:  "default",
		},
		Faults: []FaultDefinition{
			{ID: "f1", Type: LatencyFault},
		},
	}

	results, err := framework.RunExperiment(context.Background(), exp)
	if err != nil || results == nil || results.Status != CompletedExperimentStatus {
		t.Fatalf("RunExperiment results=%#v err=%v", results, err)
	}
	if framework.experiments["exp1"] == nil {
		t.Fatalf("expected experiment to be stored in framework")
	}

	exp.Target.Identifier = "forbidden"
	if _, err := framework.RunExperiment(context.Background(), exp); err == nil {
		t.Fatalf("RunExperiment expected forbidden target error")
	}
}

func TestChaosEngineeringFramework_RecommendationsAndHelpers(t *testing.T) {
	framework := NewChaosEngineeringFramework(&ChaosEngineeringConfig{})

	recs := framework.generateRecommendations(nil, nil)
	if len(recs) == 0 {
		t.Fatalf("expected fallback recommendation")
	}

	exp := &ChaosExperiment{Type: NetworkChaos}
	results := &ExperimentResults{Failures: nil, Recovery: &RecoveryResults{Successful: true}}
	recs = framework.generateRecommendations(exp, results)
	if len(recs) == 0 {
		t.Fatalf("expected recommendations")
	}

	results.Failures = []ExperimentFailure{{Severity: CriticalSeverity}}
	recs = framework.generateRecommendations(exp, results)
	if len(recs) == 0 {
		t.Fatalf("expected recommendations for failures")
	}
}

func TestChaosEngineeringHelpers_ScoresBlastRadiusAndSummaries(t *testing.T) {
	if score := CalculateResilienceScore(nil); score != 0 {
		t.Fatalf("CalculateResilienceScore(nil)=%v, want 0", score)
	}

	score := CalculateResilienceScore(&ExperimentResults{
		Failures:        []ExperimentFailure{{}, {}},
		HypothesisValid: false,
		Recovery:        &RecoveryResults{Successful: false},
	})
	if score <= 0 || score > 100 {
		t.Fatalf("unexpected resilience score: %v", score)
	}

	br := GenerateBlastRadius(nil)
	if br.Scope != "unknown" {
		t.Fatalf("GenerateBlastRadius(nil).Scope=%q, want %q", br.Scope, "unknown")
	}

	br = GenerateBlastRadius(&ChaosExperiment{
		Type:     DatabaseChaos,
		Duration: 10 * time.Second,
		Target: ExperimentTarget{
			Name:      "db",
			Namespace: "default",
		},
	})
	if br.Scope == "" || br.Severity == "" || br.Impact == nil {
		t.Fatalf("unexpected blast radius: %#v", br)
	}

	hyp := &ExperimentHypothesis{
		ExpectedRecovery: &RecoveryConfig{Timeout: 1 * time.Second},
	}
	results := &ExperimentResults{
		Status:    CompletedExperimentStatus,
		Failures:  []ExperimentFailure{},
		Recovery:  &RecoveryResults{Duration: 2 * time.Second},
		Duration:  2 * time.Second,
		Summary:   "ok",
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}
	if framework := NewChaosEngineeringFramework(&ChaosEngineeringConfig{}); framework.validateHypothesis(hyp, results) {
		t.Fatalf("validateHypothesis expected false due to recovery timeout")
	}

	framework := NewChaosEngineeringFramework(&ChaosEngineeringConfig{})
	if impact := framework.calculateImpact(nil, nil); impact != "unknown" {
		t.Fatalf("calculateImpact(nil)=%q, want %q", impact, "unknown")
	}
	if impact := framework.calculateImpact(&ChaosExperiment{}, &ExperimentResults{Failures: []ExperimentFailure{{Severity: CriticalSeverity}}}); impact != "critical" {
		t.Fatalf("calculateImpact critical=%q, want %q", impact, "critical")
	}
	if impact := framework.calculateImpact(&ChaosExperiment{}, &ExperimentResults{Failures: make([]ExperimentFailure, 6)}); impact != "high" {
		t.Fatalf("calculateImpact high=%q, want %q", impact, "high")
	}
	if impact := framework.calculateImpact(&ChaosExperiment{}, &ExperimentResults{Failures: []ExperimentFailure{{}}}); impact != "medium" {
		t.Fatalf("calculateImpact medium=%q, want %q", impact, "medium")
	}
	if impact := framework.calculateImpact(&ChaosExperiment{}, &ExperimentResults{Failures: nil}); impact != "low" {
		t.Fatalf("calculateImpact low=%q, want %q", impact, "low")
	}

	summary := framework.generateExperimentSummary(nil, nil)
	if !strings.Contains(summary, "No experiment data") {
		t.Fatalf("unexpected summary: %q", summary)
	}
	summary = framework.generateExperimentSummary(&ChaosExperiment{ID: "id", Name: "name"}, &ExperimentResults{
		Status:          CompletedExperimentStatus,
		Duration:        2 * time.Second,
		HypothesisValid: true,
		Recovery:        &RecoveryResults{Successful: true, Duration: 500 * time.Millisecond},
		Failures:        []ExperimentFailure{{}, {}},
	})
	if summary == "" {
		t.Fatalf("expected non-empty summary")
	}
}

func TestContractTestSuite_Basics(t *testing.T) {
	suite := NewContractTestSuite()

	if _, err := suite.CreateContractTest("missing", NewBasicContractValidator()); err == nil {
		t.Fatalf("CreateContractTest(missing) expected error")
	}

	contract := &Contract{
		ID:       "c1",
		Name:     "contract",
		Version:  "v1",
		Provider: "provider",
		Consumer: "consumer",
	}
	suite.AddContract(contract)

	if _, err := suite.CreateContractTest(contract.Name, NewBasicContractValidator()); err != nil {
		t.Fatalf("CreateContractTest error: %v", err)
	}

	results, err := suite.RunContractTests()
	if err != nil {
		t.Fatalf("RunContractTests error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("RunContractTests expected 1 result, got %d", len(results))
	}
}

func TestChaosReporter_DefaultTemplate(t *testing.T) {
	reporter := NewChaosReporter()
	if reporter.templates["experiment"] == nil {
		t.Fatalf("expected default experiment template")
	}
	if len(reporter.templates["experiment"].Sections) == 0 {
		t.Fatalf("expected template sections")
	}
}
