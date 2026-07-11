package cleanup

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/auth/internal/constants"
)

func NewDedup(rdb *redis.Client, ttl time.Duration) *Dedup {
	return &Dedup{rdb: rdb, ttl: ttl}
}

func (d *Dedup) TryClaim(ctx context.Context, resourceType, resourceID, jobID string) (bool, string, error) {
	key := dedupKey(resourceType, resourceID)
	res, err := d.rdb.SetArgs(ctx, key, jobID, redis.SetArgs{Mode: setModeNX, TTL: d.ttl}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, constants.EmptyString, err
	}
	if err == nil && res == setResultOK {
		return true, jobID, nil
	}
	current, getErr := d.rdb.Get(ctx, key).Result()
	if getErr != nil && !errors.Is(getErr, redis.Nil) {
		return false, constants.EmptyString, getErr
	}
	return false, current, nil
}

func (d *Dedup) Release(ctx context.Context, resourceType, resourceID string) error {
	return d.rdb.Del(ctx, dedupKey(resourceType, resourceID)).Err()
}
