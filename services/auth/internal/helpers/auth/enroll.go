package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/messages"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notificationsclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var (
	issueInviteScript      = redis.NewScript(constants.ScriptIssueInvite)
	revokeInviteScript     = redis.NewScript(constants.ScriptRevokeInvite)
	consumeEnrollScript    = redis.NewScript(constants.ScriptConsumeEnrollToken)
	dropEnrollTokensScript = redis.NewScript(constants.ScriptDropEnrollTokens)
)

// Redis is untrusted, so it holds only the digest: a key read from it opens no registration.
func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// An enrollment token lets a browser on another host open a registration for
// the signed-in user: random, bound to that user, short-lived and single use.
func CreateEnrollToken(userID string) (token string, expiresAt time.Time, err error) {
	if err := RefuseFrozen(userID); err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, time.Time{}, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	token, err = shared.GenerateSessionToken()
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	ttl := time.Duration(constants.RedisTTLEnrollToken) * time.Second
	key := constants.RedisKeyPrefixEnrollToken + tokenDigest(token)
	entry, err := redishelper.SignEntry(key, userID, ttl)
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	// Indexed by user, so a break-glass recovery can drop the links a session minted (DropLinks).
	ownedBy := constants.RedisKeyPrefixEnrollTokensOf + userID
	_, err = rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, key, entry, ttl)
		pipe.SAdd(ctx, ownedBy, tokenDigest(token))
		pipe.Expire(ctx, ownedBy, ttl)
		return nil
	})
	if err != nil {
		return constants.EmptyString, time.Time{}, fmt.Errorf(string(constants.ErrFailedStoreEnrollToken), err.Error())
	}
	return token, time.Now().UTC().Add(ttl), nil
}

// Every link of the account: the invite (break-glass's) and the links its own sessions minted.
func DropLinks(userID string) error {
	if err := DropInviteKeys(userID); err != nil {
		return err
	}
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	keys := []string{constants.RedisKeyPrefixEnrollTokensOf + userID}
	if err := dropEnrollTokensScript.Run(ctx, rdb, keys, constants.RedisKeyPrefixEnrollToken).Err(); err != nil {
		return fmt.Errorf(string(constants.ErrFailedDropEnrollTokens), shared.IdentityHash(userID), err)
	}
	return nil
}

// An invite is an enrollment token another user, or break-glass, issues for targetID. Each
// user has at most one live link, so issuing replaces the previous one in the same step.
func IssueInvite(targetID, issuerID string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, time.Time{}, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	token, err = shared.GenerateSessionToken()
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	issuedAt := time.Now().UTC()
	digest := tokenDigest(token)
	inviteKey := constants.RedisKeyPrefixInvite + digest
	entry, err := redishelper.SignEntry(inviteKey, targetID, ttl)
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	keys := []string{constants.RedisKeyPrefixInviteOf + targetID, inviteKey}
	if err := issueInviteScript.Run(ctx, rdb, keys, constants.RedisKeyPrefixInvite, entry, digest, int64(ttl/time.Second)).Err(); err != nil {
		return constants.EmptyString, time.Time{}, fmt.Errorf(string(constants.ErrFailedStoreEnrollToken), err.Error())
	}
	// A failed write answers without the token, so the digest just stored opens nothing.
	expiresAt = issuedAt.Add(ttl)
	invite := &userresource.Invite{
		IssuedAt:  issuedAt.Format(constants.TimeFormatRFC3339),
		ExpiresAt: expiresAt.Format(constants.TimeFormatRFC3339),
		IssuedBy:  issuerID,
	}
	if err := setInvite(targetID, invite, constants.EmptyString); err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	if hasPasskeys, _ := CheckUserHasExistingPasskeys(targetID); hasPasskeys {
		notifyTarget(targetID, notificationsclient.TypeEnrollLinkCreated, constants.NoticeEnrollLinkCreatedTitle,
			fmt.Sprintf(string(constants.NoticeEnrollLinkCreatedMessage), issuerName(issuerID, targetID)))
	}
	return token, expiresAt, nil
}

