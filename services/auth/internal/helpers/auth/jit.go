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
		return GetUserWithErrorHandling(email, userClient.GetUserByEmail)
	case http.StatusConflict:
		existing, fetchErr := GetUserWithErrorHandling(email, userClient.GetUserByEmail)
		if fetchErr != nil {
			return nil, fetchErr
		}
		RepairRoleIfMissing(existing, userClient, email)
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
		Bootstrap:        config.IsBootstrapAdmin(email),
	}
}

func RepairRoleIfMissing(user *userresource.UserAsResource, userClient *userclient.Client, email string) {
	if len(user.AssignedRolesIDs) != constants.DefaultInitValue {
		return
	}
	if err := RepairMissingRole(user, userClient, email); err != nil {
		jitLg.Error(err.Error())
		return
	}
	jitLg.Info(fmt.Sprintf(string(constants.LogJIT409RoleRepair), shared.IdentityHash(email)))
}
