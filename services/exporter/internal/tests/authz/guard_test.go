package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	callerID = "u-00001-0000-0001"
	victimID = "u-00002-0000-0002"
	adminID  = "r-00000-0000-0001"
)

func requestAs(identity xauthz.Identity) *http.Request {
	r := httptest.NewRequest(http.MethodPatch, "/v1/users/"+callerID, nil)
	return r.WithContext(xauthz.WithIdentity(r.Context(), identity))
}

func userWithLevel(level roledata.PermissionLevel) xauthz.Identity {
	return xauthz.Identity{
		UserID: callerID,
		Grants: xauthz.Grants{
			Levels: map[string]roledata.PermissionLevel{roledata.ScopeUsers: level},
		},
	}
}

func profileEdit() map[string]any {
	return map[string]any{constants.FieldName: "New Name"}
}

func rolePromotion() map[string]any {
	return map[string]any{constants.FieldAssignedRolesIDs: []any{adminID}}
}

// The escalation this whole layer exists to stop.
func TestGuardUserPatchBlocksSelfPromotion(t *testing.T) {
	levels := []roledata.PermissionLevel{
		roledata.PermissionLevelReadOnly,
		roledata.PermissionLevelContributor,
		roledata.PermissionLevelOwner,
		roledata.PermissionLevelAdmin,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			w := httptest.NewRecorder()

			allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(level)), callerID, rolePromotion())

			if allowed {
				t.Fatalf("%s user promoted itself", level)
			}
			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
		})
	}
}

func TestGuardUserPatchBlocksPrivilegeEditBelowOwner(t *testing.T) {
	levels := []roledata.PermissionLevel{
		roledata.PermissionLevelReadOnly,
		roledata.PermissionLevelContributor,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			w := httptest.NewRecorder()

			allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(level)), victimID, rolePromotion())

			if allowed {
				t.Fatalf("%s user granted roles to another user", level)
			}
			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
		})
	}
}

func TestGuardUserPatchAllowsOwnerToGrantOthers(t *testing.T) {
	w := httptest.NewRecorder()

	allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelOwner)), victimID, rolePromotion())

	if !allowed {
		t.Error("owner could not assign roles to another user")
	}
}

// A profile edit must stay open to the user's own routine level.
func TestGuardUserPatchAllowsOwnProfileEdit(t *testing.T) {
	w := httptest.NewRecorder()

	allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelReadOnly)), callerID, profileEdit())

	if !allowed {
		t.Error("read-only user could not edit its own profile")
	}
}

func TestGuardUserPatchBlocksEveryPrivilegedField(t *testing.T) {
	fields := []string{
		constants.FieldAssignedRolesIDs,
		constants.FieldAssignedGroupsIDs,
		constants.FieldStatus,
	}

	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			w := httptest.NewRecorder()
			body := map[string]any{field: "anything"}

			if authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelAdmin)), callerID, body) {
				t.Errorf("%q was editable on self", field)
			}
		})
	}
}

// A privilege edit mixed into a profile edit must not slip through.
func TestGuardUserPatchBlocksMixedBody(t *testing.T) {
	w := httptest.NewRecorder()
	body := profileEdit()
	body[constants.FieldAssignedRolesIDs] = []any{adminID}

	if authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelOwner)), callerID, body) {
		t.Error("privilege change hidden in a profile edit was allowed")
	}
}

func TestGuardUserPatchDeniesWithoutIdentity(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/v1/users/"+callerID, nil)

	if authz.GuardUserPatch(w, r, victimID, rolePromotion()) {
		t.Error("privilege change allowed with no identity in context")
	}
}

// Peer services act with the service token and are already trusted.
func TestGuardUserPatchAllowsInternalCaller(t *testing.T) {
	w := httptest.NewRecorder()
	identity := xauthz.Identity{Internal: true}

	if !authz.GuardUserPatch(w, requestAs(identity), callerID, rolePromotion()) {
		t.Error("internal caller was blocked")
	}
}

func TestGuardSelfUser(t *testing.T) {
	tests := []struct {
		name     string
		identity xauthz.Identity
		target   string
		want     bool
	}{
		{"own record", userWithLevel(roledata.PermissionLevelReadOnly), callerID, true},
		{"another user", userWithLevel(roledata.PermissionLevelAdmin), victimID, false},
		{"internal caller", xauthz.Identity{Internal: true}, victimID, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			if got := authz.GuardSelfUser(w, requestAs(tt.identity), tt.target); got != tt.want {
				t.Errorf("GuardSelfUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardSelfUserDeniesWithoutIdentity(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)

	if authz.GuardSelfUser(w, r, callerID) {
		t.Error("allowed with no identity in context")
	}
}

func TestGuardRoleDeletion(t *testing.T) {
	tests := []struct {
		name string
		role *roledata.RoleAsResource
		want bool
	}{
		{"no protection block", &roledata.RoleAsResource{}, true},
		{"deletion allowed", &roledata.RoleAsResource{Protection: &roledata.Protection{}}, true},
		{
			name: "deletion prevented",
			role: &roledata.RoleAsResource{Protection: &roledata.Protection{PreventDeletion: true}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			if got := authz.GuardRoleDeletion(w, tt.role); got != tt.want {
				t.Errorf("GuardRoleDeletion() = %v, want %v", got, tt.want)
			}
		})
	}
}
