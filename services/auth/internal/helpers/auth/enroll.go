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
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var (
	issueInviteScript   = redis.NewScript(constants.ScriptIssueInvite)
	revokeInviteScript  = redis.NewScript(constants.ScriptRevokeInvite)
	consumeEnrollScript = redis.NewScript(constants.ScriptConsumeEnrollToken)
)

// Redis is untrusted, so it holds only the digest: a key read from it opens no registration.
func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
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

	if err := rdb.Set(ctx, constants.RedisKeyPrefixEnrollToken+tokenDigest(token), userID, ttl).Err(); err != nil {
		return constants.EmptyString, time.Time{}, fmt.Errorf(string(constants.ErrFailedStoreEnrollToken), err.Error())
	}
	return token, time.Now().UTC().Add(ttl), nil
}

// An invite is an enrollment token another user issues for targetID. Each user has
// at most one live link, so issuing replaces the previous one in the same step.
func IssueInvite(targetID, issuerID string) (token string, expiresAt time.Time, err error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, time.Time{}, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	token, err = shared.GenerateSessionToken()
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	ttl := config.EnrollInviteTTL()
	issuedAt := time.Now().UTC()
	digest := tokenDigest(token)
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	keys := []string{constants.RedisKeyPrefixInviteOf + targetID, constants.RedisKeyPrefixInvite + digest}
	if err := issueInviteScript.Run(ctx, rdb, keys, constants.RedisKeyPrefixInvite, targetID, digest, int64(ttl/time.Second)).Err(); err != nil {
		return constants.EmptyString, time.Time{}, fmt.Errorf(string(constants.ErrFailedStoreEnrollToken), err.Error())
	}
	// A failed write answers without the token, so the digest just stored opens nothing.
	expiresAt = issuedAt.Add(ttl)
	invite := &userresource.Invite{
		IssuedAt:  issuedAt.Format(constants.TimeFormatRFC3339),
		ExpiresAt: expiresAt.Format(constants.TimeFormatRFC3339),
		IssuedBy:  issuerID,
	}
	if err := setInvite(targetID, invite); err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	if hasPasskeys, _ := CheckUserHasExistingPasskeys(targetID); hasPasskeys {
		notifyTarget(targetID, notificationsclient.TypeEnrollLinkCreated, constants.NoticeEnrollLinkCreatedTitle,
			fmt.Sprintf(string(constants.NoticeEnrollLinkCreatedMessage), issuerName(issuerID)))
	}
	return token, expiresAt, nil
}

func RevokeInvite(targetID string) error {
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
	return setInvite(targetID, nil)
}

// A stored passkey ends a pending invite however it was enrolled: a link issued while the account
// had none must not add another later. Its owner hears about a passkey added through a link.
func CompleteInvite(user *userresource.User, enrolled bool) {
	if user.Status.Invite == nil {
		return
	}
	if err := RevokeInvite(user.ID); err != nil {
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

// The token is consumed atomically on first presentation, whatever the ceremony
// then does: two concurrent starts can never both be authorized by it.
func ResolveEnrollToken(token string) (string, error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return constants.EmptyString, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()

	digest := tokenDigest(token)
	keys := []string{constants.RedisKeyPrefixEnrollToken + digest, constants.RedisKeyPrefixInvite + digest}
	userID, err := consumeEnrollScript.Run(ctx, rdb, keys, constants.RedisKeyPrefixInviteOf, digest).Text()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return constants.EmptyString, errors.New(string(constants.ErrEnrollTokenInvalid))
		}
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedGetEnrollToken), err.Error())
	}
	return userID, nil
}

// No phase in the status: the exporter merges it, so phase and lastLoginAt stay as stored.
func setInvite(userID string, invite *userresource.Invite) error {
	resp := clients.GetUserClient().PatchUserByID(userID, map[string]any{
		constants.UserFieldStatus: map[string]any{constants.UserStatusFieldInvite: invite},
	})
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

func issuerName(issuerID string) string {
	issuer, err := clients.GetUserClient().GetUserByID(issuerID)
	if err != nil || issuer.Fullname == constants.EmptyString {
		return string(constants.NoticeIssuerUnknown)
	}
	return issuer.Fullname
}
