package stream

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type DedupClient struct {
	redis *redis.Client
}

func NewDedupClient(r *redis.Client) *DedupClient {
	return &DedupClient{redis: r}
}

func (c *DedupClient) Guard(ctx context.Context, appName, cycleID string, ttl time.Duration) (bool, error) {
	key := "dedup:" + appName + ":" + cycleID
	return c.redis.SetNX(ctx, key, "1", ttl).Result()
}

func (c *DedupClient) Release(ctx context.Context, appName, cycleID string) error {
	key := "dedup:" + appName + ":" + cycleID
	return c.redis.Del(ctx, key).Err()
}
