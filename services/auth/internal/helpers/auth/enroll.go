package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/auth/internal/constants"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	"github.com/telark/auth/internal/helpers/shared"
)

func enrollTokenKey(token string) string {
	return constants.RedisKeyPrefixEnrollToken + token
}

// An enrollment token lets a browser on another host open a registration for
// the signed-in user: random, bound to that user, short-lived and single use.
func CreateEnrollToken(userID string) (token string, expiresAt time.Time, err error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, time.Time{}, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	token, err = shared.GenerateSessionToken()
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	ttl := time.Duration(constants.RedisTTLEnrollToken) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	if err := rdb.Set(ctx, enrollTokenKey(token), userID, ttl).Err(); err != nil {
		return constants.EmptyString, time.Time{}, fmt.Errorf(string(constants.ErrFailedStoreEnrollToken), err.Error())
	}
	return token, time.Now().UTC().Add(ttl), nil
}

// The token is consumed atomically on first presentation, whatever the ceremony
// then does: two concurrent starts can never both be authorized by it.
func ResolveEnrollToken(token string) (string, error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	userID, err := rdb.GetDel(ctx, enrollTokenKey(token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return constants.EmptyString, errors.New(string(constants.ErrEnrollTokenInvalid))
		}
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedGetEnrollToken), err.Error())
	}
	return userID, nil
}