func RevokeInvite(targetID string) error {
	return closeInvite(targetID, constants.EmptyString)
}

// acceptedAt, when set, is stored in the same write that clears the invite.
func closeInvite(targetID, acceptedAt string) error {
	if err := DropInviteKeys(targetID); err != nil {
		return err
	}
	return setInvite(targetID, nil, acceptedAt)
}

// While break-glass recovers an account, nothing starts or finishes a sign-in or a registration for it, no session
// mints a link, and no passkey, identity or session is written for it (auth checks right before each write). It lapses
// on its own if break-glass stops halfway.
func Freeze(userID string) error {
	return frozenKey(userID, func(ctx context.Context, rdb *redis.Client, key string) error {
		return rdb.Set(ctx, key, constants.RedisChallengeEntry, time.Duration(constants.RedisTTLRecoveryFreeze)*time.Second).Err()
	})
}

// Extends the freeze, and fails if it lapsed: a recovery whose freeze had a gap can't promise what it revoked.
func Refreeze(userID string) error {
	return frozenKey(userID, func(ctx context.Context, rdb *redis.Client, key string) error {
		kept, err := rdb.Expire(ctx, key, time.Duration(constants.RedisTTLRecoveryFreeze)*time.Second).Result()
		if err == nil && !kept {
			err = errors.New(string(constants.ErrRecoveryFreezeLost))
		}
		return err
	})
}

func Unfreeze(userID string) error {
	return frozenKey(userID, func(ctx context.Context, rdb *redis.Client, key string) error {
		return rdb.Del(ctx, key).Err()
	})
}

// A Redis that can't answer refuses too.
func RefuseFrozen(userID string) error {
	return frozenKey(userID, func(ctx context.Context, rdb *redis.Client, key string) error {
		frozen, err := rdb.Exists(ctx, key).Result()
		if err == nil && frozen > constants.DefaultInitValue {
			return errors.New(string(constants.ErrAccountRecovering))
		}
		return err
	})
}

func frozenKey(userID string, op func(context.Context, *redis.Client, string) error) error {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()
	return op(ctx, rdb, constants.RedisKeyPrefixFrozen+userID)
}

// The link alone, without the record: a user being deleted keeps no invite to clear.
func DropInviteKeys(targetID string) error {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	keys := []string{constants.RedisKeyPrefixInviteOf + targetID}
	if err := revokeInviteScript.Run(ctx, rdb, keys, constants.RedisKeyPrefixInvite).Err(); err != nil {
		return fmt.Errorf(string(constants.ErrFailedRevokeInvite), err.Error())
	}
	return nil
}

// A stored passkey ends a pending invite however it was enrolled (a link issued while the account had
// none must not add another later) and records when. Its owner hears about a passkey added through a link.
func CompleteInvite(user *userresource.User, enrolled bool) {
	if user.Status.Invite == nil {
		return
	}
	if err := closeInvite(user.ID, time.Now().UTC().Format(constants.TimeFormatRFC3339)); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnInviteCloseFailed), shared.IdentityHash(user.ID), err))
	}
	if !enrolled {
		return
	}
	passkeys, err := clients.GetPasskeyClient().GetAllPasskeysByUser(user.ID)
	if err != nil || len(passkeys) > constants.DefaultIncrementValue {
		notifyTarget(user.ID, notificationsclient.TypeEnrollLinkUsed, constants.NoticeEnrollLinkUsedTitle,
			string(constants.NoticeEnrollLinkUsedMessage))
	}
}

