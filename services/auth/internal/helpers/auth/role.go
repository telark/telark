package auth

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/users"
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

// Decided on the email the identity provider verified, never the editable stored
// one. Idempotent: patches only what the record lacks (marker added after the fact).
func EnsureBootstrapAdmin(user *userresource.User, verifiedEmail string, userClient *userclient.Client) {
	if !config.IsBootstrapAdmin(verifiedEmail) {
		return
	}
	patch := map[string]any{}
	if !HasAdminRole(user.RoleRefs) {
		adminID := constants.BuiltInRoleAdmin
		patch[constants.SpecFieldRoleRefs] = append(slices.Clone(user.RoleRefs), &adminID)
	}
	if !user.Bootstrap {
		patch[constants.UserFieldBootstrap] = true
	}
	if len(patch) == constants.DefaultInitValue {
		return
	}
	identityHash := shared.IdentityHash(user.Email)
	if resp := userClient.PatchUserByID(user.ID, patch); resp.Status != http.StatusOK {
		lg.Error(fmt.Sprintf(string(constants.ErrOIDCAdminPromotionFailed), identityHash, resp.Status))
		return
	}
	lg.Info(fmt.Sprintf(string(constants.LogOIDCAdminPromoted), identityHash))
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
