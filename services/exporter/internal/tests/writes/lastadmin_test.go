package writes

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	roleutils "github.com/telark/telark/services/exporter/internal/utils/resources/role"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	adminRoleID    = "r-00000-0000-0001"
	adminGroupID   = "ug-00001-0000-0001"
	adminA         = "u-0000a-0000-0001"
	adminB         = "u-0000b-0000-0002"
	groupAdminUser = "u-0000c-0000-0003"
	bootstrapUser  = "u-0000d-0000-0004"
	plainUser      = "u-0000e-0000-0005"
	phaseSuspended = "suspended"
	keyLastLoginAt = "lastLoginAt"
	stampTime      = "2026-09-30T12:00:00Z"
	keyRoleRefs    = "roleRefs"
	keyGroupRefs   = "groupRefs"
	keyFullname    = "fullname"
	keyScope       = "scope"
	keyLevel       = "level"
	keyType        = "type"
	keyExpiresAt   = "expiresAt"
	pastTime       = "2020-01-01T00:00:00Z"
)

func crSeedOf(t *testing.T, md base.Metadata, id string, record any) seed {
	t.Helper()
	spec, err := sharedutils.StructToSpecMap(record)
	if err != nil {
		t.Fatal(err)
	}
	return crSeed(md, object(md, id, spec, nil))
}

func userRecord(id string, roles, groups []string, bootstrap bool) *userdata.User {
	user := &userdata.User{ID: id, Status: userdata.UserStatus{Phase: phaseActive}, Bootstrap: bootstrap}
	for i := range roles {
		user.RoleRefs = append(user.RoleRefs, &roles[i])
	}
	for i := range groups {
		user.GroupRefs = append(user.GroupRefs, &groups[i])
	}
	return user
}

func adminUser(t *testing.T, id string, roles, groups []string, bootstrap bool) seed {
	t.Helper()
	return crSeedOf(t, v1alpha1.UserMetadata, id, userRecord(id, roles, groups, bootstrap))
}

func adminDirectory(t *testing.T, users ...seed) *dynamicfake.FakeDynamicClient {
	t.Helper()
	role := roledata.AccessRole{
		Status:               roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}},
	}
	group := groupdata.Group{RoleRefs: []string{adminRoleID}, UserRefs: []string{groupAdminUser}}
	return installFake(t, append(users,
		crSeedOf(t, v1alpha1.AccessRoleMetadata, adminRoleID, role),
		crSeedOf(t, v1alpha1.GroupMetadata, adminGroupID, group),
		adminUser(t, plainUser, nil, nil, false),
	)...)
}

func directAdmin() *userdata.User {
	return userRecord(adminA, []string{adminRoleID}, nil, false)
}

func groupMember() *userdata.User {
	return userRecord(groupAdminUser, nil, []string{adminGroupID}, false)
}

func adminGroup() *groupdata.Group {
	return &groupdata.Group{ID: adminGroupID, RoleRefs: []string{adminRoleID}, UserRefs: []string{groupAdminUser}}
}

func suspend() map[string]any {
	return map[string]any{constants.FieldStatus: map[string]any{keyPhase: phaseSuspended}}
}

func lastLoginStamp() map[string]any {
	return map[string]any{constants.FieldStatus: map[string]any{keyLastLoginAt: stampTime}}
}

type lastAdminCase struct {
	name  string
	users func(t *testing.T) []seed
	guard func(w http.ResponseWriter) bool
	want  bool
}

func lastAdminCases() []lastAdminCase {
	direct := func(t *testing.T) seed { return adminUser(t, adminA, []string{adminRoleID}, nil, false) }
	return []lastAdminCase{
		{"delete one of two admins", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, adminB, []string{adminRoleID}, nil, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardUserDeleteLastAdmin(w, directAdmin()) }, true},
		{"suspend one of two admins", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, adminB, []string{adminRoleID}, nil, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardUserPatchLastAdmin(w, directAdmin(), suspend()) }, true},
		{"delete an admin while the bootstrap admin remains", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, bootstrapUser, []string{adminRoleID}, nil, true)}
		}, func(w http.ResponseWriter) bool { return authz.GuardUserDeleteLastAdmin(w, directAdmin()) }, true},
		{"delete the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool { return authz.GuardUserDeleteLastAdmin(w, directAdmin()) }, false},
		{"suspend the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool { return authz.GuardUserPatchLastAdmin(w, directAdmin(), suspend()) }, false},
		{"stamp the last admin's login", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool {
				return authz.GuardUserPatchLastAdmin(w, directAdmin(), lastLoginStamp())
			}, true},
		{"demote the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool {
				return authz.GuardUserPatchLastAdmin(w, directAdmin(), map[string]any{keyRoleRefs: []any{}})
			}, false},
		{"profile edit of the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool {
				return authz.GuardUserPatchLastAdmin(w, directAdmin(), map[string]any{keyFullname: newName})
			}, true},
		{"leave the only admin group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool {
			return authz.GuardUserPatchLastAdmin(w, groupMember(), map[string]any{keyGroupRefs: []any{}})
		}, false},
		{"remove the last admin from the group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool {
			return authz.GuardGroupPatchLastAdmin(w, adminGroup(), map[string]any{}, []string{groupAdminUser})
		}, false},
		{"strip Admin from the only admin group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool {
			return authz.GuardGroupPatchLastAdmin(w, adminGroup(), map[string]any{keyRoleRefs: []any{}}, nil)
		}, false},
		{"delete the only admin group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardGroupDeleteLastAdmin(w, adminGroupID) }, false},
		{"delete the admin group while a direct admin remains", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardGroupDeleteLastAdmin(w, adminGroupID) }, true},
		{"no admin to lose", func(*testing.T) []seed { return nil },
			func(w http.ResponseWriter) bool {
				return authz.GuardUserDeleteLastAdmin(w, userRecord(plainUser, nil, nil, false))
			}, true},
	}
}

