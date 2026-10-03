package authz

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

// Auth writes the invite and its use time with the service token; a session, Admin on ALL
// included, may neither forge either on a create or a patch nor clear a live invite.
func TestUserInviteIsReservedToServices(t *testing.T) {
	victim := userHolding(victimID, nil, nil)
	bodies := map[string]map[string]any{
		"forged":      {constants.FieldStatus: map[string]any{constants.FieldInvite: map[string]any{"issuedBy": callerID}}},
		"cleared":     {constants.FieldStatus: map[string]any{constants.FieldInvite: nil}},
		"marked used": {constants.FieldStatus: map[string]any{constants.FieldInviteAcceptedAt: deletedTimestamp}},
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			expectForbidden(t, w, authz.GuardUserPatch(w, requestAs(allAdmin()), victim, body), "session patching the invite")
			expectInviteRefusal(t, w)
			w = httptest.NewRecorder()
			expectForbidden(t, w, authz.GuardUserCreate(w, requestAs(allAdmin()), body), "session creating with an invite")
			expectInviteRefusal(t, w)
			if !authz.GuardUserPatch(httptest.NewRecorder(), requestAs(internalIdentity), victim, body) ||
				!authz.GuardUserCreate(httptest.NewRecorder(), requestAs(internalIdentity), body) {
				t.Fatal("service writing the invite refused")
			}
		})
	}
}

func expectInviteRefusal(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(w.Body.String(), constants.ErrAuthzInviteFieldReserved) {
		t.Fatalf("refusal = %s, want %q", w.Body.String(), constants.ErrAuthzInviteFieldReserved)
	}
}

func TestGuardReservedEmail(t *testing.T) {
	t.Setenv(constants.BootstrapAdminEnv, " Root@Example.com ")
	envmanager.InitBootstrapAdmin()

	const reserved, ordinary = "root@example.com", "test@example.com"
	moved, holder, bootstrap, onChart := userHolding(userPlain, nil, nil), userHolding(userPlain, nil, nil),
		fakeUsers()[userBootstrap], fakeUsers()[userBootstrap]
	moved.Email, holder.Email, bootstrap.Email, onChart.Email = ordinary, "ROOT@example.com", ordinary, reserved

	tests := []struct {
		name     string
		identity xauthz.Identity
		target   *userdata.User
		email    string
		want     bool
	}{
		{"session creating on a bootstrap mailbox", allAdmin(), nil, reserved, false},
		{"account moving onto a bootstrap mailbox", as(userPlain, allAdmin()), moved, reserved, false},
		{"account resending the bootstrap mailbox it holds", as(userPlain, allAdmin()), holder, reserved, true},
		{"bootstrap account taking its mailbox back", as(userBootstrap, allAdmin()), bootstrap, " Root@Example.com", true},
		{"bootstrap account leaving its mailbox", as(userBootstrap, allAdmin()), onChart, ordinary, false},
		{"bootstrap account resending a drifted email", as(userBootstrap, allAdmin()), bootstrap, ordinary, true},
		{"ordinary mailbox", allAdmin(), nil, ordinary, true},
		{"service provisioning a bootstrap mailbox", internalIdentity, nil, reserved, true},
		{"service moving the bootstrap account", internalIdentity, onChart, ordinary, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardReservedEmail(w, requestAs(tt.identity), tt.target, tt.email); !tt.want {
				expectForbidden(t, w, got, tt.name)
			} else if !got {
				t.Fatalf("refused: %d %s", w.Code, w.Body.String())
			}
		})
	}

	t.Setenv(constants.BootstrapAdminEnv, constants.EmptyString)
	envmanager.InitBootstrapAdmin()
	if !authz.GuardReservedEmail(httptest.NewRecorder(), requestAs(allAdmin()), nil, constants.EmptyString) {
		t.Fatal("unset bootstrap admin reserved the empty email")
	}
	w := httptest.NewRecorder()
	expectForbidden(t, w, authz.GuardReservedEmail(w, requestAs(as(userBootstrap, allAdmin())), onChart, ordinary),
		"bootstrap account changing its email with no chart value")
}
