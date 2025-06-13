package features

import (
	"context"
	"sync"
	"time"
)

// MemoryCacheConfig configures the memory cache
type MemoryCacheConfig struct {
	MaxSize         int64          `json:"max_size"`
	TTL             time.Duration  `json:"ttl"`
	CleanupInterval time.Duration  `json:"cleanup_interval"`
	EvictionPolicy  EvictionPolicy `json:"eviction_policy"`
}

// EvictionPolicy defines how items are evicted from cache
type EvictionPolicy int

const (
	EvictionLRU EvictionPolicy = iota
	EvictionLFU
	EvictionFIFO
	EvictionRandom
)

// MemoryCache implements a high-performance in-memory cache
type MemoryCache struct {
	config        MemoryCacheConfig
	items         map[string]*cacheItem
	lruList       *lruList
	stats         CacheStats
	mu            sync.RWMutex
	stopCh        chan struct{}
	cleanupTicker *time.Ticker
}

// cacheItem represents a cached item
type cacheItem struct {
	key         string
	value       interface{}
	expiry      time.Time
	accessTime  time.Time
	accessCount int64
	size        int64

	// LRU list pointers
	prev *cacheItem
	next *cacheItem
}

// lruList manages the LRU ordering
type lruList struct {
	head *cacheItem
	tail *cacheItem
	size int64
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache(config MemoryCacheConfig) *MemoryCache {
	if config.MaxSize == 0 {
		config.MaxSize = 1000 // Default max items
	}

	if config.TTL == 0 {
		config.TTL = 5 * time.Minute
	}

	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Minute
	}

	cache := &MemoryCache{
		config:  config,
		items:   make(map[string]*cacheItem),
		lruList: &lruList{},
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	cache.startCleanup()

	return cache
}

// Get retrieves an item from cache
func (c *MemoryCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	start := time.Now()

	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		c.updateStats(false, time.Since(start))
		return nil, false, nil
	}

	// Check if expired
	if time.Now().After(item.expiry) {
		c.mu.Lock()
		c.removeItem(key)
		c.mu.Unlock()
		c.updateStats(false, time.Since(start))
		return nil, false, nil
	}

	// Update access information
	c.mu.Lock()
	item.accessTime = time.Now()
	item.accessCount++
	c.lruList.moveToFront(item)
	c.mu.Unlock()

	c.updateStats(true, time.Since(start))
	return item.value, true, nil
}

// Set stores an item in cache
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.config.TTL
	}

	expiry := time.Now().Add(ttl)
	size := c.estimateSize(value)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if item already exists
	if existing, exists := c.items[key]; exists {
		// Update existing item
		existing.value = value
		existing.expiry = expiry
		existing.accessTime = time.Now()
		existing.size = size
		c.lruList.moveToFront(existing)
		c.stats.Sets++
		return nil
	}

	// Create new item
	item := &cacheItem{
		key:         key,
		value:       value,
		expiry:      expiry,
		accessTime:  time.Now(),
		accessCount: 1,
		size:        size,
	}

	// Check if we need to evict items
	for c.lruList.size >= c.config.MaxSize {
		c.evictLRU()
	}

	// Add to cache
	c.items[key] = item
	c.lruList.addToFront(item)
	c.stats.Sets++
	c.stats.Size++
	c.stats.MemoryUsage += size

	return nil
}

// Delete removes an item from cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeItem(key)
	c.stats.Deletes++
	return nil
}

// Clear removes all items from cache
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
	c.lruList = &lruList{}
	c.stats.Size = 0
	c.stats.MemoryUsage = 0

	return nil
}

// Stats returns cache statistics
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}

	return stats
}

// Keys returns all keys matching the pattern
func (c *MemoryCache) Keys(pattern string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		// Simple pattern matching - in production you might want regex
		if pattern == "*" || pattern == "" {
			keys = append(keys, key)
		}
		// Add more sophisticated pattern matching if needed
	}
	return keys, nil
}

// Exists checks if a key exists in cache
func (c *MemoryCache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false
	}

	// Check if expired
	if time.Now().After(item.expiry) {
		return false
	}

	return true
}

