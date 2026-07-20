package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	xauthz "github.com/telark/x-ware/authz"
)

func noIdentityRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/", nil)
}

func TestGuardCategoryScope(t *testing.T) {
	level := roledata.PermissionLevelContributor
	tests := []struct {
		name  string
		req   *http.Request
		scope string
		want  bool
	}{
		{"missing identity", noIdentityRequest(), roledata.ScopeRoles, false},
		{"internal bypasses", requestAs(xauthz.Identity{Internal: true}), roledata.ScopeRoles, true},
		{"unknown scope denied", requestAs(xauthz.Identity{UserID: "u1"}), "galaxy", false},
		{"known scope without grant denied", requestAs(xauthz.Identity{UserID: "u1"}), roledata.ScopeRoles, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardCategoryScope(w, tt.req, tt.scope, level); got != tt.want {
				t.Errorf("GuardCategoryScope = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardSelfSessionToken(t *testing.T) {
	w := httptest.NewRecorder()
	if authz.GuardSelfSessionToken(w, noIdentityRequest(), "tok") {
		t.Error("session guard passed without identity")
	}

	if !authz.GuardSelfSessionToken(httptest.NewRecorder(), requestAs(xauthz.Identity{Internal: true}), "tok") {
		t.Error("internal caller should pass session guard")
	}

	// A non-internal caller cannot resolve the token (no session backend), so it is denied.
	if authz.GuardSelfSessionToken(httptest.NewRecorder(), requestAs(xauthz.Identity{UserID: "u1"}), "tok") {
		t.Error("unresolvable token should be denied")
	}
}
