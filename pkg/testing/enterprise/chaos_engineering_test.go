package enterprise

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestChaosEngineeringFramework_Creation(t *testing.T) {
	config := &ChaosEngineeringConfig{
		Environment:              "test",
		SafetyMode:               true,
		MaxConcurrentExperiments: 5,
		DefaultTimeout:           30 * time.Minute,
		MonitoringInterval:       30 * time.Second,
		RetentionPeriod:          7 * 24 * time.Hour,
		Notifications: NotificationConfig{
			Enabled: true,
		},
		Security: SecurityConfig{
			RequireApproval:   true,
			AuditLogging:      true,
			EncryptionEnabled: true,
		},
	}

	framework := NewChaosEngineeringFramework(config)

	if framework == nil {
		t.Fatal("Framework should not be nil")
	}
	if framework.experiments == nil {
		t.Error("Experiments map should not be nil")
	}
	if framework.scheduler == nil {
		t.Error("Scheduler should not be nil")
	}
	if framework.reporter == nil {
		t.Error("Reporter should not be nil")
	}
	if framework.config != config {
		t.Error("Config should match")
	}
}

func TestChaosExperiment_Creation(t *testing.T) {
	now := time.Now()
	experiment := &ChaosExperiment{
		ID:          "test-experiment-1",
		Name:        "Network Latency Test",
		Description: "Test system resilience to network latency",
		Type:        NetworkChaos,
		Target: ExperimentTarget{
			Type:       ServiceTarget,
			Identifier: "user-service",
			Scope:      SingleInstanceScope,
		},
		Faults: []FaultDefinition{
			{
				ID:          "latency-fault-1",
				Type:        LatencyFault,
				Severity:    MediumSeverity,
				Duration:    5 * time.Minute,
				Probability: 1.0,
				Parameters: map[string]interface{}{
					"delay": 100 * time.Millisecond,
				},
				Recovery: &RecoveryConfig{
					Automatic:     true,
					Timeout:       30 * time.Second,
					RetryAttempts: 3,
					RetryDelay:    5 * time.Second,
					Rollback:      true,
				},
			},
		},
		Hypothesis: "System should maintain <200ms response time with graceful degradation",
		Duration:   10 * time.Minute,
		Status:     ExperimentPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if experiment.ID != "test-experiment-1" {
		t.Errorf("Expected ID 'test-experiment-1', got %s", experiment.ID)
	}
	if experiment.Name != "Network Latency Test" {
		t.Errorf("Expected Name 'Network Latency Test', got %s", experiment.Name)
	}
	if experiment.Type != NetworkChaos {
		t.Errorf("Expected Type NetworkChaos, got %s", experiment.Type)
	}
	if experiment.Target.Type != ServiceTarget {
		t.Errorf("Expected Target Type ServiceTarget, got %s", experiment.Target.Type)
	}
	if experiment.Target.Scope != SingleInstanceScope {
		t.Errorf("Expected Target Scope SingleInstanceScope, got %s", experiment.Target.Scope)
	}
	if len(experiment.Faults) != 1 {
		t.Errorf("Expected 1 fault, got %d", len(experiment.Faults))
	}
	if experiment.Faults[0].Type != LatencyFault {
		t.Errorf("Expected fault type LatencyFault, got %s", experiment.Faults[0].Type)
	}
	if experiment.Faults[0].Severity != MediumSeverity {
		t.Errorf("Expected fault severity MediumSeverity, got %s", experiment.Faults[0].Severity)
	}
	if experiment.Status != ExperimentPending {
		t.Errorf("Expected status ExperimentPending, got %s", experiment.Status)
	}
	if experiment.Description != "Test system resilience to network latency" {
		t.Errorf("Expected Description 'Test system resilience to network latency', got %s", experiment.Description)
	}
	if experiment.Hypothesis != "System should maintain <200ms response time with graceful degradation" {
		t.Errorf("Expected Hypothesis 'System should maintain <200ms response time with graceful degradation', got %s", experiment.Hypothesis)
	}
	if experiment.Duration != 10*time.Minute {
		t.Errorf("Expected Duration 10m, got %v", experiment.Duration)
	}
	if experiment.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if experiment.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestChaosEngineeringFramework_RunExperiment(t *testing.T) {
	config := &ChaosEngineeringConfig{
		Environment:              "test",
		SafetyMode:               true,
		MaxConcurrentExperiments: 5,
		DefaultTimeout:           30 * time.Minute,
		MonitoringInterval:       1 * time.Second, // Fast for testing
		Security: SecurityConfig{
			RequireApproval: false, // Disable for testing
		},
	}

	framework := NewChaosEngineeringFramework(config)

	experiment := &ChaosExperiment{
		ID:          "test-experiment-2",
		Name:        "Service Error Test",
		Description: "Test system resilience to service errors",
		Type:        ServiceChaos,
		Target: ExperimentTarget{
			Type:       ServiceTarget,
			Identifier: "payment-service",
			Scope:      SingleInstanceScope,
		},
		Faults: []FaultDefinition{
			{
				ID:          "error-fault-1",
				Type:        ErrorFault,
				Severity:    LowSeverity,
				Duration:    2 * time.Second, // Short for testing
				Probability: 1.0,
				Parameters: map[string]interface{}{
					"error_rate":  0.1,
					"status_code": 500,
				},
			},
		},
		Hypothesis: "System should handle 10% error rate gracefully",
		Duration:   3 * time.Second, // Short for testing
		Status:     ExperimentPending,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := framework.RunExperiment(ctx, experiment)

	if err != nil {
		t.Fatalf("RunExperiment failed: %v", err)
	}
	if results == nil {
		t.Fatal("Results should not be nil")
	}
	if results.Status != ExperimentCompleted {
		t.Errorf("Expected status ExperimentCompleted, got %s", results.Status)
	}
	if !results.EndTime.After(results.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
	if results.Duration <= 0 {
		t.Error("Duration should be positive")
	}
	if results.Summary == "" {
		t.Error("Summary should not be empty")
	}
	if len(results.Recommendations) == 0 {
		t.Error("Recommendations should not be empty")
	}
	if results.Metrics == nil {
		t.Error("Metrics should not be nil")
	}
	if results.Recovery == nil {
		t.Error("Recovery should not be nil")
	}
	if !results.Recovery.Attempted {
		t.Error("Recovery should be attempted")
	}
}

func TestChaosExperiment_Validation(t *testing.T) {
	config := &ChaosEngineeringConfig{
		Environment: "test",
		Security: SecurityConfig{
			RequireApproval:  false,
			ForbiddenTargets: []string{"production-db"},
		},
	}

	framework := NewChaosEngineeringFramework(config)

	tests := []struct {
		name        string
		experiment  *ChaosExperiment
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid experiment",
			experiment: &ChaosExperiment{
				ID:   "valid-exp",
				Name: "Valid Experiment",
				Faults: []FaultDefinition{
					{
						ID:   "fault-1",
						Type: LatencyFault,
					},
				},
				Target: ExperimentTarget{
					Identifier: "test-service",
				},
			},
			expectError: false,
		},
		{
			name: "missing ID",
			experiment: &ChaosExperiment{
				Name: "Missing ID Experiment",
				Faults: []FaultDefinition{
					{
						ID:   "fault-1",
						Type: LatencyFault,
					},
				},
			},
			expectError: true,
			errorMsg:    "experiment ID is required",
		},
		{
			name: "missing name",
			experiment: &ChaosExperiment{
				ID: "missing-name-exp",
				Faults: []FaultDefinition{
					{
						ID:   "fault-1",
						Type: LatencyFault,
					},
				},
			},
			expectError: true,
			errorMsg:    "experiment name is required",
		},
		{
			name: "no faults",
			experiment: &ChaosExperiment{
				ID:     "no-faults-exp",
				Name:   "No Faults Experiment",
				Faults: []FaultDefinition{},
			},
			expectError: true,
			errorMsg:    "experiment must have at least one fault",
		},
		{
			name: "forbidden target",
			experiment: &ChaosExperiment{
				ID:   "forbidden-exp",
				Name: "Forbidden Target Experiment",
				Faults: []FaultDefinition{
					{
						ID:   "fault-1",
						Type: LatencyFault,
					},
				},
				Target: ExperimentTarget{
					Identifier: "production-db",
				},
			},
			expectError: true,
			errorMsg:    "target production-db is forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := framework.validateExperiment(tt.experiment)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestFaultInjectors(t *testing.T) {
	ctx := context.Background()

	t.Run("NetworkFaultInjector", func(t *testing.T) {
		config := &NetworkFaultConfig{
			Interface:     "eth0",
			DefaultDelay:  50 * time.Millisecond,
			DefaultLoss:   0.01,
			DefaultJitter: 10 * time.Millisecond,
			MaxBandwidth:  1000000,
		}

		injector := NewNetworkFaultInjector(config)
		if injector == nil {
			t.Fatal("Injector should not be nil")
		}

		fault := FaultDefinition{
			ID:       "network-fault-1",
			Type:     LatencyFault,
			Duration: 1 * time.Minute,
			Parameters: map[string]interface{}{
				"delay": 100 * time.Millisecond,
			},
		}

		target := ExperimentTarget{
			Type:       NetworkTarget,
			Identifier: "test-network",
		}

		// Test injection
		err := injector.Inject(ctx, fault, target)
		if err != nil {
			t.Errorf("Inject failed: %v", err)
		}

		// Test status
		status, err := injector.Status(ctx, fault, target)
		if err != nil {
			t.Errorf("Status failed: %v", err)
		}
		if !status.Active {
			t.Error("Status should be active")
		}
		if status.StartTime.IsZero() {
			t.Error("StartTime should not be zero")
		}

		// Test removal
		err = injector.Remove(ctx, fault, target)
		if err != nil {
			t.Errorf("Remove failed: %v", err)
		}

		// Verify removal
		status, err = injector.Status(ctx, fault, target)
		if err != nil {
			t.Errorf("Status after removal failed: %v", err)
		}
		if status.Active {
			t.Error("Status should not be active after removal")
		}
	})

	t.Run("ServiceFaultInjector", func(t *testing.T) {
		config := &ServiceFaultConfig{
			ServiceName:    "test-service",
			BaseURL:        "http://localhost:8080",
			DefaultTimeout: 30 * time.Second,
			ErrorRates: map[string]float64{
				"GET":  0.01,
				"POST": 0.02,
			},
		}

		injector := NewServiceFaultInjector(config)
		if injector == nil {
			t.Fatal("Injector should not be nil")
		}

		fault := FaultDefinition{
			ID:       "service-fault-1",
			Type:     ServiceUnavailable,
			Duration: 1 * time.Minute,
			Parameters: map[string]interface{}{
				"status_code": 503,
			},
		}

		target := ExperimentTarget{
			Type:       ServiceTarget,
			Identifier: "test-service",
		}

		// Test injection
		err := injector.Inject(ctx, fault, target)
		if err != nil {
			t.Errorf("Inject failed: %v", err)
		}

		// Test status
		status, err := injector.Status(ctx, fault, target)
		if err != nil {
			t.Errorf("Status failed: %v", err)
		}
		if !status.Active {
			t.Error("Status should be active")
		}

		// Test removal
		err = injector.Remove(ctx, fault, target)
		if err != nil {
			t.Errorf("Remove failed: %v", err)
		}
	})

	t.Run("ResourceFaultInjector", func(t *testing.T) {
		config := &ResourceFaultConfig{
			CPULimits: map[string]float64{
				"test": 80.0,
			},
			MemoryLimits: map[string]int64{
				"test": 1024 * 1024 * 1024, // 1GB
			},
		}

		injector := NewResourceFaultInjector(config)
		if injector == nil {
			t.Fatal("Injector should not be nil")
		}

		fault := FaultDefinition{
			ID:       "resource-fault-1",
			Type:     ResourceExhaustion,
			Duration: 1 * time.Minute,
			Parameters: map[string]interface{}{
				"resource_type": "cpu",
				"percentage":    75.0,
			},
		}

		target := ExperimentTarget{
			Type:       InfrastructureTarget,
			Identifier: "test-instance",
		}

		// Test injection
		err := injector.Inject(ctx, fault, target)
		if err != nil {
			t.Errorf("Inject failed: %v", err)
		}

		// Test status
		status, err := injector.Status(ctx, fault, target)
		if err != nil {
			t.Errorf("Status failed: %v", err)
		}
		if !status.Active {
			t.Error("Status should be active")
		}

		// Test removal
		err = injector.Remove(ctx, fault, target)
		if err != nil {
			t.Errorf("Remove failed: %v", err)
		}
	})
}

func TestChaosGameDay(t *testing.T) {
	now := time.Now()
	gameDay := &ChaosGameDay{
		ID:          "game-day-1",
		Name:        "Q4 Resilience Game Day",
		Description: "Quarterly chaos engineering exercise",
		Scenarios: []GameDayScenario{
			{
				ID:          "scenario-1",
				Name:        "Database Failover",
				Description: "Test database failover procedures",
				Type:        DisasterRecoveryScenario,
				Experiments: []string{"db-failover-exp"},
				Duration:    30 * time.Minute,
				Sequence:    1,
			},
			{
				ID:           "scenario-2",
				Name:         "Network Partition",
				Description:  "Test network partition handling",
				Type:         PerformanceScenario,
				Experiments:  []string{"network-partition-exp"},
				Duration:     20 * time.Minute,
				Sequence:     2,
				Dependencies: []string{"scenario-1"},
			},
		},
		Participants: []Participant{
			{
				ID:   "participant-1",
				Name: "John Doe",
				Role: IncidentCommanderRole,
				Team: "SRE",
				Contact: ContactInfo{
					Email:  "john.doe@company.com",
					Phone:  "+1-555-0123",
					OnCall: true,
				},
				Skills: []string{"incident-management", "kubernetes"},
			},
		},
		Schedule: &GameDaySchedule{
			StartTime: now.Add(24 * time.Hour),
			EndTime:   now.Add(28 * time.Hour),
			Duration:  4 * time.Hour,
			TimeZone:  "UTC",
			Breaks: []Break{
				{
					Name:      "Lunch Break",
					StartTime: now.Add(26 * time.Hour),
					Duration:  1 * time.Hour,
					Type:      LunchBreak,
				},
			},
		},
		Objectives: []string{
			"Validate disaster recovery procedures",
			"Test team coordination during incidents",
			"Identify system weaknesses",
		},
		Success: []SuccessCriteria{
			{
				ID:          "criteria-1",
				Name:        "Recovery Time",
				Description: "System recovery within 15 minutes",
				Type:        TimeCriteria,
				Target:      15 * time.Minute,
				Weight:      0.4,
			},
			{
				ID:          "criteria-2",
				Name:        "Data Integrity",
				Description: "No data loss during failover",
				Type:        QualityCriteria,
				Target:      true,
				Weight:      0.6,
			},
		},
		Status:    PlannedGameDay,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if gameDay.ID != "game-day-1" {
		t.Errorf("Expected ID 'game-day-1', got %s", gameDay.ID)
	}
	if gameDay.Name != "Q4 Resilience Game Day" {
		t.Errorf("Expected Name 'Q4 Resilience Game Day', got %s", gameDay.Name)
	}
	if len(gameDay.Scenarios) != 2 {
		t.Errorf("Expected 2 scenarios, got %d", len(gameDay.Scenarios))
	}
	if len(gameDay.Participants) != 1 {
		t.Errorf("Expected 1 participant, got %d", len(gameDay.Participants))
	}
	if len(gameDay.Success) != 2 {
		t.Errorf("Expected 2 success criteria, got %d", len(gameDay.Success))
	}
	if gameDay.Status != PlannedGameDay {
		t.Errorf("Expected status PlannedGameDay, got %s", gameDay.Status)
	}
	if gameDay.Participants[0].Role != IncidentCommanderRole {
		t.Errorf("Expected role IncidentCommanderRole, got %s", gameDay.Participants[0].Role)
	}
	if gameDay.Scenarios[0].Type != DisasterRecoveryScenario {
		t.Errorf("Expected scenario type DisasterRecoveryScenario, got %s", gameDay.Scenarios[0].Type)
	}
	if gameDay.Description != "Quarterly chaos engineering exercise" {
		t.Errorf("Expected Description 'Quarterly chaos engineering exercise', got %s", gameDay.Description)
	}
	if gameDay.Schedule == nil {
		t.Error("Schedule should not be nil")
	}
	if len(gameDay.Objectives) != 3 {
		t.Errorf("Expected 3 objectives, got %d", len(gameDay.Objectives))
	}
	if gameDay.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if gameDay.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestResilienceMetrics(t *testing.T) {
	now := time.Now()
	metrics := &ResilienceMetrics{
		MTTR:            15 * time.Minute,
		MTBF:            72 * time.Hour,
		Availability:    99.9,
		ErrorBudget:     85.0,
		LastIncident:    now.Add(-24 * time.Hour),
		IncidentCount:   3,
		ResilienceScore: 95.0,
		Trends:          make(map[string]interface{}),
		ExperimentCount: 10,
		LastUpdated:     now,
	}

	// Create ExperimentResults for testing the score calculation
	results := &ExperimentResults{
		ExperimentID:    "test-exp",
		Status:          ExperimentCompleted,
		HypothesisValid: true,
		Failures:        []ExperimentFailure{},
		Recovery: &RecoveryResults{
			Successful: true,
		},
	}

	score := CalculateResilienceScore(results)
	if score <= 0.0 {
		t.Error("Score should be greater than 0")
	}
	if score > 100.0 {
		t.Error("Score should be less than or equal to 100")
	}

	// Test with high availability metrics
	metrics.Availability = 99.99
	metrics.MTTR = 5 * time.Minute
	metrics.MTBF = 168 * time.Hour // 1 week
	metrics.ErrorBudget = 95.0

	// Test with better results
	betterResults := &ExperimentResults{
		ExperimentID:    "test-exp-2",
		Status:          ExperimentCompleted,
		HypothesisValid: true,
		Failures:        []ExperimentFailure{},
		Recovery: &RecoveryResults{
			Successful: true,
		},
	}

	highScore := CalculateResilienceScore(betterResults)
	if highScore <= 0.0 {
		t.Error("High availability should result in positive score")
	}
	if highScore > 100.0 {
		t.Error("Score should not exceed 100")
	}

	// Test that metrics fields are properly set and accessible
	if metrics.LastIncident.IsZero() {
		t.Error("LastIncident should not be zero")
	}
	if metrics.IncidentCount != 3 {
		t.Errorf("Expected IncidentCount 3, got %d", metrics.IncidentCount)
	}
	if metrics.ResilienceScore != 95.0 {
		t.Errorf("Expected ResilienceScore 95.0, got %f", metrics.ResilienceScore)
	}
	if metrics.Trends == nil {
		t.Error("Trends should not be nil")
	}
	if metrics.ExperimentCount != 10 {
		t.Errorf("Expected ExperimentCount 10, got %d", metrics.ExperimentCount)
	}
	if metrics.LastUpdated.IsZero() {
		t.Error("LastUpdated should not be zero")
	}
}

func TestBlastRadius(t *testing.T) {
	experiment := &ChaosExperiment{
		ID:   "test-blast-radius",
		Name: "Test Blast Radius",
		Type: ServiceChaos,
		Target: ExperimentTarget{
			Type:       ServiceTarget,
			Name:       "user-service",
			Identifier: "user-service",
			Scope:      ClusterScope,
		},
		Duration: 10 * time.Minute,
	}

	blastRadius := GenerateBlastRadius(experiment)

	if blastRadius == nil {
		t.Fatal("BlastRadius should not be nil")
	}
	if blastRadius.Scope != "service" {
		t.Errorf("Expected scope 'service', got %s", blastRadius.Scope)
	}
	if blastRadius.Severity != "high" {
		t.Errorf("Expected severity 'high' for ServiceChaos, got %s", blastRadius.Severity)
	}
	if blastRadius.Impact == nil {
		t.Error("Impact should not be nil")
	}

	// Check impact contains expected fields
	if target, exists := blastRadius.Impact["target"]; !exists || target != "user-service" {
		t.Errorf("Expected impact target 'user-service', got %v", target)
	}
	if expType, exists := blastRadius.Impact["type"]; !exists || expType != "service" {
		t.Errorf("Expected impact type 'service', got %v", expType)
	}

	// Test with different experiment type
	experiment2 := &ChaosExperiment{
		ID:   "test-blast-radius-2",
		Name: "Test Network Blast Radius",
		Type: NetworkChaos,
		Target: ExperimentTarget{
			Type:       NetworkTarget,
			Name:       "network-component",
			Identifier: "network-component",
			Scope:      ClusterScope,
		},
		Duration: 5 * time.Minute,
	}

	blastRadius2 := GenerateBlastRadius(experiment2)
	if blastRadius2.Scope != "network" {
		t.Errorf("Expected scope 'network' for NetworkChaos, got %s", blastRadius2.Scope)
	}
	if blastRadius2.Severity != "medium" {
		t.Errorf("Expected severity 'medium' for NetworkChaos, got %s", blastRadius2.Severity)
	}
}

func TestChaosPolicy(t *testing.T) {
	policy := &ChaosPolicy{
		ID:          "policy-1",
		Name:        "Production Safety Policy",
		Description: "Safety policy for production chaos experiments",
		Rules: []PolicyRule{
			{
				ID:        "rule-1",
				Name:      "Blast Radius Limit",
				Type:      BlastRadiusRule,
				Condition: "max_percentage <= 10",
				Action:    DenyPolicyAction,
				Severity:  ErrorPolicySeverity,
				Parameters: map[string]interface{}{
					"max_percentage": 10.0,
				},
				Enabled: true,
			},
			{
				ID:        "rule-2",
				Name:      "Time Window Limit",
				Type:      TimeWindowRule,
				Condition: "duration <= 30m",
				Action:    RequireApprovalPolicy,
				Severity:  WarningPolicySeverity,
				Parameters: map[string]interface{}{
					"max_duration": 30 * time.Minute,
				},
				Enabled: true,
			},
			{
				ID:        "rule-3",
				Name:      "Critical Severity Approval",
				Type:      ApprovalRule,
				Condition: "severity == critical",
				Action:    RequireApprovalPolicy,
				Severity:  CriticalPolicySeverity,
				Parameters: map[string]interface{}{
					"require_approval": true,
				},
				Enabled: true,
			},
		},
		Enforcement: StrictEnforcementPolicy,
		Scope: PolicyScope{
			Type:    GlobalChaosScope,
			Targets: []string{"production"},
		},
		Version:   "1.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if policy.ID != "policy-1" {
		t.Errorf("Expected ID 'policy-1', got %s", policy.ID)
	}
	if policy.Name != "Production Safety Policy" {
		t.Errorf("Expected Name 'Production Safety Policy', got %s", policy.Name)
	}
	if len(policy.Rules) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(policy.Rules))
	}
	if policy.Enforcement != StrictEnforcementPolicy {
		t.Errorf("Expected enforcement StrictEnforcementPolicy, got %s", policy.Enforcement)
	}
	if policy.Scope.Type != GlobalChaosScope {
		t.Errorf("Expected scope type GlobalChaosScope, got %s", policy.Scope.Type)
	}

	// Test policy validation
	experiment := &ChaosExperiment{
		ID:       "test-exp",
		Name:     "Test Experiment",
		Duration: 45 * time.Minute, // Exceeds policy limit
		Target: ExperimentTarget{
			Scope: ClusterScope,
		},
		Faults: []FaultDefinition{
			{
				Severity: CriticalSeverity,
			},
		},
	}

	violations := ValidateExperimentSafety(experiment, policy)
	if len(violations) == 0 {
		t.Error("Expected violations but got none")
	}

	foundDurationViolation := false
	foundApprovalViolation := false
	for _, violation := range violations {
		if violation == "Experiment duration exceeds policy limits" {
			foundDurationViolation = true
		}
		if violation == "Critical severity experiments require approval" {
			foundApprovalViolation = true
		}
	}

	if !foundDurationViolation {
		t.Error("Expected duration violation")
	}
	if !foundApprovalViolation {
		t.Error("Expected approval violation")
	}
}

func TestChaosScheduler(t *testing.T) {
	config := &ChaosEngineeringConfig{
		Environment:              "test",
		SafetyMode:               true,
		MaxConcurrentExperiments: 10,
		DefaultTimeout:           30 * time.Minute,
		MonitoringInterval:       1 * time.Minute,
		RetentionPeriod:          7 * 24 * time.Hour,
		Notifications: NotificationConfig{
			Enabled: true,
		},
		Security: SecurityConfig{
			RequireApproval: false,
		},
	}

	scheduler := NewChaosScheduler(config)

	if scheduler == nil {
		t.Fatal("Scheduler should not be nil")
	}
	if scheduler.experiments == nil {
		t.Error("Experiments should not be nil")
	}
	if scheduler.executor == nil {
		t.Error("Executor should not be nil")
	}
	if scheduler.config == nil {
		t.Error("Config should not be nil")
	}
	if scheduler.config.MaxConcurrentExperiments != 10 {
		t.Errorf("Expected MaxConcurrentExperiments 10, got %d", scheduler.config.MaxConcurrentExperiments)
	}
	if scheduler.config.MonitoringInterval != 1*time.Minute {
		t.Errorf("Expected MonitoringInterval 1m, got %v", scheduler.config.MonitoringInterval)
	}
	if scheduler.config.Environment != "test" {
		t.Errorf("Expected Environment 'test', got %s", scheduler.config.Environment)
	}
	if !scheduler.config.SafetyMode {
		t.Error("Expected SafetyMode true")
	}
}

func TestExperimentExecutor(t *testing.T) {
	config := &ChaosEngineeringConfig{
		Environment:              "test",
		SafetyMode:               true,
		MaxConcurrentExperiments: 5,
		DefaultTimeout:           30 * time.Minute,
		MonitoringInterval:       30 * time.Second,
		RetentionPeriod:          7 * 24 * time.Hour,
		Notifications: NotificationConfig{
			Enabled: true,
		},
		Security: SecurityConfig{
			RequireApproval:   true,
			AuditLogging:      true,
			EncryptionEnabled: true,
		},
	}

	executor := NewExperimentExecutor(config)

	if executor == nil {
		t.Fatal("Executor should not be nil")
	}
	if executor.workers == nil {
		t.Error("Workers should not be nil")
	}
	if executor.queue == nil {
		t.Error("Queue should not be nil")
	}
	if executor.results == nil {
		t.Error("Results should not be nil")
	}
	if executor.config == nil {
		t.Error("Config should not be nil")
	}
	if executor.config.MaxConcurrentExperiments != 5 {
		t.Errorf("Expected MaxConcurrentExperiments 5, got %d", executor.config.MaxConcurrentExperiments)
	}
	if executor.config.DefaultTimeout != 30*time.Minute {
		t.Errorf("Expected DefaultTimeout 30m, got %v", executor.config.DefaultTimeout)
	}
	if !executor.config.Security.RequireApproval {
		t.Error("Expected RequireApproval true")
	}
	if !executor.config.Security.AuditLogging {
		t.Error("Expected AuditLogging true")
	}
	if !executor.config.Security.EncryptionEnabled {
		t.Error("Expected EncryptionEnabled true")
	}
}

func TestChaosReporter(t *testing.T) {
	reporter := NewChaosReporter()

	if reporter == nil {
		t.Fatal("Reporter should not be nil")
	}
	if reporter.generators == nil {
		t.Error("Generators should not be nil")
	}
	if reporter.templates == nil {
		t.Error("Templates should not be nil")
	}
	if reporter.exporters == nil {
		t.Error("Exporters should not be nil")
	}

	// Test default templates
	experimentTemplate, exists := reporter.templates["experiment"]
	if !exists {
		t.Error("Experiment template should exist")
	}
	if experimentTemplate.ID != "experiment_template" {
		t.Errorf("Expected template ID 'experiment_template', got %s", experimentTemplate.ID)
	}
	if experimentTemplate.Name != "Chaos Experiment Report" {
		t.Errorf("Expected template name 'Chaos Experiment Report', got %s", experimentTemplate.Name)
	}
	if experimentTemplate.Type != ChaosReportType {
		t.Errorf("Expected template type ChaosReportType, got %s", experimentTemplate.Type)
	}
	if len(experimentTemplate.Sections) != 3 {
		t.Errorf("Expected 3 sections, got %d", len(experimentTemplate.Sections))
	}
}

func TestExperimentResults_Analysis(t *testing.T) {
	config := &ChaosEngineeringConfig{
		Environment:        "test",
		MonitoringInterval: 100 * time.Millisecond,
		Security: SecurityConfig{
			RequireApproval: false,
		},
	}

	framework := NewChaosEngineeringFramework(config)

	// Test hypothesis validation
	experiment := &ChaosExperiment{
		ID:         "analysis-test",
		Name:       "Analysis Test",
		Hypothesis: "System should remain stable",
		Faults: []FaultDefinition{
			{
				ID:   "test-fault",
				Type: LatencyFault,
			},
		},
		Target: ExperimentTarget{
			Identifier: "test-service",
		},
	}

	results := &ExperimentResults{
		Observations: []Observation{
			{
				Type:     MetricObservation,
				Severity: InfoObservationSeverity,
				Data: map[string]interface{}{
					"response_time_p95": 150.0,
					"error_rate":        0.02,
					"throughput":        900.0,
				},
			},
			{
				Type:     MetricObservation,
				Severity: InfoObservationSeverity,
				Data: map[string]interface{}{
					"response_time_p95": 180.0,
					"error_rate":        0.03,
					"throughput":        850.0,
				},
			},
		},
		Failures: []ExperimentFailure{},
		Recovery: &RecoveryResults{
			Successful: true,
		},
	}

	// Test hypothesis validation
	valid := framework.validateHypothesis(experiment, results)
	if !valid {
		t.Error("Hypothesis should be valid with no critical errors")
	}

	// Test impact calculation
	impact := framework.calculateImpact(results)
	if impact == nil {
		t.Fatal("Impact should not be nil")
	}
	if _, exists := impact["avg_response_time"]; !exists {
		t.Error("Impact should contain avg_response_time")
	}
	if _, exists := impact["avg_error_rate"]; !exists {
		t.Error("Impact should contain avg_error_rate")
	}
	if _, exists := impact["avg_throughput"]; !exists {
		t.Error("Impact should contain avg_throughput")
	}
	if impact["failure_count"] != 0 {
		t.Errorf("Expected failure_count 0, got %v", impact["failure_count"])
	}
	if impact["recovery_successful"] != true {
		t.Errorf("Expected recovery_successful true, got %v", impact["recovery_successful"])
	}

	// Test summary generation
	summary := framework.generateExperimentSummary(experiment, results)
	if summary == "" {
		t.Error("Summary should not be empty")
	}
	if !contains(summary, "Analysis Test") {
		t.Error("Summary should contain experiment name")
	}
	if !contains(summary, "Hypothesis validated") {
		t.Error("Summary should contain hypothesis validation")
	}
	if !contains(summary, "recovery completed successfully") {
		t.Error("Summary should contain recovery status")
	}

	// Test recommendations
	recommendations := framework.generateRecommendations(experiment, results)
	if len(recommendations) == 0 {
		t.Error("Recommendations should not be empty")
	}
	if !contains(recommendations[0], "good resilience") {
		t.Error("Should recommend good resilience")
	}
}

func TestChaosEngineeringFramework_Performance(t *testing.T) {
	config := &ChaosEngineeringConfig{
		Environment:              "test",
		MaxConcurrentExperiments: 10,
		MonitoringInterval:       10 * time.Millisecond,
		Security: SecurityConfig{
			RequireApproval: false,
		},
	}

	framework := NewChaosEngineeringFramework(config)

	// Performance test: Run multiple experiments
	numExperiments := 5
	experiments := make([]*ChaosExperiment, numExperiments)

	for i := 0; i < numExperiments; i++ {
		experiments[i] = &ChaosExperiment{
			ID:       fmt.Sprintf("perf-test-%d", i),
			Name:     fmt.Sprintf("Performance Test %d", i),
			Duration: 100 * time.Millisecond, // Very short for testing
			Faults: []FaultDefinition{
				{
					ID:       fmt.Sprintf("fault-%d", i),
					Type:     LatencyFault,
					Duration: 50 * time.Millisecond,
				},
			},
			Target: ExperimentTarget{
				Identifier: fmt.Sprintf("service-%d", i),
			},
		}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run experiments sequentially for this test
	for _, exp := range experiments {
		results, err := framework.RunExperiment(ctx, exp)
		if err != nil {
			t.Fatalf("RunExperiment failed: %v", err)
		}
		if results.Status != CompletedExperimentStatus {
			t.Errorf("Expected status CompletedExperimentStatus, got %s", results.Status)
		}
	}

	duration := time.Since(start)
	if duration >= 3*time.Second {
		t.Errorf("Performance test should complete in <3s, took %v", duration)
	}

	// Verify all experiments are stored
	if len(framework.experiments) != numExperiments {
		t.Errorf("Expected %d experiments stored, got %d", numExperiments, len(framework.experiments))
	}
}

func BenchmarkChaosExperiment_Validation(b *testing.B) {
	config := &ChaosEngineeringConfig{
		Environment: "benchmark",
		Security: SecurityConfig{
			RequireApproval: false,
		},
	}

	framework := NewChaosEngineeringFramework(config)

	experiment := &ChaosExperiment{
		ID:   "benchmark-exp",
		Name: "Benchmark Experiment",
		Faults: []FaultDefinition{
			{
				ID:   "benchmark-fault",
				Type: LatencyFault,
			},
		},
		Target: ExperimentTarget{
			Identifier: "benchmark-service",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := framework.validateExperiment(experiment)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFaultInjection(b *testing.B) {
	config := &NetworkFaultConfig{
		DefaultDelay: 50 * time.Millisecond,
	}

	injector := NewNetworkFaultInjector(config)
	ctx := context.Background()

	fault := FaultDefinition{
		ID:   "benchmark-fault",
		Type: LatencyFault,
		Parameters: map[string]interface{}{
			"delay": 100 * time.Millisecond,
		},
	}

	target := ExperimentTarget{
		Type:       NetworkTarget,
		Identifier: "benchmark-target",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := injector.Inject(ctx, fault, target)
		if err != nil {
			b.Fatal(err)
		}
		err = injector.Remove(ctx, fault, target)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResilienceScoreCalculation(b *testing.B) {
	results := &ExperimentResults{
		ExperimentID:    "benchmark-test",
		Status:          CompletedExperimentStatus,
		StartTime:       time.Now().Add(-15 * time.Minute),
		EndTime:         time.Now(),
		Duration:        15 * time.Minute,
		HypothesisValid: true,
		Observations:    []Observation{},
		Failures:        []ExperimentFailure{},
		Recovery: &RecoveryResults{
			Successful: true,
			Duration:   2 * time.Minute,
		},
		Metrics: map[string]interface{}{
			"mttr":         15 * time.Minute,
			"mtbf":         72 * time.Hour,
			"availability": 99.9,
			"error_budget": 85.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		score := CalculateResilienceScore(results)
		if score < 0 || score > 100 {
			b.Fatal("Invalid resilience score")
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
