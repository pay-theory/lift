package services

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServiceRegistryRegisterHappyPath(t *testing.T) {
	t.Parallel()

	discovery := newFakeDiscovery()
	loadBalancer := newFakeLoadBalancer(nil)

	registry := NewServiceRegistry(
		RegistryConfig{EnableMetrics: false},
		discovery,
		loadBalancer,
	)

	cfg := &ServiceConfig{
		Name:    "payments",
		Version: "v1",
		Endpoints: []ServiceEndpoint{
			{Host: "localhost", Port: 8080, Protocol: "http"},
		},
	}

	if err := registry.Register(context.Background(), cfg); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, ok := registry.GetService("payments"); !ok {
		t.Fatal("expected service to be stored in local registry")
	}

	if got := discovery.registerCount("payments"); got != 1 {
		t.Fatalf("expected discovery.Register called once, got %d", got)
	}
}

func TestServiceRegistryRegisterValidationErrors(t *testing.T) {
	t.Parallel()

	validEndpoint := ServiceEndpoint{Host: "localhost", Port: 8080, Protocol: "http"}
	tests := []struct {
		name      string
		config    *ServiceConfig
		registry  RegistryConfig
		wantError string
	}{
		{
			name: "missing name",
			config: &ServiceConfig{
				Version:   "v1",
				Endpoints: []ServiceEndpoint{validEndpoint},
			},
			wantError: "service name is required",
		},
		{
			name: "missing version",
			config: &ServiceConfig{
				Name:      "orders",
				Endpoints: []ServiceEndpoint{validEndpoint},
			},
			wantError: "service version is required",
		},
		{
			name: "invalid port",
			config: &ServiceConfig{
				Name:    "orders",
				Version: "v1",
				Endpoints: []ServiceEndpoint{
					{Host: "localhost", Port: 70000, Protocol: "http"},
				},
			},
			wantError: "invalid port 70000",
		},
		{
			name: "tenant isolation without tenant id",
			config: &ServiceConfig{
				Name:      "orders",
				Version:   "v1",
				Endpoints: []ServiceEndpoint{validEndpoint},
			},
			registry:  RegistryConfig{TenantIsolation: true},
			wantError: "tenant ID is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			discovery := newFakeDiscovery()
			registry := NewServiceRegistry(tt.registry, discovery, newFakeLoadBalancer(nil))

			err := registry.Register(context.Background(), tt.config)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error to contain %q, got %q", tt.wantError, err.Error())
			}

			if got := discovery.registerCount(tt.config.Name); got != 0 {
				t.Fatalf("expected discovery.Register not called, got %d calls", got)
			}
		})
	}
}

func TestServiceRegistryDiscoverUsesCacheAndLoadBalancer(t *testing.T) {
	t.Parallel()

	instances := []*ServiceInstance{
		{
			ID:          "inst-1",
			ServiceName: "registry-test",
			Health:      HealthStatus{Status: "healthy"},
			Version:     "v1",
		},
		{
			ID:          "inst-2",
			ServiceName: "registry-test",
			Health:      HealthStatus{Status: "healthy"},
			Version:     "v1",
		},
	}

	discovery := newFakeDiscovery()
	discovery.setDiscoverResponse("registry-test", instances)

	cache := newFakeCache()
	loadBalancer := newFakeLoadBalancer(func(instances []*ServiceInstance, strategy LoadBalanceStrategy) *ServiceInstance {
		if len(instances) < 2 {
			t.Fatalf("expected at least two instances, got %d", len(instances))
		}
		return instances[1]
	})

	registry := NewServiceRegistry(
		RegistryConfig{
			EnableCaching: true,
			EnableMetrics: false,
		},
		discovery,
		loadBalancer,
	)
	registry.cache = cache

	opts := DiscoveryOptions{Strategy: WeightedRandom}

	first, err := registry.Discover(context.Background(), "registry-test", opts)
	if err != nil {
		t.Fatalf("Discover() first call error = %v", err)
	}

	if first == nil || first.ID != "inst-2" {
		t.Fatalf("expected load balancer to select inst-2, got %+v", first)
	}

	second, err := registry.Discover(context.Background(), "registry-test", opts)
	if err != nil {
		t.Fatalf("Discover() second call error = %v", err)
	}

	if second == nil || second.ID != "inst-2" {
		t.Fatalf("expected cached load balancer selection inst-2, got %+v", second)
	}

	if got := discovery.discoverCount("registry-test"); got != 1 {
		t.Fatalf("expected discovery.Discover called once, got %d", got)
	}

	if cache.getCalls() != 2 {
		t.Fatalf("expected cache.Get called twice, got %d", cache.getCalls())
	}

	if cache.setCalls() != 1 {
		t.Fatalf("expected cache.Set called once, got %d", cache.setCalls())
	}

	if loadBalancer.lastStrategy() != WeightedRandom {
		t.Fatalf("expected load balancer strategy WeightedRandom, got %s", loadBalancer.lastStrategy())
	}
}

