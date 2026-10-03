package authz

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
)

// Bootstrap users are never deleted through the API, and a caller below Admin is
// told an administrator account does not exist.
func GuardUserDelete(ctx context.Context, targetID string) (int, error) {
	caller, ok := authz.FromContext(ctx)
	if !ok {
		return http.StatusUnauthorized, errors.New(string(dataerrors.ErrAuthzIdentityMissing))
	}
	if caller.Internal {
		return http.StatusOK, nil
	}
	if caller.UserID == targetID {
		return http.StatusForbidden, errors.New(string(constants.ErrCleanupSelfDelete))
	}

	target, err := clientSource{}.User(targetID)
	if errors.Is(err, authz.ErrGone) {
		return http.StatusOK, nil
	}
	if err != nil {
		return userLookupFailure(err)
	}
	targetAdmin, err := adminOnAll(targetID)
	if err != nil {
		return userLookupFailure(err)
	}
	if !target.Bootstrap && !targetAdmin {
		return http.StatusOK, nil
	}
	return guardProtectedTarget(caller, target.Bootstrap)
}

// Deleting a role takes its levels from every holder, so it is capped like
// authoring one: each level must be within the caller's own on that scope or ALL.
func GuardRoleDelete(ctx context.Context, roleID string) (int, error) {
	caller, ok := authz.FromContext(ctx)
	if !ok {
		return http.StatusUnauthorized, errors.New(string(dataerrors.ErrAuthzIdentityMissing))
	}
	if caller.Internal {
		return http.StatusOK, nil
	}
	return rolesWithinCaller(caller, []string{roleID})
}

// Deleting a group takes its roles from every member.
func GuardGroupDelete(ctx context.Context, groupID string) (int, error) {
	caller, ok := authz.FromContext(ctx)
	if !ok {
		return http.StatusUnauthorized, errors.New(string(dataerrors.ErrAuthzIdentityMissing))
	}
	if caller.Internal {
		return http.StatusOK, nil
	}
	group, err := clientSource{}.Group(groupID)
	if err != nil {
		return removedLookupFailure(err)
	}
	return rolesWithinCaller(caller, group.RoleRefs)
}

func rolesWithinCaller(caller authz.Identity, roleIDs []string) (int, error) {
	for _, roleID := range roleIDs {
		role, err := clientSource{}.Role(roleID)
		if err != nil {
			if status, lookupErr := removedLookupFailure(err); lookupErr != nil {
				return status, lookupErr
			}
			continue
		}
		for _, entry := range role.ScopesAndPermissions {
			if !authz.Allows(caller, authz.Requirement{Scope: entry.Scope, MinLevel: entry.Level}) {
				return http.StatusForbidden, fmt.Errorf(string(constants.ErrAuthzRemovedRoleExceedsCaller), role.Name, entry.Level, entry.Scope)
			}
		}
	}
	return http.StatusOK, nil
}

// A record already gone takes nothing away from anyone; any other failure is an outage.
func removedLookupFailure(err error) (int, error) {
	if errors.Is(err, authz.ErrNotFound) || errors.Is(err, authz.ErrGone) {
		return http.StatusOK, nil
	}
	return http.StatusServiceUnavailable, errors.New(string(dataerrors.ErrAuthzResolverUnavailable))
}

func guardProtectedTarget(caller authz.Identity, targetBootstrap bool) (int, error) {
	if !isAdmin(caller.Grants) {
		return http.StatusNotFound, errors.New(string(constants.ErrUserNotFound))
	}
	if targetBootstrap {
		return http.StatusForbidden, errors.New(string(constants.ErrCleanupBootstrapManaged))
	}
	return http.StatusOK, nil
}

func userLookupFailure(err error) (int, error) {
	if errors.Is(err, authz.ErrNotFound) {
		return http.StatusNotFound, errors.New(string(constants.ErrUserNotFound))
	}
	return http.StatusServiceUnavailable, errors.New(string(dataerrors.ErrAuthzResolverUnavailable))
}

func isAdmin(grants authz.Grants) bool {
	return grants.Levels[roledata.ScopeAll].Covers(roledata.PermissionLevelAdmin)
}

// A suspended administrator is still an administrator for this rule, so the
// account phase is masked before the grants are collected.
func adminOnAll(userID string) (bool, error) {
	grants, err := authz.CollectGrants(activeSource{}, lg, userID)
	if err != nil {
		return false, err
	}
	return isAdmin(grants), nil
}

type activeSource struct{ clientSource }

