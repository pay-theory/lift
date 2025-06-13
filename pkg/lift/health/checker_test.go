package health

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestHealthManager_BasicOperations(t *testing.T) {
	config := DefaultHealthManagerConfig()
	config.CacheEnabled = false // Disable cache for testing
	manager := NewHealthManager(config)

	// Test empty manager
	checkers := manager.ListCheckers()
	if len(checkers) != 0 {
		t.Errorf("expected 0 checkers, got %d", len(checkers))
	}

	// Register a checker
	healthyChecker := NewAlwaysHealthyChecker("test-healthy")
	err := manager.RegisterChecker("test-healthy", healthyChecker)
	if err != nil {
		t.Fatalf("failed to register checker: %v", err)
	}

	// Check it was registered
	checkers = manager.ListCheckers()
	if len(checkers) != 1 {
		t.Errorf("expected 1 checker, got %d", len(checkers))
	}

	if checkers[0] != "test-healthy" {
		t.Errorf("expected checker name 'test-healthy', got %s", checkers[0])
	}

	// Test duplicate registration
	err = manager.RegisterChecker("test-healthy", healthyChecker)
	if err == nil {
		t.Error("expected error when registering duplicate checker")
	}

	// Unregister checker
	err = manager.UnregisterChecker("test-healthy")
	if err != nil {
		t.Fatalf("failed to unregister checker: %v", err)
	}

	// Check it was unregistered
	checkers = manager.ListCheckers()
	if len(checkers) != 0 {
		t.Errorf("expected 0 checkers after unregister, got %d", len(checkers))
	}

	// Test unregistering non-existent checker
	err = manager.UnregisterChecker("non-existent")
	if err == nil {
		t.Error("expected error when unregistering non-existent checker")
	}
}

func TestHealthManager_CheckComponent(t *testing.T) {
	manager := NewHealthManager(DefaultHealthManagerConfig())

	// Register checkers
	healthyChecker := NewAlwaysHealthyChecker("healthy")
	unhealthyChecker := NewAlwaysUnhealthyChecker("unhealthy")

	manager.RegisterChecker("healthy", healthyChecker)
	manager.RegisterChecker("unhealthy", unhealthyChecker)

	ctx := context.Background()

	// Test healthy checker
	status, err := manager.CheckComponent(ctx, "healthy")
	if err != nil {
		t.Fatalf("failed to check healthy component: %v", err)
	}

	if status.Status != StatusHealthy {
		t.Errorf("expected healthy status, got %s", status.Status)
	}

	// Test unhealthy checker
	status, err = manager.CheckComponent(ctx, "unhealthy")
	if err != nil {
		t.Fatalf("failed to check unhealthy component: %v", err)
	}

	if status.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy status, got %s", status.Status)
	}

	// Test non-existent checker
	_, err = manager.CheckComponent(ctx, "non-existent")
	if err == nil {
		t.Error("expected error when checking non-existent component")
	}
}

func TestHealthManager_CheckAll(t *testing.T) {
	config := DefaultHealthManagerConfig()
	config.ParallelChecks = false // Test sequential first
	manager := NewHealthManager(config)

	// Register multiple checkers
	manager.RegisterChecker("healthy1", NewAlwaysHealthyChecker("healthy1"))
	manager.RegisterChecker("healthy2", NewAlwaysHealthyChecker("healthy2"))
	manager.RegisterChecker("unhealthy1", NewAlwaysUnhealthyChecker("unhealthy1"))

	ctx := context.Background()
	results := manager.CheckAll(ctx)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Check individual results
	if results["healthy1"].Status != StatusHealthy {
		t.Errorf("expected healthy1 to be healthy, got %s", results["healthy1"].Status)
	}

	if results["healthy2"].Status != StatusHealthy {
		t.Errorf("expected healthy2 to be healthy, got %s", results["healthy2"].Status)
	}

	if results["unhealthy1"].Status != StatusUnhealthy {
		t.Errorf("expected unhealthy1 to be unhealthy, got %s", results["unhealthy1"].Status)
	}
}

