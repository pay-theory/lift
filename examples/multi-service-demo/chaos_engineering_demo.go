package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pay-theory/lift/pkg/testing/enterprise"
)

func chaosEngineeringDemo() {
	fmt.Println("🔥 Chaos Engineering Framework Demo")
	fmt.Println("===================================")

	// Initialize chaos engineering framework
	config := &enterprise.ChaosEngineeringConfig{
		Environment:              "demo",
		SafetyMode:               true,
		MaxConcurrentExperiments: 3,
		DefaultTimeout:           10 * time.Minute,
		MonitoringInterval:       5 * time.Second,
		RetentionPeriod:          24 * time.Hour,
		Notifications: enterprise.NotificationConfig{
			Enabled: true,
		},
		Security: enterprise.SecurityConfig{
			RequireApproval:   false, // Disabled for demo
			ForbiddenTargets:  []string{"production-db", "payment-gateway"},
			MaxSeverity:       "high",
			AuditLogging:      true,
			EncryptionEnabled: true,
		},
	}

	framework := enterprise.NewChaosEngineeringFramework(config)

	// Demo 1: Network Latency Experiment
	fmt.Println("\n🌐 Demo 1: Network Latency Chaos Experiment")
	fmt.Println("--------------------------------------------")

	networkExperiment := createNetworkLatencyExperiment()
	runExperimentDemo(framework, networkExperiment)

	// Demo 2: Service Unavailability Experiment
	fmt.Println("\n🚫 Demo 2: Service Unavailability Chaos Experiment")
	fmt.Println("--------------------------------------------------")

	serviceExperiment := createServiceUnavailabilityExperiment()
	runExperimentDemo(framework, serviceExperiment)

	// Demo 3: Resource Exhaustion Experiment
	fmt.Println("\n💾 Demo 3: Resource Exhaustion Chaos Experiment")
	fmt.Println("-----------------------------------------------")

	resourceExperiment := createResourceExhaustionExperiment()
	runExperimentDemo(framework, resourceExperiment)

	// Demo 4: Resilience Metrics Analysis
	fmt.Println("\n📊 Demo 4: Resilience Metrics Analysis")
	fmt.Println("--------------------------------------")

	runResilienceMetricsDemo()

	fmt.Println("\n✅ Chaos Engineering Demo Complete!")
	fmt.Println("===================================")
}

