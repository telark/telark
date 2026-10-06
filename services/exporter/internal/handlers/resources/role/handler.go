package role

import (
	"fmt"
	"net/http"
	"time"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	"github.com/telark/telark/services/exporter/internal/handlers/resources/shared"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	roleutils "github.com/telark/telark/services/exporter/internal/utils/resources/role"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreateRoleResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpecFor[roledata.AccessRole](w, r)
		if err != nil {
			return
		}

		if !authz.GuardRoleReservedFields(w, r, nil, body) {
			return
		}

		resourcesshared.StampCreateAudit(r, body)
		role, err := roleutils.ExtractRoleSpecFromRequestBody(body)
		if err != nil {
			responseutils.SendResponse(
				w,
				http.StatusBadRequest,
				response.OperationError,
				err.Error(),
				nil,
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

func createRoleResource(w http.ResponseWriter, role *roledata.AccessRole, optimizer *performance.Optimizer) {
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
		metadata.AccessRoleMetadata,
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

func ListRoleResourcesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return shared.ListResourceWithCacheInvalidation(metadata.AccessRoleMetadata)
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

		body, err := sharedutils.GetSpecFor[roledata.AccessRole](w, r)
		if err != nil {
			return
		}

		if !authz.GuardNotTerminating(w, r, existingRole.DeletionTimestamp) ||
			!roleutils.ValidateProtectionFlags(existingRole, body, w) ||
			!authz.GuardRoleReservedFields(w, r, existingRole, body) {
			return
		}

		resourcesshared.StampPatchAudit(r, body)
		mergedRole, ok := roleutils.ExtractAndMergeRoleForPatch(existingRole, body, w)
		if !ok || !authz.GuardPatchedRoleLevels(w, r, existingRole, mergedRole, body) ||
			!authz.GuardRolePatchLastAdmin(w, existingRole, mergedRole) {
			return
		}

		patchRoleResource(w, roleID, body, optimizer)
		// A role edit changes the permissions of every user holding it,
		// directly or through a group, so no single user's grants can be
		// targeted for invalidation.
		authz.BumpGeneration(r.Context())
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

	generics.GenericPatchCustomResource(w, metadata.AccessRoleMetadata, roleID, specPatchData)
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

		if !authz.GuardRoleDeletion(w, existingRole) || !authz.GuardRoleWithinCaller(w, r, existingRole) ||
			!authz.GuardRoleDeleteLastAdmin(w, existingRole) {
			return
		}
		if existingRole.Protection != nil && existingRole.Protection.SoftDelete {
			softDeleteRole(w, r, roleID, optimizer)
			return
		}

		lock := concurrency.GetLock(roleID)
		lock.Lock()
		defer lock.Unlock()

		deleteResult := api.DeleteCustomResourceByName(roleID, metadata.AccessRoleMetadata)
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

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), roleID, metadata.AccessRoleMetadata.Kind)
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

// The record stays for history; grants count only Active roles, so it grants nothing.
func softDeleteRole(w http.ResponseWriter, r *http.Request, roleID string, optimizer *performance.Optimizer) {
	body := map[string]any{
		constants.FieldStatus:    string(roledata.RoleStatusDeleted),
		constants.FieldDeletedAt: time.Now().UTC().Format(time.RFC3339),
	}
	resourcesshared.StampPatchAudit(r, body)
	patchRoleResource(w, roleID, body, optimizer)
	authz.BumpGeneration(r.Context())
}
