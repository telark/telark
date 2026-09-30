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
	return c.redis.SetNX(ctx, dedupKey(appName, cycleID), dedupValue, ttl).Result()
}

func (c *DedupClient) Release(ctx context.Context, appName, cycleID string) error {
	return c.redis.Del(ctx, dedupKey(appName, cycleID)).Err()
}

func dedupKey(appName, cycleID string) string {
	return dedupKeyPrefix + appName + keySeparator + cycleID
}
