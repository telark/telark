package auth

import (
	"fmt"
	"net/http"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/resources/users"
)

func ResolveInitialRoleID(email string) string {
	if config.IsBootstrapAdmin(email) {
		return constants.BuiltInRoleAdmin
	}
	return constants.BuiltInRoleReadOnly
}

func RepairMissingRole(user *userresource.UserAsResource, userClient *userclient.Client, email string) error {
	if len(user.AssignedRolesIDs) > constants.DefaultInitValue {
		return nil
	}
	roleID := ResolveInitialRoleID(email)
	resp := userClient.PatchUserByID(user.ID, map[string]any{"assignedRolesIDs": []*string{&roleID}})
	if resp.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrFailedRoleRepair), shared.IdentityHash(email), resp.Status)
	}
	return nil
}
