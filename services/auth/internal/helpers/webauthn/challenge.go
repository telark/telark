package webauthn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
)

// A user's ceremonies refuse to start while break-glass recovers the account, but the registration its link opens.
func StoreChallenge(userID, challenge, enrollDigest string) error {
	if err := authhelper.RefuseFrozen(userID, enrollDigest); err != nil {
		return err
	}
	return StoreChallengeAt(constants.RedisKeyPrefixChallenge+userID, challenge)
}

// Each ceremony has its own key, named by its challenge's digest, so another start or a stray finish for the same
// account neither replaces nor spends it, and whoever reads Redis still can't answer the challenge.
func StoreChallengeAt(key, challenge string) error {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	digest, err := challengeDigest(challenge)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	ttl := time.Duration(constants.RedisTTLChallenge) * time.Second
	if err := rdb.Set(ctx, key+constants.ColonSeparator+digest, constants.RedisChallengeEntry, ttl).Err(); err != nil {
		return fmt.Errorf(string(constants.ErrFailedCreateChallenge), err.Error())
	}
	return nil
}

func ValidateAndGetChallenge(userID string, bodyBytes []byte) (*authdata.AuthChallenge, error) {
	if err := authhelper.RefuseFrozen(userID, constants.EmptyString); err != nil {
		return nil, err
	}
	challenge, err := ConsumeChallengeAt(constants.RedisKeyPrefixChallenge+userID, bodyBytes)
	if err != nil {
		return nil, err
	}
	challenge.UserID = userID
	return challenge, nil
}

// The challenge the signed client data names picks its ceremony, spent on first read: a failed or replayed
// assertion must restart it.
func ConsumeChallengeAt(key string, bodyBytes []byte) (*authdata.AuthChallenge, error) {
	challenge, err := clientChallenge(bodyBytes)
	if err != nil {
		return nil, errors.New(string(constants.ErrChallengeNotFound))
	}
	digest, err := challengeDigest(challenge)
	if err != nil {
		return nil, errors.New(string(constants.ErrChallengeNotFound))
	}
	rdb := redishelper.GetClient()
	if rdb == nil {
		return nil, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	err = rdb.GetDel(ctx, key+constants.ColonSeparator+digest).Err()
	if errors.Is(err, redis.Nil) {
		return nil, errors.New(string(constants.ErrChallengeNotFound))
	}
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedGetChallenge), err.Error())
	}
	return &authdata.AuthChallenge{Challenge: challenge}, nil
}

// Every open ceremony of the account, sign-ins and registrations alike, so none started before a recovery
// finishes after it.
// The walk covers the whole keyspace, so it gets its own, longer timeout: on a large Redis the 3-second one of a
// single command would make break-glass fail closed.
func DropCeremonies(userID string) error {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisScanTimeout)
	defer cancel()

	prefix := constants.RedisKeyPrefixChallenge + userID + constants.ColonSeparator
	keys, err := scanKeys(ctx, rdb, prefix)
	if err != nil || len(keys) == constants.DefaultInitValue {
		return err
	}
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf(string(constants.ErrFailedDeleteChallenge), err.Error())
	}
	return nil
}

func scanKeys(ctx context.Context, rdb *redis.Client, prefix string) ([]string, error) {
	var keys []string
	iter := rdb.Scan(ctx, constants.DefaultInitValue, prefix+constants.RedisMatchAll, constants.RedisScanCount).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedGetChallenge), err.Error())
	}
	return keys, nil
}

// A registration finish that carries no session is bound to its ceremony by the
// challenge the authenticator signed, never by a caller-supplied identity header.
func StoreRegistrationChallengeOwner(challenge, userID string) error {
	return storeCeremonyEntry(constants.RedisKeyPrefixRegistrationOwner, challenge, userID)
}

// An enrolled ceremony was opened with a one-time enrollment token, so its session-less finish
// may add a passkey to an account that already has some, and spends the token its digest names.
func StoreEnrolledCeremony(challenge, enrollDigest string) error {
	return storeCeremonyEntry(constants.RedisKeyPrefixEnrolledCeremony, challenge, enrollDigest)
}

