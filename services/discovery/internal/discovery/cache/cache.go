package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	insightsdata "github.com/telark/data/insights"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
)

func CacheKey(namespace, name string) string {
	return insightsdata.DocumentKey(namespace, name)
}

func GetInsights(ctx context.Context, rdb *redis.Client, namespace, name string) (*application.AppInsights, error) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	if rdb == nil {
		lg.Warn(MsgRedisClientNil)
		return nil, nil
	}
	key := CacheKey(namespace, name)
	raw, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		lg.Warn(fmt.Sprintf(MsgCacheGetFailed, key, err))
		return nil, err
	}
	return Decode(key, raw), nil
}

// Nil for an undecodable or legacy-shaped document: callers treat both as absent.
func Decode(key, raw string) *application.AppInsights {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	var doc application.AppInsights
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		lg.Warn(fmt.Sprintf(MsgCacheDeserializeFailed, key, err, len(raw)))
		return nil
	}
	if doc.Version == constants.DefaultInitValue && doc.LastRun.Status == constants.EmptyString {
		lg.Warn(fmt.Sprintf(MsgCacheDeserializeFailed, key, MsgLegacyDocument, len(raw)))
		return nil
	}
	return &doc
}