func TestServiceRegistryDiscoverNoInstancesError(t *testing.T) {
	t.Parallel()

	discovery := newFakeDiscovery()
	discovery.setDiscoverResponse("registry-test", []*ServiceInstance{
		{
			ID:          "inst-1",
			ServiceName: "registry-test",
			Version:     "v1",
			Health:      HealthStatus{Status: "unhealthy"},
		},
	})

	registry := NewServiceRegistry(
		RegistryConfig{EnableMetrics: false},
		discovery,
		newFakeLoadBalancer(nil),
	)

	_, err := registry.Discover(
		context.Background(),
		"registry-test",
		DiscoveryOptions{
			Version:          "v2",
			IncludeUnhealthy: false,
		},
	)

	if err == nil {
		t.Fatal("expected error when no suitable instances are found")
	}

	if !strings.Contains(err.Error(), "no suitable instances") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceRegistryDiscoverAppliesFilters(t *testing.T) {
	t.Parallel()

	discovery := newFakeDiscovery()
	loadBalancer := newFakeLoadBalancer(nil)

	instances := []*ServiceInstance{
		{
			ID:          "healthy-primary",
			ServiceName: "registry-test",
			Version:     "v1",
			TenantID:    "tenant-1",
			Metadata:    map[string]string{"tags": "primary"},
			Health:      HealthStatus{Status: "healthy"},
		},
		{
			ID:          "unhealthy-secondary",
			ServiceName: "registry-test",
			Version:     "v1",
			TenantID:    "tenant-1",
			Metadata:    map[string]string{"tags": "secondary"},
			Health:      HealthStatus{Status: "unhealthy"},
		},
		{
			ID:          "other-tenant",
			ServiceName: "registry-test",
			Version:     "v1",
			TenantID:    "tenant-2",
			Metadata:    map[string]string{"tags": "primary"},
			Health:      HealthStatus{Status: "healthy"},
		},
	}
	discovery.setDiscoverResponse("registry-test", instances)

	registry := NewServiceRegistry(
		RegistryConfig{
			TenantIsolation: true,
			EnableMetrics:   false,
		},
		discovery,
		loadBalancer,
	)

	opts := DiscoveryOptions{
		TenantID:         "tenant-1",
		Version:          "v1",
		Tags:             []string{"primary"},
		IncludeUnhealthy: false,
		MaxInstances:     1,
		Strategy:         RoundRobin,
	}

	selected, err := registry.Discover(context.Background(), "registry-test", opts)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if selected == nil || selected.ID != "healthy-primary" {
		t.Fatalf("expected healthy primary instance, got %+v", selected)
	}

	if got := len(loadBalancer.lastInstances()); got != 1 {
		t.Fatalf("expected load balancer to receive 1 filtered instance, got %d", got)
	}

	opts.IncludeUnhealthy = true
	opts.Tags = []string{"secondary"}

	selected, err = registry.Discover(context.Background(), "registry-test", opts)
	if err != nil {
		t.Fatalf("Discover() with IncludeUnhealthy error = %v", err)
	}

	if selected == nil || selected.ID != "unhealthy-secondary" {
		t.Fatalf("expected secondary instance when unhealthy allowed, got %+v", selected)
	}
}

func TestServiceRegistryDeregister(t *testing.T) {
	t.Parallel()

	discovery := newFakeDiscovery()
	cache := newFakeCache()
	loadBalancer := newFakeLoadBalancer(nil)

	registry := NewServiceRegistry(
		RegistryConfig{
			EnableCaching: true,
			EnableMetrics: false,
		},
		discovery,
		loadBalancer,
	)
	registry.cache = cache
	registry.services["orders"] = &ServiceConfig{Name: "orders"}

	if err := registry.Deregister(context.Background(), "orders"); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	if discovery.deregisterCount("orders") != 1 {
		t.Fatalf("expected discovery.Deregister called once, got %d", discovery.deregisterCount("orders"))
	}

	if cache.clearCalls() != 1 {
		t.Fatalf("expected cache.Clear called once, got %d", cache.clearCalls())
	}

	if _, exists := registry.services["orders"]; exists {
		t.Fatal("expected service removed from local registry")
	}
}

func TestServiceRegistryGetStats(t *testing.T) {
	t.Parallel()

	cache := newFakeCache()
	cache.setStats(CacheStats{Hits: 5, Size: 2})

	loadBalancer := newFakeLoadBalancer(nil)
	loadBalancer.setStats(LoadBalancerStats{TotalRequests: 7})

	discovery := newFakeDiscovery()

	registry := NewServiceRegistry(
		RegistryConfig{EnableMetrics: false},
		discovery,
		loadBalancer,
	)
	registry.cache = cache
	registry.services["svc"] = &ServiceConfig{Name: "svc"}

	stats := registry.GetStats()

	if stats.RegisteredServices != 1 {
		t.Fatalf("expected 1 registered service, got %d", stats.RegisteredServices)
	}

	if stats.CacheStats.Hits != 5 {
		t.Fatalf("expected cache hits 5, got %d", stats.CacheStats.Hits)
	}

	if stats.LoadBalancerStats.TotalRequests != 7 {
		t.Fatalf("expected load balancer total requests 7, got %d", stats.LoadBalancerStats.TotalRequests)
	}
}

type fakeDiscovery struct {
	mu            sync.Mutex
	registered    map[string]int
	deregistered  map[string]int
	discoverCalls map[string]int
	responses     map[string][]*ServiceInstance
	registerErr   error
	deregisterErr error
	discoverErr   error
	watchCh       chan []*ServiceInstance
	healthStatus  *HealthStatus
	healthErr     error
}

func newFakeDiscovery() *fakeDiscovery {
	return &fakeDiscovery{
		registered:    make(map[string]int),
		deregistered:  make(map[string]int),
		discoverCalls: make(map[string]int),
		responses:     make(map[string][]*ServiceInstance),
	}
}

func (f *fakeDiscovery) Register(_ context.Context, cfg *ServiceConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.registerErr != nil {
		return f.registerErr
	}
	f.registered[cfg.Name]++
	return nil
}

func (f *fakeDiscovery) Deregister(_ context.Context, serviceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deregisterErr != nil {
		return f.deregisterErr
	}
	f.deregistered[serviceID]++
	return nil
}

func (f *fakeDiscovery) Discover(_ context.Context, serviceName string) ([]*ServiceInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.discoverErr != nil {
		return nil, f.discoverErr
	}

	f.discoverCalls[serviceName]++

	instances := f.responses[serviceName]
	cloned := make([]*ServiceInstance, len(instances))
	for i, inst := range instances {
		cloned[i] = cloneInstance(inst)
	}
	return cloned, nil
}

func (f *fakeDiscovery) Watch(_ context.Context, _ string) (<-chan []*ServiceInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watchCh, nil
}

func (f *fakeDiscovery) HealthCheck(_ context.Context, _ *ServiceInstance) (*HealthStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.healthErr != nil {
		return nil, f.healthErr
	}
	if f.healthStatus == nil {
		return nil, nil
	}
	status := *f.healthStatus
	return &status, nil
}

func (f *fakeDiscovery) setDiscoverResponse(service string, instances []*ServiceInstance) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[service] = instances
}

