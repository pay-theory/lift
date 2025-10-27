package services

import (
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultLoadBalancerRoundRobinPerService(t *testing.T) {
	t.Parallel()

	lb := NewDefaultLoadBalancer()

	serviceA := []*ServiceInstance{
		{ID: "a-1", ServiceName: "svc-a"},
		{ID: "a-2", ServiceName: "svc-a"},
	}
	serviceB := []*ServiceInstance{
		{ID: "b-1", ServiceName: "svc-b"},
		{ID: "b-2", ServiceName: "svc-b"},
	}

	gotA := []string{
		lb.Select(serviceA, RoundRobin).ID,
		lb.Select(serviceA, RoundRobin).ID,
		lb.Select(serviceA, RoundRobin).ID,
	}
	wantA := []string{"a-1", "a-2", "a-1"}
	for i := range wantA {
		if gotA[i] != wantA[i] {
			t.Fatalf("round robin for service A mismatch at %d: want %s got %s", i, wantA[i], gotA[i])
		}
	}

	gotB := []string{
		lb.Select(serviceB, RoundRobin).ID,
		lb.Select(serviceB, RoundRobin).ID,
	}
	wantB := []string{"b-1", "b-2"}
	for i := range wantB {
		if gotB[i] != wantB[i] {
			t.Fatalf("round robin for service B mismatch at %d: want %s got %s", i, wantB[i], gotB[i])
		}
	}
}

func TestDefaultLoadBalancerWeightedRandomHonorsWeights(t *testing.T) {
	t.Parallel()

	lb := NewDefaultLoadBalancer()
	lb.rand = rand.New(rand.NewSource(1)) // deterministic

	instances := []*ServiceInstance{
		{ID: "low", ServiceName: "svc", Weight: 1},
		{ID: "high", ServiceName: "svc", Weight: 5},
	}

	const attempts = 100
	var highCount int
	for i := 0; i < attempts; i++ {
		selected := lb.Select(instances, WeightedRandom)
		if selected == nil {
			t.Fatal("expected non-nil selection")
		}
		if selected.ID == "high" {
			highCount++
		}
	}

	if highCount <= attempts/2 {
		t.Fatalf("expected weighted random to favor high weight, got %d/%d", highCount, attempts)
	}
}

func TestDefaultLoadBalancerLeastConnections(t *testing.T) {
	t.Parallel()

	lb := NewDefaultLoadBalancer()
	instances := []*ServiceInstance{
		{ID: "inst-1", ServiceName: "svc"},
		{ID: "inst-2", ServiceName: "svc"},
	}

	first := lb.Select(instances, LeastConnections)
	second := lb.Select(instances, LeastConnections)
	if first == nil || second == nil {
		t.Fatal("expected selections for least connections")
	}
	if first.ID == second.ID {
		t.Fatalf("expected different instances based on connection counts, got %s twice", first.ID)
	}

	lb.ReleaseConnection(first.ID)
	lb.ReleaseConnection(first.ID) // extra release should not underflow

	lb.mu.RLock()
	counter := lb.connectionCounts[first.ID]
	lb.mu.RUnlock()
	if counter == nil {
		t.Fatalf("expected connection count entry for %s", first.ID)
	}
	if count := atomic.LoadInt64(counter); count != 0 {
		t.Fatalf("expected connection count to return to zero, got %d", count)
	}
}

func TestDefaultLoadBalancerHealthyAndLocalFirst(t *testing.T) {
	t.Parallel()

	lb := NewDefaultLoadBalancer()
	instances := []*ServiceInstance{
		{ID: "healthy-1", ServiceName: "svc", Health: HealthStatus{Status: "healthy"}},
		{ID: "healthy-2", ServiceName: "svc", Health: HealthStatus{Status: "healthy"}},
		{ID: "unhealthy", ServiceName: "svc", Health: HealthStatus{Status: "unhealthy"}},
	}

	first := lb.Select(instances, HealthyFirst)
	second := lb.Select(instances, HealthyFirst)
	if first == nil || first.Health.Status != "healthy" {
		t.Fatalf("expected first selection to be healthy, got %+v", first)
	}
	if second == nil || second.Health.Status != "healthy" {
		t.Fatalf("expected second selection to be healthy, got %+v", second)
	}
	if first.ID == second.ID {
		t.Fatalf("expected healthy-first to rotate healthy instances, got %s twice", first.ID)
	}

	for _, inst := range instances {
		inst.Health.Status = "unhealthy"
	}
	fallback := lb.Select(instances, HealthyFirst)
	if fallback == nil {
		t.Fatal("expected fallback selection when all instances unhealthy")
	}

	localSequence := []string{
		lb.Select(instances, LocalFirst).ID,
		lb.Select(instances, LocalFirst).ID,
		lb.Select(instances, LocalFirst).ID,
	}
	expected := []string{"healthy-1", "healthy-2", "unhealthy"}
	for i := range expected {
		if localSequence[i] != expected[i] {
			t.Fatalf("expected local-first to follow round robin order, want %s got %s", expected[i], localSequence[i])
		}
	}
}

func TestDefaultLoadBalancerStats(t *testing.T) {
	t.Parallel()

	lb := NewDefaultLoadBalancer()
	lb.rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	instances := []*ServiceInstance{
		{ID: "inst", ServiceName: "svc"},
	}

	lb.Select(instances, RoundRobin)
	lb.Select(instances, RoundRobin)
	lb.Select(nil, RoundRobin)

	stats := lb.GetStats()
	if stats.TotalRequests != 3 {
		t.Fatalf("expected total requests 3, got %d", stats.TotalRequests)
	}
	if stats.SuccessfulSelections != 2 {
		t.Fatalf("expected successful selections 2, got %d", stats.SuccessfulSelections)
	}
	if stats.FailedSelections != 1 {
		t.Fatalf("expected failed selections 1, got %d", stats.FailedSelections)
	}
	if stats.AverageLatency < 0 {
		t.Fatalf("expected non-negative average latency, got %d", stats.AverageLatency)
	}
}
