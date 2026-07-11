package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
)

func ApplicationHasRedisState(ctx context.Context, rdb *redis.Client, appName string) bool {
	if rdb == nil || appName == constants.EmptyString {
		return false
	}
	patterns := []string{
		constants.KeyPrefixOpState + appName + constants.ColonSeparator + constants.Wildcard,
		constants.KeyPrefixDedup + appName + constants.ColonSeparator + constants.Wildcard,
	}
	for i := range patterns {
		keys, _, err := rdb.Scan(ctx, 0, patterns[i], 1).Result()
		if err != nil {
			continue
		}
		if len(keys) > constants.DefaultInitValue {
			return true
		}
	}
	return false
}
