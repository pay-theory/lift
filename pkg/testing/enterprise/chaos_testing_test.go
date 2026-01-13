package enterprise

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestChaosEngineeringTester_HealthAndFailures(t *testing.T) {
	tester := NewChaosEngineeringTester(nil)

	healthy, err := tester.CheckSystemHealth(context.Background())
	if err != nil || !healthy {
		t.Fatalf("CheckSystemHealth healthy=%v err=%v", healthy, err)
	}
	if _, ok := tester.metrics["health_check_timestamp"]; !ok {
		t.Fatalf("expected health_check_timestamp metric")
	}

	tester.safetyChecks = false
	if err := tester.InjectDatabaseFailure(context.Background(), "connection_timeout", 0); err == nil {
		t.Fatalf("InjectDatabaseFailure expected error when safety checks disabled")
	}

	tester.safetyChecks = true
	if err := tester.InjectDatabaseFailure(context.Background(), "connection_timeout", 0); err != nil {
		t.Fatalf("InjectDatabaseFailure(connection_timeout) error: %v", err)
	}
	if err := tester.InjectDatabaseFailure(context.Background(), "connection_refused", 0); err != nil {
		t.Fatalf("InjectDatabaseFailure(connection_refused) error: %v", err)
	}
	if err := tester.InjectDatabaseFailure(context.Background(), "slow_query", 0); err != nil {
		t.Fatalf("InjectDatabaseFailure(slow_query) error: %v", err)
	}
	if err := tester.InjectDatabaseFailure(context.Background(), "unknown", 0); err == nil {
		t.Fatalf("InjectDatabaseFailure(unknown) expected error")
	}
}

func TestChaosEngineeringTester_CPUAndMemoryAndLatency(t *testing.T) {
	tester := NewChaosEngineeringTester(nil)

	if err := tester.InjectCPUSpike(context.Background(), 96, 0); err == nil {
		t.Fatalf("InjectCPUSpike expected safety error")
	}

	tester.safetyChecks = false
	numCPU := runtime.NumCPU()
	percent := (100 + numCPU - 1) / numCPU // ceil(100/numCPU)
	if percent == 0 {
		percent = 1
	}
	if err := tester.InjectCPUSpike(context.Background(), percent, 2*time.Millisecond); err != nil {
		t.Fatalf("InjectCPUSpike error: %v", err)
	}

	tester.safetyChecks = true
	if err := tester.InjectMemoryPressure(context.Background(), 91, 0); err == nil {
		t.Fatalf("InjectMemoryPressure expected safety error")
	}
	if err := tester.InjectMemoryPressure(context.Background(), 1, 15*time.Millisecond); err != nil {
		t.Fatalf("InjectMemoryPressure error: %v", err)
	}

	if err := tester.InjectAPILatency(context.Background(), 10001, 100, 0); err == nil {
		t.Fatalf("InjectAPILatency expected safety error")
	}
	if err := tester.InjectAPILatency(context.Background(), 0, 100, 25*time.Millisecond); err != nil {
		t.Fatalf("InjectAPILatency error: %v", err)
	}
}

func TestChaosEngineeringTester_logChaosEvent_MonitoringToggle(t *testing.T) {
	tester := NewChaosEngineeringTester(nil)

	before := len(tester.metrics)
	tester.logChaosEvent("evt", "started", map[string]any{"k": "v"})
	if len(tester.metrics) <= before {
		t.Fatalf("expected logChaosEvent to store metric")
	}

	tester.monitoringEnabled = false
	before = len(tester.metrics)
	tester.logChaosEvent("evt", "stopped", map[string]any{"k": "v"})
	if len(tester.metrics) != before {
		t.Fatalf("expected no metric write when monitoring disabled")
	}
}
