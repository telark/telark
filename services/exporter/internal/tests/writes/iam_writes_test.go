package writes

import (
	"net/http"
	"slices"
	"testing"

	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	callerID      = "u-00010-0000-0010"
	otherOwnerID  = "u-00011-0000-0011"
	storedRoleID  = "r-00002-0000-0002"
	storedGroupID = "ug-00003-0000-0003"
	originalActor = "u-00012-0000-0012"
	rolesPath     = "/api/v1/accessroles"
	groupsPath    = "/api/v1/groups"
	keyCreatedBy  = "createdBy"
	keyUpdatedBy  = "lastUpdatedBy"
	keyDeletedAt  = "deletedAt"
	keyDesc       = "description"
	testCategory  = "c-00001-0000-0001"
	newDesc       = "changed"
	pathSep       = "/"
)

func adminCaller() xauthz.Identity {
	return xauthz.Identity{UserID: callerID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin},
	}}
}

func rolesOwner(id string) xauthz.Identity {
	return xauthz.Identity{UserID: id, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeRoles: roledata.PermissionLevelOwner},
	}}
}

func jsonBody(t *testing.T, body map[string]any) string {
	t.Helper()
	return string(mustJSON(t, body))
}

func forged(body map[string]any) map[string]any {
	body[keyCreatedBy], body[keyUpdatedBy] = forgedID, forgedID
	return body
}

func storedRole(t *testing.T, protection roledata.Protection) seed {
	t.Helper()
	author := originalActor
	return crSeedOf(t, v1alpha1.AccessRoleMetadata, storedRoleID, roledata.AccessRole{
		Name: appName, Description: newDesc, CategoryRef: testCategory, Type: roledata.RoleTypeCustom,
		Status: roledata.RoleStatusActive, CreatedBy: &author, Protection: &protection,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeRoles, Level: roledata.PermissionLevelReadOnly}},
	})
}

func storedSpec(t *testing.T, client *dynamicfake.FakeDynamicClient, md base.Metadata, name string) map[string]any {
	t.Helper()
	spec, found, err := unstructured.NestedMap(stored(t, client, md, name).Object, keySpec)
	if err != nil || !found {
		t.Fatalf("%s has no spec: %v", name, err)
	}
	return spec
}

// Seen live: a group edit without userRefs removed the group from every
// member, and on the only Admin-granting group answered a false 409.
func TestGroupPatchWithoutMembersKeepsThem(t *testing.T) {
	tests := []struct {
		name, group, member string
		body                map[string]any
		directory           func(t *testing.T) *dynamicfake.FakeDynamicClient
	}{
		{"description of the only admin group", adminGroupID, groupAdminUser, map[string]any{keyDesc: newDesc},
			func(t *testing.T) *dynamicfake.FakeDynamicClient {
				return adminDirectory(t, adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false))
			}},
		{"roles of a mixed group", mixedGroupID, plainUser, map[string]any{keyRoleRefs: []any{}}, mixedDirectory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.directory(t)
			if w := patchAs(t, adminCaller(), groupsPrefix+tt.group, jsonBody(t, tt.body)); w.Code != http.StatusOK {
				t.Fatalf("patch = %d %s", w.Code, w.Body.String())
			}
			if groups := storedList(t, client, v1alpha1.UserMetadata, tt.member, keyGroupRefs); !slices.Contains(groups, tt.group) {
				t.Errorf("member lost the group: %v", groups)
			}
			if members := storedList(t, client, v1alpha1.GroupMetadata, tt.group, keyUserRefs); !slices.Contains(members, tt.member) {
				t.Errorf("group lost the member: %v", members)
			}
		})
	}
}

