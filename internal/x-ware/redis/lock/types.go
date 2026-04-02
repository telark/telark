package lock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedLock defines the interface for a Redis-based distributed lock
// with fencing tokens for safe concurrent access across replicas.
type DistributedLock interface {
	Acquire(ctx context.Context, key, holder string, ttl time.Duration) (fenceToken string, acquired bool, err error)
	Release(ctx context.Context, key, holder string) (bool, error)
	Extend(ctx context.Context, key, holder string, ttl time.Duration) (bool, error)
	FenceToken(ctx context.Context, key string) (string, error)
}

// Lock implements DistributedLock using go-redis/v9 Watch/TxPipelined
// for optimistic locking with fencing tokens.
type Lock struct {
	client *redis.Client
}
