package diff

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/discovery/internal/constants"
)

func genProcessingLockKey(appName string, generation int) string {
	return fmt.Sprintf("%s%s:%d:processing", constants.KeyPrefixLockGen, appName, generation)
}

func AcquireGenProcessingLock(
	ctx context.Context,
	rdb *redis.Client,
	appName string,
	generation int,
) (lockKey string, acquired bool) {
	if rdb == nil || appName == constants.EmptyString {
		return constants.EmptyString, true
	}
	key := genProcessingLockKey(appName, generation)
	result, err := rdb.SetArgs(ctx, key, constants.DefaultAddValue, redis.SetArgs{
		Mode: "NX",
		TTL:  constants.DefaultLockTTL,
	}).Result()
	if err != nil || result != "OK" {
		return constants.EmptyString, false
	}
	return key, true
}

func ReleaseGenProcessingLock(rdb *redis.Client, lockKey string) {
	if rdb == nil || lockKey == constants.EmptyString {
		return
	}
	_ = rdb.Del(context.Background(), lockKey).Err()
}