func TestHealthManager_CheckAllParallel(t *testing.T) {
	config := DefaultHealthManagerConfig()
	config.ParallelChecks = true
	manager := NewHealthManager(config)

	// Register checkers with delays to test parallelism
	slowChecker := NewCustomHealthChecker("slow", func(ctx context.Context) HealthStatus {
		time.Sleep(100 * time.Millisecond)
		return HealthStatus{
			Status:    StatusHealthy,
			Timestamp: time.Now(),
			Duration:  100 * time.Millisecond,
			Message:   "Slow but healthy",
		}
	})

	manager.RegisterChecker("slow1", slowChecker)
	manager.RegisterChecker("slow2", slowChecker)
	manager.RegisterChecker("slow3", slowChecker)

	ctx := context.Background()
	start := time.Now()
	results := manager.CheckAll(ctx)
	duration := time.Since(start)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// With parallel execution, total time should be ~100ms, not ~300ms
	if duration > 200*time.Millisecond {
		t.Errorf("parallel execution took too long: %v (expected ~100ms)", duration)
	}
}

func TestHealthManager_OverallHealth(t *testing.T) {
	manager := NewHealthManager(DefaultHealthManagerConfig())
	ctx := context.Background()

	// Test with no checkers
	overall := manager.OverallHealth(ctx)
	if overall.Status != StatusUnknown {
		t.Errorf("expected unknown status with no checkers, got %s", overall.Status)
	}

	// Test with all healthy
	manager.RegisterChecker("healthy1", NewAlwaysHealthyChecker("healthy1"))
	manager.RegisterChecker("healthy2", NewAlwaysHealthyChecker("healthy2"))

	overall = manager.OverallHealth(ctx)
	if overall.Status != StatusHealthy {
		t.Errorf("expected healthy status with all healthy checkers, got %s", overall.Status)
	}

	// Test with one unhealthy
	manager.RegisterChecker("unhealthy1", NewAlwaysUnhealthyChecker("unhealthy1"))

	overall = manager.OverallHealth(ctx)
	if overall.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy status with one unhealthy checker, got %s", overall.Status)
	}

	// Remove unhealthy, add degraded
	manager.UnregisterChecker("unhealthy1")
	degradedChecker := NewCustomHealthChecker("degraded", func(ctx context.Context) HealthStatus {
		return HealthStatus{
			Status:    StatusDegraded,
			Timestamp: time.Now(),
			Duration:  time.Microsecond,
			Message:   "Degraded service",
		}
	})
	manager.RegisterChecker("degraded1", degradedChecker)

	overall = manager.OverallHealth(ctx)
	if overall.Status != StatusDegraded {
		t.Errorf("expected degraded status with degraded checker, got %s", overall.Status)
	}
}

func TestHealthManager_Timeout(t *testing.T) {
	config := DefaultHealthManagerConfig()
	config.Timeout = 50 * time.Millisecond
	manager := NewHealthManager(config)

	// Create a slow checker that will timeout
	slowChecker := NewCustomHealthChecker("slow", func(ctx context.Context) HealthStatus {
		time.Sleep(100 * time.Millisecond) // Longer than timeout
		return HealthStatus{
			Status:    StatusHealthy,
			Timestamp: time.Now(),
			Duration:  100 * time.Millisecond,
			Message:   "Should not reach here",
		}
	})

	manager.RegisterChecker("slow", slowChecker)

	ctx := context.Background()
	status, err := manager.CheckComponent(ctx, "slow")
	if err != nil {
		t.Fatalf("failed to check component: %v", err)
	}

	if status.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy status due to timeout, got %s", status.Status)
	}

	if status.Error == "" {
		t.Error("expected error message for timeout")
	}
}