func (s activeSource) User(userID string) (*userdata.User, error) {
	user, err := s.clientSource.User(userID)
	if err != nil {
		return nil, err
	}
	active := *user
	active.Status.Phase = string(userdata.AccountPhaseActive)
	return &active, nil
}

// An enroll link lets whoever holds it sign in as the target, so issuing one is
// capped like a role assignment and refused where it would take over an account.
func GuardEnrollLinkIssue(ctx context.Context, targetID string) (int, error) {
	return guardEnrollLink(ctx, targetID, true)
}

// Revoking is gated like issuing, minus the refusals of a suspended or terminating target.
func GuardEnrollLinkRevoke(ctx context.Context, targetID string) (int, error) {
	return guardEnrollLink(ctx, targetID, false)
}

func guardEnrollLink(ctx context.Context, targetID string, issuing bool) (int, error) {
	caller, ok := authz.FromContext(ctx)
	if !ok {
		return http.StatusUnauthorized, errors.New(string(dataerrors.ErrAuthzIdentityMissing))
	}
	if caller.Internal {
		return http.StatusOK, nil
	}
	if caller.UserID == targetID {
		return http.StatusForbidden, errors.New(string(constants.ErrEnrollLinkSelf))
	}

	// The exporter answers 410 for a record only the cleanup finalizer still holds.
	target, err := clientSource{}.User(targetID)
	if errors.Is(err, authz.ErrGone) {
		if issuing {
			return http.StatusGone, errors.New(string(constants.ErrEnrollLinkDeleting))
		}
		return http.StatusOK, nil
	}
	if err != nil {
		return userLookupFailure(err)
	}
	grants, err := authz.CollectGrants(activeSource{}, lg, targetID)
	if err != nil {
		return userLookupFailure(err)
	}
	if status, err := guardEnrollTarget(caller, target, grants, issuing); err != nil {
		return status, err
	}
	return guardRecovery(caller, targetID)
}

// A hidden account answers like a missing one, the bootstrap account is enrolled only
// by break-glass, and every level the target holds must be within the caller's own.
func guardEnrollTarget(caller authz.Identity, target *userdata.User, grants authz.Grants, issuing bool) (int, error) {
	if (target.Bootstrap || isAdmin(grants)) && !isAdmin(caller.Grants) {
		return http.StatusNotFound, errors.New(string(constants.ErrUserNotFound))
	}
	if target.Bootstrap {
		return http.StatusForbidden, errors.New(string(constants.ErrEnrollLinkBootstrap))
	}
	if issuing && userdata.AccountPhase(target.Status.Phase) == userdata.AccountPhaseSuspended {
		return http.StatusConflict, errors.New(string(constants.ErrEnrollLinkSuspended))
	}
	for scope, level := range grants.Levels {
		if !authz.Allows(caller, authz.Requirement{Scope: scope, MinLevel: level}) {
			return http.StatusForbidden, fmt.Errorf(string(constants.ErrEnrollLinkAboveLevel), level, scope)
		}
	}
	return http.StatusOK, nil
}

// A link onto an account that already has a passkey can hand that account to its
// issuer, so only the bootstrap account or an Admin on ALL may open one.
func guardRecovery(caller authz.Identity, targetID string) (int, error) {
	if isAdmin(caller.Grants) {
		return http.StatusOK, nil
	}
	hasPasskeys, err := authhelper.CheckUserHasExistingPasskeys(targetID)
	if err != nil {
		return http.StatusServiceUnavailable, errors.New(string(dataerrors.ErrAuthzResolverUnavailable))
	}
	if !hasPasskeys {
		return http.StatusOK, nil
	}
	return requireBootstrap(caller, constants.ErrEnrollLinkRecovery)
}

// Who may sign in, and how, is changed only by the chart's bootstrap account.
func GuardBootstrapCaller(ctx context.Context) (int, error) {
	caller, ok := authz.FromContext(ctx)
	if !ok {
		return http.StatusUnauthorized, errors.New(string(dataerrors.ErrAuthzIdentityMissing))
	}
	if caller.Internal {
		return http.StatusOK, nil
	}
	return requireBootstrap(caller, constants.ErrSignInSettingsBootstrap)
}

// Grants can be handed out and the bootstrap marker cannot, so it is read from the
// caller's own record rather than inferred from the session's grants.
func requireBootstrap(caller authz.Identity, refusal dataerrors.Error) (int, error) {
	user, err := clientSource{}.User(caller.UserID)
	if err != nil {
		return http.StatusServiceUnavailable, errors.New(string(dataerrors.ErrAuthzResolverUnavailable))
	}
	if !user.Bootstrap {
		return http.StatusForbidden, errors.New(string(refusal))
	}
	return http.StatusOK, nil
}