// Any change that would leave no active user holding Admin on ALL, directly or
// through a group, is refused with 409; the bootstrap account counts as one.
func TestLastAdminGuard(t *testing.T) {
	for _, c := range slices.Concat(lastAdminCases(), roleLastAdminCases()) {
		t.Run(c.name, func(t *testing.T) {
			adminDirectory(t, c.users(t)...)
			w := httptest.NewRecorder()
			if got := c.guard(w); got != c.want {
				t.Fatalf("guard = %v, want %v (%d %s)", got, c.want, w.Code, w.Body.String())
			}
			if !c.want && w.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
			}
		})
	}
}

func adminRoleRecord() *roledata.AccessRole {
	return &roledata.AccessRole{
		ID: adminRoleID, Status: roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}},
	}
}

func rolePatch(field string, value any) func(w http.ResponseWriter) bool {
	return func(w http.ResponseWriter) bool {
		existing := adminRoleRecord()
		merged, ok := roleutils.ExtractAndMergeRoleForPatch(existing, map[string]any{field: value}, w)
		return ok && authz.GuardRolePatchLastAdmin(w, existing, merged)
	}
}

// Seen in review: the role itself was the unguarded way to leave no administrator.
func roleLastAdminCases() []lastAdminCase {
	direct := func(t *testing.T) []seed { return []seed{adminUser(t, adminA, []string{adminRoleID}, nil, false)} }
	readOnlyAll := []any{map[string]any{keyScope: roledata.ScopeAll, keyLevel: string(roledata.PermissionLevelReadOnly)}}
	expired := map[string]any{keyType: string(roledata.ValidityTypeTemporary), keyExpiresAt: pastTime}
	inactive := string(roledata.RoleStatusInactive)
	deleteAdminRole := func(w http.ResponseWriter) bool { return authz.GuardRoleDeleteLastAdmin(w, adminRoleRecord()) }
	return []lastAdminCase{
		{"strip Admin from the only admin role", direct, rolePatch(constants.FieldScopesAndPermissions, readOnlyAll), false},
		{"empty the only admin role", direct, rolePatch(constants.FieldScopesAndPermissions, []any{}), false},
		{"deactivate the only admin role", direct, rolePatch(constants.FieldStatus, inactive), false},
		{"expire the only admin role", direct, rolePatch(constants.FieldValidity, expired), false},
		{"rename the only admin role", direct, rolePatch(keyName, newName), true},
		{"deactivate the admin role while the bootstrap admin remains", func(t *testing.T) []seed {
			return append(direct(t), adminUser(t, bootstrapUser, nil, nil, true))
		}, rolePatch(constants.FieldStatus, inactive), true},
		{"delete the only admin role", direct, deleteAdminRole, false},
		{"delete the admin role held through the admin group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, deleteAdminRole, false},
		{"delete a role granting no Admin", direct, func(w http.ResponseWriter) bool {
			return authz.GuardRoleDeleteLastAdmin(w, &roledata.AccessRole{ID: storedRoleID, Status: roledata.RoleStatusActive})
		}, true},
	}
}

// The role handlers ran no last-admin check: deactivating or deleting the only admin role answered 200.
func TestRoleEditsKeepAnAdministrator(t *testing.T) {
	tests := []struct {
		name, method, body string
	}{
		{"deactivate", http.MethodPatch, `{"status":"` + string(roledata.RoleStatusInactive) + `"}`},
		{"delete", http.MethodDelete, constants.EmptyString},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adminDirectory(t, adminUser(t, adminA, []string{adminRoleID}, nil, false))
			if w := serveAs(t, adminCaller(), tt.method, rolesPath+pathSep+adminRoleID, tt.body); w.Code != http.StatusConflict {
				t.Fatalf("%s = %d %s, want %d", tt.name, w.Code, w.Body.String(), http.StatusConflict)
			}
		})
	}
}
