package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryServiceCache_DeleteAndClear_UpdateStats(t *testing.T) {
	cacheIface := NewMemoryServiceCacheWithSize(10)
	memCache := cacheIface.(*MemoryServiceCache)

	memCache.Set("a", []*ServiceInstance{{ID: "a"}}, time.Minute)
	memCache.Set("b", []*ServiceInstance{{ID: "b"}}, time.Minute)

	stats := memCache.Stats()
	require.Equal(t, int64(2), stats.Size)

	memCache.Delete("a")
	_, ok := memCache.Get("a")
	require.False(t, ok)

	stats = memCache.Stats()
	require.Equal(t, int64(1), stats.Size)
	require.Equal(t, int64(1), stats.Deletes)

	memCache.Clear()
	stats = memCache.Stats()
	require.Equal(t, int64(0), stats.Size)
}

func TestTTLServiceCache_DelegatesOperations(t *testing.T) {
	delegate := newStubServiceCache()
	cache := NewTTLServiceCache(delegate, 50*time.Millisecond)
	t.Cleanup(cache.Close)

	delegate.setResponse("key", []*ServiceInstance{{ID: "inst"}}, true)
	got, ok := cache.Get("key")
	require.True(t, ok)
	require.Len(t, got, 1)

	cache.Set("key2", []*ServiceInstance{{ID: "inst2"}}, time.Second)
	cache.Delete("key2")
	cache.Clear()
	_ = cache.Stats()

	require.GreaterOrEqual(t, delegate.getCount(), 1)
	require.GreaterOrEqual(t, delegate.setCount(), 1)
	require.GreaterOrEqual(t, delegate.deleteCount(), 1)
	require.GreaterOrEqual(t, delegate.clearCount(), 1)
}
