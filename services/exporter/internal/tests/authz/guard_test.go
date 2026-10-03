package authz

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/rest/base"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	roleutils "github.com/telark/telark/services/exporter/internal/utils/resources/role"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

const (
	callerID = "u-00001-0000-0001"
	victimID = "u-00002-0000-0002"
	adminID  = "r-00000-0000-0001"

	lockedRoleName = "locked"
	newRoleName    = "renamed"
	errorLogMarker = "[ERROR]"
	unparsableBody = "{"
	unknownKeyBody = `{"bogus":true}`
	mistypedValue  = true
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

func target(userID string) *userdata.User {
	return &userdata.User{ID: userID}
}

func profileEdit() map[string]any {
	return map[string]any{constants.FieldName: "New Name"}
}

func rolePromotion() map[string]any {
	return map[string]any{constants.FieldRoleRefs: []any{adminID}}
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

			allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(level)), target(callerID), rolePromotion())

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

			allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(level)), target(victimID), rolePromotion())

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

	allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelOwner)), target(victimID), rolePromotion())

	if !allowed {
		t.Error("owner could not assign roles to another user")
	}
}

// A profile edit must stay open to the user's own routine level.
func TestGuardUserPatchAllowsOwnProfileEdit(t *testing.T) {
	w := httptest.NewRecorder()

	allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelReadOnly)), target(callerID), profileEdit())

	if !allowed {
		t.Error("read-only user could not edit its own profile")
	}
}

// A profile edit is not a privilege, but it must still be confined to the
// account owner: nobody may rename or re-email a stranger's account.
func TestGuardUserPatchBlocksProfileEditOnAnotherUser(t *testing.T) {
	w := httptest.NewRecorder()

	allowed := authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelAdmin)), target(victimID), profileEdit())

	if allowed {
		t.Error("a caller edited another user's profile fields")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestGuardUserPatchBlocksEveryPrivilegedField(t *testing.T) {
	fields := []string{
		constants.FieldRoleRefs,
		constants.FieldGroupRefs,
		constants.FieldStatus,
	}

	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			w := httptest.NewRecorder()
			body := map[string]any{field: "anything"}

			if authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelAdmin)), target(callerID), body) {
				t.Errorf("%q was editable on self", field)
			}
		})
	}
}

// A privilege edit mixed into a profile edit must not slip through.
func TestGuardUserPatchBlocksMixedBody(t *testing.T) {
	w := httptest.NewRecorder()
	body := profileEdit()
	body[constants.FieldRoleRefs] = []any{adminID}

	if authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelOwner)), target(callerID), body) {
		t.Error("privilege change hidden in a profile edit was allowed")
	}
}

func TestGuardUserPatchDeniesWithoutIdentity(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/v1/users/"+callerID, nil)

	if authz.GuardUserPatch(w, r, target(victimID), rolePromotion()) {
		t.Error("privilege change allowed with no identity in context")
	}
}

// Peer services act with the service token and are already trusted.
func TestGuardUserPatchAllowsInternalCaller(t *testing.T) {
	w := httptest.NewRecorder()
	identity := xauthz.Identity{Internal: true}

	if !authz.GuardUserPatch(w, requestAs(identity), target(callerID), rolePromotion()) {
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
		role *roledata.AccessRole
		want bool
	}{
		{"no protection block", &roledata.AccessRole{}, true},
		{"deletion allowed", &roledata.AccessRole{Protection: &roledata.Protection{}}, true},
		{
			name: "deletion prevented",
			role: &roledata.AccessRole{Protection: &roledata.Protection{PreventDeletion: true}},
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	fn()
	os.Stdout = original
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// Seen live: every expected refusal printed an [ERROR] line, which buried real failures in log scans.
func TestGuardRefusalsAreNotLoggedAsErrors(t *testing.T) {
	locked := &roledata.AccessRole{Name: lockedRoleName, Protection: &roledata.Protection{LockName: true}}
	refusals := map[string]func(http.ResponseWriter) bool{
		"self promotion": func(w http.ResponseWriter) bool {
			return authz.GuardUserPatch(w, requestAs(userWithLevel(roledata.PermissionLevelAdmin)), target(callerID), rolePromotion())
		},
		"protected role": func(w http.ResponseWriter) bool {
			return roleutils.ValidateProtectionFlags(locked, map[string]any{constants.FieldName: newRoleName}, w)
		},
		"unknown reference": func(w http.ResponseWriter) bool {
			return authz.GuardReferencedIDs(w, constants.ResourceUser, []string{unknownID})
		},
	}
	for name, refuse := range refusals {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			out := captureStdout(t, func() {
				if refuse(w) {
					t.Error("allowed")
				}
			})
			if strings.Contains(out, errorLogMarker) {
				t.Fatalf("refusal %d logged as an error: %s", w.Code, out)
			}
		})
	}
}

func bodyRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPatch, "/v1/roles/"+adminID, strings.NewReader(body))
}

func specAccepted(w http.ResponseWriter, body string) bool {
	_, err := sharedutils.GetSpec(w, bodyRequest(body))
	return err == nil
}

func mistypedStatus() map[string]any {
	return map[string]any{constants.FieldStatus: mistypedValue}
}

// Seen live: a body the client got wrong printed an [ERROR] line like a real failure. A 5xx still logs.
func TestBodyErrorsAreNotLoggedAsErrors(t *testing.T) {
	cases := map[string]struct {
		accepted func(http.ResponseWriter) bool
		status   int
	}{
		"unparsable body": {func(w http.ResponseWriter) bool { return specAccepted(w, unparsableBody) }, http.StatusUnprocessableEntity},
		"oversized body": {func(w http.ResponseWriter) bool {
			return specAccepted(w, unparsableBody+strings.Repeat(" ", int(base.MaxRequestBodySize)))
		}, http.StatusRequestEntityTooLarge},
		"unknown key": {func(w http.ResponseWriter) bool {
			_, err := sharedutils.GetSpecFor[roledata.AccessRole](w, bodyRequest(unknownKeyBody))
			return err == nil
		}, http.StatusBadRequest},
		"mistyped user admin field": {func(w http.ResponseWriter) bool {
			return authz.GuardUserPatchLastAdmin(w, target(victimID), mistypedStatus())
		}, http.StatusBadRequest},
		"mistyped role field": {func(w http.ResponseWriter) bool {
			_, merged := roleutils.ExtractAndMergeRoleForPatch(&roledata.AccessRole{}, mistypedStatus(), w)
			return merged
		}, http.StatusBadRequest},
		"role without a name": {func(w http.ResponseWriter) bool {
			return roleutils.ValidateAndPrepareRole(&roledata.AccessRole{}, w) == nil
		}, http.StatusBadRequest},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			out := captureStdout(t, func() {
				if c.accepted(w) {
					t.Error("accepted")
				}
			})
			if w.Code != c.status || strings.Contains(out, errorLogMarker) {
				t.Fatalf("answered %d (want %d), logged %q", w.Code, c.status, out)
			}
		})
	}
	out := captureStdout(t, func() {
		sharedutils.LogAndReturnError(httptest.NewRecorder(), http.StatusInternalServerError, string(constants.ErrResourceLookupFailed), errBackend)
	})
	if !strings.Contains(out, errorLogMarker) {
		t.Fatal("a 5xx no longer logs an error")
	}
}
