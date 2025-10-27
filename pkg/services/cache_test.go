package services

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryServiceCacheHitMissAndStats(t *testing.T) {
	t.Parallel()

	cacheIface := NewMemoryServiceCacheWithSize(10)
	memCache, ok := cacheIface.(*MemoryServiceCache)
	if !ok {
		t.Fatalf("expected *MemoryServiceCache, got %T", cacheIface)
	}

	instances := []*ServiceInstance{
		{ID: "instance-1"},
	}

	memCache.Set("service", instances, time.Minute)

	got, ok := memCache.Get("service")
	if !ok {
		t.Fatal("expected cache hit on existing key")
	}
	if len(got) != 1 || got[0].ID != "instance-1" {
		t.Fatalf("unexpected cached instances: %+v", got)
	}

	if _, ok := memCache.Get("missing"); ok {
		t.Fatal("expected cache miss on unknown key")
	}

	stats := memCache.Stats()
	if stats.Hits != 1 {
		t.Fatalf("expected 1 cache hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("expected 1 cache miss, got %d", stats.Misses)
	}
	if stats.Sets != 1 {
		t.Fatalf("expected 1 cache set, got %d", stats.Sets)
	}
	if stats.Size != 1 {
		t.Fatalf("expected cache size 1, got %d", stats.Size)
	}
}

func TestMemoryServiceCacheExpiryAndEviction(t *testing.T) {
	t.Parallel()

	cacheIface := NewMemoryServiceCacheWithSize(1)
	memCache := cacheIface.(*MemoryServiceCache)

	memCache.Set("first", []*ServiceInstance{{ID: "first"}}, time.Minute)

	memCache.mu.Lock()
	if entry, exists := memCache.items["first"]; exists {
		entry.expiry = time.Now().Add(-time.Second)
	}
	memCache.mu.Unlock()

	if _, ok := memCache.Get("first"); ok {
		t.Fatal("expected expired entry to be removed")
	}

	memCache.Set("first", []*ServiceInstance{{ID: "first"}}, time.Minute)
	memCache.Set("second", []*ServiceInstance{{ID: "second"}}, time.Minute)

	if _, ok := memCache.Get("first"); ok {
		t.Fatal("expected first entry to be evicted when capacity exceeded")
	}

	if _, ok := memCache.Get("second"); !ok {
		t.Fatal("expected second entry to remain after eviction")
	}
}

func TestMemoryServiceCacheDeepCopyProtection(t *testing.T) {
	t.Parallel()

	cacheIface := NewMemoryServiceCacheWithSize(5)
	memCache := cacheIface.(*MemoryServiceCache)

	original := []*ServiceInstance{
		{
			ID:       "deep-copy",
			Metadata: map[string]string{"state": "original"},
		},
	}

	memCache.Set("service", original, time.Minute)

	retrieved, ok := memCache.Get("service")
	if !ok {
		t.Fatal("expected cache hit")
	}

	retrieved[0] = &ServiceInstance{ID: "mutated"}
	retrieved = append(retrieved, &ServiceInstance{ID: "new"})

	again, ok := memCache.Get("service")
	if !ok {
		t.Fatal("expected cache hit after mutation")
	}

	if len(again) != 1 {
		t.Fatalf("expected stored slice size to remain 1, got %d", len(again))
	}

	if again[0].ID != "deep-copy" {
		t.Fatalf("expected stored instance ID unchanged, got %s", again[0].ID)
	}

	if state := again[0].Metadata["state"]; state != "original" {
		t.Fatalf("expected metadata to remain original, got %s", state)
	}

	if len(retrieved) == len(again) {
		t.Fatal("expected appended element to not affect cached slice length")
	}
}

func TestTTLServiceCacheCloseStopsCleanup(t *testing.T) {
	t.Parallel()

	delegate := newStubServiceCache()
	ttlCache := NewTTLServiceCache(delegate, 5*time.Millisecond)
	defer func() {
		// Ensure resources cleaned even on failure
		select {
		case <-ttlCache.stopCh:
		default:
			ttlCache.Close()
		}
	}()

	time.Sleep(15 * time.Millisecond) // allow cleanup loop to start
	ttlCache.Close()

	select {
	case <-ttlCache.stopCh:
		// Closed as expected
	default:
		t.Fatal("expected stopCh to be closed after Close()")
	}
}

func TestMultiTierServiceCachePromotionAndStats(t *testing.T) {
	t.Parallel()

	l1 := newStubServiceCache()
	l2 := newStubServiceCache()

	target := []*ServiceInstance{{ID: "multi-tier"}}
	l2.setResponse("service", target, true)

	cache := NewMultiTierServiceCache(l1, l2)

	result, ok := cache.Get("service")
	if !ok {
		t.Fatal("expected multi-tier cache hit after L2 promotion")
	}
	if len(result) != 1 || result[0].ID != "multi-tier" {
		t.Fatalf("unexpected promoted instance %+v", result)
	}

	if l1.getCount() != 1 {
		t.Fatalf("expected L1 Get called once, got %d", l1.getCount())
	}
	if l2.getCount() != 1 {
		t.Fatalf("expected L2 Get called once, got %d", l2.getCount())
	}
	if l1.setCount() != 1 {
		t.Fatalf("expected L1 Set called once for promotion, got %d", l1.setCount())
	}
	if ttl := l1.lastTTL(); ttl > 5*time.Minute {
		t.Fatalf("expected L1 TTL capped at 5m, got %v", ttl)
	}

	cache.Set("service", target, 10*time.Minute)
	if ttl := l1.lastTTL(); ttl != 5*time.Minute {
		t.Fatalf("expected L1 TTL capped at 5m on Set, got %v", ttl)
	}
	if ttl := l2.lastTTL(); ttl != 10*time.Minute {
		t.Fatalf("expected L2 TTL to remain original, got %v", ttl)
	}

	cache.Delete("service")
	if l1.deleteCount() != 1 || l2.deleteCount() != 1 {
		t.Fatal("expected Delete to propagate to both caches")
	}

	cache.Clear()
	if l1.clearCount() != 1 || l2.clearCount() != 1 {
		t.Fatal("expected Clear to propagate to both caches")
	}

	l1.setStats(CacheStats{Hits: 2, Sets: 3, Size: 1})
	l2.setStats(CacheStats{Hits: 1, Sets: 5, Size: 2})

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Fatalf("expected aggregated hits 1, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("expected aggregated misses 1, got %d", stats.Misses)
	}
	if stats.Sets != 8 {
		t.Fatalf("expected aggregated sets 8, got %d", stats.Sets)
	}
	if stats.Size != 3 {
		t.Fatalf("expected aggregated size 3, got %d", stats.Size)
	}
	if stats.HitRate <= 0 || stats.HitRate >= 1 {
		t.Fatalf("expected hit rate between 0 and 1, got %f", stats.HitRate)
	}
}

type stubServiceCache struct {
	mu        sync.Mutex
	responses map[string]cacheResponse
	stats     CacheStats
	gets      int
	sets      int
	deletes   int
	clears    int
	ttl       time.Duration
}

type cacheResponse struct {
	instances []*ServiceInstance
	found     bool
}

func newStubServiceCache() *stubServiceCache {
	return &stubServiceCache{
		responses: make(map[string]cacheResponse),
	}
}

func (s *stubServiceCache) Get(key string) ([]*ServiceInstance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets++
	resp, ok := s.responses[key]
	if !ok {
		return nil, false
	}
	if !resp.found {
		return nil, false
	}
	copied := make([]*ServiceInstance, len(resp.instances))
	copy(copied, resp.instances)
	return copied, true
}

func (s *stubServiceCache) Set(key string, instances []*ServiceInstance, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets++
	s.ttl = ttl
	copied := make([]*ServiceInstance, len(instances))
	copy(copied, instances)
	s.responses[key] = cacheResponse{
		instances: copied,
		found:     true,
	}
}

func (s *stubServiceCache) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	delete(s.responses, key)
}

func (s *stubServiceCache) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clears++
	s.responses = make(map[string]cacheResponse)
}

func (s *stubServiceCache) Stats() CacheStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *stubServiceCache) setResponse(key string, instances []*ServiceInstance, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]*ServiceInstance, len(instances))
	copy(copied, instances)
	s.responses[key] = cacheResponse{
		instances: copied,
		found:     found,
	}
}

func (s *stubServiceCache) setStats(stats CacheStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats = stats
}

func (s *stubServiceCache) getCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gets
}

func (s *stubServiceCache) setCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sets
}

func (s *stubServiceCache) deleteCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletes
}

func (s *stubServiceCache) clearCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clears
}

func (s *stubServiceCache) lastTTL() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ttl
}

var _ ServiceCache = (*stubServiceCache)(nil)
