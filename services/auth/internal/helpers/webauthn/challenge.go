package webauthn

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	"github.com/telark/auth/internal/helpers/shared"
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
		return fmt.Errorf(string(constants.ErrRedisChallengeConflict), shared.IdentityHash(userID))
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

// A registration finish that carries no session is bound to its ceremony by the
// challenge the authenticator signed, never by a caller-supplied identity header.
func StoreRegistrationChallengeOwner(challenge, userID string) error {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	key, err := registrationOwnerKey(challenge)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	if err := rdb.Set(ctx, key, userID, time.Duration(constants.RedisTTLChallenge)*time.Second).Err(); err != nil {
		return fmt.Errorf(string(constants.ErrFailedCreateChallenge), err.Error())
	}
	return nil
}

func RegistrationChallengeOwner(r *http.Request) (string, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedReadRequestBody), err.Error())
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	_, clientDataJSONB64, _, err := extractRegistrationData(bodyBytes)
	if err != nil {
		return constants.EmptyString, err
	}
	clientData, err := parseClientData(clientDataJSONB64)
	if err != nil {
		return constants.EmptyString, err
	}
	challenge, ok := clientData[constants.WebAuthnKeyChallenge].(string)
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrMissingChallengeInClientData))
	}
	key, err := registrationOwnerKey(challenge)
	if err != nil {
		return constants.EmptyString, err
	}

	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	userID, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return constants.EmptyString, errors.New(string(constants.ErrChallengeNotFound))
		}
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedGetChallenge), err.Error())
	}
	return userID, nil
}

func cleanupRegistrationChallengeOwner(challenge string) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		lg.Error(string(constants.ErrRedisClientUnavailable))
		return
	}
	key, err := registrationOwnerKey(challenge)
	if err != nil {
		lg.Warn(err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	if err := rdb.Del(ctx, key).Err(); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.ErrFailedDeleteChallenge), err.Error()))
	}
}

// The stored challenge and the one echoed in clientDataJSON may differ in base64
// padding/alphabet, so both are keyed by their decoded bytes.
func registrationOwnerKey(challenge string) (string, error) {
	decoded, err := authhelper.DecodeBase64URLWithFallback(challenge)
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedDecodeChallenge), err)
	}
	return constants.RedisKeyPrefixRegistrationOwner + base64.RawURLEncoding.EncodeToString(decoded), nil
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
		lg.Warn(fmt.Sprintf(string(constants.ErrFailedDeleteChallenge), err.Error()))
	}
}