func storeCeremonyEntry(prefix, challenge, value string) error {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	id, err := challengeDigest(challenge)
	if err != nil {
		return err
	}
	ttl := time.Duration(constants.RedisTTLChallenge) * time.Second
	entry, err := redishelper.SignEntry(prefix+id, value, ttl)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	if err := rdb.Set(ctx, prefix+id, entry, ttl).Err(); err != nil {
		return fmt.Errorf(string(constants.ErrFailedCreateChallenge), err.Error())
	}
	return nil
}

func RegistrationChallengeOwner(r *http.Request) (userID, enrollDigest string, err error) {
	bodyBytes, err := ReadAndRestoreRequestBody(r)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}

	challenge, err := clientChallenge(bodyBytes)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}
	return ceremonyOwner(challenge)
}

// Read before the ceremony verifies, because a verified finish deletes the enrolled entry.
func ceremonyOwner(challenge string) (userID, enrollDigest string, err error) {
	id, err := challengeDigest(challenge)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}

	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, constants.EmptyString, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	userID, found, err := ceremonyEntry(ctx, rdb, constants.RedisKeyPrefixRegistrationOwner+id)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}
	if !found {
		return constants.EmptyString, constants.EmptyString, errors.New(string(constants.ErrChallengeNotFound))
	}
	enrollDigest, _, err = ceremonyEntry(ctx, rdb, constants.RedisKeyPrefixEnrolledCeremony+id)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}
	return userID, enrollDigest, nil
}

// An entry that doesn't verify makes the ceremony unknown, like one nobody opened.
func ceremonyEntry(ctx context.Context, rdb *redis.Client, key string) (value string, found bool, err error) {
	entry, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return constants.EmptyString, false, nil
	}
	if err != nil {
		return constants.EmptyString, false, fmt.Errorf(string(constants.ErrFailedGetChallenge), err.Error())
	}
	value, ok := redishelper.VerifyEntry(key, entry)
	if !ok {
		return constants.EmptyString, false, errors.New(string(constants.ErrChallengeNotFound))
	}
	return value, true, nil
}

func cleanupRegistrationChallengeOwner(challenge string) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		lg.Error(string(constants.ErrRedisClientUnavailable))
		return
	}
	id, err := challengeDigest(challenge)
	if err != nil {
		lg.Warn(err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	if err := rdb.Del(ctx, constants.RedisKeyPrefixRegistrationOwner+id, constants.RedisKeyPrefixEnrolledCeremony+id).Err(); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.ErrFailedDeleteChallenge), err.Error()))
	}
}

// The issued challenge and the one echoed in clientDataJSON may differ in base64
// padding/alphabet, so both are digested from their decoded bytes.
func challengeDigest(challenge string) (string, error) {
	decoded, err := authhelper.DecodeBase64URLWithFallback(challenge)
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedDecodeChallenge), err)
	}
	sum := sha256.Sum256(decoded)
	return hex.EncodeToString(sum[:]), nil
}

// The challenge the authenticator signed, as a registration or an assertion body echoes it.
func clientChallenge(bodyBytes []byte) (string, error) {
	var credMap map[string]any
	if err := json.Unmarshal(bodyBytes, &credMap); err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedParseRequestBody), err)
	}
	response, ok := credMap[constants.WebAuthnKeyResponse].(map[string]any)
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrInvalidResponseStructure))
	}
	clientDataJSONB64, ok := response[constants.WebAuthnKeyClientDataJSON].(string)
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrMissingClientDataJSON))
	}
	clientData, err := parseClientData(clientDataJSONB64)
	if err != nil {
		return constants.EmptyString, err
	}
	challenge, ok := clientData[constants.WebAuthnKeyChallenge].(string)
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrMissingChallengeInClientData))
	}
	return challenge, nil
}

func parseClientData(clientDataJSONB64 string) (map[string]any, error) {
	clientDataJSON, err := authhelper.DecodeBase64URLWithFallback(clientDataJSONB64)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedDecodeClientDataJSON), err)
	}

	var clientData map[string]any
	if err := json.Unmarshal(clientDataJSON, &clientData); err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedParseClientDataJSON), err)
	}
	return clientData, nil
}