// Seen live: a profile save, or an admin's role edit, removed the user from every group.
func TestUserPatchWithoutGroupsKeepsMemberships(t *testing.T) {
	tests := []struct {
		name     string
		identity xauthz.Identity
		body     map[string]any
	}{
		{"own profile", xauthz.Identity{UserID: plainUser}, map[string]any{keyFullname: newName}},
		{"roles by an admin", adminCaller(), map[string]any{keyRoleRefs: []any{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mixedDirectory(t)
			if w := patchAs(t, tt.identity, usersPrefix+plainUser, jsonBody(t, tt.body)); w.Code != http.StatusOK {
				t.Fatalf("patch = %d %s", w.Code, w.Body.String())
			}
			if members := storedList(t, client, v1alpha1.GroupMetadata, mixedGroupID, keyUserRefs); !slices.Contains(members, plainUser) {
				t.Errorf("group lost the member: %v", members)
			}
		})
	}
}

// Seen live: createdBy and lastUpdatedBy were stored as the body gave them,
// and an edit without them was never stamped.
func TestPatchStampsTheCallerAsLastEditor(t *testing.T) {
	author := originalActor
	tests := []struct {
		name string
		md   base.Metadata
		id   string
		path string
		seed func(t *testing.T) seed
	}{
		{"role", v1alpha1.AccessRoleMetadata, storedRoleID, rolesPath, func(t *testing.T) seed {
			return storedRole(t, roledata.Protection{})
		}},
		{"group", v1alpha1.GroupMetadata, storedGroupID, groupsPath, func(t *testing.T) seed {
			return crSeedOf(t, v1alpha1.GroupMetadata, storedGroupID, map[string]any{keyName: appName, keyCreatedBy: author})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, tt.seed(t))
			for _, body := range []map[string]any{{keyDesc: newName}, forged(map[string]any{keyDesc: newDesc})} {
				if w := patchAs(t, adminCaller(), tt.path+pathSep+tt.id, jsonBody(t, body)); w.Code != http.StatusOK {
					t.Fatalf("patch %v = %d %s", body, w.Code, w.Body.String())
				}
				spec := storedSpec(t, client, tt.md, tt.id)
				if spec[keyCreatedBy] != author || spec[keyUpdatedBy] != callerID {
					t.Errorf("after %v: audit = %v / %v, want %s / %s", body, spec[keyCreatedBy], spec[keyUpdatedBy], author, callerID)
				}
			}
		})
	}
}

// protection.softDelete keeps the role on record, marked Deleted, so it grants nothing.
func TestSoftDeleteKeepsTheRoleAsDeleted(t *testing.T) {
	client := installFake(t, storedRole(t, roledata.Protection{SoftDelete: true}))
	if w := serveAs(t, adminCaller(), http.MethodDelete, rolesPath+pathSep+storedRoleID, constants.EmptyString); w.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	spec := storedSpec(t, client, v1alpha1.AccessRoleMetadata, storedRoleID)
	if spec[constants.FieldStatus] != string(roledata.RoleStatusDeleted) || spec[keyDeletedAt] == nil || spec[keyUpdatedBy] != callerID {
		t.Errorf("soft-deleted spec = %v", spec)
	}
}

// Only the creator (or an Admin on ALL) may lift a lock, and a lock lifted in
// the same patch no longer blocks the edit it guarded.
func TestRoleLocksAndWhoMayLiftThem(t *testing.T) {
	unlockAndRename := map[string]any{keyName: newName, constants.FieldProtection: map[string]any{constants.FieldLockName: false}}
	tests := []struct {
		name     string
		identity xauthz.Identity
		body     map[string]any
		want     int
		wantName string
	}{
		{"rename a locked name", adminCaller(), map[string]any{keyName: newName}, http.StatusForbidden, appName},
		{"non-creator lifts the lock", rolesOwner(otherOwnerID), unlockAndRename, http.StatusForbidden, appName},
		{"creator lifts the lock and renames", rolesOwner(originalActor), unlockAndRename, http.StatusOK, newName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, storedRole(t, roledata.Protection{LockName: true}))
			if w := patchAs(t, tt.identity, rolesPath+pathSep+storedRoleID, jsonBody(t, tt.body)); w.Code != tt.want {
				t.Fatalf("patch = %d %s, want %d", w.Code, w.Body.String(), tt.want)
			}
			if got := storedSpec(t, client, v1alpha1.AccessRoleMetadata, storedRoleID)[keyName]; got != tt.wantName {
				t.Errorf("name = %v, want %s", got, tt.wantName)
			}
		})
	}
}
