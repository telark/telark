package group

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/finalizers"
	groupdata "github.com/telark/data/resources/group"
	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/constants"
	"github.com/telark/exporter/exporters/generics"
	"github.com/telark/exporter/handlers/resources/shared"
	notiftypes "github.com/telark/exporter/types/notifications"
	"github.com/telark/exporter/utils/concurrency"
	notifdispatch "github.com/telark/exporter/utils/notifications"
	"github.com/telark/exporter/utils/performance"
	grouputils "github.com/telark/exporter/utils/resources/group"
	resourcesutils "github.com/telark/exporter/utils/resources/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CreateGroupResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
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

		createGroupResource(w, group, optimizer)
	}
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

	cache.SmartInvalidateListCache(optimizer, constants.ResourceGroup, string(constants.OpCreate))
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

		resourcesutils.SendFilteredResourceResponse(w, resource)
	}
}

func ListGroupResourcesWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.ListResourceWithCacheInvalidation(optimizer, metadata.GroupAsResourceMetadata)
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

		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		_, membersPatched := body[constants.FieldAssignedUsersIDs]
		oldMembers := append([]string(nil), existingGroup.AssignedUsersIDs...)
		groupName := existingGroup.Name

		if !grouputils.ExtractAndMergeGroupForPatch(existingGroup, body, w) {
			return
		}

		ok2 := patchGroupResource(w, groupID, body, optimizer)
		if ok2 && membersPatched {
			emitGroupMembershipChanged(groupID, groupName, oldMembers, body)
		}
	}
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

		resourcesutils.InvalidateResourceCaches(optimizer, constants.ResourceGroup, groupID)
		lock := concurrency.GetLock(groupID)
		lock.Lock()
		defer lock.Unlock()

		deleteResult := api.DeleteCustomResourceByName(groupID, metadata.GroupAsResourceMetadata)
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
