package authz

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/x-ware/authz"
	rediscache "github.com/telark/telark/internal/x-ware/redis/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
)

var generationFloor atomic.Int64

func grantsBinding(generation, userID string) string {
	return rediscache.BuildKey(constants.AuthzKeyGrants, generation, userID)
}

func grantsKey(generation, userID string) string {
	return rediscache.BuildKey(constants.AuthzKeyPrefix, constants.AuthzKeyGrants, generation, userID)
}

func generationKey() string {
	return rediscache.BuildKey(constants.AuthzKeyPrefix, constants.AuthzKeyGeneration)
}

func cachedGrants(ctx context.Context, gen, userID string) (authz.Grants, bool) {
	client := exprdb.Get()
	if client == nil {
		return authz.Grants{}, false
	}

	raw, err := client.Get(ctx, grantsKey(gen, userID)).Result()
	if err != nil || raw == constants.EmptyString {
		return authz.Grants{}, false
	}

	// An entry that is not ours decides nothing: it is dropped, and the grants
	// are recomputed from the resources that actually own them.
	payload, ok := VerifyCacheEntry(grantsBinding(gen, userID), raw)
	if !ok {
		lg.Warn(fmtLog(constants.LogAuthzGrantsCacheUnsigned, userID))
		return authz.Grants{}, false
	}

	var entry grantsEntry
	if err := json.Unmarshal([]byte(payload), &entry); err != nil {
		lg.Warn(fmtLog(constants.LogAuthzGrantsCacheReadFailed, err))
		return authz.Grants{}, false
	}

	// A copied entry kept alive past its TTL would otherwise replay revoked grants.
	if time.Since(time.Unix(entry.IssuedAt, constants.DefaultInitValue)) > constants.AuthzGrantsTTL {
		return authz.Grants{}, false
	}

	return entry.Grants, true
}

func SignGrantsEntry(gen, userID string, grants authz.Grants, issuedAt time.Time) (string, bool) {
	raw, err := json.Marshal(grantsEntry{IssuedAt: issuedAt.Unix(), Grants: grants})
	if err != nil {
		lg.Warn(fmtLog(constants.LogAuthzGrantsCacheWriteFailed, err))
		return constants.EmptyString, false
	}
	return SignCacheEntry(grantsBinding(gen, userID), string(raw))
}

func storeGrants(ctx context.Context, gen, userID string, grants authz.Grants) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	// Caching unsigned would be caching something we could not later trust.
	signed, ok := SignGrantsEntry(gen, userID, grants, time.Now())
	if !ok {
		return
	}

	client.Set(ctx, grantsKey(gen, userID), signed, constants.AuthzGrantsTTL)
}

// Redis is writable by anyone who reaches it, so a generation lower than one
// this process has seen is a rollback: the cache is bypassed rather than trusted.
func generation(ctx context.Context) (string, bool) {
	client := exprdb.Get()
	if client == nil {
		return constants.EmptyString, false
	}

	value, err := client.Get(ctx, generationKey()).Result()
	if errors.Is(err, redis.Nil) {
		return value, generationFloor.Load() == constants.DefaultInitValue
	}
	if err != nil {
		return constants.EmptyString, false
	}

	return value, observeGeneration(value)
}

// Tests share one process across fresh Redis instances.
func ResetGenerationFloor() {
	generationFloor.Store(constants.DefaultInitValue)
}

func observeGeneration(value string) bool {
	seen, err := strconv.ParseInt(value, constants.GenerationBase, constants.GenerationBits)
	if err != nil {
		return false
	}
	for {
		floor := generationFloor.Load()
		if seen < floor {
			return false
		}
		if seen == floor || generationFloor.CompareAndSwap(floor, seen) {
			return true
		}
	}
}

// BumpGeneration invalidates every cached grant at once. A role or group edit
// changes the permissions of an unknown set of users, so the generation moves
// rather than each affected key being hunted down and deleted.
func BumpGeneration(ctx context.Context) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	next, err := client.Incr(ctx, generationKey()).Result()
	if err != nil {
		lg.Warn(fmtLog(constants.LogAuthzGenerationBumpFailed, err))
		return
	}
	// A counter lost with Redis restarts below the floor; lift it above so the cache is usable again.
	if floor := generationFloor.Load(); next <= floor {
		next = floor + constants.DefaultIncrementValue
		if err := client.Set(ctx, generationKey(), next, constants.NoExpiration).Err(); err != nil {
			lg.Warn(fmtLog(constants.LogAuthzGenerationBumpFailed, err))
			return
		}
	}
	observeGeneration(strconv.FormatInt(next, constants.GenerationBase))
}

func ForgetUserGrants(ctx context.Context, userID string) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	gen, _ := generation(ctx)
	client.Del(ctx, grantsKey(gen, userID))
}
