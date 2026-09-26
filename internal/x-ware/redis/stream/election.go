package stream

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/x-ware/constants"
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
	return expireIfHeld(ctx, c.redis, c.key, c.leaderID, c.ttl)
}

func (c *ElectionClient) Resign(ctx context.Context) error {
	_, err := delIfHeld(ctx, c.redis, c.key, c.leaderID)
	if err == redis.TxFailedErr {
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
			return constants.EmptyString, nil
		}
		return constants.EmptyString, err
	}
	return held, nil
}

func (c *ElectionClient) ClearIfHeld(ctx context.Context, expected string) (bool, error) {
	cleared, err := delIfHeld(ctx, c.redis, c.key, expected)
	if err == redis.TxFailedErr {
		return false, nil
	}
	return cleared, err
}