// TTL returns the time to live for a key
func (c *MemoryCache) TTL(key string) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return -1 // Key doesn't exist
	}

	remaining := time.Until(item.expiry)
	if remaining < 0 {
		return 0 // Expired
	}

	return remaining
}

// Close shuts down the cache
func (c *MemoryCache) Close() error {
	close(c.stopCh)
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
	}
	return nil
}

// removeItem removes an item from cache (must be called with lock held)
func (c *MemoryCache) removeItem(key string) {
	if item, exists := c.items[key]; exists {
		delete(c.items, key)
		c.lruList.remove(item)
		c.stats.Size--
		c.stats.MemoryUsage -= item.size
	}
}

// evictLRU evicts the least recently used item
func (c *MemoryCache) evictLRU() {
	if c.lruList.tail != nil {
		c.removeItem(c.lruList.tail.key)
	}
}

// estimateSize estimates the memory size of a value
func (c *MemoryCache) estimateSize(value interface{}) int64 {
	// This is a simplified size estimation
	// In a production system, you might want more accurate sizing
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case map[string]interface{}:
		return int64(len(v) * 50) // Rough estimate
	default:
		return 100 // Default estimate
	}
}

// updateStats updates cache statistics
func (c *MemoryCache) updateStats(hit bool, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if hit {
		c.stats.Hits++
	} else {
		c.stats.Misses++
	}

	// Update average latency
	total := c.stats.Hits + c.stats.Misses
	if total == 1 {
		c.stats.AvgLatency = duration
	} else {
		avgNanos := (c.stats.AvgLatency.Nanoseconds()*(total-1) + duration.Nanoseconds()) / total
		c.stats.AvgLatency = time.Duration(avgNanos)
	}
}

// startCleanup starts the background cleanup goroutine
func (c *MemoryCache) startCleanup() {
	c.cleanupTicker = time.NewTicker(c.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-c.cleanupTicker.C:
				c.cleanup()
			case <-c.stopCh:
				return
			}
		}
	}()
}

// cleanup removes expired items
func (c *MemoryCache) cleanup() {
	now := time.Now()
	expiredKeys := make([]string, 0)

	c.mu.RLock()
	for key, item := range c.items {
		if now.After(item.expiry) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	c.mu.RUnlock()

	if len(expiredKeys) > 0 {
		c.mu.Lock()
		for _, key := range expiredKeys {
			c.removeItem(key)
		}
		c.mu.Unlock()
	}
}

// LRU list methods

// addToFront adds an item to the front of the LRU list
func (l *lruList) addToFront(item *cacheItem) {
	if l.head == nil {
		l.head = item
		l.tail = item
	} else {
		item.next = l.head
		l.head.prev = item
		l.head = item
	}
	l.size++
}

// remove removes an item from the LRU list
func (l *lruList) remove(item *cacheItem) {
	if item.prev != nil {
		item.prev.next = item.next
	} else {
		l.head = item.next
	}

	if item.next != nil {
		item.next.prev = item.prev
	} else {
		l.tail = item.prev
	}

	item.prev = nil
	item.next = nil
	l.size--
}

// moveToFront moves an item to the front of the LRU list
func (l *lruList) moveToFront(item *cacheItem) {
	if l.head == item {
		return // Already at front
	}

	l.remove(item)
	l.addToFront(item)
}

// Convenience constructors

// NewSimpleMemoryCache creates a simple memory cache with defaults
func NewSimpleMemoryCache() *MemoryCache {
	return NewMemoryCache(MemoryCacheConfig{
		MaxSize:         1000,
		TTL:             5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		EvictionPolicy:  EvictionLRU,
	})
}

// NewLargeMemoryCache creates a memory cache optimized for larger datasets
func NewLargeMemoryCache() *MemoryCache {
	return NewMemoryCache(MemoryCacheConfig{
		MaxSize:         10000,
		TTL:             15 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		EvictionPolicy:  EvictionLRU,
	})
}

// NewFastMemoryCache creates a memory cache optimized for speed
func NewFastMemoryCache() *MemoryCache {
	return NewMemoryCache(MemoryCacheConfig{
		MaxSize:         500,
		TTL:             1 * time.Minute,
		CleanupInterval: 30 * time.Second,
		EvictionPolicy:  EvictionLRU,
	})
}
