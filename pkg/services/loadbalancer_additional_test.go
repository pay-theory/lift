package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultLoadBalancer_ResetAndNoopUpdateWeights(t *testing.T) {
	lb := NewDefaultLoadBalancer()

	instances := []*ServiceInstance{
		{ID: "i1", ServiceName: "svc"},
		{ID: "i2", ServiceName: "svc"},
	}

	// Advance the round robin counter.
	require.Equal(t, "i1", lb.Select(instances, RoundRobin).ID)
	require.Equal(t, "i2", lb.Select(instances, RoundRobin).ID)

	// Create a connection count to verify Reset clears it.
	require.NotNil(t, lb.Select(instances, LeastConnections))

	require.NoError(t, lb.UpdateWeights(instances))

	lb.Reset()

	// After reset, round robin starts over and stats are cleared.
	require.Equal(t, "i1", lb.Select(instances, RoundRobin).ID)
	stats := lb.GetStats()
	require.Equal(t, int64(1), stats.TotalRequests)
}

func TestHealthAwareLoadBalancer_FiltersByLastSeen(t *testing.T) {
	delegate := newFakeLoadBalancer(nil)
	delegate.stats = LoadBalancerStats{TotalRequests: 7}

	lb := NewHealthAwareLoadBalancer(delegate, 10*time.Second)

	now := time.Now()
	instances := []*ServiceInstance{
		{ID: "old", ServiceName: "svc", LastSeen: now.Add(-time.Hour)},
		{ID: "fresh", ServiceName: "svc", LastSeen: now},
	}

	selected := lb.Select(instances, RoundRobin)
	require.NotNil(t, selected)

	delegate.mu.Lock()
	require.Len(t, delegate.lastInsts, 1)
	require.Equal(t, "fresh", delegate.lastInsts[0].ID)
	delegate.mu.Unlock()

	// If no healthy instances, fall back to all instances.
	for _, inst := range instances {
		inst.LastSeen = now.Add(-time.Hour)
	}
	_ = lb.Select(instances, RoundRobin)

	delegate.mu.Lock()
	require.Len(t, delegate.lastInsts, 2)
	delegate.mu.Unlock()

	require.NoError(t, lb.UpdateWeights(instances))
	require.Equal(t, delegate.GetStats(), lb.GetStats())
}

func TestWeightedLoadBalancer_AppliesDynamicWeights(t *testing.T) {
	delegate := newFakeLoadBalancer(nil)
	wlb := NewWeightedLoadBalancer(delegate)

	instances := []*ServiceInstance{
		{ID: "i1", ServiceName: "svc", Weight: 1},
		{ID: "i2", ServiceName: "svc", Weight: 1},
	}

	wlb.SetWeight("i2", 99)
	require.Equal(t, 99, wlb.GetWeight("i2"))
	require.Equal(t, 1, wlb.GetWeight("missing"))

	_ = wlb.Select(instances, WeightedRandom)
	require.Equal(t, 99, instances[1].Weight)

	delegate.mu.Lock()
	require.Equal(t, WeightedRandom, delegate.lastStrat)
	delegate.mu.Unlock()

	require.NoError(t, wlb.UpdateWeights(instances))
	_ = wlb.GetStats()
}

func TestLoadBalancerConvenienceConstructors(t *testing.T) {
	require.NotNil(t, NewRoundRobinLoadBalancer())
	require.NotNil(t, NewWeightedRandomLoadBalancer())
	require.NotNil(t, NewLeastConnectionsLoadBalancer())
	require.NotNil(t, NewHealthyFirstLoadBalancer(5*time.Second))
}
