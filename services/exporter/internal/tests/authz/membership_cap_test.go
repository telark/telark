package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	xauthz "github.com/telark/x-ware/authz"
)

func groupsAndUsersOwner() xauthz.Identity {
	return xauthz.Identity{UserID: callerID, Grants: xauthz.Grants{Levels: map[string]roledata.PermissionLevel{
		roledata.ScopeGroups: roledata.PermissionLevelOwner,
		roledata.ScopeUsers:  roledata.PermissionLevelOwner,
	}}}
}

// Joining a group hands out its roles, so an Owner on groups could otherwise
// make an accomplice Admin by adding them to an administrators' group.
func TestMembershipIsCappedByTheGroupsRoles(t *testing.T) {
	owner := groupsAndUsersOwner()
	tests := []struct {
		name  string
		check func(w http.ResponseWriter) bool
		want  bool
	}{
		{"user side: join admin group", func(w http.ResponseWriter) bool {
			return authz.GuardUserPatch(w, requestAs(owner), &userdata.UserAsResource{ID: victimID}, groupsOf(groupAdmin))
		}, false},
		{"user side: join group within level", func(w http.ResponseWriter) bool {
			return authz.GuardUserPatch(w, requestAs(owner), &userdata.UserAsResource{ID: victimID}, groupsOf(groupPlain))
		}, true},
		{"user side: already a member", func(w http.ResponseWriter) bool {
			existing := &userdata.UserAsResource{ID: victimID, AssignedGroupsIDs: ptrs([]string{groupAdmin})}
			return authz.GuardUserPatch(w, requestAs(owner), existing, groupsOf(groupAdmin))
		}, true},
		{"user create: in admin group", func(w http.ResponseWriter) bool {
			return authz.GuardUserCreate(w, requestAs(owner), groupsOf(groupAdmin))
		}, false},
		{"group side: add member to admin group", func(w http.ResponseWriter) bool {
			group := &groupdata.GroupAsResource{AssignedRolesIDs: []string{roleAllAdmin}}
			return authz.GuardGroupMembersPatch(w, requestAs(owner), group, members(victimID))
		}, false},
		{"group side: add member to group within level", func(w http.ResponseWriter) bool {
			group := &groupdata.GroupAsResource{AssignedRolesIDs: []string{roleUsersOwner}}
			return authz.GuardGroupMembersPatch(w, requestAs(owner), group, members(victimID))
		}, true},
		{"group create: admin role and a member", func(w http.ResponseWriter) bool {
			body := members(victimID)
			body[constants.FieldAssignedRolesIDs] = []any{roleAllAdmin}
			return authz.GuardGroupMembersPatch(w, requestAs(owner), &groupdata.GroupAsResource{}, body)
		}, false},
		{"group side: removing a member is not capped", func(w http.ResponseWriter) bool {
			group := &groupdata.GroupAsResource{AssignedRolesIDs: []string{roleAllAdmin}, AssignedUsersIDs: []string{victimID}}
			return authz.GuardGroupMembersPatch(w, requestAs(owner), group, members())
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := tt.check(w); got != tt.want {
				t.Fatalf("allowed = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}
