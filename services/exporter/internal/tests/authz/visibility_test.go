package authz

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	userdata "github.com/telark/telark/internal/data/resources/user"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
)

const cacheKey = "users:list:1"

func allAdmin() xauthz.Identity {
	return levels(roledata.ScopeAll, roledata.PermissionLevelAdmin)
}

func as(userID string, identity xauthz.Identity) xauthz.Identity {
	identity.UserID = userID
	return identity
}

func TestRestricted(t *testing.T) {
	tests := []struct {
		name string
		r    *http.Request
		want bool
	}{
		{"no identity", httptest.NewRequest(http.MethodGet, "/", nil), true},
		{"internal", requestAs(internalIdentity), false},
		{"admin on all", requestAs(allAdmin()), false},
		{"owner on users", requestAs(levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)), true},
		{"admin on users only", requestAs(levels(roledata.ScopeUsers, roledata.PermissionLevelAdmin)), true},
	}
	key := authz.RestrictedKey(func(*http.Request) string { return cacheKey })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authz.Restricted(tt.r); got != tt.want {
				t.Fatalf("Restricted = %v, want %v", got, tt.want)
			}
			if blanked := key(tt.r) == constants.EmptyString; blanked != tt.want {
				t.Fatalf("cache key blanked = %v, want %v", blanked, tt.want)
			}
		})
	}
}

type generations map[string]string

func (g generations) ListGeneration(resourceType string) string { return g[resourceType] }

// Restricted callers share one entry apart from the full list; a write to any
// input of who is hidden moves it.
func TestRestrictedListKey(t *testing.T) {
	gens := generations{}
	key := authz.RestrictedListKey(gens, func(*http.Request) string { return cacheKey })
	owner := requestAs(levels(roledata.ScopeUsers, roledata.PermissionLevelOwner))
	reader := requestAs(as(userPlain, levels(roledata.ScopeUsers, roledata.PermissionLevelReadOnly)))

	if key(requestAs(allAdmin())) != cacheKey || key(requestAs(internalIdentity)) != cacheKey {
		t.Fatal("an unrestricted caller left the full list entry")
	}
	restricted := key(owner)
	if restricted == constants.EmptyString || restricted == cacheKey || key(reader) != restricted {
		t.Fatalf("restricted keys = %q, %q, want one entry apart from %q", restricted, key(reader), cacheKey)
	}
	for _, resourceType := range []string{constants.ResourceUser, constants.ResourceGroup, constants.ResourceRole} {
		gens[resourceType] = strconv.Itoa(len(gens) + constants.DefaultIncrementValue)
		moved := key(owner)
		if moved == restricted {
			t.Fatalf("a %s write left the restricted key at %q", resourceType, moved)
		}
		restricted = moved
	}
	blank := authz.RestrictedListKey(gens, func(*http.Request) string { return constants.EmptyString })
	if got := blank(owner); got != constants.EmptyString {
		t.Fatalf("uncacheable request got key %q", got)
	}
}

