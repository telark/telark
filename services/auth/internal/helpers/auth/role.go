package auth

import (
	"fmt"
	"net/http"
	"slices"

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

func HasAdminRole(roleIDs []*string) bool {
	return slices.ContainsFunc(roleIDs, func(rid *string) bool {
		return rid != nil && *rid == constants.BuiltInRoleAdmin
	})
}

func RepairMissingRole(user *userresource.UserAsResource, userClient *userclient.Client, email string) error {
	if len(user.AssignedRolesIDs) > constants.DefaultInitValue {
		return nil
	}
	roleID := ResolveInitialRoleID(email)
	resp := userClient.PatchUserByID(user.ID, map[string]any{constants.SpecFieldAssignedRolesIDs: []*string{&roleID}})
	if resp.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrFailedRoleRepair), shared.IdentityHash(email), resp.Status)
	}
	return nil
}
