package cleanup

import (
	"context"
	"fmt"
	"net/http"

	authclients "github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/data/resources/finalizers"
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/rest/response"
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
		List:            listRoles,
		Patch:           patchRole,
		AddFinalizer:    addRoleFinalizer,
		RemoveFinalizer: removeRoleFinalizer,
	},
}

func listUsers(_ context.Context) ([]*resourcesshared.CleanupView, error) {
	return authclients.GetUserClient().ListCleanupViews()
}

func listGroups(_ context.Context) ([]*resourcesshared.CleanupView, error) {
	return authclients.GetGroupClient().ListCleanupViews()
}

func listRoles(_ context.Context) ([]*resourcesshared.CleanupView, error) {
	return authclients.GetRoleClient().ListCleanupViews()
}

func patchUser(_ context.Context, id string, body map[string]any) *response.GenericResponse {
	return authclients.GetUserClient().PatchUserByID(id, body)
}

func patchGroup(_ context.Context, id string, body map[string]any) *response.GenericResponse {
	return authclients.GetGroupClient().PatchGroupByID(id, body)
}

func patchRole(_ context.Context, id string, body map[string]any) *response.GenericResponse {
	return authclients.GetRoleClient().PatchRoleByID(id, body)
}

func addUserFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetUserClient().AddFinalizer(id, name)
}

func addGroupFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetGroupClient().AddFinalizer(id, name)
}

func addRoleFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetRoleClient().AddFinalizer(id, name)
}

func removeUserFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetUserClient().RemoveFinalizer(id, name)
}

func removeGroupFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetGroupClient().RemoveFinalizer(id, name)
}

func removeRoleFinalizer(_ context.Context, id, name string) *response.GenericResponse {
	return authclients.GetRoleClient().RemoveFinalizer(id, name)
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
