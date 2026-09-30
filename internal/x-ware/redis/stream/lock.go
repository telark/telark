package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type LockClient struct {
	redis *redis.Client
}

func NewLockClient(r *redis.Client) *LockClient {
	return &LockClient{redis: r}
}

func (c *LockClient) Acquire(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return c.redis.SetNX(ctx, key, value, ttl).Result()
}

func (c *LockClient) Release(ctx context.Context, key, value string) error {
	for range LockReleaseMaxRetries {
		_, err := delIfHeld(ctx, c.redis, key, value)
		if err == nil {
			return nil
		}
		if errors.Is(err, redis.TxFailedErr) {
			time.Sleep(LockReleaseRetryDelay)
			continue
		}
		return err
	}
	return fmt.Errorf(string(LockReleaseFailedMessage), LockReleaseMaxRetries, key)
}

func (c *LockClient) Extend(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return expireIfHeld(ctx, c.redis, key, value, ttl)
}

// A missing key or one held by someone else is not an error: the caller simply
// no longer owns it. A WATCH conflict surfaces as redis.TxFailedErr so callers can retry.
func delIfHeld(ctx context.Context, r *redis.Client, key, value string) (bool, error) {
	deleted := false
	txf := func(tx *redis.Tx) error {
		held, err := tx.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				return nil
			}
			return err
		}
		if held != value {
			return nil
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, key)
			return nil
		})
		deleted = err == nil
		return err
	}
	return deleted, r.Watch(ctx, txf, key)
}

func expireIfHeld(ctx context.Context, r *redis.Client, key, value string, ttl time.Duration) (bool, error) {
	txf := func(tx *redis.Tx) error {
		held, err := tx.Get(ctx, key).Result()
		if err != nil {
			return err
		}
		if held != value {
			return redis.Nil
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.PExpire(ctx, key, ttl)
			return nil
		})
		return err
	}

	err := r.Watch(ctx, txf, key)
	if err != nil {
		if err == redis.Nil || err == redis.TxFailedErr {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
