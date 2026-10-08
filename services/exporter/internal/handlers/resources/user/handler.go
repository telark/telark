package user

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/kcore/crds/api"
	kubeshared "github.com/telark/telark/internal/kcore/shared"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	"github.com/telark/telark/services/exporter/internal/membership"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	sessionutils "github.com/telark/telark/services/exporter/internal/utils/auth/session"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	notifdispatch "github.com/telark/telark/services/exporter/internal/utils/notifications"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var lg = constants.GetLogger(constants.PrefixMain)

func CreateUserResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpecFor[userdata.User](w, r)
		if err != nil {
			return
		}

		if !authz.GuardUserCreate(w, r, body) {
			return
		}

		user, err := userutils.ExtractUserSpecFromRequestBody(body)
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

		if !authz.GuardReservedEmail(w, r, nil, user.Email) {
			return
		}
		if err := userutils.ValidateAndPrepareUser(user, w); err != nil {
			return
		}

		roleIDs, groupIDs := notifdispatch.Deref(user.RoleRefs), notifdispatch.Deref(user.GroupRefs)
		if !authz.GuardReferencedIDs(w, constants.ResourceRole, roleIDs) ||
			!authz.GuardReferencedIDs(w, constants.ResourceGroup, groupIDs) {
			return
		}
		if !mirrorGroups(w, r, optimizer, user.ID, groupIDs, nil) {
			return
		}

		createUserResource(w, user, optimizer)
	}
}

// Counterparts first: see membership.MirrorUserGroups.
func mirrorGroups(w http.ResponseWriter, r *http.Request, optimizer *performance.Optimizer, userID string, added, removed []string) bool {
	if err := membership.MirrorUserGroups(r.Context(), optimizer, userID, added, removed); err != nil {
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
		return false
	}
	return true
}

func createUserResource(w http.ResponseWriter, user *userdata.User, optimizer *performance.Optimizer) {
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
		metadata.UserMetadata,
		user.ID,
		spec,
		[]string{finalizers.UserCleanup},
	)

	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUser)
}

func GetUserByIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil || !authz.GuardUserRead(w, r, userID) {
			return
		}

		resource, ok := userutils.FindUserByIDOrRespond(w, userID)
		if !ok || !visibleUser(w, r, resource) {
			return
		}

		// Peers resolve grants through this route: a user held only by the
		// cleanup finalizer must read as gone, not as an active account.
		if resource.GetDeletionTimestamp() != nil {
			responseutils.LogAndSendResponse(w, http.StatusGone, response.OperationError, string(constants.ErrUserBeingDeleted), nil, nil)
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

// A restricted caller must not learn that an administrator exists: the record
// answers 404 exactly like a missing one.
func visibleUser(w http.ResponseWriter, r *http.Request, resource *unstructured.Unstructured) bool {
	user, err := userutils.ExtractUserFromUnstructured(resource)
	if err != nil || user == nil {
		responseutils.LogAndSendResponse(
			w, http.StatusInternalServerError, response.OperationError, string(errors.ErrRestUnmarshalResourceToJSON), nil, err,
		)
		return false
	}
	return authz.GuardHiddenUser(w, r, user)
}

func GetUserByUsernameWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := sharedutils.GetPathParam(w, r, constants.UsernameParam)
		if err != nil {
			return
		}

		resource, ok := userutils.FindUserByUsernameOrRespond(w, username)
		if !ok || !visibleUser(w, r, resource) {
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
		if !ok || !visibleUser(w, r, resource) {
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
		if !ok || !visibleUser(w, r, resource) {
			return
		}

		resourcesshared.SendFilteredResourceResponse(w, resource)
	}
}

func GetUserNamesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := requestedUserIDs(r)
		if len(ids) == constants.DefaultInitValue {
			responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
				string(constants.ErrUserIDsRequired), nil, nil)
			return
		}
		if len(ids) > constants.UserNamesMaxIDs {
			responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
				fmt.Sprintf(string(constants.ErrUserIDsTooMany), constants.UserNamesMaxIDs), nil, nil)
			return
		}

		names, err := userutils.UsernamesByID(ids)
		if err != nil {
			responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
			return
		}
		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessListRes), names, nil)
	}
}

func requestedUserIDs(r *http.Request) []string {
	ids := strings.Split(r.URL.Query().Get(constants.IDsParam), constants.UserIDsSeparator)
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}
	slices.Sort(ids)
	return slices.DeleteFunc(slices.Compact(ids), func(id string) bool { return id == constants.EmptyString })
}

func ListUserResourcesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authz.Restricted(r) {
			generics.GenericListCustomResources(w, metadata.UserMetadata)
			return
		}
		hidden := authz.HiddenUsers()
		generics.GenericListCustomResourcesKeeping(w, metadata.UserMetadata, func(item *unstructured.Unstructured) bool {
			user, err := userutils.ExtractUserFromUnstructured(item)
			return err == nil && user != nil && !hidden(user)
		})
	}
}

func PatchUserByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		// Judged before the body, so a hidden administrator answers 404 like a
		// missing id, not 400 for a malformed body.
		existingUser, ok := userutils.GetExistingUserForPatch(w, userID)
		if !ok || !authz.GuardUserTarget(w, r, existingUser, nil, false) {
			return
		}

		body, err := sharedutils.GetSpecFor[userdata.User](w, r)
		if err != nil {
			return
		}

		addedGroups, removedGroups, ok := guardUserPatch(w, r, existingUser, body)
		if !ok {
			return
		}

		_, rolesPatched := body[constants.FieldRoleRefs]
		oldRoles := existingUser.RoleRefs

		if !userutils.ExtractAndMergeUserForPatch(existingUser, body, w) {
			return
		}

		if !mirrorGroups(w, r, optimizer, userID, addedGroups, removedGroups) {
			return
		}
		if patchUserResource(w, r, userID, body, optimizer) && rolesPatched {
			emitRoleChanged(userID, oldRoles, body)
		}
	}
}

