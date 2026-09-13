package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/resources/users"
)

var jitLg = constants.GetLogger(constants.LoggerPrefixAuthService)

func JitProvisionUserByEmail(
	userClient *userclient.Client, email string,
) (*userresource.UserAsResource, error) {
	if !config.IsSelfRegistrationEnabled() {
		jitLg.Info(fmt.Sprintf(string(constants.LogJITSelfRegistrationBlock), shared.IdentityHash(email)))
		return nil, errors.New(string(constants.ErrSelfRegistrationDisabled))
	}
	username, err := BuildUsername(email)
	if err != nil {
		return nil, err
	}
	resp := userClient.CreateUser(buildJitUser(email, username))
	switch resp.Status {
	case http.StatusCreated, http.StatusOK:
		return resolveExistingByEmail(userClient, email)
	case http.StatusConflict:
		existing, fetchErr := resolveExistingByEmail(userClient, email)
		if fetchErr != nil {
			return nil, fetchErr
		}
		repairRoleIfMissingByEmail(existing, userClient, email)
		return existing, nil
	default:
		return nil, fmt.Errorf(string(constants.ErrFailedCreateUser),
			shared.IdentityHash(email), resp.Status, resp.Message)
	}
}

func buildJitUser(email, username string) *userresource.UserAsResource {
	roleID := ResolveInitialRoleID(email)
	return &userresource.UserAsResource{
		Username:         username,
		Fullname:         BuildFullnameFromEmail(email),
		Email:            email,
		CreationDate:     time.Now().UTC().Format(time.RFC3339),
		Status:           userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)},
		AssignedRolesIDs: []*string{&roleID},
	}
}

func resolveExistingByEmail(
	userClient *userclient.Client, email string,
) (*userresource.UserAsResource, error) {
	existing, fetchErr := userClient.GetUserByEmail(email)
	if fetchErr != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedGetUser),
			shared.IdentityHash(email), fetchErr.Error())
	}
	if existing == nil {
		return nil, errors.New(string(constants.ErrUserNotFound))
	}
	return existing, nil
}

func repairRoleIfMissingByEmail(
	user *userresource.UserAsResource, userClient *userclient.Client, email string,
) {
	if len(user.AssignedRolesIDs) != constants.DefaultInitValue {
		return
	}
	if err := RepairMissingRole(user, userClient, email); err != nil {
		jitLg.Error(err.Error())
		return
	}
	jitLg.Info(fmt.Sprintf(string(constants.LogJIT409RoleRepair), shared.IdentityHash(email)))
}
