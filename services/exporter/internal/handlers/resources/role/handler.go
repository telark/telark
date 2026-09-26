package role

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/finalizers"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	"github.com/telark/exporter/internal/handlers/resources/shared"
	"github.com/telark/exporter/internal/utils/concurrency"
	"github.com/telark/exporter/internal/utils/performance"
	roleutils "github.com/telark/exporter/internal/utils/resources/role"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
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

		if !authz.GuardRoleLevels(w, r, role.ScopesAndPermissions) {
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

	if role.Validity != nil && role.Validity.Type == roledata.ValidityTypeTemporary {
		if validitySpec, ok := spec[constants.FieldValidity].(map[string]any); ok {
			validitySpec[constants.FieldAutoRevoke] = role.Validity.AutoRevoke
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

		if !authz.GuardHiddenUserID(w, r, userID) {
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

func ListRoleResourcesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return shared.ListResourceWithCacheInvalidation(metadata.RoleAsResourceMetadata)
}

func ListRolesByUserIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.UserIDParam)
		if err != nil {
			return
		}

		if !authz.GuardHiddenUserID(w, r, userID) {
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

		if !authz.GuardNotTerminating(w, r, existingRole.DeletionTimestamp) ||
			!roleutils.ValidateProtectionFlags(existingRole, body, w) {
			return
		}

		if !guardPatchedLevels(w, r, body) {
			return
		}

		if !roleutils.ExtractAndMergeRoleForPatch(existingRole, body, w) {
			return
		}

		patchRoleResource(w, roleID, body, optimizer)
		// A role edit changes the permissions of every user holding it,
		// directly or through a group, so no single user's grants can be
		// targeted for invalidation.
		authz.BumpGeneration(r.Context())
	}
}

func guardPatchedLevels(w http.ResponseWriter, r *http.Request, body map[string]any) bool {
	if _, patched := body[constants.FieldScopesAndPermissions]; !patched {
		return true
	}
	role, err := sharedutils.ExtractStructFromBody[roledata.RoleAsResource](body)
	if err != nil {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
		return false
	}
	return authz.GuardRoleLevels(w, r, role.ScopesAndPermissions)
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

		existingRole, ok := roleutils.GetExistingRoleForPatch(w, roleID)
		if !ok {
			return
		}

		if !authz.GuardRoleDeletion(w, existingRole) {
			return
		}

		lock := concurrency.GetLock(roleID)
		lock.Lock()
		defer lock.Unlock()

		deleteResult := api.DeleteCustomResourceByName(roleID, metadata.RoleAsResourceMetadata)
		resourcesshared.InvalidateResourceCaches(optimizer, constants.ResourceRole, roleID)
		authz.BumpGeneration(r.Context())
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