func guardUserPatch(
	w http.ResponseWriter, r *http.Request, existing *userdata.User, body map[string]any,
) (addedGroups, removedGroups []string, ok bool) {
	if !authz.GuardUserTarget(w, r, existing, body, false) ||
		!authz.GuardNotTerminating(w, r, existing.DeletionTimestamp) ||
		!authz.GuardUserPatch(w, r, existing, body) {
		return nil, nil, false
	}
	if email, provided := body[constants.FieldEmail].(string); provided && !authz.GuardReservedEmail(w, r, existing, email) {
		return nil, nil, false
	}

	newRoles := notifdispatch.ExtractNewRoleIDsFromBody(body, constants.FieldRoleRefs)
	addedRoles, _ := notifdispatch.DiffPtrStringSlices(existing.RoleRefs, newRoles)
	// An absent groupRefs would diff as every group left.
	if _, groupsPatched := body[constants.FieldGroupRefs]; groupsPatched {
		newGroups := notifdispatch.ExtractNewRoleIDsFromBody(body, constants.FieldGroupRefs)
		addedGroups, removedGroups = notifdispatch.DiffPtrStringSlices(existing.GroupRefs, newGroups)
	}
	if !authz.GuardReferencedIDs(w, constants.ResourceRole, addedRoles) ||
		!authz.GuardReferencedIDs(w, constants.ResourceGroup, addedGroups) ||
		!authz.GuardUserPatchLastAdmin(w, existing, body) {
		return nil, nil, false
	}
	return addedGroups, removedGroups, true
}

func patchUserResource(w http.ResponseWriter, r *http.Request, userID string, body map[string]any, optimizer *performance.Optimizer) bool {
	resourcesshared.AddLastUpdateDateToPatchBody(body)
	specPatchData := map[string]any{
		constants.SpecField: body,
	}

	lock := concurrency.GetLock(userID)
	lock.Lock()
	defer lock.Unlock()

	rc := sharedutils.NewResponseCapture(w)
	generics.GenericPatchCustomResource(rc, metadata.UserMetadata, userID, specPatchData)
	userutils.InvalidateUserCaches(optimizer, userID)
	authz.ForgetUserGrants(r.Context(), userID)
	return rc.Status() == http.StatusOK
}

func emitRoleChanged(userID string, oldRoles []*string, body map[string]any) {
	newRoles := notifdispatch.ExtractNewRoleIDsFromBody(body, constants.FieldRoleRefs)
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
		parts = append(parts, fmt.Sprintf(string(constants.NotifRolesGranted), roleCount(len(added))))
	}
	if len(removed) > constants.DefaultInitValue {
		parts = append(parts, fmt.Sprintf(string(constants.NotifRolesRevoked), roleCount(len(removed))))
	}
	return strings.Join(parts, constants.NotifSentenceSeparator)
}

func roleCount(count int) string {
	if count == constants.DefaultIncrementValue {
		return string(constants.NotifOneRole)
	}
	return fmt.Sprintf(string(constants.NotifRoleCount), count)
}

func DeleteUserByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		existingUser, ok := userutils.GetExistingUserForPatch(w, userID)
		if !ok || !authz.GuardUserTarget(w, r, existingUser, nil, true) || !authz.GuardUserDeleteLastAdmin(w, existingUser) {
			return
		}

		deleteResult := deleteUserResource(r.Context(), optimizer, userID)
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

		// The finalizer keeps the record until the cleanup sweeper runs; the
		// sessions go now so revocation does not wait for it.
		if err := sessionutils.PurgeSessionsForUser(userID); err != nil {
			lg.Warn(fmt.Sprintf(string(constants.WarnUserSessionsPurgeFailed), userID, err))
		}
		if err := passkeyutils.PurgePasskeysForUser(userID); err != nil {
			lg.Warn(fmt.Sprintf(string(constants.WarnUserPasskeysPurgeFailed), userID, err))
		}
		// Same for the groups' member lists, outside the user's lock (see membership.setMember).
		if err := membership.MirrorUserGroups(r.Context(), optimizer, userID, nil, notifdispatch.Deref(existingUser.GroupRefs)); err != nil {
			lg.Warn(fmt.Sprintf(string(constants.WarnMembershipsNotStripped), metadata.UserMetadata.Kind, userID, err))
		}

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), userID, metadata.UserMetadata.Kind)
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

func deleteUserResource(ctx context.Context, optimizer *performance.Optimizer, userID string) kubeshared.KubernetesAPIData {
	userutils.InvalidateUserCaches(optimizer, userID)
	authz.ForgetUserGrants(ctx, userID)
	lock := concurrency.GetLock(userID)
	lock.Lock()
	defer lock.Unlock()

	deleteResult := api.DeleteCustomResourceByName(userID, metadata.UserMetadata)
	// A request racing the delete may have refilled both caches; forgetting
	// again under the lock is what makes the revocation immediate.
	userutils.InvalidateUserCaches(optimizer, userID)
	authz.ForgetUserGrants(ctx, userID)
	return deleteResult
}
