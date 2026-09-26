package authz

import (
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

func members(userIDs ...string) map[string]any {
	ids := make([]any, constants.DefaultInitValue, len(userIDs))
	for _, id := range userIDs {
		ids = append(ids, id)
	}
	return map[string]any{constants.FieldAssignedUsersIDs: ids}
}

func withoutGroupRule(action string) xauthz.Identity {
	owner := levels(roledata.ScopeGroups, roledata.PermissionLevelOwner)
	return denied(owner, roledata.ScopeGroups, xauthz.RuleKey(roledata.ScopeGroups, action))
}

func withoutUserRule(action string) xauthz.Identity {
	owner := levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)
	return denied(owner, roledata.ScopeUsers, xauthz.RuleKey(roledata.ScopeUsers, action))
}

func groupsOf(groupIDs ...string) map[string]any {
	ids := make([]any, constants.DefaultInitValue, len(groupIDs))
	for _, id := range groupIDs {
		ids = append(ids, id)
	}
	return map[string]any{constants.FieldAssignedGroupsIDs: ids}
}

// Seen live: a Contributor added himself to a group holding a stronger role,
// and an Owner denied both member rules edited members anyway.
func TestGuardGroupMembersPatchRules(t *testing.T) {
	owner := levels(roledata.ScopeGroups, roledata.PermissionLevelOwner)
	contributor := levels(roledata.ScopeGroups, roledata.PermissionLevelContributor)
	noAdd := withoutGroupRule(roledata.ActionAddUserToGroup)
	noRemove := withoutGroupRule(roledata.ActionRemoveUserFromGroup)

	tests := []struct {
		name     string
		identity xauthz.Identity
		existing []string
		body     map[string]any
		want     bool
	}{
		{"owner adds", owner, nil, members(victimID), true},
		{"owner removes", owner, []string{victimID}, members(), true},
		{"contributor adds", contributor, nil, members(victimID), false},
		{"owner adds himself", owner, nil, members(owner.UserID), false},
		{"owner removes himself", owner, []string{owner.UserID}, members(), false},
		{"denied add adds", noAdd, nil, members(victimID), false},
		{"denied add removes", noAdd, []string{victimID}, members(), true},
		{"denied remove removes", noRemove, []string{victimID}, members(), false},
		{"denied remove adds", noRemove, nil, members(victimID), true},
		{"members untouched", contributor, nil, map[string]any{constants.FieldName: "renamed"}, true},
		{"internal", internalIdentity, nil, members(victimID), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardGroupMembersPatch(w, requestAs(tt.identity), &groupdata.GroupAsResource{AssignedUsersIDs: tt.existing}, tt.body)
			if got != tt.want {
				t.Fatalf("GuardGroupMembersPatch = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}

// A removal is judged by the remove rule alone: denied "add" must not block
// it, and denied "remove" must.
func TestGuardUserPatchAppliesRemoveRules(t *testing.T) {
	noAttach := withoutUserRule(roledata.ActionAttachRoleToUser)
	noDetach := withoutUserRule(roledata.ActionRemoveRoleFromUser)
	noAdd := withoutGroupRule(roledata.ActionAddUserToGroup)
	noRemove := withoutGroupRule(roledata.ActionRemoveUserFromGroup)
	roleID, groupID := roleUsersOwner, groupPlain
	holder := &userdata.UserAsResource{ID: victimID, AssignedRolesIDs: []*string{&roleID}, AssignedGroupsIDs: []*string{&groupID}}

	tests := []struct {
		name     string
		identity xauthz.Identity
		body     map[string]any
		want     bool
	}{
		{"no attach rule removes role", noAttach, assigning(), true},
		{"no remove rule removes role", noDetach, assigning(), false},
		{"no remove rule keeps role", noDetach, assigning(roleUsersOwner), true},
		{"no add rule removes group", noAdd, groupsOf(), true},
		{"no remove rule removes group", noRemove, groupsOf(), false},
		{"no add rule adds group", noAdd, groupsOf(groupPlain, groupAdmin), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardUserPatch(w, requestAs(tt.identity), holder, tt.body)
			if got != tt.want {
				t.Fatalf("GuardUserPatch = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestGuardUserCreateAppliesAddRules(t *testing.T) {
	w := httptest.NewRecorder()
	noAdd := withoutGroupRule(roledata.ActionAddUserToGroup)
	expectForbidden(t, w, authz.GuardUserCreate(w, requestAs(noAdd), groupsOf(groupPlain)), "create with a group under a denied add rule")
}

// Dangling and terminating ids are refused by name; an unreadable backend fails closed.
func TestGuardReferencedIDs(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		ids    []string
		want   bool
		status int
		names  string
	}{
		{"live user and group", constants.ResourceUser, []string{userPlain}, true, http.StatusOK, constants.EmptyString},
		{"unknown user", constants.ResourceUser, []string{userPlain, unknownID}, false, http.StatusBadRequest, unknownID},
		{"terminating user", constants.ResourceUser, []string{userTerminating}, false, http.StatusBadRequest, userTerminating},
		{"terminating group", constants.ResourceGroup, []string{groupTerminating}, false, http.StatusBadRequest, groupTerminating},
		{"terminating role", constants.ResourceRole, []string{roleTerminating}, false, http.StatusBadRequest, roleTerminating},
		{"unreadable role", constants.ResourceRole, []string{roleUnreadable}, false, http.StatusServiceUnavailable, constants.EmptyString},
		{"nothing referenced", constants.ResourceRole, nil, true, http.StatusOK, constants.EmptyString},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardReferencedIDs(w, tt.kind, tt.ids); got != tt.want {
				t.Fatalf("GuardReferencedIDs = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && w.Code != tt.status {
				t.Fatalf("status = %d, want %d", w.Code, tt.status)
			}
			if !strings.Contains(w.Body.String(), tt.names) {
				t.Fatalf("denial %q does not name %q", w.Body.String(), tt.names)
			}
		})
	}
}

func TestGuardNotTerminating(t *testing.T) {
	deleted := deletedTimestamp
	w := httptest.NewRecorder()
	if !authz.GuardNotTerminating(w, requestAs(userWithLevel(roledata.PermissionLevelAdmin)), nil) {
		t.Fatal("live record refused")
	}
	if !authz.GuardNotTerminating(w, requestAs(internalIdentity), &deleted) {
		t.Fatal("cleanup cascade refused")
	}
	w = httptest.NewRecorder()
	if authz.GuardNotTerminating(w, requestAs(userWithLevel(roledata.PermissionLevelAdmin)), &deleted) || w.Code != http.StatusGone {
		t.Fatalf("terminating record: got %d, want 410", w.Code)
	}
}
