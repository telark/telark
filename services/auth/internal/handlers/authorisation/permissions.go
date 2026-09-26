package authorisation

import (
	"fmt"
	"net/http"
	"time"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	"github.com/telark/auth/internal/helpers/shared"
	roleresource "github.com/telark/data/resources/role"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

func GetPermissions(w http.ResponseWriter, r *http.Request) {
	userID, err := authhelper.ValidateSessionFromRequest(r)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
		return
	}

	resp, err := resolveUserPermissions(userID)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusForbidden, err)
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, resp)
}

func resolveUserPermissions(userID string) (*PermissionsResponse, error) {
	user, err := authhelper.GetUserByIDWithErrorHandling(userID)
	if err != nil {
		return nil, err
	}

	roleMap := collectDirectRoles(user.RoleRefs)
	collectInheritedRoles(user.GroupRefs, roleMap)

	resolvedRoles := resolveRoles(roleMap)
	return &PermissionsResponse{UserID: userID, Roles: resolvedRoles}, nil
}

func collectDirectRoles(assignedRoleIDs []*string) map[string][]RoleSource {
	roleMap := make(map[string][]RoleSource)
	for _, rid := range assignedRoleIDs {
		if rid == nil {
			continue
		}
		roleMap[*rid] = append(roleMap[*rid], RoleSource{Kind: constants.RoleSourceDirect})
	}
	return roleMap
}

func collectInheritedRoles(assignedGroupIDs []*string, roleMap map[string][]RoleSource) {
	groupClient := clients.GetGroupClient()
	for _, gidPtr := range assignedGroupIDs {
		if gidPtr == nil {
			continue
		}
		groupID := *gidPtr
		group, err := groupClient.GetGroupByID(groupID)
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrFailedLoadGroup), groupID, err))
			continue
		}
		if group.DeletionTimestamp != nil {
			continue
		}
		for _, rid := range group.RoleRefs {
			roleMap[rid] = append(roleMap[rid], RoleSource{Kind: constants.RoleSourceInherited, GroupID: groupID})
		}
	}
}

func resolveRoles(roleMap map[string][]RoleSource) []ResolvedRole {
	roleClient := clients.GetAccessRoleClient()
	resolvedRoles := make([]ResolvedRole, constants.DefaultInitValue, len(roleMap))
	for roleID, sources := range roleMap {
		role, err := roleClient.GetAccessRoleByID(roleID)
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrFailedLoadRole), roleID, err))
			continue
		}
		if role.DeletionTimestamp != nil {
			continue
		}
		resolvedRoles = append(resolvedRoles, ResolvedRole{
			RoleID:    role.ID,
			RoleName:  role.Name,
			Status:    role.Status,
			Priority:  role.Priority,
			IsExpired: isRoleExpired(role),
			Sources:   sources,
			Scopes:    buildResolvedScopes(role.ScopesAndPermissions),
		})
	}
	return resolvedRoles
}

func buildResolvedScopes(sps []roleresource.ScopeAndPermissions) []ResolvedScope {
	scopes := make([]ResolvedScope, constants.DefaultInitValue, len(sps))
	for _, sp := range sps {
		rs := ResolvedScope{Scope: sp.Scope, Level: sp.Level}
		if sp.Rules != nil {
			rs.Rules = *sp.Rules
		}
		scopes = append(scopes, rs)
	}
	return scopes
}

func isRoleExpired(role *roleresource.AccessRole) bool {
	if role.Validity == nil {
		return false
	}
	if role.Validity.Type != roleresource.ValidityTypeTemporary {
		return false
	}
	if role.Validity.ExpiresAt == nil {
		return false
	}
	// Same rule as x-ware/authz: an unparsable expiry is expired, never permanent.
	expiry, err := time.Parse(constants.TimeFormatRFC3339, *role.Validity.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().After(expiry)
}
