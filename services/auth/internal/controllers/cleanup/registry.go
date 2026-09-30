package cleanup

import (
	"context"
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/resources/finalizers"
	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/internal/rest/response"
	authclients "github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
)

type ResourceOps struct {
	Finalizer       string
	List            ListFn
	Patch           PatchFn
	AddFinalizer    AddFinalizerFn
	RemoveFinalizer RemoveFinalizerFn
}

func GetResourceOps(resourceType string) (ResourceOps, bool) {
	ops, ok := resourceRegistry[resourceType]
	return ops, ok
}

func RegisteredResourceTypes() []string {
	return []string{
		finalizers.ResourceTypeUsers,
		finalizers.ResourceTypeGroups,
		finalizers.ResourceTypeRoles,
	}
}

var resourceRegistry = map[string]ResourceOps{
	finalizers.ResourceTypeUsers: {
		Finalizer:       finalizers.UserCleanup,
		List:            listUsers,
		Patch:           patchUser,
		AddFinalizer:    addUserFinalizer,
		RemoveFinalizer: removeUserFinalizer,
	},
	finalizers.ResourceTypeGroups: {
		Finalizer:       finalizers.GroupCleanup,
		List:            listGroups,
		Patch:           patchGroup,
		AddFinalizer:    addGroupFinalizer,
		RemoveFinalizer: removeGroupFinalizer,
	},
	finalizers.ResourceTypeRoles: {
		Finalizer:       finalizers.RoleCleanup,
		List:            listAccessRoles,
		Patch:           patchAccessRole,
		AddFinalizer:    addAccessRoleFinalizer,
		RemoveFinalizer: removeAccessRoleFinalizer,
	},
}

func listUsers(_ context.Context) ([]*resourcesshared.CleanupView, error) {
	return authclients.GetUserClient().ListCleanupViews()
}

func listGroups(_ context.Context) ([]*resourcesshared.CleanupView, error) {
	return authclients.GetGroupClient().ListCleanupViews()
}

func listAccessRoles(_ context.Context) ([]*resourcesshared.CleanupView, error) {
	return authclients.GetAccessRoleClient().ListCleanupViews()
}

func patchUser(_ context.Context, id string, body map[string]any) *response.GenericResponse {
	return authclients.GetUserClient().PatchUserByID(id, body)
}

func patchGroup(_ context.Context, id string, body map[string]any) *response.GenericResponse {
	return authclients.GetGroupClient().PatchGroupByID(id, body)
}

func patchAccessRole(_ context.Context, id string, body map[string]any) *response.GenericResponse {
	return authclients.GetAccessRoleClient().PatchAccessRoleByID(id, body)
}

func addUserFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetUserClient().AddFinalizer(id, name)
}

func addGroupFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetGroupClient().AddFinalizer(id, name)
}

func addAccessRoleFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetAccessRoleClient().AddFinalizer(id, name)
}

func removeUserFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetUserClient().RemoveFinalizer(id, name)
}

func removeGroupFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetGroupClient().RemoveFinalizer(id, name)
}

func removeAccessRoleFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetAccessRoleClient().RemoveFinalizer(id, name)
}

// A session already gone counts as purged; any other failure requeues the job.
func purgeUserSessions(userID string) error {
	client := authclients.GetSessionClient()
	refs, err := client.ListSessionRefsByUser(userID)
	if err != nil {
		return fmt.Errorf(string(constants.ErrCleanupListSessionsFailed), userID, err)
	}
	for _, ref := range refs {
		resp := client.DeleteSessionByToken(ref)
		if resp == nil || (resp.Status != http.StatusOK && resp.Status != http.StatusNotFound) {
			status := constants.DefaultInitValue
			if resp != nil {
				status = resp.Status
			}
			return fmt.Errorf(string(constants.ErrCleanupDeleteSessionFailed), userID, ref, status)
		}
	}
	return nil
}

// Passkeys go with the account; one already gone counts as purged, any other failure requeues.
func purgeUserPasskeys(userID string) error {
	client := authclients.GetPasskeyClient()
	passkeys, err := client.GetAllPasskeysByUser(userID)
	if err != nil {
		return fmt.Errorf(string(constants.ErrCleanupListPasskeysFailed), userID, err)
	}
	for _, passkey := range passkeys {
		if passkey == nil {
			continue
		}
		resp := client.DeletePasskeyByUserAndCredentialID(userID, passkey.CredentialID, true)
		if resp == nil || (resp.Status != http.StatusOK && resp.Status != http.StatusNotFound) {
			status := constants.DefaultInitValue
			if resp != nil {
				status = resp.Status
			}
			return fmt.Errorf(string(constants.ErrCleanupDeletePasskeyFailed), userID, status)
		}
	}
	return nil
}

func purgeUser(userID string) error {
	if err := purgeUserSessions(userID); err != nil {
		return err
	}
	return purgeUserPasskeys(userID)
}
