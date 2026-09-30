package auth

import (
	"fmt"
	"net/http"
	"slices"

	userresource "github.com/telark/telark/internal/data/resources/user"
	userclient "github.com/telark/telark/internal/rest/clients/users"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
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
