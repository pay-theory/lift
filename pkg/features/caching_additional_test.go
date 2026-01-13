package features

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
	"github.com/stretchr/testify/require"
)

type testCacheStore struct {
	mu       sync.Mutex
	items    map[string]any
	ttl      time.Duration
	stats    CacheStats
	useStats bool

	getErr    error
	setErr    error
	deleteErr error
	clearErr  error
	closeErr  error
}

func newTestCacheStore() *testCacheStore {
	return &testCacheStore{
		items: make(map[string]any),
		ttl:   time.Minute,
	}
}

func (s *testCacheStore) Get(_ context.Context, key string) (any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return nil, false, s.getErr
	}
	value, found := s.items[key]
	return value, found, nil
}

func (s *testCacheStore) Set(_ context.Context, key string, value any, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setErr != nil {
		return s.setErr
	}
	s.items[key] = value
	return nil
}

func (s *testCacheStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.items, key)
	return nil
}

func (s *testCacheStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearErr != nil {
		return s.clearErr
	}
	s.items = make(map[string]any)
	return nil
}

func (s *testCacheStore) Stats() CacheStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.useStats {
		return s.stats
	}
	size := int64(len(s.items))
	return CacheStats{
		Size:        size,
		MemoryUsage: size * 64,
	}
}

func (s *testCacheStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closeErr != nil {
		return s.closeErr
	}
	s.items = make(map[string]any)
	return nil
}

func (s *testCacheStore) Keys(_ string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.items))
	for key := range s.items {
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *testCacheStore) Exists(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, found := s.items[key]
	return found
}

func (s *testCacheStore) TTL(key string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.items[key]; found {
		return s.ttl
	}
	return 0
}

func TestCacheMiddleware_Handle_CacheHitAndMiss(t *testing.T) {
	store := newTestCacheStore()

	middleware := NewCacheMiddleware(CacheConfig{
		Store:           store,
		DefaultTTL:      time.Minute,
		EnableMetrics:   true,
		TenantIsolation: true,
		KeyFunc: func(_ *lift.Context) string {
			return ""
		},
	})

	calls := 0
	next := lift.HandlerFunc(func(ctx *lift.Context) error {
		calls++
		return ctx.JSON(map[string]any{"message": "ok"})
	})

	ctx1 := createCacheTestContext(httpGET, "/things", nil)
	ctx1.Request.Headers["User-Agent"] = "ua-test"
	ctx1.Request.QueryParams["page"] = "1"
	ctx1.Set("tenant_id", "tenant-1")

	key := middleware.generateKey(ctx1)

	require.NoError(t, middleware.Handle(ctx1, next))
	require.Equal(t, 1, calls)
	require.Equal(t, "MISS", ctx1.Response.Headers["X-Cache"])
	require.Equal(t, key, ctx1.Response.Headers["X-Cache-Key"])
	require.True(t, store.Exists(key))

	ctx2 := createCacheTestContext(httpGET, "/things", nil)
	ctx2.Request.Headers["User-Agent"] = "ua-test"
	ctx2.Request.QueryParams["page"] = "1"
	ctx2.Set("tenant_id", "tenant-1")

	require.NoError(t, middleware.Handle(ctx2, next))
	require.Equal(t, 1, calls)
	require.Equal(t, "HIT", ctx2.Response.Headers["X-Cache"])
	require.Equal(t, key, ctx2.Response.Headers["X-Cache-Key"])
	require.Equal(t, map[string]any{"message": "ok"}, ctx2.Response.Body)

	stats := middleware.GetStats()
	require.Equal(t, int64(1), stats.Hits)
	require.Equal(t, int64(1), stats.Misses)
	require.Equal(t, int64(1), stats.Sets)
	require.InDelta(t, 0.5, stats.HitRate, 0.0001)
}

