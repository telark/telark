package authz

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	roleAllReadOnly = "r-00000-0000-0004"
	roleUsersOwner  = "r-00000-0000-0005"
	roleUsersAdmin  = "r-00000-0000-0006"
	roleAppsOwner   = "r-00000-0000-0007"
	roleUnreadable  = "r-00000-0000-0008"
	roleAllReadName = "ReadOnly"
)

var errBackend = errors.New("apiserver unreachable")

// Ids absent from the maps answer not found, like a dangling reference in a CR,
// so the older tests keep their meaning.
type fakeSource struct {
	users  map[string]*userdata.UserAsResource
	groups map[string]*groupdata.GroupAsResource
	roles  map[string]*roledata.RoleAsResource
}

func (f fakeSource) User(userID string) (*userdata.UserAsResource, error) {
	return lookup(f.users, userID)
}

func (f fakeSource) Group(groupID string) (*groupdata.GroupAsResource, error) {
	return lookup(f.groups, groupID)
}

func (f fakeSource) Role(roleID string) (*roledata.RoleAsResource, error) {
	if roleID == roleUnreadable {
		return nil, errBackend
	}
	return lookup(f.roles, roleID)
}

func lookup[T any](records map[string]*T, id string) (*T, error) {
	record, ok := records[id]
	if !ok {
		return nil, xauthz.ErrNotFound
	}
	return record, nil
}

func roleGranting(name, scope string, level roledata.PermissionLevel) *roledata.RoleAsResource {
	return &roledata.RoleAsResource{
		Name:                 name,
		Status:               roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: scope, Level: level}},
	}
}

func TestMain(m *testing.M) {
	authz.UseGrantSource(fakeSource{
		users:  fakeUsers(),
		groups: fakeGroups(),
		roles: map[string]*roledata.RoleAsResource{
			roleAllReadOnly: roleGranting(roleAllReadName, roledata.ScopeAll, roledata.PermissionLevelReadOnly),
			roleUsersOwner:  roleGranting("users-owner", roledata.ScopeUsers, roledata.PermissionLevelOwner),
			roleUsersAdmin:  roleGranting("users-admin", roledata.ScopeUsers, roledata.PermissionLevelAdmin),
			roleAppsOwner:   roleGranting("apps-owner", roledata.ScopeApplications, roledata.PermissionLevelOwner),
			roleAllAdmin:    roleGranting("all-admin", roledata.ScopeAll, roledata.PermissionLevelAdmin),
			roleAllAdminOff: inactive(roleGranting("all-admin-off", roledata.ScopeAll, roledata.PermissionLevelAdmin)),
			roleTerminating: terminating(roleGranting("gone", roledata.ScopeUsers, roledata.PermissionLevelOwner)),
		},
	})
	m.Run()
}

func assigning(roleIDs ...string) map[string]any {
	ids := make([]any, constants.DefaultInitValue, len(roleIDs))
	for _, id := range roleIDs {
		ids = append(ids, id)
	}
	return map[string]any{constants.FieldAssignedRolesIDs: ids}
}

