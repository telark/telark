package stream

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type ElectionClient struct {
	redis    *redis.Client
	leaderID string
	key      string
	ttl      time.Duration
}

func NewElectionClient(r *redis.Client, leaderID, key string, ttl time.Duration) *ElectionClient {
	return &ElectionClient{
		redis:    r,
		leaderID: leaderID,
		key:      key,
		ttl:      ttl,
	}
}

func (c *ElectionClient) Campaign(ctx context.Context) (bool, error) {
	return c.redis.SetNX(ctx, c.key, c.leaderID, c.ttl).Result()
}

func (c *ElectionClient) Renew(ctx context.Context) (bool, error) {
	txf := func(tx *redis.Tx) error {
		held, err := tx.Get(ctx, c.key).Result()
		if err != nil {
			return err
		}
		if held != c.leaderID {
			return redis.Nil
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.PExpire(ctx, c.key, c.ttl)
			return nil
		})
		return err
	}

	err := c.redis.Watch(ctx, txf, c.key)
	if err != nil {
		if err == redis.Nil || err == redis.TxFailedErr {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *ElectionClient) Resign(ctx context.Context) error {
	txf := func(tx *redis.Tx) error {
		held, err := tx.Get(ctx, c.key).Result()
		if err != nil {
			if err == redis.Nil {
				return nil
			}
			return err
		}
		if held != c.leaderID {
			return nil
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, c.key)
			return nil
		})
		return err
	}

	err := c.redis.Watch(ctx, txf, c.key)
	if err != nil && err == redis.TxFailedErr {
		return nil
	}
	return err
}

func (c *ElectionClient) IsLeader(ctx context.Context) (bool, error) {
	held, err := c.redis.Get(ctx, c.key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	return held == c.leaderID, nil
}

func (c *ElectionClient) CurrentLeader(ctx context.Context) (string, error) {
	held, err := c.redis.Get(ctx, c.key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return held, nil
}

// ClearIfHeld atomically removes the leader claim only if the stored value
// matches expected. Returns true if the claim was cleared.
func (c *ElectionClient) ClearIfHeld(ctx context.Context, expected string) (bool, error) {
	cleared := false
	txf := func(tx *redis.Tx) error {
		held, err := tx.Get(ctx, c.key).Result()
		if err != nil {
			if err == redis.Nil {
				return nil
			}
			return err
		}
		if held != expected {
			return nil
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, c.key)
			return nil
		})
		if err == nil {
			cleared = true
		}
		return err
	}

	err := c.redis.Watch(ctx, txf, c.key)
	if err != nil && err == redis.TxFailedErr {
		return false, nil
	}
	return cleared, err
}
