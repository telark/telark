package group

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/finalizers"
	groupdata "github.com/telark/data/resources/group"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	"github.com/telark/exporter/internal/membership"
	notiftypes "github.com/telark/exporter/internal/types/notifications"
	"github.com/telark/exporter/internal/utils/concurrency"
	notifdispatch "github.com/telark/exporter/internal/utils/notifications"
	"github.com/telark/exporter/internal/utils/performance"
	grouputils "github.com/telark/exporter/internal/utils/resources/group"
	resourcesutils "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateGroupResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpecFor[groupdata.GroupAsResource](w, r)
		if err != nil {
			return
		}

		// A new group may carry roles or members only from a caller who could attach them afterwards.
		if !authz.GuardGroupRolesPatch(w, r, nil, body) || !authz.GuardGroupMembersPatch(w, r, &groupdata.GroupAsResource{}, body) {
			return
		}

		group, err := grouputils.ExtractGroupSpecFromRequestBody(body)
		if err != nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusBadRequest,
				response.OperationError,
				err.Error(),
				nil,
				err,
			)
			return
		}

		if err := grouputils.ValidateAndPrepareGroup(group, w); err != nil {
			return
		}

		if !authz.GuardReferencedIDs(w, constants.ResourceUser, group.AssignedUsersIDs) ||
			!authz.GuardReferencedIDs(w, constants.ResourceRole, group.AssignedRolesIDs) {
			return
		}
		if !mirrorMembers(w, r, optimizer, group.ID, group.AssignedUsersIDs, nil) {
			return
		}

		createGroupResource(w, group, optimizer)
		// The members' grants were retired before the group existed; the next
		// lookup must see it.
		authz.BumpGeneration(r.Context())
	}
}

// Counterparts first: see membership.MirrorGroupMembers.
func mirrorMembers(w http.ResponseWriter, r *http.Request, optimizer *performance.Optimizer, groupID string, added, removed []string) bool {
	if err := membership.MirrorGroupMembers(r.Context(), optimizer, groupID, added, removed); err != nil {
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
		return false
	}
	return true
}

func createGroupResource(w http.ResponseWriter, group *groupdata.GroupAsResource, optimizer *performance.Optimizer) {
	spec, err := sharedutils.StructToSpecMap(group)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return
	}

	lock := concurrency.GetLock(group.ID)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericCreateCustomResourceWithFinalizers(
		w,
		metadata.GroupAsResourceMetadata,
		group.ID,
		spec,
		[]string{finalizers.GroupCleanup},
	)

	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceGroup)
}

func GetGroupByIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		resource, ok := grouputils.FindGroupByIDOrRespond(w, groupID)
		if !ok {
			return
		}

		hidden, ok := hiddenMembers(w, r)
		if !ok {
			return
		}
		grouputils.StripMembers(resource, hidden)
		resourcesutils.SendFilteredResourceResponse(w, resource)
	}
}

func ListGroupResourcesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		hidden, ok := hiddenMembers(w, r)
		if !ok {
			return
		}
		if hidden == nil {
			generics.GenericListCustomResources(w, metadata.GroupAsResourceMetadata)
			return
		}
		generics.GenericListCustomResourcesKeeping(w, metadata.GroupAsResourceMetadata, func(item *unstructured.Unstructured) bool {
			grouputils.StripMembers(item, hidden)
			return true
		})
	}
}

// Administrators are not listed as members to a restricted caller; nil means
// nothing to strip.
func hiddenMembers(w http.ResponseWriter, r *http.Request) (map[string]bool, bool) {
	if !authz.Restricted(r) {
		return nil, true
	}
	hidden, err := authz.HiddenUserIDs()
	if err != nil {
		responseutils.LogAndSendResponse(
			w, http.StatusServiceUnavailable, response.OperationUnavailable, string(constants.ErrResourceLookupFailed), nil, err,
		)
		return nil, false
	}
	return hidden, true
}

func PatchGroupByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		existingGroup, ok := grouputils.GetExistingGroupForPatch(w, groupID)
		if !ok {
			return
		}

		body, err := sharedutils.GetSpecFor[groupdata.GroupAsResource](w, r)
		if err != nil {
			return
		}

		addedMembers, removedMembers, ok := guardGroupPatch(w, r, existingGroup, body)
		if !ok {
			return
		}

		_, membersPatched := body[constants.FieldAssignedUsersIDs]
		oldMembers := append([]string(nil), existingGroup.AssignedUsersIDs...)
		groupName := existingGroup.Name

		if !grouputils.ExtractAndMergeGroupForPatch(existingGroup, body, w) {
			return
		}

		if !mirrorMembers(w, r, optimizer, groupID, addedMembers, removedMembers) {
			return
		}
		ok2 := patchGroupResource(w, groupID, body, optimizer)
		// Membership and role changes both alter the grants of an unknown set
		// of users, so every cached grant is retired rather than one user's.
		authz.BumpGeneration(r.Context())
		if ok2 && membersPatched {
			emitGroupMembershipChanged(groupID, groupName, oldMembers, body)
		}
	}
}

