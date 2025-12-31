package features

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryCache_AdditionalMethods(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		MaxSize: 123,
		TTL:     0, // exercise default TTL behavior
	})

	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "k1", "v1", 0)) // default expiration branch
	require.NoError(t, cache.Set(ctx, "k2", "v2", time.Minute))

	keys, err := cache.Keys("*")
	require.NoError(t, err)
	require.Len(t, keys, 2)

	stats := cache.Stats()
	require.Equal(t, int64(2), stats.Size)
	require.Equal(t, int64(123), stats.MaxSize)

	require.Equal(t, time.Minute, cache.TTL("k1"))
	require.Equal(t, time.Duration(0), cache.TTL("missing"))

	require.NoError(t, cache.Clear(ctx))
	require.False(t, cache.Exists("k1"))

	require.NoError(t, cache.Set(ctx, "k3", "v3", time.Minute))
	require.NoError(t, cache.Close())
	require.False(t, cache.Exists("k3"))
}

