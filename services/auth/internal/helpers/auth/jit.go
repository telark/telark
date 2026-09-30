package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	userresource "github.com/telark/telark/internal/data/resources/user"
	userclient "github.com/telark/telark/internal/rest/clients/users"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var jitLg = constants.GetLogger(constants.LoggerPrefixAuthService)

func checkSelfRegistration(email string) error {
	if !config.IsSelfRegistrationEnabled() {
		jitLg.Info(fmt.Sprintf(string(constants.LogJITSelfRegistrationBlock), shared.IdentityHash(email)))
		return errors.New(string(constants.ErrSelfRegistrationDisabled))
	}
	return nil
}

func pendingUserKey(id string) string {
	return constants.RedisKeyPrefixPendingUser + id
}

// Self-registration creates the account only once the passkey ceremony verifies:
// start parks the unsaved user under a random ID, so an abandoned start leaves nothing behind.
func storePendingUser(email string) (*userresource.User, string, error) {
	if err := checkSelfRegistration(email); err != nil {
		return nil, constants.EmptyString, err
	}
	username, err := BuildUsername(email)
	if err != nil {
		return nil, constants.EmptyString, err
	}
	user := buildJitUser(email, username)
	raw, err := json.Marshal(user)
	if err != nil {
		return nil, constants.EmptyString, fmt.Errorf(string(constants.ErrFailedStorePendingUser), err.Error())
	}
	rdb := redishelper.GetClient()
	if rdb == nil {
		return nil, constants.EmptyString, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	id, err := shared.GenerateSessionToken()
	if err != nil {
		return nil, constants.EmptyString, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()
	if err := rdb.Set(ctx, pendingUserKey(id), raw, time.Duration(constants.RedisTTLChallenge)*time.Second).Err(); err != nil {
		return nil, constants.EmptyString, fmt.Errorf(string(constants.ErrFailedStorePendingUser), err.Error())
	}
	return user, id, nil
}

func pendingUser(id string) (*userresource.User, bool, error) {
	rdb := redishelper.GetClient()
	if rdb == nil {
		return nil, false, errors.New(string(constants.ErrRedisClientUnavailable))
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.RedisChallengeOpTimeout)
	defer cancel()
	raw, err := rdb.Get(ctx, pendingUserKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf(string(constants.ErrFailedGetPendingUser), err.Error())
	}
	var user userresource.User
	if err := json.Unmarshal(raw, &user); err != nil {
		return nil, false, fmt.Errorf(string(constants.ErrFailedGetPendingUser), err.Error())
	}
	return &user, true, nil
}

// A 409 means the email was taken after start: an existing account is never handed
// to a bare-email ceremony, only to a session or an enrollment token.
func CreatePendingUser(user *userresource.User) (*userresource.User, error) {
	if err := checkSelfRegistration(user.Email); err != nil {
		return nil, err
	}
	userClient := clients.GetUserClient()
	resp := userClient.CreateUser(user)
	switch resp.Status {
	case http.StatusCreated, http.StatusOK:
		return GetUserWithErrorHandling(user.Email, userClient.GetUserByEmail)
	case http.StatusConflict:
		return nil, errors.New(string(constants.ErrRegistrationNeedsProof))
	default:
		return nil, fmt.Errorf(string(constants.ErrFailedCreateUser),
			shared.IdentityHash(user.Email), resp.Status, resp.Message)
	}
}

// A self-registration is all-or-nothing: an account whose passkey could not be
// saved is deleted again.
func DiscardPendingUser(userID string) {
	if resp := clients.GetUserClient().DeleteUserByID(userID); resp == nil || resp.Status != http.StatusOK {
		jitLg.Error(fmt.Sprintf(string(constants.ErrFailedDiscardUser), shared.IdentityHash(userID)))
	}
}

// Self-registration verifies nothing about the email, so the account starts
// ReadOnly; Admin and the bootstrap marker come only from a verified identity.
func buildJitUser(email, username string) *userresource.User {
	roleID := constants.BuiltInRoleReadOnly
	return &userresource.User{
		Username:     username,
		Fullname:     BuildFullnameFromEmail(email),
		Email:        email,
		CreationDate: time.Now().UTC().Format(time.RFC3339),
		Status:       userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)},
		RoleRefs:     []*string{&roleID},
	}
}

func RepairRoleIfMissing(user *userresource.User, userClient *userclient.Client, roleID string) {
	if len(user.RoleRefs) != constants.DefaultInitValue {
		return
	}
	if err := RepairMissingRole(user, userClient, roleID); err != nil {
		jitLg.Error(err.Error())
		return
	}
	jitLg.Info(fmt.Sprintf(string(constants.LogJIT409RoleRepair), shared.IdentityHash(user.Email)))
}