// An administrator is anyone holding Admin on ALL through an active role,
// directly or through a live group, plus every bootstrap account.
func TestHiddenUsers(t *testing.T) {
	users := fakeUsers()
	tests := []struct {
		name string
		user *userdata.User
		want bool
	}{
		{"plain owner", users[userPlain], false},
		{"direct admin", users[userAdmin], true},
		{"bootstrap", users[userBootstrap], true},
		{"admin through group", users[userGroupAdmin], true},
		{"inactive admin role and terminating admin group", users[userAdminOff], false},
		{"unreadable role", userHolding(unknownID, []string{roleUnreadable}, nil), true},
	}
	hidden := authz.HiddenUsers()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hidden(tt.user); got != tt.want {
				t.Fatalf("hidden = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGuardHiddenUserAnswersNotFoundToRestrictedCallers(t *testing.T) {
	users := fakeUsers()
	owner := levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)

	w := httptest.NewRecorder()
	if authz.GuardHiddenUser(w, requestAs(owner), users[userAdmin]) || w.Code != http.StatusNotFound {
		t.Fatalf("owner reading an admin: got %d, want 404", w.Code)
	}
	if !authz.GuardHiddenUser(httptest.NewRecorder(), requestAs(owner), users[userPlain]) {
		t.Fatal("owner reading a plain user refused")
	}
	if !authz.GuardHiddenUser(httptest.NewRecorder(), requestAs(allAdmin()), users[userBootstrap]) {
		t.Fatal("admin reading a bootstrap account refused")
	}
}

func TestGuardUserTarget(t *testing.T) {
	users := fakeUsers()
	owner := levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)
	regularAdmin := as(userAdmin, allAdmin())
	bootstrap := as(userBootstrap, allAdmin())
	suspend := map[string]any{constants.FieldStatus: map[string]any{"phase": "suspended"}}
	bootstrapFlag := map[string]any{constants.FieldBootstrap: true}

	tests := []struct {
		name     string
		identity xauthz.Identity
		target   *userdata.User
		body     map[string]any
		deleting bool
		want     bool
		status   int
	}{
		{"internal deletes bootstrap", internalIdentity, users[userBootstrap], nil, true, false, http.StatusForbidden},
		{"internal patches bootstrap", internalIdentity, users[userBootstrap], suspend, false, true, http.StatusOK},
		{"session writes bootstrap flag", regularAdmin, users[userPlain], bootstrapFlag, false, false, http.StatusForbidden},
		{"self delete", as(userPlain, owner), users[userPlain], nil, true, false, http.StatusForbidden},
		{"bootstrap self delete", bootstrap, users[userBootstrap], nil, true, false, http.StatusForbidden},
		{"self profile edit", as(userPlain, owner), users[userPlain], profileEdit(), false, true, http.StatusOK},
		{"bootstrap self profile edit", bootstrap, users[userBootstrap], profileEdit(), false, true, http.StatusOK},
		{"owner patches admin", as(userPlain, owner), users[userAdmin], rolePromotion(), false, false, http.StatusNotFound},
		{"owner deletes bootstrap", as(userPlain, owner), users[userBootstrap], nil, true, false, http.StatusNotFound},
		{"owner patches plain user", as(userPlain, owner), users[victimID], rolePromotion(), false, true, http.StatusOK},
		{"admin patches bootstrap", regularAdmin, users[userBootstrap], profileEdit(), false, false, http.StatusForbidden},
		{"admin deletes admin", regularAdmin, users[userGroupAdmin], nil, true, true, http.StatusOK},
		{"admin suspends admin", regularAdmin, users[userGroupAdmin], suspend, false, true, http.StatusOK},
		{"admin changes admin roles", regularAdmin, users[userGroupAdmin], rolePromotion(), false, true, http.StatusOK},
		{"admin deletes plain user", regularAdmin, users[userPlain], nil, true, true, http.StatusOK},
		{"bootstrap deletes admin", bootstrap, users[userAdmin], nil, true, true, http.StatusOK},
		{"bootstrap suspends admin", bootstrap, users[userAdmin], suspend, false, true, http.StatusOK},
		{"bootstrap deletes bootstrap", bootstrap, userHolding(userGroupAdmin, nil, nil), nil, true, true, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardUserTarget(w, requestAs(tt.identity), tt.target, tt.body, tt.deleting)
			if got != tt.want {
				t.Fatalf("GuardUserTarget = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && w.Code != tt.status {
				t.Fatalf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestGuardUserCreateRefusesBootstrapFlag(t *testing.T) {
	w := httptest.NewRecorder()
	bootstrapFlag := map[string]any{constants.FieldBootstrap: true}
	expectForbidden(t, w, authz.GuardUserCreate(w, requestAs(allAdmin()), bootstrapFlag), "session creating a bootstrap account")
	if !authz.GuardUserCreate(httptest.NewRecorder(), requestAs(internalIdentity), bootstrapFlag) {
		t.Fatal("service seeding a bootstrap account refused")
	}
}

func TestGuardReservedEmail(t *testing.T) {
	t.Setenv(constants.BootstrapAdminEnv, " Root@Example.com ")
	envmanager.InitBootstrapAdmin()

	w := httptest.NewRecorder()
	expectForbidden(t, w, authz.GuardReservedEmail(w, requestAs(allAdmin()), "root@example.com"), "session claiming a bootstrap mailbox")
	if !authz.GuardReservedEmail(httptest.NewRecorder(), requestAs(allAdmin()), "test@example.com") {
		t.Fatal("ordinary mailbox refused")
	}
	if !authz.GuardReservedEmail(httptest.NewRecorder(), requestAs(internalIdentity), "root@example.com") {
		t.Fatal("service provisioning a bootstrap mailbox refused")
	}

	t.Setenv(constants.BootstrapAdminEnv, constants.EmptyString)
	envmanager.InitBootstrapAdmin()
	if !authz.GuardReservedEmail(httptest.NewRecorder(), requestAs(allAdmin()), constants.EmptyString) {
		t.Fatal("unset bootstrap admin reserved the empty email")
	}
}