// The escalation seen live: a users Owner holding nothing outside users handed
// out the built-in ReadOnly role, which grants ALL.
func TestGuardUserCreateCapsAssignedRolesAtCallerLevel(t *testing.T) {
	usersOwner := levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)

	tests := []struct {
		name     string
		identity xauthz.Identity
		body     map[string]any
		want     bool
	}{
		{"users owner assigns ALL read-only", usersOwner, assigning(roleAllReadOnly), false},
		{"users owner assigns users admin", usersOwner, assigning(roleUsersAdmin), false},
		{"users owner assigns users owner", usersOwner, assigning(roleUsersOwner), true},
		{"users owner assigns one ok one above", usersOwner, assigning(roleUsersOwner, roleAppsOwner), false},
		{"users owner assigns unknown role", usersOwner, assigning(adminID), true},
		{"all owner assigns ALL read-only", levels(roledata.ScopeAll, roledata.PermissionLevelOwner), assigning(roleAllReadOnly), true},
		{"all owner assigns users admin", levels(roledata.ScopeAll, roledata.PermissionLevelOwner), assigning(roleUsersAdmin), false},
		{"all admin assigns anything", levels(roledata.ScopeAll, roledata.PermissionLevelAdmin), assigning(roleUsersAdmin, roleAppsOwner), true},
		{"internal assigns anything", internalIdentity, assigning(roleUsersAdmin), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardUserCreate(w, requestAs(tt.identity), tt.body)
			if got != tt.want {
				t.Fatalf("GuardUserCreate = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}

func TestGuardAssignedRoleDenialNamesRoleAndScope(t *testing.T) {
	w := httptest.NewRecorder()
	usersOwner := levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)
	expectForbidden(t, w, authz.GuardUserCreate(w, requestAs(usersOwner), assigning(roleAllReadOnly)), "assigning above own level")
	for _, want := range []string{roleAllReadName, roledata.ScopeAll, string(roledata.PermissionLevelReadOnly)} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("denial %q does not name %q", w.Body.String(), want)
		}
	}
}

// An unreadable role must not pass as if it granted nothing.
func TestGuardAssignedRoleLookupFailureFailsClosed(t *testing.T) {
	w := httptest.NewRecorder()
	if authz.GuardUserCreate(w, requestAs(levels(roledata.ScopeAll, roledata.PermissionLevelAdmin)), assigning(roleUnreadable)) {
		t.Fatal("unreadable role was assigned")
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

// Only what the patch adds is capped: a role the user already holds, or one
// being removed, was granted by someone entitled to.
func TestGuardUserPatchCapsOnlyAddedRoles(t *testing.T) {
	usersOwner := levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)
	allReadOnly := roleAllReadOnly
	existing := []*string{&allReadOnly}

	tests := []struct {
		name     string
		existing []*string
		body     map[string]any
		want     bool
	}{
		{"keeps ALL read-only, adds users owner", existing, assigning(roleAllReadOnly, roleUsersOwner), true},
		{"removes ALL read-only", existing, assigning(), true},
		{"adds ALL read-only", nil, assigning(roleAllReadOnly), false},
		{"swaps ALL read-only for users admin", existing, assigning(roleUsersAdmin), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardUserPatch(w, requestAs(usersOwner), &userdata.UserAsResource{ID: victimID, AssignedRolesIDs: tt.existing}, tt.body)
			if got != tt.want {
				t.Fatalf("GuardUserPatch = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestGuardGroupRolesCapsAttachedRoles(t *testing.T) {
	groupsOwner := levels(roledata.ScopeGroups, roledata.PermissionLevelOwner)
	groupsAndAppsOwner := levels(roledata.ScopeGroups, roledata.PermissionLevelOwner)
	groupsAndAppsOwner.Grants.Levels[roledata.ScopeApplications] = roledata.PermissionLevelOwner

	tests := []struct {
		name     string
		identity xauthz.Identity
		existing []string
		body     map[string]any
		want     bool
	}{
		{"groups owner attaches apps owner", groupsOwner, nil, assigning(roleAppsOwner), false},
		{"groups and apps owner attaches apps owner", groupsAndAppsOwner, nil, assigning(roleAppsOwner), true},
		{"groups owner keeps apps owner already attached", groupsOwner, []string{roleAppsOwner}, assigning(roleAppsOwner, roleA), true},
		{"groups owner detaches apps owner", groupsOwner, []string{roleAppsOwner}, assigning(), true},
		{"create with ALL read-only", groupsOwner, nil, assigning(roleAllReadOnly), false},
		{"internal", internalIdentity, nil, assigning(roleAllReadOnly), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardGroupRolesPatch(w, requestAs(tt.identity), tt.existing, tt.body)
			if got != tt.want {
				t.Fatalf("GuardGroupRolesPatch = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
		})
	}
}