func guardGroupPatch(
	w http.ResponseWriter, r *http.Request, existing *groupdata.GroupAsResource, body map[string]any,
) (addedMembers, removedMembers []string, ok bool) {
	if !authz.GuardNotTerminating(w, r, existing.DeletionTimestamp) ||
		!authz.GuardGroupRolesPatch(w, r, existing.AssignedRolesIDs, body) ||
		!authz.GuardGroupMembersPatch(w, r, existing, body) {
		return nil, nil, false
	}

	newMembers := notifdispatch.ExtractNewStringIDsFromBody(body, constants.FieldAssignedUsersIDs)
	addedMembers, removedMembers = notifdispatch.DiffStringSlices(existing.AssignedUsersIDs, newMembers)
	newRoles := notifdispatch.ExtractNewStringIDsFromBody(body, constants.FieldAssignedRolesIDs)
	addedRoles, _ := notifdispatch.DiffStringSlices(existing.AssignedRolesIDs, newRoles)
	if !authz.GuardReferencedIDs(w, constants.ResourceUser, addedMembers) ||
		!authz.GuardReferencedIDs(w, constants.ResourceRole, addedRoles) {
		return nil, nil, false
	}
	return addedMembers, removedMembers, true
}

func patchGroupResource(w http.ResponseWriter, groupID string, body map[string]any, optimizer *performance.Optimizer) bool {
	resourcesutils.AddLastUpdateDateToPatchBody(body)
	specPatchData := map[string]any{
		constants.SpecField: body,
	}

	lock := concurrency.GetLock(groupID)
	lock.Lock()
	defer lock.Unlock()

	rc := sharedutils.NewResponseCapture(w)
	generics.GenericPatchCustomResource(rc, metadata.GroupAsResourceMetadata, groupID, specPatchData)
	resourcesutils.InvalidateResourceCaches(optimizer, constants.ResourceGroup, groupID)
	return rc.Status() == http.StatusOK
}

func emitGroupMembershipChanged(groupID, groupName string, oldMembers []string, body map[string]any) {
	newMembers := notifdispatch.ExtractNewStringIDsFromBody(body, constants.FieldAssignedUsersIDs)
	added, removed := notifdispatch.DiffStringSlices(oldMembers, newMembers)
	for _, userID := range added {
		emitGroupChange(userID, groupID, groupName, notiftypes.GroupActionAdded)
	}
	for _, userID := range removed {
		emitGroupChange(userID, groupID, groupName, notiftypes.GroupActionRemoved)
	}
}

func emitGroupChange(userID, groupID, groupName, action string) {
	notifdispatch.Emit(notiftypes.Notification{
		UserID:   userID,
		Type:     notiftypes.TypeGroupMembershipChanged,
		Title:    titleForGroupAction(action),
		Message:  messageForGroupAction(action, groupName),
		Severity: notiftypes.SeverityInfo,
		Metadata: map[string]any{
			notiftypes.MetaKeyTargetID:  groupID,
			notiftypes.MetaKeyGroupID:   groupID,
			notiftypes.MetaKeyGroupName: groupName,
			notiftypes.MetaKeyAction:    action,
		},
	})
}

func titleForGroupAction(action string) string {
	if action == notiftypes.GroupActionAdded {
		return "Added to group"
	}
	return "Removed from group"
}

func messageForGroupAction(action, groupName string) string {
	if action == notiftypes.GroupActionAdded {
		return fmt.Sprintf("You have been added to the group **%s**.", groupName)
	}
	return fmt.Sprintf("You have been removed from the group **%s**.", groupName)
}

func DeleteGroupByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		_, ok := grouputils.FindGroupByIDOrRespond(w, groupID)
		if !ok {
			return
		}

		lock := concurrency.GetLock(groupID)
		lock.Lock()
		defer lock.Unlock()

		deleteResult := api.DeleteCustomResourceByName(groupID, metadata.GroupAsResourceMetadata)
		resourcesutils.InvalidateResourceCaches(optimizer, constants.ResourceGroup, groupID)
		authz.BumpGeneration(r.Context())
		if deleteResult.Status != http.StatusOK {
			errorMsg := sharedutils.GenerateResourceError(errors.ErrDeleteRes, groupID, deleteResult.Error)
			responseutils.LogAndSendResponse(
				w,
				deleteResult.Status,
				response.OperationError,
				errorMsg,
				nil,
				deleteResult.Error,
			)
			return
		}

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), groupID, metadata.GroupAsResourceMetadata.Kind)
		responseutils.LogAndSendResponse(
			w,
			http.StatusOK,
			response.OperationSuccess,
			msg,
			nil,
			nil,
		)
	}
}
