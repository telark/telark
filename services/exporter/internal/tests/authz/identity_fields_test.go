package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	xauthz "github.com/telark/x-ware/authz"
)

func linkedTarget(id string) *userdata.User {
	return &userdata.User{
		ID: id, Email: "test@example.com", Username: "testuser",
		Identities: []*userdata.UserIdentity{{Provider: "google", Issuer: "https://accounts.google.com", Subject: "111"}},
	}
}

func identitiesBody(subject string) []any {
	return []any{map[string]any{"provider": "google", "issuer": "https://accounts.google.com", "subject": subject}}
}

// Whoever sets another user's mailbox or login identity can sign in as them.
func TestGuardUserPatchIdentityFields(t *testing.T) {
	owner := userWithLevel(roledata.PermissionLevelOwner)
	self := userWithLevel(roledata.PermissionLevelReadOnly)
	existingRoles := []any{}
	tests := []struct {
		name     string
		identity xauthz.Identity
		target   string
		body     map[string]any
		want     bool
	}{
		{"owner rewrites identities beside a role edit", owner, victimID,
			map[string]any{constants.FieldRoleRefs: existingRoles, constants.FieldIdentities: identitiesBody("attacker")}, false},
		{"owner rewrites email beside a role edit", owner, victimID,
			map[string]any{constants.FieldRoleRefs: existingRoles, constants.FieldEmail: "attacker@example.com"}, false},
		{"owner rewrites username beside a role edit", owner, victimID,
			map[string]any{constants.FieldRoleRefs: existingRoles, constants.FieldUsername: "attacker"}, false},
		{"owner echoes unchanged identity fields", owner, victimID,
			map[string]any{
				constants.FieldRoleRefs: existingRoles, constants.FieldEmail: "test@example.com",
				constants.FieldIdentities: identitiesBody("111"),
			}, true},
		{"self changes email", self, callerID, map[string]any{constants.FieldEmail: "new@example.com"}, true},
		{"self changes username", self, callerID, map[string]any{constants.FieldUsername: "testuser2"}, true},
		{"self links another identity", self, callerID, map[string]any{constants.FieldIdentities: identitiesBody("222")}, false},
		{"internal links an identity", internalIdentity, victimID, map[string]any{constants.FieldIdentities: identitiesBody("222")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardUserPatch(w, requestAs(tt.identity), linkedTarget(tt.target), tt.body); got != tt.want {
				t.Fatalf("GuardUserPatch = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}

func TestGuardUserCreateRefusesIdentities(t *testing.T) {
	admin := levels(roledata.ScopeAll, roledata.PermissionLevelAdmin)
	w := httptest.NewRecorder()
	linked := map[string]any{constants.FieldIdentities: identitiesBody("s")}
	expectForbidden(t, w, authz.GuardUserCreate(w, requestAs(admin), linked), "session creating a linked identity")
	if !authz.GuardUserCreate(httptest.NewRecorder(), requestAs(admin), map[string]any{constants.FieldIdentities: []any{}}) {
		t.Error("an empty identities list was refused")
	}
	if !authz.GuardUserCreate(httptest.NewRecorder(), requestAs(internalIdentity), map[string]any{constants.FieldIdentities: identitiesBody("s")}) {
		t.Error("internal create with an identity was refused")
	}
}
