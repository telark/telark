package cache

import (
	"context"
	"time"

	"github.com/telark/x-ware/redis/core"
	"github.com/redis/go-redis/v9"
)

func NewCacheFromClient(client *redis.Client, ttl time.Duration) *RedisCache {
	return &RedisCache{Client: client, TTL: ttl}
}

func NewCacheFromManager(manager core.RedisManagerInterface, ttl time.Duration) (*RedisCache, error) {
	c, err := manager.GetClient()
	if err != nil {
		return nil, err
	}
	return &RedisCache{Client: c.Client, TTL: ttl}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return c.Client.Get(ctx, key).Result()
}

func (c *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.TTL
	}
	return c.Client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Del(ctx context.Context, keys ...string) (int64, error) {
	return c.Client.Del(ctx, keys...).Result()
}

func (c *RedisCache) Flush(ctx context.Context) error {
	return c.Client.FlushDB(ctx).Err()
}

func (c *RedisCache) Close() error {
	return c.Client.Close()
}

func (c *RedisCache) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	return c.Client.Scan(ctx, cursor, match, count).Result()
}
