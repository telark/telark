package webauthn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/auth/internal/constants"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	authdata "github.com/telark/data/auth"
)

func StoreChallenge(userID string, challenge string) error {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	key := constants.RedisKeyPrefixChallenge + userID
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	set, err := rdb.SetNX(ctx, key, challenge,
		time.Duration(constants.RedisTTLChallenge)*time.Second).Result()
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedCreateChallenge), err.Error())
	}
	if !set {
		return fmt.Errorf(string(constants.ErrRedisChallengeConflict), userID)
	}
	return nil
}

func ValidateAndGetChallenge(userID string) (*authdata.AuthChallenge, error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return nil, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	key := constants.RedisKeyPrefixChallenge + userID
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.New(string(constants.ErrChallengeNotFound))
		}
		return nil, fmt.Errorf(string(constants.ErrFailedGetChallenge), err.Error())
	}
	return &authdata.AuthChallenge{Challenge: val, UserID: userID}, nil
}

func CleanupChallenge(userID string) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		lg.Error(string(constants.ErrRedisClientUnavailable))
		return
	}
	key := constants.RedisKeyPrefixChallenge + userID
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	if err := rdb.Del(ctx, key).Err(); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedDeleteChallenge), err.Error()))
	}
}
