package reuse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const numCacheShards = 32

type cacheItem struct {
	value      any
	expiration time.Time
}

type cacheShard struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

// MemoryCache is a high-performance 32-bucket sharded L1 in-memory cache with optional Redis L2 sync.
type MemoryCache struct {
	shards      [numCacheShards]*cacheShard
	redisClient *redis.Client
}

var (
	defaultCache *MemoryCache
	cacheOnce    sync.Once
)

func getCacheShardIndex(key string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return hash % numCacheShards
}

// GetCache returns the singleton MemoryCache instance
func GetCache() *MemoryCache {
	cacheOnce.Do(func() {
		mc := &MemoryCache{}
		for i := 0; i < numCacheShards; i++ {
			mc.shards[i] = &cacheShard{
				items: make(map[string]cacheItem),
			}
		}
		defaultCache = mc
		go defaultCache.startCleanup(2 * time.Minute)
	})
	return defaultCache
}

// SetRedis connects Redis L2 client for distributed multi-instance caching
func (c *MemoryCache) SetRedis(client *redis.Client) {
	c.redisClient = client
}

// Set stores a key-value pair in sharded L1 memory and optional L2 Redis
func (c *MemoryCache) Set(key string, value any, ttl time.Duration) {
	shardIdx := getCacheShardIndex(key)
	shard := c.shards[shardIdx]

	shard.mu.Lock()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	shard.items[key] = cacheItem{
		value:      value,
		expiration: exp,
	}
	shard.mu.Unlock()

	// Sync to Redis L2 if active
	if c.redisClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			var strVal string
			switch v := value.(type) {
			case string:
				strVal = v
			case []byte:
				strVal = string(v)
			case bool:
				strVal = fmt.Sprintf("%t", v)
			default:
				if b, err := json.Marshal(v); err == nil {
					strVal = string(b)
				} else {
					strVal = fmt.Sprintf("%v", v)
				}
			}
			_ = c.redisClient.Set(ctx, key, strVal, ttl).Err()
		}()
	}
}

// Get retrieves a value from L1 sharded cache first, falling back to L2 Redis if missing
func (c *MemoryCache) Get(key string) (any, bool) {
	shardIdx := getCacheShardIndex(key)
	shard := c.shards[shardIdx]

	shard.mu.RLock()
	item, found := shard.items[key]
	shard.mu.RUnlock()

	if found {
		if !item.expiration.IsZero() && time.Now().After(item.expiration) {
			c.Delete(key)
			return nil, false
		}
		return item.value, true
	}

	// L1 miss: try L2 Redis if connected
	if c.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		val, err := c.redisClient.Get(ctx, key).Result()
		if err == nil && val != "" {
			// Populate L1 cache for subsequent requests
			c.Set(key, val, 1*time.Minute)
			return val, true
		}
	}

	return nil, false
}

// Delete removes an item from L1 sharded cache and L2 Redis
func (c *MemoryCache) Delete(key string) {
	shardIdx := getCacheShardIndex(key)
	shard := c.shards[shardIdx]

	shard.mu.Lock()
	delete(shard.items, key)
	shard.mu.Unlock()

	if c.redisClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			_ = c.redisClient.Del(ctx, key).Err()
		}()
	}
}

// DeleteByPrefix removes all items matching prefix across all 32 shards
func (c *MemoryCache) DeleteByPrefix(prefix string) {
	for i := 0; i < numCacheShards; i++ {
		shard := c.shards[i]
		shard.mu.Lock()
		for k := range shard.items {
			if strings.HasPrefix(k, prefix) {
				delete(shard.items, k)
			}
		}
		shard.mu.Unlock()
	}

	if c.redisClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			iter := c.redisClient.Scan(ctx, 0, prefix+"*", 0).Iterator()
			for iter.Next(ctx) {
				_ = c.redisClient.Del(ctx, iter.Val()).Err()
			}
		}()
	}
}

// Flush removes all items from all shards
func (c *MemoryCache) Flush() {
	for i := 0; i < numCacheShards; i++ {
		shard := c.shards[i]
		shard.mu.Lock()
		shard.items = make(map[string]cacheItem)
		shard.mu.Unlock()
	}

	if c.redisClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = c.redisClient.FlushDB(ctx).Err()
		}()
	}
}

func (c *MemoryCache) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now()
		for i := 0; i < numCacheShards; i++ {
			shard := c.shards[i]
			shard.mu.Lock()
			for k, v := range shard.items {
				if !v.expiration.IsZero() && now.After(v.expiration) {
					delete(shard.items, k)
				}
			}
			shard.mu.Unlock()
		}
	}
}
