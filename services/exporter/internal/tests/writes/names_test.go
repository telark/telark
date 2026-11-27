package writes

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	dataerrors "github.com/telark/telark/internal/data/errors"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const (
	takenRoleName  = "oncall-engineer"
	takenGroupName = "release-crew"
	renamedRoleID  = "r-00007-0000-0007"
	renamedGroupID = "ug-00004-0000-0004"
	listDown       = "api down"
)

func namedRole(t *testing.T, id, name string, status roledata.RoleStatus) seed {
	t.Helper()
	return crSeedOf(t, roleMD, id, roledata.AccessRole{
		Name: name, Description: newDesc, CategoryRef: testCategory, Type: roledata.RoleTypeCustom,
		Status: status, Protection: &roledata.Protection{},
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeRoles, Level: roledata.PermissionLevelReadOnly}},
	})
}

func namedGroup(t *testing.T, id, name string) seed {
	t.Helper()
	return crSeedOf(t, groupMD, id, groupdata.Group{Name: name, Description: newDesc, CategoryRef: testCategory})
}

func terminating(s seed) seed {
	now := metav1.Now()
	s.obj.SetDeletionTimestamp(&now)
	return s
}

func roleCreateBody(name string) map[string]any {
	return map[string]any{
		keyName: name, keyDesc: newDesc, constants.FieldCategoryRef: testCategory,
		constants.FieldScopesAndPermissions: []any{map[string]any{
			constants.FieldScope: roledata.ScopeRoles, keyLevel: string(roledata.PermissionLevelReadOnly),
		}},
	}
}

func groupCreateBody(name string) map[string]any {
	return map[string]any{keyName: name, keyDesc: newDesc, constants.FieldCategoryRef: testCategory}
}

func responseMessage(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("response %s: %v", body, err)
	}
	return envelope.Message
}

// Seen live: a second role and a second group named like an existing one were both created.
func TestDuplicateRoleAndGroupNamesAreRefused(t *testing.T) {
	tests := []struct {
		name, method, path string
		body               map[string]any
		want               dataerrors.Error
	}{
		{"role create", http.MethodPost, rolesPath, roleCreateBody(pad(strings.ToUpper(takenRoleName))), constants.ErrRoleNameAlreadyExists},
		{"role rename", http.MethodPatch, rolesPath + pathSep + renamedRoleID,
			map[string]any{keyName: strings.ToUpper(takenRoleName)}, constants.ErrRoleNameAlreadyExists},
		{"group create", http.MethodPost, groupsPath, groupCreateBody(pad(strings.ToUpper(takenGroupName))), constants.ErrGroupNameAlreadyExists},
		{"group rename", http.MethodPatch, groupsPath + pathSep + renamedGroupID,
			map[string]any{keyName: strings.ToUpper(takenGroupName)}, constants.ErrGroupNameAlreadyExists},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t,
				namedRole(t, storedRoleID, takenRoleName, roledata.RoleStatusActive),
				namedRole(t, renamedRoleID, newName, roledata.RoleStatusActive),
				namedGroup(t, storedGroupID, takenGroupName),
				namedGroup(t, renamedGroupID, newName),
			)
			w := serveAs(t, adminCaller(), tt.method, tt.path, jsonBody(t, tt.body))
			if w.Code != http.StatusConflict || responseMessage(t, w.Body.Bytes()) != string(tt.want) {
				t.Fatalf("%s = %d %s, want 409 %q", tt.name, w.Code, w.Body.String(), tt.want)
			}
			if got := writes(t, client); len(got) != constants.DefaultInitValue {
				t.Fatalf("writes = %+v, want none", got)
			}
		})
	}
}

// A record on its way out no longer holds its name, and a record keeps its own.
func TestRoleAndGroupNamesFreeToUse(t *testing.T) {
	tests := []struct {
		name, method, path, resource string
		body                         map[string]any
		existing                     func(t *testing.T) seed
	}{
		{"role create beside a role being deleted", http.MethodPost, rolesPath, roleMD.Plural, roleCreateBody(takenRoleName),
			func(t *testing.T) seed {
				return terminating(namedRole(t, storedRoleID, takenRoleName, roledata.RoleStatusActive))
			}},
		{"role create beside a soft-deleted role", http.MethodPost, rolesPath, roleMD.Plural, roleCreateBody(takenRoleName),
			func(t *testing.T) seed { return namedRole(t, storedRoleID, takenRoleName, roledata.RoleStatusDeleted) }},
		{"role renamed to its own name in capitals", http.MethodPatch, rolesPath + pathSep + storedRoleID, roleMD.Plural,
			map[string]any{keyName: strings.ToUpper(takenRoleName)},
			func(t *testing.T) seed { return namedRole(t, storedRoleID, takenRoleName, roledata.RoleStatusActive) }},
		{"group create beside a group being deleted", http.MethodPost, groupsPath, groupMD.Plural, groupCreateBody(takenGroupName),
			func(t *testing.T) seed { return terminating(namedGroup(t, storedGroupID, takenGroupName)) }},
		{"group renamed to its own name in capitals", http.MethodPatch, groupsPath + pathSep + storedGroupID, groupMD.Plural,
			map[string]any{keyName: strings.ToUpper(takenGroupName)},
			func(t *testing.T) seed { return namedGroup(t, storedGroupID, takenGroupName) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, tt.existing(t))
			k8sclient.SetDynamicClient(jsonCreates{client})
			w := serveAs(t, adminCaller(), tt.method, tt.path, jsonBody(t, tt.body))
			if w.Code != http.StatusOK {
				t.Fatalf("%s = %d %s, want 200", tt.name, w.Code, w.Body.String())
			}
			if !slices.ContainsFunc(writes(t, client), func(got write) bool { return got.resource == tt.resource }) {
				t.Fatalf("%s wrote no %s", tt.name, tt.resource)
			}
		})
	}
}

// A create that can't read the names in use must not go through.
func TestRoleCreateRefusedWhenNamesCannotBeRead(t *testing.T) {
	client := installFake(t)
	client.PrependReactor(verbList, roleMD.Plural, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New(listDown)
	})
	w := serveAs(t, adminCaller(), http.MethodPost, rolesPath, jsonBody(t, roleCreateBody(takenRoleName)))
	if w.Code < http.StatusInternalServerError {
		t.Fatalf("create = %d %s, want a server error", w.Code, w.Body.String())
	}
	if got := writes(t, client); len(got) != constants.DefaultInitValue {
		t.Fatalf("writes = %+v, want none", got)
	}
}

// A role leaving Deleted takes its name back: one whose name went to another role stays out.
func TestRestoredRoleCannotTakeBackATakenName(t *testing.T) {
	client := installFake(t,
		namedRole(t, storedRoleID, takenRoleName, roledata.RoleStatusDeleted),
		namedRole(t, renamedRoleID, takenRoleName, roledata.RoleStatusActive),
	)
	restore := map[string]any{constants.FieldStatus: string(roledata.RoleStatusActive)}
	w := serveAs(t, adminCaller(), http.MethodPatch, rolesPath+pathSep+storedRoleID, jsonBody(t, restore))
	if w.Code != http.StatusConflict || responseMessage(t, w.Body.Bytes()) != string(constants.ErrRoleNameAlreadyExists) {
		t.Fatalf("restore = %d %s, want 409 %q", w.Code, w.Body.String(), constants.ErrRoleNameAlreadyExists)
	}
	if got := writes(t, client); len(got) != constants.DefaultInitValue {
		t.Fatalf("writes = %+v, want none", got)
	}
}