func (f *fakeDiscovery) registerCount(service string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registered[service]
}

func (f *fakeDiscovery) discoverCount(service string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.discoverCalls[service]
}

func (f *fakeDiscovery) deregisterCount(service string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deregistered[service]
}

type fakeLoadBalancer struct {
	mu        sync.Mutex
	selectFn  func([]*ServiceInstance, LoadBalanceStrategy) *ServiceInstance
	lastStrat LoadBalanceStrategy
	lastInsts []*ServiceInstance
	selects   int
	stats     LoadBalancerStats
}

func newFakeLoadBalancer(selectFn func([]*ServiceInstance, LoadBalanceStrategy) *ServiceInstance) *fakeLoadBalancer {
	return &fakeLoadBalancer{
		selectFn: selectFn,
	}
}

func (f *fakeLoadBalancer) Select(instances []*ServiceInstance, strategy LoadBalanceStrategy) *ServiceInstance {
	f.mu.Lock()
	f.lastStrat = strategy
	f.lastInsts = append([]*ServiceInstance(nil), instances...)
	f.selects++
	selectFn := f.selectFn
	f.mu.Unlock()

	if selectFn != nil {
		return selectFn(instances, strategy)
	}
	if len(instances) == 0 {
		return nil
	}
	return instances[0]
}