func createNetworkLatencyExperiment() *enterprise.ChaosExperiment {
	return &enterprise.ChaosExperiment{
		ID:          "network-latency-demo-001",
		Name:        "API Gateway Network Latency Test",
		Description: "Test system resilience to increased network latency",
		Type:        enterprise.NetworkChaos,
		Target: enterprise.ExperimentTarget{
			Type:       enterprise.ServiceTarget,
			Identifier: "api-gateway",
			Scope:      enterprise.SingleInstanceScope,
		},
		Faults: []enterprise.FaultDefinition{
			{
				ID:          "latency-fault-001",
				Type:        enterprise.LatencyFault,
				Severity:    enterprise.MediumSeverity,
				Duration:    30 * time.Second,
				Probability: 1.0,
				Parameters: map[string]any{
					"delay": 200 * time.Millisecond,
				},
				Recovery: &enterprise.RecoveryConfig{
					Automatic:     true,
					Timeout:       10 * time.Second,
					RetryAttempts: 3,
					RetryDelay:    2 * time.Second,
					Rollback:      true,
				},
			},
		},
		Hypothesis: "API Gateway should maintain response time with graceful degradation",
		Duration:   45 * time.Second,
		Status:     enterprise.ExperimentPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func createServiceUnavailabilityExperiment() *enterprise.ChaosExperiment {
	return &enterprise.ChaosExperiment{
		ID:          "service-unavailable-demo-002",
		Name:        "User Service Unavailability Test",
		Description: "Test system behavior when user service becomes unavailable",
		Type:        enterprise.ServiceChaos,
		Target: enterprise.ExperimentTarget{
			Type:       enterprise.ServiceTarget,
			Identifier: "user-service",
			Scope:      enterprise.SingleInstanceScope,
		},
		Faults: []enterprise.FaultDefinition{
			{
				ID:          "unavailable-fault-001",
				Type:        enterprise.ServiceUnavailable,
				Severity:    enterprise.HighSeverity,
				Duration:    20 * time.Second,
				Probability: 1.0,
				Parameters: map[string]any{
					"status_code": 503,
					"message":     "Service Temporarily Unavailable",
				},
				Recovery: &enterprise.RecoveryConfig{
					Automatic:     true,
					Timeout:       15 * time.Second,
					RetryAttempts: 2,
					RetryDelay:    3 * time.Second,
					Rollback:      true,
				},
			},
		},
		Hypothesis: "System should handle service unavailability with circuit breaker",
		Duration:   35 * time.Second,
		Status:     enterprise.ExperimentPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func createResourceExhaustionExperiment() *enterprise.ChaosExperiment {
	return &enterprise.ChaosExperiment{
		ID:          "resource-exhaustion-demo-003",
		Name:        "Database CPU Exhaustion Test",
		Description: "Test system behavior under database CPU exhaustion",
		Type:        enterprise.ResourceChaos,
		Target: enterprise.ExperimentTarget{
			Type:       enterprise.DatabaseTarget,
			Identifier: "demo-database",
			Scope:      enterprise.SingleInstanceScope,
		},
		Faults: []enterprise.FaultDefinition{
			{
				ID:          "cpu-exhaustion-fault-001",
				Type:        enterprise.ResourceExhaustion,
				Severity:    enterprise.HighSeverity,
				Duration:    25 * time.Second,
				Probability: 1.0,
				Parameters: map[string]any{
					"resource_type": "cpu",
					"percentage":    85.0,
				},
				Recovery: &enterprise.RecoveryConfig{
					Automatic:     true,
					Timeout:       20 * time.Second,
					RetryAttempts: 3,
					RetryDelay:    5 * time.Second,
					Rollback:      true,
				},
			},
		},
		Hypothesis: "System should maintain database performance within limits",
		Duration:   40 * time.Second,
		Status:     enterprise.ExperimentPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func runExperimentDemo(framework *enterprise.ChaosEngineeringFramework, experiment *enterprise.ChaosExperiment) {
	fmt.Printf("🔬 Running Experiment: %s\n", experiment.Name)
	fmt.Printf("   ID: %s\n", experiment.ID)
	fmt.Printf("   Type: %s\n", experiment.Type)
	fmt.Printf("   Target: %s (%s)\n", experiment.Target.Identifier, experiment.Target.Type)
	fmt.Printf("   Hypothesis: %s\n", experiment.Hypothesis)
	fmt.Printf("   Duration: %v\n", experiment.Duration)
	fmt.Printf("   Faults: %d\n", len(experiment.Faults))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), experiment.Duration+30*time.Second)
	defer cancel()

	// Run the experiment
	fmt.Println("   Status: Starting experiment...")
	start := time.Now()

	results, err := framework.RunExperiment(ctx, experiment)
	if err != nil {
		log.Printf("❌ Experiment failed: %v", err)
		return
	}

	duration := time.Since(start)
	fmt.Printf("   Status: Completed in %v\n", duration)

	// Display results
	fmt.Println("\n📊 Experiment Results:")
	fmt.Printf("   Status: %s\n", results.Status)
	fmt.Printf("   Duration: %v\n", results.Duration)
	fmt.Printf("   Hypothesis Valid: %t\n", results.HypothesisValid)
	fmt.Printf("   Observations: %d\n", len(results.Observations))
	fmt.Printf("   Failures: %d\n", len(results.Failures))

	if results.Recovery != nil {
		fmt.Printf("   Recovery: %s (Successful: %t)\n", results.Recovery.Method, results.Recovery.Successful)
		fmt.Printf("   Recovery Duration: %v\n", results.Recovery.Duration)
	}

	// Display summary and recommendations
	fmt.Printf("\n📝 Summary: %s\n", results.Summary)

	if len(results.Recommendations) > 0 {
		fmt.Println("\n💡 Recommendations:")
		for i, rec := range results.Recommendations {
			fmt.Printf("   %d. %s\n", i+1, rec)
		}
	}

	// Display key metrics
	if impact, ok := results.Metrics["impact"].(map[string]any); ok {
		fmt.Println("\n📈 Impact Metrics:")
		for key, value := range impact {
			fmt.Printf("   %s: %v\n", key, value)
		}
	}

	fmt.Println()
}

func runResilienceMetricsDemo() {
	// Simulate current system metrics
	currentMetrics := &enterprise.ResilienceMetrics{
		MTTR:            12 * time.Minute,
		MTBF:            168 * time.Hour, // 1 week
		Availability:    99.95,
		ErrorBudget:     92.5,
		LastIncident:    time.Now().Add(-72 * time.Hour),
		IncidentCount:   2,
		ResilienceScore: 0, // Will be calculated
		Trends: map[string]any{
			"mttr_trend":         "improving",
			"availability_trend": "stable",
			"incident_frequency": "decreasing",
		},
		ExperimentCount: 15,
		LastUpdated:     time.Now(),
	}

	// Create mock experiment results for score calculation
	mockResults := &enterprise.ExperimentResults{
		ExperimentID:    "resilience-assessment",
		Status:          enterprise.CompletedExperimentStatus,
		HypothesisValid: true,
		Failures:        []enterprise.ExperimentFailure{},
		Recovery: &enterprise.RecoveryResults{
			Successful: true,
			Duration:   currentMetrics.MTTR,
		},
	}

	// Calculate resilience score
	currentMetrics.ResilienceScore = enterprise.CalculateResilienceScore(mockResults)

	fmt.Printf("📊 Current System Resilience Metrics\n")
	fmt.Printf("   MTTR (Mean Time To Recovery): %v\n", currentMetrics.MTTR)
	fmt.Printf("   MTBF (Mean Time Between Failures): %v\n", currentMetrics.MTBF)
	fmt.Printf("   Availability: %.2f%%\n", currentMetrics.Availability)
	fmt.Printf("   Error Budget Remaining: %.1f%%\n", currentMetrics.ErrorBudget)
	fmt.Printf("   Experiment Count: %d\n", currentMetrics.ExperimentCount)
	fmt.Printf("   Incident Count (Last 30 days): %d\n", currentMetrics.IncidentCount)
	fmt.Printf("   Overall Resilience Score: %.1f/100\n", currentMetrics.ResilienceScore)
	fmt.Printf("   Last Incident: %v ago\n", time.Since(currentMetrics.LastIncident).Round(time.Hour))

	fmt.Println("\n📈 Trends:")
	for key, value := range currentMetrics.Trends {
		fmt.Printf("   %s: %v\n", key, value)
	}

	// Test blast radius with a mock experiment
	mockExperiment := &enterprise.ChaosExperiment{
		ID:   "blast-radius-demo",
		Type: enterprise.ServiceChaos,
		Target: enterprise.ExperimentTarget{
			Type:       enterprise.ServiceTarget,
			Name:       "payment-service",
			Identifier: "payment-service",
			Scope:      enterprise.ClusterScope,
		},
		Duration: 30 * time.Second,
	}

	blastRadius := enterprise.GenerateBlastRadius(mockExperiment)

	fmt.Println("\n💥 Blast Radius Assessment:")
	fmt.Printf("   Scope: %s\n", blastRadius.Scope)
	fmt.Printf("   Severity: %s\n", blastRadius.Severity)

	fmt.Println("\n📊 Impact Analysis:")
	for key, value := range blastRadius.Impact {
		fmt.Printf("   %s: %v\n", key, value)
	}
}
