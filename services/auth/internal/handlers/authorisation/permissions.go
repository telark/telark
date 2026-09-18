package authorisation

import (
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

type roleEntry struct {
	sources []RoleSource
}

func resolveUserPermissions(userID string) (*PermissionsResponse, error) {
	user, err := authhelper.GetUserByIDWithErrorHandling(userID)
	if err != nil {
		return nil, err
	}

	roleMap := collectDirectRoles(user.AssignedRolesIDs)
	collectInheritedRoles(user.AssignedGroupsIDs, roleMap)

	resolvedRoles := resolveRoles(roleMap)
	return &PermissionsResponse{UserID: userID, Roles: resolvedRoles}, nil
}

func collectDirectRoles(assignedRoleIDs []*string) map[string]*roleEntry {
	roleMap := make(map[string]*roleEntry)
	for _, rid := range assignedRoleIDs {
		if rid == nil {
			continue
		}
		id := *rid
		if _, exists := roleMap[id]; !exists {
			roleMap[id] = &roleEntry{}
		}
		roleMap[id].sources = append(roleMap[id].sources, RoleSource{Kind: "direct"})
	}
	return roleMap
}

func collectInheritedRoles(assignedGroupIDs []*string, roleMap map[string]*roleEntry) {
	groupClient := clients.GetGroupClient()
	for _, gidPtr := range assignedGroupIDs {
		if gidPtr == nil {
			continue
		}
		groupID := *gidPtr
		group, err := groupClient.GetGroupByID(groupID)
		if err != nil {
			lg.Error("failed to load group " + groupID + ": " + err.Error())
			continue
		}
		for _, rid := range group.AssignedRolesIDs {
			if _, exists := roleMap[rid]; !exists {
				roleMap[rid] = &roleEntry{}
			}
			roleMap[rid].sources = append(roleMap[rid].sources, RoleSource{Kind: "inherited", GroupID: groupID})
		}
	}
}

func resolveRoles(roleMap map[string]*roleEntry) []ResolvedRole {
	roleClient := clients.GetRoleClient()
	resolvedRoles := make([]ResolvedRole, constants.DefaultInitValue, len(roleMap))
	for roleID, entry := range roleMap {
		role, err := roleClient.GetRoleByID(roleID)
		if err != nil {
			lg.Error("failed to load role " + roleID + ": " + err.Error())
			continue
		}
		resolvedRoles = append(resolvedRoles, ResolvedRole{
			RoleID:    role.ID,
			RoleName:  role.Name,
			Status:    role.Status,
			Priority:  role.Priority,
			IsExpired: isRoleExpired(role),
			Sources:   entry.sources,
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

func isRoleExpired(role *roleresource.RoleAsResource) bool {
	if role.Validity == nil {
		return false
	}
	if role.Validity.Type != roleresource.ValidityTypeTemporary {
		return false
	}
	if role.Validity.ExpiresAt == nil {
		return false
	}
	expiry, err := time.Parse(constants.TimeFormatRFC3339, *role.Validity.ExpiresAt)
	if err != nil {
		return false
	}
	return time.Now().After(expiry)
}
