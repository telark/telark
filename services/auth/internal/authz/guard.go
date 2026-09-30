package authz

import (
	"context"
	"errors"
	"net/http"

	"github.com/telark/auth/internal/constants"
	dataerrors "github.com/telark/data/errors"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/x-ware/authz"
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

func CallerIsAdminOnAll(ctx context.Context) bool {
	caller, ok := authz.FromContext(ctx)
	if !ok {
		return false
	}
	return caller.Internal || isAdmin(caller.Grants)
}
