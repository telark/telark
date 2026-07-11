package oidc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/telark/auth/internal/constants"
	redishelper "github.com/telark/auth/internal/helpers/redis"
)

func GenerateAndStoreNonce() (string, error) {
	b := make([]byte, constants.OIDCNonceByteLen)
	if _, err := rand.Read(b); err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrOIDCNonceStoreFailed), err)
	}
	nonce := hex.EncodeToString(b)
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	key := constants.RedisKeyPrefixNonce + nonce
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()
	if err := rdb.Set(ctx, key, nonce, time.Duration(constants.RedisTTLNonce)*time.Second).Err(); err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrOIDCNonceStoreFailed), err)
	}
	return nonce, nil
}

func VerifyAndConsumeNonce(nonce string) error {
	if nonce == constants.EmptyString {
		return errors.New(string(constants.ErrOIDCNonceMissing))
	}
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	key := constants.RedisKeyPrefixNonce + nonce
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()
	n, err := rdb.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("%s: %v", string(constants.ErrOIDCNonceInvalid), err)
	}
	if n == constants.DefaultInitValue {
		return errors.New(string(constants.ErrOIDCNonceInvalid))
	}
	return nil
}
