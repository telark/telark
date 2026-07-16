package authz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/telark/exporter/internal/constants"
	exprdb "github.com/telark/exporter/internal/redis"
	"github.com/telark/x-ware/authz"
	rediscache "github.com/telark/x-ware/redis/cache"
)

func sessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return rediscache.BuildKey(constants.AuthzKeyPrefix, constants.AuthzKeySession, hex.EncodeToString(digest[:]))
}

func grantsKey(generation, userID string) string {
	return rediscache.BuildKey(constants.AuthzKeyPrefix, constants.AuthzKeyGrants, generation, userID)
}

func generationKey() string {
	return rediscache.BuildKey(constants.AuthzKeyPrefix, constants.AuthzKeyGeneration)
}

func cachedSessionUserID(ctx context.Context, token string) (string, bool) {
	client := exprdb.Get()
	if client == nil {
		return constants.EmptyString, false
	}

	userID, err := client.Get(ctx, sessionKey(token)).Result()
	if err != nil || userID == constants.EmptyString {
		return constants.EmptyString, false
	}

	return userID, true
}

func storeSessionUserID(ctx context.Context, token, userID string, ttl time.Duration) {
	client := exprdb.Get()
	if client == nil || ttl <= constants.AuthzNoTTL {
		return
	}

	client.Set(ctx, sessionKey(token), userID, ttl)
}

// ForgetSession drops a token the moment its session is deleted, so logout
// cannot leave a usable cache entry behind for the rest of the TTL.
func ForgetSession(ctx context.Context, token string) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	client.Del(ctx, sessionKey(token))
}

func cachedGrants(ctx context.Context, userID string) (authz.Grants, bool) {
	client := exprdb.Get()
	if client == nil {
		return authz.Grants{}, false
	}

	raw, err := client.Get(ctx, grantsKey(generation(ctx), userID)).Result()
	if err != nil || raw == constants.EmptyString {
		return authz.Grants{}, false
	}

	var grants authz.Grants
	if err := json.Unmarshal([]byte(raw), &grants); err != nil {
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

	client.Set(ctx, grantsKey(generation(ctx), userID), raw, constants.AuthzGrantsTTL)
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

// ForgetUserGrants drops one user's cached grants for the current generation.
func ForgetUserGrants(ctx context.Context, userID string) {
	client := exprdb.Get()
	if client == nil {
		return
	}

	client.Del(ctx, grantsKey(generation(ctx), userID))
}
