package utils

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// ConnectRedis initializes a tuned Go-Redis connection pool with automatic health check ping.
// If Redis is unreachable, it logs a warning and returns nil (enabling graceful fallback to in-memory L1 cache).
func ConnectRedis(redisURL string) *redis.Client {
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("WARNING: Unable to parse REDIS_URL (%s): %v. Proceeding in L1 local memory mode.", redisURL, err)
		return nil
	}

	opt.PoolSize = 25
	opt.MinIdleConns = 5
	opt.DialTimeout = 3 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("NOTICE: Redis server ping failed at %s (%v). Operating in high-speed L1 in-memory mode.", opt.Addr, err)
		return nil
	}

	log.Printf("Redis L2 distributed cache pool connected successfully to %s!", opt.Addr)
	return client
}
