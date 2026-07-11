package forcesync

import (
	"context"
	"errors"
	"time"

	"github.com/telark/discovery/constants"
	"github.com/redis/go-redis/v9"
)

func NewDedup(rdb *redis.Client, ttl time.Duration) *Dedup {
	return &Dedup{rdb: rdb, ttl: ttl}
}

func (d *Dedup) TryClaim(ctx context.Context, appName, jobID string) (bool, string, error) {
	key := dedupKey(appName)
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

const (
	setModeNX    = "NX"
	setResultOK  = "OK"
)

func (d *Dedup) Release(ctx context.Context, appName string) error {
	return d.rdb.Del(ctx, dedupKey(appName)).Err()
}

func dedupKey(appName string) string {
	return constants.ForceSyncDedupKeyPrefix + appName
}
