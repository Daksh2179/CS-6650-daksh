package store

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient() (*RedisClient, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("REDIS_ADDR env var required")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: addr, // e.g. "my-redis.abc.cache.amazonaws.com:6379"
	})
	return &RedisClient{client: rdb}, nil
}

// IncrSeq atomically increments and returns the per-album photo sequence number.
// First call for an album returns 1, second returns 2, etc.
func (r *RedisClient) IncrSeq(ctx context.Context, albumID string) (int64, error) {
	key := fmt.Sprintf("seq:%s", albumID)
	return r.client.Incr(ctx, key).Result()
}