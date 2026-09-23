package authz

import (
	"context"
	"encoding/json"

	"github.com/telark/exporter/internal/constants"
	exprdb "github.com/telark/exporter/internal/redis"
	"github.com/telark/x-ware/authz"
	rediscache "github.com/telark/x-ware/redis/cache"
)

func grantsBinding(generation, userID string) string {
	return rediscache.BuildKey(constants.AuthzKeyGrants, generation, userID)
}

func grantsKey(generation, userID string) string {
	return rediscache.BuildKey(constants.AuthzKeyPrefix, constants.AuthzKeyGrants, generation, userID)
}

func generationKey() string {
	return rediscache.BuildKey(constants.AuthzKeyPrefix, constants.AuthzKeyGeneration)
}

func cachedGrants(ctx context.Context, userID string) (authz.Grants, bool) {
	client := exprdb.Get()
	if client == nil {
		return authz.Grants{}, false
	}

	gen := generation(ctx)
	raw, err := client.Get(ctx, grantsKey(gen, userID)).Result()
	if err != nil || raw == constants.EmptyString {
		return authz.Grants{}, false
	}

	// An entry that is not ours decides nothing: it is dropped, and the grants
	// are recomputed from the resources that actually own them. The generation
	// is bound in too, so rolling it back cannot replay a stale entry.
	payload, ok := VerifyCacheEntry(grantsBinding(gen, userID), raw)
	if !ok {
		lg.Warn(fmtLog(constants.LogAuthzGrantsCacheUnsigned, userID))
		return authz.Grants{}, false
	}

	var grants authz.Grants
	if err := json.Unmarshal([]byte(payload), &grants); err != nil {
		lg.Warn(fmtLog(constants.LogAuthzGrantsCacheReadFailed, err))
		return authz.Grants{}, false
	}

	return grants, true
}

func storeGrants(ctx context.Context, userID string, grants authz.Grants) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	raw, err := json.Marshal(grants)
	if err != nil {
		lg.Warn(fmtLog(constants.LogAuthzGrantsCacheWriteFailed, err))
		return
	}

	gen := generation(ctx)

	// Caching unsigned would be caching something we could not later trust.
	signed, ok := SignCacheEntry(grantsBinding(gen, userID), string(raw))
	if !ok {
		return
	}

	client.Set(ctx, grantsKey(gen, userID), signed, constants.AuthzGrantsTTL)
}

func generation(ctx context.Context) string {
	client := exprdb.Get()
	if client == nil {
		return constants.EmptyString
	}

	value, err := client.Get(ctx, generationKey()).Result()
	if err != nil {
		return constants.EmptyString
	}

	return value
}

// BumpGeneration invalidates every cached grant at once. A role or group edit
// changes the permissions of an unknown set of users, so the generation moves
// rather than each affected key being hunted down and deleted.
func BumpGeneration(ctx context.Context) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	if err := client.Incr(ctx, generationKey()).Err(); err != nil {
		lg.Warn(fmtLog(constants.LogAuthzGenerationBumpFailed, err))
	}
}

func ForgetUserGrants(ctx context.Context, userID string) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	client.Del(ctx, grantsKey(generation(ctx), userID))
}