func TestHealthManager_Cache(t *testing.T) {
	config := DefaultHealthManagerConfig()
	config.CacheEnabled = true
	config.CacheDuration = 100 * time.Millisecond
	manager := NewHealthManager(config)

	callCount := 0
	countingChecker := NewCustomHealthChecker("counting", func(ctx context.Context) HealthStatus {
		callCount++
		return HealthStatus{
			Status:    StatusHealthy,
			Timestamp: time.Now(),
			Duration:  time.Microsecond,
			Message:   "Healthy",
		}
	})

	manager.RegisterChecker("counting", countingChecker)

	ctx := context.Background()

	// First call should execute the checker
	manager.CheckComponent(ctx, "counting")
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}

	// Second call should use cache
	manager.CheckComponent(ctx, "counting")
	if callCount != 1 {
		t.Errorf("expected 1 call (cached), got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call should execute the checker again
	manager.CheckComponent(ctx, "counting")
	if callCount != 2 {
		t.Errorf("expected 2 calls (cache expired), got %d", callCount)
	}
}

func TestHealthManager_PanicRecovery(t *testing.T) {
	manager := NewHealthManager(DefaultHealthManagerConfig())

	panicChecker := NewCustomHealthChecker("panic", func(ctx context.Context) HealthStatus {
		panic("test panic")
	})

	manager.RegisterChecker("panic", panicChecker)

	ctx := context.Background()
	status, err := manager.CheckComponent(ctx, "panic")
	if err != nil {
		t.Fatalf("failed to check component: %v", err)
	}

	if status.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy status due to panic, got %s", status.Status)
	}

	if status.Error == "" {
		t.Error("expected error message for panic")
	}
}

func TestBuiltInCheckers(t *testing.T) {
	t.Run("AlwaysHealthyChecker", func(t *testing.T) {
		checker := NewAlwaysHealthyChecker("test")
		ctx := context.Background()

		status := checker.Check(ctx)
		if status.Status != StatusHealthy {
			t.Errorf("expected healthy status, got %s", status.Status)
		}

		if checker.Name() != "test" {
			t.Errorf("expected name 'test', got %s", checker.Name())
		}
	})

	t.Run("AlwaysUnhealthyChecker", func(t *testing.T) {
		checker := NewAlwaysUnhealthyChecker("test")
		ctx := context.Background()

		status := checker.Check(ctx)
		if status.Status != StatusUnhealthy {
			t.Errorf("expected unhealthy status, got %s", status.Status)
		}

		if status.Error == "" {
			t.Error("expected error message")
		}
	})

	t.Run("MemoryHealthChecker", func(t *testing.T) {
		checker := NewMemoryHealthChecker("memory")
		ctx := context.Background()

		status := checker.Check(ctx)
		// Should be healthy for normal test runs
		if status.Status != StatusHealthy {
			t.Errorf("expected healthy status, got %s", status.Status)
		}

		if status.Details == nil {
			t.Error("expected memory details")
		}

		// Check that memory stats are included
		if _, ok := status.Details["alloc"]; !ok {
			t.Error("expected alloc in details")
		}
	})

	t.Run("CustomHealthChecker", func(t *testing.T) {
		customFn := func(ctx context.Context) HealthStatus {
			return HealthStatus{
				Status:    StatusDegraded,
				Timestamp: time.Now(),
				Duration:  time.Microsecond,
				Message:   "Custom check",
			}
		}

		checker := NewCustomHealthChecker("custom", customFn)
		ctx := context.Background()

		status := checker.Check(ctx)
		if status.Status != StatusDegraded {
			t.Errorf("expected degraded status, got %s", status.Status)
		}

		if status.Message != "Custom check" {
			t.Errorf("expected 'Custom check' message, got %s", status.Message)
		}
	})
}

func BenchmarkHealthManager_CheckAll(b *testing.B) {
	manager := NewHealthManager(DefaultHealthManagerConfig())

	// Register multiple fast checkers
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("checker-%d", i)
		manager.RegisterChecker(name, NewAlwaysHealthyChecker(name))
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CheckAll(ctx)
	}
}

func BenchmarkHealthManager_CheckComponent(b *testing.B) {
	manager := NewHealthManager(DefaultHealthManagerConfig())
	manager.RegisterChecker("test", NewAlwaysHealthyChecker("test"))

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CheckComponent(ctx, "test")
	}
}

func BenchmarkHealthManager_OverallHealth(b *testing.B) {
	manager := NewHealthManager(DefaultHealthManagerConfig())

	// Register multiple checkers
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker-%d", i)
		manager.RegisterChecker(name, NewAlwaysHealthyChecker(name))
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.OverallHealth(ctx)
	}
}