// Opening a registration only reads the token: a canceled or failed ceremony leaves the link usable.
func ResolveEnrollToken(token string) (userID, digest string, err error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, constants.EmptyString, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	digest = tokenDigest(token)
	key := constants.RedisKeyPrefixEnrollToken + digest
	entry, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		key = constants.RedisKeyPrefixInvite + digest
		entry, err = rdb.Get(ctx, key).Result()
	}
	if errors.Is(err, redis.Nil) {
		return constants.EmptyString, constants.EmptyString, errors.New(string(constants.ErrEnrollTokenInvalid))
	}
	if err != nil {
		return constants.EmptyString, constants.EmptyString, fmt.Errorf(string(constants.ErrFailedGetEnrollToken), err.Error())
	}
	userID, ok := redishelper.VerifyEntry(key, entry)
	if !ok {
		return constants.EmptyString, constants.EmptyString, errors.New(string(constants.ErrEnrollTokenInvalid))
	}
	return userID, digest, nil
}

// The finish that saves a passkey spends the token atomically, so of the ceremonies one link
// opened only one saves a passkey with it, and only for the account the token names.
func SpendEnrollToken(userID, digest string) error {
	// No link opened the ceremony.
	if digest == constants.EmptyString {
		return nil
	}
	rdb := redishelper.GetClient()
	if rdb == nil {
		return errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	keys := []string{constants.RedisKeyPrefixEnrollToken + digest, constants.RedisKeyPrefixInvite + digest}
	spent, err := consumeEnrollScript.Run(ctx, rdb, keys, constants.RedisKeyPrefixInviteOf, digest, userID).StringSlice()
	if errors.Is(err, redis.Nil) {
		return errors.New(string(constants.ErrEnrollTokenInvalid))
	}
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedGetEnrollToken), err.Error())
	}
	entry, key := spent[constants.DefaultInitValue], spent[constants.DefaultIncrementValue]
	if owner, ok := redishelper.VerifyEntry(key, entry); !ok || owner != userID {
		return errors.New(string(constants.ErrEnrollTokenInvalid))
	}
	return nil
}

// No phase in the status: the exporter merges it, so phase and lastLoginAt stay as stored.
func setInvite(userID string, invite *userresource.Invite, acceptedAt string) error {
	status := map[string]any{constants.UserStatusFieldInvite: invite}
	if acceptedAt != constants.EmptyString {
		status[constants.UserStatusFieldInviteAccepted] = acceptedAt
	}
	resp := clients.GetUserClient().PatchUserByID(userID, map[string]any{constants.UserFieldStatus: status})
	if resp.Status == http.StatusOK {
		return nil
	}
	// A record deleted (404) or being deleted (410) has no invite left to clear.
	if invite == nil && (resp.Status == http.StatusNotFound || resp.Status == http.StatusGone) {
		return nil
	}
	return fmt.Errorf(string(constants.ErrFailedSetInvite), shared.IdentityHash(userID), resp.Status)
}

// A failed notice never fails the link it reports.
func notifyTarget(userID, notificationType string, title messages.Message, message string) {
	resp := clients.GetNotificationsClient().Emit(context.Background(), notificationsclient.Notification{
		UserID:   userID,
		Type:     notificationType,
		Title:    string(title),
		Message:  message,
		Severity: notificationsclient.SeverityWarning,
	})
	if resp.Status >= http.StatusBadRequest {
		lg.Warn(fmt.Sprintf(string(constants.WarnEnrollNoticeFailed), shared.IdentityHash(userID)))
	}
}

// Only the bootstrap account itself reads its own name.
func issuerName(issuerID, targetID string) string {
	// Break-glass passes no issuer: an operator ran it, not a user.
	if issuerID == constants.EmptyString {
		return string(constants.NoticeIssuerBreakGlass)
	}
	issuer, err := clients.GetUserClient().GetUserByID(issuerID)
	switch {
	case err != nil || issuer.Fullname == constants.EmptyString:
		return string(constants.NoticeIssuerUnknown)
	case issuer.Bootstrap && issuerID != targetID:
		return string(constants.NoticeIssuerBootstrap)
	}
	return issuer.Fullname
}
