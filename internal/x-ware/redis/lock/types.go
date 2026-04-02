package lock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type DistributedLock interface {
	Acquire(ctx context.Context, key, holder string, ttl time.Duration) (fenceToken string, acquired bool, err error)
	Release(ctx context.Context, key, holder string) (bool, error)
	Extend(ctx context.Context, key, holder string, ttl time.Duration) (bool, error)
	FenceToken(ctx context.Context, key string) (string, error)
}
type Lock struct {
	client *redis.Client
}
