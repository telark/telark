package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type KV interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) (int64, error)
	Flush(ctx context.Context) error
	Close() error
}

type RedisCache struct {
	Client *redis.Client
	TTL    time.Duration
}

type Scanner interface {
	Scan(ctx context.Context, cursor uint64, match string, count int64) (keys []string, nextCursor uint64, err error)
}