func (f *fakeLoadBalancer) UpdateWeights(_ []*ServiceInstance) error {
	return nil
}

func (f *fakeLoadBalancer) GetStats() LoadBalancerStats {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stats
}

func (f *fakeLoadBalancer) lastStrategy() LoadBalanceStrategy {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastStrat
}

func (f *fakeLoadBalancer) lastInstances() []*ServiceInstance {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*ServiceInstance(nil), f.lastInsts...)
}

func (f *fakeLoadBalancer) setStats(stats LoadBalancerStats) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stats = stats
}

type fakeCache struct {
	mu       sync.Mutex
	entries  map[string][]*ServiceInstance
	getCnt   int
	setCnt   int
	delCnt   int
	clearCnt int
	stats    CacheStats
}

func newFakeCache() *fakeCache {
	return &fakeCache{
		entries: make(map[string][]*ServiceInstance),
	}
}

func (f *fakeCache) Get(key string) ([]*ServiceInstance, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCnt++
	instances, ok := f.entries[key]
	if !ok {
		return nil, false
	}
	copied := make([]*ServiceInstance, len(instances))
	copy(copied, instances)
	return copied, true
}

func (f *fakeCache) Set(key string, instances []*ServiceInstance, _ time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setCnt++
	copied := make([]*ServiceInstance, len(instances))
	copy(copied, instances)
	f.entries[key] = copied
}

func (f *fakeCache) Delete(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delCnt++
	delete(f.entries, key)
}

func (f *fakeCache) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearCnt++
	f.entries = make(map[string][]*ServiceInstance)
}

func (f *fakeCache) Stats() CacheStats {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stats
}

func (f *fakeCache) getCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getCnt
}

func (f *fakeCache) setCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.setCnt
}

func (f *fakeCache) clearCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clearCnt
}

func (f *fakeCache) setStats(stats CacheStats) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stats = stats
}

func cloneInstance(src *ServiceInstance) *ServiceInstance {
	if src == nil {
		return nil
	}
	dst := *src
	if src.Metadata != nil {
		dst.Metadata = make(map[string]string, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}
	return &dst
}

var (
	_ ServiceDiscovery = (*fakeDiscovery)(nil)
	_ LoadBalancer     = (*fakeLoadBalancer)(nil)
	_ ServiceCache     = (*fakeCache)(nil)
)
