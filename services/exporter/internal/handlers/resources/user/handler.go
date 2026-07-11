package user

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/finalizers"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	"github.com/telark/exporter/internal/handlers/resources/shared"
	notiftypes "github.com/telark/exporter/internal/types/notifications"
	"github.com/telark/exporter/internal/utils/concurrency"
	notifdispatch "github.com/telark/exporter/internal/utils/notifications"
	"github.com/telark/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	userutils "github.com/telark/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CreateUserResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		user, err := userutils.ExtractUserSpecFromRequestBody(body)
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

		if err := userutils.ValidateAndPrepareUser(user, w); err != nil {
			return
		}

		createUserResource(w, user, optimizer)
	}
}

func createUserResource(w http.ResponseWriter, user *userdata.UserAsResource, optimizer *performance.Optimizer) {
	spec, err := sharedutils.StructToSpecMap(user)
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

	lock := concurrency.GetLock(user.ID)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericCreateCustomResourceWithFinalizers(
		w,
		metadata.UserAsResourceMetadata,
		user.ID,
		spec,
		[]string{finalizers.UserCleanup},
	)

	cache.SmartInvalidateListCache(optimizer, constants.ResourceUser, string(constants.OpCreate))
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUser)
}

func GetUserByIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		resource, ok := userutils.FindUserByIDOrRespond(w, userID)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func GetUserByUsernameWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := sharedutils.GetPathParam(w, r, constants.UsernameParam)
		if err != nil {
			return
		}

		resource, ok := userutils.FindUserByUsernameOrRespond(w, username)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func GetUserByEmailWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		email, err := sharedutils.GetPathParam(w, r, constants.EmailParam)
		if err != nil {
			return
		}

		resource, ok := userutils.FindUserByEmailOrRespond(w, email)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func GetUserByIdentityWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		issuer := r.URL.Query().Get("issuer")
		subject := r.URL.Query().Get("subject")

		if provider == constants.EmptyString || issuer == constants.EmptyString ||
			subject == constants.EmptyString {
			responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
				"provider, issuer and subject query params are required", nil, nil)
			return
		}

		resource, ok := userutils.FindUserByIdentityOrRespond(w, provider, issuer, subject)
		if !ok {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func ListUserResourcesWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.ListResourceWithCacheInvalidation(optimizer, metadata.UserAsResourceMetadata)
}

func PatchUserByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		existingUser, ok := userutils.GetExistingUserForPatch(w, userID)
		if !ok {
			return
		}

		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		_, rolesPatched := body[constants.FieldAssignedRolesIDs]
		oldRoles := existingUser.AssignedRolesIDs

		if !userutils.ExtractAndMergeUserForPatch(existingUser, body, w) {
			return
		}

		// Update lastUpdateDate on patch
		resourcesshared.AddLastUpdateDateToPatchBody(body)

		specPatchData := map[string]any{
			constants.SpecField: body,
		}

		lock := concurrency.GetLock(userID)
		lock.Lock()
		defer lock.Unlock()

		rc := sharedutils.NewResponseCapture(w)
		generics.GenericPatchCustomResource(rc, metadata.UserAsResourceMetadata, userID, specPatchData)
		userutils.InvalidateUserCaches(optimizer, userID)

		if rc.Status() == http.StatusOK && rolesPatched {
			emitRoleChanged(userID, oldRoles, body)
		}
	}
}

func emitRoleChanged(userID string, oldRoles []*string, body map[string]any) {
	newRoles := notifdispatch.ExtractNewRoleIDsFromBody(body, constants.FieldAssignedRolesIDs)
	added, removed := notifdispatch.DiffPtrStringSlices(oldRoles, newRoles)
	if len(added) == constants.DefaultInitValue && len(removed) == constants.DefaultInitValue {
		return
	}
	notifdispatch.Emit(notiftypes.Notification{
		UserID:   userID,
		Type:     notiftypes.TypeRoleChanged,
		Title:    "Your roles were updated",
		Message:  buildRoleChangeMessage(added, removed),
		Severity: notiftypes.SeverityInfo,
		Metadata: map[string]any{
			notiftypes.MetaKeyTargetID:       userID,
			notiftypes.MetaKeyAddedRoleIDs:   added,
			notiftypes.MetaKeyRemovedRoleIDs: removed,
		},
	})
}

func buildRoleChangeMessage(added, removed []string) string {
	parts := make([]string, constants.DefaultInitValue, len(added)+len(removed))
	if len(added) > constants.DefaultInitValue {
		parts = append(parts, fmt.Sprintf("granted %d role(s)", len(added)))
	}
	if len(removed) > constants.DefaultInitValue {
		parts = append(parts, fmt.Sprintf("revoked %d role(s)", len(removed)))
	}
	return strings.Join(parts, "; ")
}

func DeleteUserByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		_, ok := userutils.FindUserByIDOrRespond(w, userID)
		if !ok {
			return
		}

		userutils.InvalidateUserCaches(optimizer, userID)
		lock := concurrency.GetLock(userID)
		lock.Lock()
		defer lock.Unlock()

		deleteResult := api.DeleteCustomResourceByName(userID, metadata.UserAsResourceMetadata)
		if deleteResult.Status != http.StatusOK {
			errorMsg := sharedutils.GenerateResourceError(errors.ErrDeleteRes, userID, deleteResult.Error)
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

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), userID, metadata.UserAsResourceMetadata.Kind)
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
