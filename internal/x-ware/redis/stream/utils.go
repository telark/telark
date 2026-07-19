package stream

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/x-ware/constants"
)

func (c *StateClient) SetValue(ctx context.Context, key, value string, ttl time.Duration) error {
	if c == nil || c.redis == nil {
		return nil
	}
	return c.redis.Set(ctx, key, value, ttl).Err()
}

func (c *StateClient) GetValue(ctx context.Context, key string) (string, error) {
	if c == nil || c.redis == nil {
		return constants.EmptyString, nil
	}
	v, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return constants.EmptyString, nil
	}
	if err != nil {
		return constants.EmptyString, err
	}
	return v, nil
}
