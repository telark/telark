package group

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	"github.com/telark/telark/services/exporter/internal/membership"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	notifdispatch "github.com/telark/telark/services/exporter/internal/utils/notifications"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	grouputils "github.com/telark/telark/services/exporter/internal/utils/resources/group"
	resourcesutils "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateGroupResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpecFor[groupdata.Group](w, r)
		if err != nil {
			return
		}

		// A new group may carry roles or members only from a caller who could attach them afterwards.
		if !authz.GuardGroupRolesPatch(w, r, nil, body) || !authz.GuardGroupMembersPatch(w, r, &groupdata.Group{}, body) {
			return
		}

		resourcesutils.StampCreateAudit(r, body)
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

		hidden, ok := hiddenMembers(w, r)
		if !ok || !authz.GuardMemberIDs(w, hidden, group.UserRefs) ||
			!authz.GuardReferencedIDs(w, constants.ResourceRole, group.RoleRefs) {
			return
		}
		if !mirrorMembers(w, r, optimizer, group.ID, group.UserRefs, nil) {
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

func createGroupResource(w http.ResponseWriter, group *groupdata.Group, optimizer *performance.Optimizer) {
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
		metadata.GroupMetadata,
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
			generics.GenericListCustomResources(w, metadata.GroupMetadata)
			return
		}
		generics.GenericListCustomResourcesKeeping(w, metadata.GroupMetadata, func(item *unstructured.Unstructured) bool {
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

		body, err := sharedutils.GetSpecFor[groupdata.Group](w, r)
		if err != nil {
			return
		}

		addedMembers, removedMembers, ok := guardGroupPatch(w, r, existingGroup, body)
		if !ok {
			return
		}

		_, membersPatched := body[constants.FieldUserRefs]
		oldMembers := append([]string(nil), existingGroup.UserRefs...)
		groupName := existingGroup.Name

		resourcesutils.StampPatchAudit(r, body)
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

// A restricted caller edits the member list they were shown: every check sees
// the hidden members as absent, and the write keeps them.
func guardGroupPatch(
	w http.ResponseWriter, r *http.Request, existing *groupdata.Group, body map[string]any,
) (addedMembers, removedMembers []string, ok bool) {
	var hidden map[string]bool
	_, membersPatched := body[constants.FieldUserRefs]
	if membersPatched {
		if hidden, ok = hiddenMembers(w, r); !ok {
			return nil, nil, false
		}
	}
	shown := *existing
	shown.UserRefs = slices.DeleteFunc(slices.Clone(existing.UserRefs), func(id string) bool { return hidden[id] })

	if !authz.GuardNotTerminating(w, r, existing.DeletionTimestamp) ||
		!authz.GuardGroupRolesPatch(w, r, existing.RoleRefs, body) ||
		!authz.GuardGroupMembersPatch(w, r, &shown, body) {
		return nil, nil, false
	}

	// An absent userRefs would diff as every member removed.
	if membersPatched {
		newMembers := notifdispatch.ExtractNewStringIDsFromBody(body, constants.FieldUserRefs)
		addedMembers, removedMembers = notifdispatch.DiffStringSlices(shown.UserRefs, newMembers)
	}
	newRoles := notifdispatch.ExtractNewStringIDsFromBody(body, constants.FieldRoleRefs)
	addedRoles, _ := notifdispatch.DiffStringSlices(existing.RoleRefs, newRoles)
	if !authz.GuardMemberIDs(w, hidden, addedMembers) ||
		!authz.GuardReferencedIDs(w, constants.ResourceRole, addedRoles) ||
		!authz.GuardGroupPatchLastAdmin(w, existing, body, removedMembers) {
		return nil, nil, false
	}
	grouputils.KeepHiddenMembers(body, existing.UserRefs, hidden)
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
	generics.GenericPatchCustomResource(rc, metadata.GroupMetadata, groupID, specPatchData)
	resourcesutils.InvalidateResourceCaches(optimizer, constants.ResourceGroup, groupID)
	return rc.Status() == http.StatusOK
}

func emitGroupMembershipChanged(groupID, groupName string, oldMembers []string, body map[string]any) {
	newMembers := notifdispatch.ExtractNewStringIDsFromBody(body, constants.FieldUserRefs)
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

		existingGroup, ok := grouputils.GetExistingGroupForPatch(w, groupID)
		if !ok || !authz.GuardGroupRolesWithinCaller(w, r, existingGroup) || !authz.GuardGroupDeleteLastAdmin(w, groupID) {
			return
		}

		lock := concurrency.GetLock(groupID)
		lock.Lock()
		defer lock.Unlock()

		deleteResult := api.DeleteCustomResourceByName(groupID, metadata.GroupMetadata)
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

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), groupID, metadata.GroupMetadata.Kind)
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
