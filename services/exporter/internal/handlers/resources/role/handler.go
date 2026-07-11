package role

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/finalizers"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/constants"
	"github.com/telark/exporter/exporters/generics"
	"github.com/telark/exporter/handlers/resources/shared"
	"github.com/telark/exporter/utils/concurrency"
	"github.com/telark/exporter/utils/performance"
	roleutils "github.com/telark/exporter/utils/resources/role"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CreateRoleResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		role, err := roleutils.ExtractRoleSpecFromRequestBody(body)
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

		if err := roleutils.ValidateAndPrepareRole(role, w); err != nil {
			return
		}

		roleutils.ComputeAndSetPriority(role)
		if role.Type != roledata.RoleTypeBuiltIn {
			if err := roleutils.ValidatePriorityCapOrRespond(w, role.Priority); err != nil {
				return
			}
		}
		roleutils.ComputeAndSetVersion(role)
		createRoleResource(w, role, optimizer)
	}
}

func createRoleResource(w http.ResponseWriter, role *roledata.RoleAsResource, optimizer *performance.Optimizer) {
	spec, err := sharedutils.StructToSpecMap(role)
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

	// Ensure validity.autoRevoke is always present for temporary roles
	if role.Validity != nil && role.Validity.Type == roledata.ValidityTypeTemporary {
		if validitySpec, ok := spec["validity"].(map[string]any); ok {
			validitySpec["autoRevoke"] = role.Validity.AutoRevoke
		}
	}

	lock := concurrency.GetLock(role.ID)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericCreateCustomResourceWithFinalizers(
		w,
		metadata.RoleAsResourceMetadata,
		role.ID,
		spec,
		[]string{finalizers.RoleCleanup},
	)

	cache.SmartInvalidateListCache(optimizer, constants.ResourceRole, string(constants.OpCreate))
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceRole)
}

func GetRoleByIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		roleID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		resource, ok := roleutils.FindRoleByIDOrRespond(w, roleID)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func GetRoleByUserIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.UserIDParam)
		if err != nil {
			return
		}

		resource, ok := roleutils.FindRoleByUserIDOrRespond(w, userID)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func GetRoleByGroupIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, err := sharedutils.GetPathParam(w, r, constants.GroupIDParam)
		if err != nil {
			return
		}

		resource, ok := roleutils.FindRoleByGroupIDOrRespond(w, groupID)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func ListRoleResourcesWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.ListResourceWithCacheInvalidation(optimizer, metadata.RoleAsResourceMetadata)
}

func ListRolesByUserIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.UserIDParam)
		if err != nil {
			return
		}

		resources, ok := roleutils.ListRolesByUserIDOrRespond(w, userID)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourcesResponse(w, resources)
	}
}

func ListRolesByGroupIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID, err := sharedutils.GetPathParam(w, r, constants.GroupIDParam)
		if err != nil {
			return
		}

		resources, ok := roleutils.ListRolesByGroupIDOrRespond(w, groupID)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourcesResponse(w, resources)
	}
}

func PatchRoleByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		roleID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		existingRole, ok := roleutils.GetExistingRoleForPatch(w, roleID)
		if !ok {
			return
		}

		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		if !roleutils.ValidatePatchRequest(existingRole, body, w) {
			return
		}

		if !roleutils.ExtractAndMergeRoleForPatch(existingRole, body, w) {
			return
		}

		patchRoleResource(w, roleID, body, optimizer)
	}
}

func patchRoleResource(w http.ResponseWriter, roleID string, body map[string]any, optimizer *performance.Optimizer) {
	resourcesshared.AddLastUpdateDateToPatchBody(body)
	specPatchData := map[string]any{
		constants.SpecField: body,
	}

	lock := concurrency.GetLock(roleID)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericPatchCustomResource(w, metadata.RoleAsResourceMetadata, roleID, specPatchData)
	resourcesshared.InvalidateResourceCaches(optimizer, constants.ResourceRole, roleID)
}

func DeleteRoleByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		roleID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		_, ok := roleutils.FindRoleByIDOrRespond(w, roleID)
		if !ok {
			return
		}

		resourcesshared.InvalidateResourceCaches(optimizer, constants.ResourceRole, roleID)
		lock := concurrency.GetLock(roleID)
		lock.Lock()
		defer lock.Unlock()

		deleteResult := api.DeleteCustomResourceByName(roleID, metadata.RoleAsResourceMetadata)
		if deleteResult.Status != http.StatusOK {
			errorMsg := sharedutils.GenerateResourceError(errors.ErrDeleteRes, roleID, deleteResult.Error)
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

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), roleID, metadata.RoleAsResourceMetadata.Kind)
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