func TestCacheMiddleware_BypassNonGETAndNoCacheHeader(t *testing.T) {
	t.Run("non-GET bypasses caching but can invalidate", func(t *testing.T) {
		store := newTestCacheStore()
		middleware := NewCacheMiddleware(CacheConfig{
			Store:           store,
			DefaultTTL:      time.Minute,
			TenantIsolation: true,
			EnableMetrics:   true,
		})

		ctx := createCacheTestContext("POST", "/things", nil)
		ctx.Set("tenant_id", "tenant-1")
		ctx.Logger = &lift.NoOpLogger{}

		nextCalled := false
		next := lift.HandlerFunc(func(ctx *lift.Context) error {
			nextCalled = true
			return ctx.JSON(map[string]any{"ok": true})
		})

		require.NoError(t, middleware.Handle(ctx, next))
		require.True(t, nextCalled)
		require.Empty(t, store.items)
	})

	t.Run("Cache-Control no-cache bypasses caching", func(t *testing.T) {
		store := newTestCacheStore()
		middleware := NewCacheMiddleware(CacheConfig{
			Store:         store,
			DefaultTTL:    time.Minute,
			EnableMetrics: true,
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.Request.Headers["Cache-Control"] = "no-cache"

		nextCalled := false
		next := lift.HandlerFunc(func(ctx *lift.Context) error {
			nextCalled = true
			return ctx.JSON(map[string]any{"ok": true})
		})

		require.NoError(t, middleware.Handle(ctx, next))
		require.True(t, nextCalled)
		require.Empty(t, store.items)
	})
}

func TestCacheMiddleware_StoreErrorsStillServeResponse(t *testing.T) {
	t.Run("Get error records error and still executes handler", func(t *testing.T) {
		store := newTestCacheStore()
		store.getErr = errors.New("boom")

		middleware := NewCacheMiddleware(CacheConfig{
			Store:         store,
			DefaultTTL:    time.Minute,
			EnableMetrics: true,
			KeyFunc: func(_ *lift.Context) string {
				return ""
			},
		})

		nextCalled := false
		next := lift.HandlerFunc(func(ctx *lift.Context) error {
			nextCalled = true
			return ctx.JSON(map[string]any{"ok": true})
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)

		require.NoError(t, middleware.Handle(ctx, next))
		require.True(t, nextCalled)

		stats := middleware.GetStats()
		require.Equal(t, int64(1), stats.Evictions)
	})

	t.Run("Set error records error", func(t *testing.T) {
		store := newTestCacheStore()
		store.setErr = errors.New("boom")

		middleware := NewCacheMiddleware(CacheConfig{
			Store:         store,
			DefaultTTL:    time.Minute,
			EnableMetrics: true,
			KeyFunc: func(_ *lift.Context) string {
				return ""
			},
		})

		next := lift.HandlerFunc(func(ctx *lift.Context) error {
			return ctx.JSON(map[string]any{"ok": true})
		})

		ctx := createCacheTestContext(httpGET, "/things", nil)

		require.NoError(t, middleware.Handle(ctx, next))

		stats := middleware.GetStats()
		require.Equal(t, int64(1), stats.Evictions)
	})
}

func TestCacheMiddleware_ShouldInvalidateCustom(t *testing.T) {
	store := newTestCacheStore()
	middleware := NewCacheMiddleware(CacheConfig{
		Store:           store,
		DefaultTTL:      time.Minute,
		TenantIsolation: true,
		ShouldInvalidate: func(ctx *lift.Context) bool {
			return ctx.Request.Method == httpGET
		},
	})

	ctx := createCacheTestContext(httpGET, "/things", nil)
	ctx.Set("tenant_id", "tenant-1")
	ctx.Logger = &lift.NoOpLogger{}

	next := lift.HandlerFunc(func(ctx *lift.Context) error {
		return ctx.JSON(map[string]any{"ok": true})
	})

	require.NoError(t, middleware.Handle(ctx, next))
}

func TestCacheHelpersAndSerializer(t *testing.T) {
	t.Run("Cache wrapper and helpers build middleware", func(t *testing.T) {
		store := newTestCacheStore()

		_ = CacheWithStore(store, time.Minute)
		_ = CacheWithKey(store, time.Minute, func(ctx *lift.Context) string {
			return ctx.Request.Path
		})
		_ = CacheWithInvalidation(store, time.Minute, "*")

		mw := Cache(CacheConfig{
			Store:         store,
			DefaultTTL:    time.Minute,
			EnableMetrics: true,
			KeyFunc: func(_ *lift.Context) string {
				return ""
			},
		})

		next := lift.HandlerFunc(func(ctx *lift.Context) error {
			return ctx.JSON(map[string]any{"ok": true})
		})
		wrapped := mw(next)

		ctx := createCacheTestContext(httpGET, "/things", nil)
		require.NoError(t, wrapped.Handle(ctx))
	})

	t.Run("CacheStatsMiddleware intercepts /cache/stats", func(t *testing.T) {
		store := newTestCacheStore()
		store.useStats = true
		store.stats = CacheStats{Hits: 10, Misses: 2}

		handlerCalled := false
		next := lift.HandlerFunc(func(ctx *lift.Context) error {
			handlerCalled = true
			return ctx.JSON(map[string]any{"ok": true})
		})

		wrapped := CacheStatsMiddleware(store)(next)

		ctx := createCacheTestContext(httpGET, "/cache/stats", nil)
		require.NoError(t, wrapped.Handle(ctx))
		require.False(t, handlerCalled)
		require.Equal(t, store.stats, ctx.Response.Body)
	})

	t.Run("JSONCacheSerializer round-trips values", func(t *testing.T) {
		serializer := &JSONCacheSerializer{}
		encoded, err := serializer.Serialize(map[string]any{"a": "b"})
		require.NoError(t, err)

		var decoded map[string]any
		require.NoError(t, serializer.Deserialize(encoded, &decoded))
		require.Equal(t, map[string]any{"a": "b"}, decoded)
	})
}

func TestCacheInternals(t *testing.T) {
	t.Run("defaultKeyFunc includes query parameters", func(t *testing.T) {
		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.Request.QueryParams["q"] = "search"
		key := defaultKeyFunc(ctx)
		require.Contains(t, key, "GET:/things:")
	})

	t.Run("defaultShouldCache checks method and status code", func(t *testing.T) {
		ctx := createCacheTestContext(httpGET, "/things", nil)
		ctx.Response.Status(200)
		require.True(t, defaultShouldCache(ctx, nil))

		ctx.Response.Status(500)
		require.False(t, defaultShouldCache(ctx, nil))

		ctx.Request.Method = "POST"
		ctx.Response.Status(200)
		require.False(t, defaultShouldCache(ctx, nil))
	})
}

func TestResponseCapturerAndMultiBackendStore(t *testing.T) {
	t.Run("ResponseCapturer captures JSON data", func(t *testing.T) {
		capturer := &ResponseCapturer{Response: lift.NewResponse()}
		require.NoError(t, capturer.JSON(map[string]any{"a": "b"}))
		require.Equal(t, map[string]any{"a": "b"}, capturer.GetCapturedData())
		require.True(t, capturer.IsWritten())

		_, err := capturer.MarshalJSON()
		require.NoError(t, err)
	})

	t.Run("ResponseCapturer forwards response helpers", func(t *testing.T) {
		capturer := &ResponseCapturer{Response: lift.NewResponse()}
		capturer.Status(201)
		capturer.Header("X-Test", "1")
		require.NoError(t, capturer.Text("hello"))

		capturer = &ResponseCapturer{Response: lift.NewResponse()}
		require.NoError(t, capturer.HTML("<p>ok</p>"))

		capturer = &ResponseCapturer{Response: lift.NewResponse()}
		require.NoError(t, capturer.Binary([]byte("bin")))
		require.True(t, capturer.Response.IsBase64Encoded)
	})

	t.Run("MultiBendCacheStore supports strategies and utilities", func(t *testing.T) {
		primary := newTestCacheStore()
		secondary := newTestCacheStore()

		require.NotNil(t, NewMultiBackendCacheStore(primary, secondary, "failover"))

		key := "k"
		secondary.items[key] = map[string]any{"v": 1}

		store := NewMultiBackendCacheStore(primary, secondary, "write_back")

		value, found, err := store.Get(context.Background(), key)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, map[string]any{"v": 1}, value)
		require.True(t, primary.Exists(key))

		writeThrough := NewMultiBackendCacheStore(primary, secondary, "write_through")
		require.NoError(t, writeThrough.Set(context.Background(), "k2", "v2", time.Minute))
		require.True(t, secondary.Exists("k2"))

		primary.deleteErr = errors.New("primary delete")
		require.Error(t, writeThrough.Delete(context.Background(), "k2"))
		primary.deleteErr = nil

		secondary.clearErr = errors.New("secondary clear")
		require.Error(t, writeThrough.Clear(context.Background()))
		secondary.clearErr = nil

		primary.items["ttl"] = "x"
		primary.ttl = 0
		secondary.items["ttl"] = "x"
		secondary.ttl = 2 * time.Minute
		require.Equal(t, 2*time.Minute, writeThrough.TTL("ttl"))

		require.True(t, writeThrough.Exists(key))
		_, err = writeThrough.Keys("*")
		require.NoError(t, err)
		_ = writeThrough.Stats()

		primary.closeErr = errors.New("primary close")
		require.Error(t, writeThrough.Close())
		primary.closeErr = nil

		secondary.closeErr = errors.New("secondary close")
		require.Error(t, writeThrough.Close())
	})

	t.Run("MultiBendCacheStore returns nil on cache miss", func(t *testing.T) {
		primary := newTestCacheStore()
		secondary := newTestCacheStore()
		primary.getErr = errors.New("primary get")
		secondary.getErr = errors.New("secondary get")

		store := NewMultiBackendCacheStore(primary, secondary, "failover")
		value, found, err := store.Get(context.Background(), "missing")
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, value)
	})
}

func TestJSONCacheSerializer_InvalidTarget(t *testing.T) {
	serializer := &JSONCacheSerializer{}
	encoded, err := serializer.Serialize(map[string]any{"a": "b"})
	require.NoError(t, err)

	require.Error(t, serializer.Deserialize(encoded, nil))
}

func TestNormalizeResponseBody_DefaultBranchMarshalError(t *testing.T) {
	vm := NewValidationMiddleware(ValidationConfig{})
	_, err := vm.normalizeResponseBody(func() {})
	require.Error(t, err)
}

func TestJSONCacheSerializer_SerializeDeterministicJSON(t *testing.T) {
	serializer := &JSONCacheSerializer{}

	encoded, err := serializer.Serialize(map[string]any{"a": "b"})
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, map[string]any{"a": "b"}, decoded)
}
