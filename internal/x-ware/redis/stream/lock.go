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
		err := c.tryRelease(ctx, key, value)
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

func (c *LockClient) tryRelease(ctx context.Context, key, value string) error {
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
		return err
	}
	return c.redis.Watch(ctx, txf, key)
}

func (c *LockClient) Extend(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
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

	err := c.redis.Watch(ctx, txf, key)
	if err != nil {
		if err == redis.Nil || err == redis.TxFailedErr {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
