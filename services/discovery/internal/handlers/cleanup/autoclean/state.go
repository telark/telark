package autoclean

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/discovery/internal/constants"
)

type streakState struct {
	Count            int   `json:"count"`
	FirstEmptyUnixMs int64 `json:"firstEmptyUnixMs"`
}

func streakKey(appName string) string {
	return constants.KeyPrefixAutoCleanupEmptyStreak + appName
}

func inflightKey(appName string) string {
	return constants.KeyPrefixAutoCleanupInflight + appName
}

func loadStreak(ctx context.Context, rdb redis.Cmdable, appName string) (streakState, error) {
	raw, err := rdb.Get(ctx, streakKey(appName)).Result()
	if err == redis.Nil {
		return streakState{}, nil
	}
	if err != nil {
		return streakState{}, err
	}
	var s streakState
	if uerr := json.Unmarshal([]byte(raw), &s); uerr != nil {
		return streakState{}, uerr
	}
	return s, nil
}

func saveStreak(ctx context.Context, rdb redis.Cmdable, appName string, s streakState, ttl time.Duration) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, streakKey(appName), string(b), ttl).Err()
}

func clearStreak(ctx context.Context, rdb redis.Cmdable, appName string) error {
	return rdb.Del(ctx, streakKey(appName)).Err()
}

func markInflight(ctx context.Context, rdb redis.Cmdable, appName string, ttl time.Duration) error {
	val := strconv.FormatInt(time.Now().UTC().Unix(), constants.IntBase10)
	return rdb.Set(ctx, inflightKey(appName), val, ttl).Err()
}

func clearInflight(ctx context.Context, rdb redis.Cmdable, appName string) error {
	return rdb.Del(ctx, inflightKey(appName)).Err()
}

func isInflight(ctx context.Context, rdb redis.Cmdable, appName string) (bool, error) {
	n, err := rdb.Exists(ctx, inflightKey(appName)).Result()
	if err != nil {
		return false, err
	}
	return n > constants.DefaultInitValue, nil
}
