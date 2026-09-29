package auth

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/users"
)

func HasAdminRole(roleIDs []*string) bool {
	return slices.ContainsFunc(roleIDs, func(rid *string) bool {
		return rid != nil && *rid == constants.BuiltInRoleAdmin
	})
}

func RepairMissingRole(user *userresource.User, userClient *userclient.Client, roleID string) error {
	if len(user.RoleRefs) > constants.DefaultInitValue {
		return nil
	}
	resp := userClient.PatchUserByID(user.ID, map[string]any{constants.SpecFieldRoleRefs: []*string{&roleID}})
	if resp.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrFailedRoleRepair), shared.IdentityHash(user.Email), resp.Status)
	}
	return nil
}
