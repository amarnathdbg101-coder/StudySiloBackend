package reuse

import (
	"sync"
	"time"
)

type cacheItem struct {
	value      any
	expiration time.Time
}

// MemoryCache is a high-performance, thread-safe in-memory cache with TTL support.
// Ideal for hackathons, rate limits, OTPs, or quick caching without Redis dependencies.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

var (
	defaultCache *MemoryCache
	cacheOnce    sync.Once
)

// GetCache returns the singleton MemoryCache instance
func GetCache() *MemoryCache {
	cacheOnce.Do(func() {
		defaultCache = &MemoryCache{
			items: make(map[string]cacheItem),
		}
		// Background cleaner for expired items every 2 minutes
		go defaultCache.startCleanup(2 * time.Minute)
	})
	return defaultCache
}

// Set stores a key-value pair with a specific Time-To-Live (TTL)
func (c *MemoryCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}

	c.items[key] = cacheItem{
		value:      value,
		expiration: exp,
	}
}

// Get retrieves a value from the cache. Returns (nil, false) if missing or expired.
func (c *MemoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	item, found := c.items[key]
	c.mu.RUnlock()

	if !found {
		return nil, false
	}

	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		c.Delete(key)
		return nil, false
	}

	return item.value, true
}

// Delete removes an item from the cache
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Flush removes all items from the cache
func (c *MemoryCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheItem)
}

func (c *MemoryCache) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if !v.expiration.IsZero() && now.After(v.expiration) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}