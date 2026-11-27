package writes

import (
	"net/http"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	roledata "github.com/telark/telark/internal/data/resources/role"
	restconstants "github.com/telark/telark/internal/rest/constants"
	cleanupendpoints "github.com/telark/telark/internal/rest/endpoints/cleanup"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func withFinalizer(s seed, finalizer string) seed {
	s.obj.SetFinalizers([]string{finalizer})
	return s
}

// Seen in the code: a categories list read from the cluster answered the raw format string of a single read.
func TestCategoriesListAnswersAListMessage(t *testing.T) {
	installFake(t, categorySeed())
	w := serveAs(t, adminCaller(), http.MethodGet, categoriesPath, constants.EmptyString)
	if got := responseMessage(t, w.Body.Bytes()); w.Code != http.StatusOK || got != string(messages.SuccessListRes) {
		t.Fatalf("list = %d %q, want 200 %q", w.Code, got, messages.SuccessListRes)
	}
}

// Seen live: after a role delete the roles list kept the role for a minute or more while a GET by id
// answered 404. The list cached while the cleanup finalizer held the record outlived the record.
func TestListDropsARecordOnceItsFinalizerGoes(t *testing.T) {
	tests := []struct {
		name, kind, finalizer, listPath, id string
		md                                  base.Metadata
		reader                              xauthz.Identity
		record                              func(t *testing.T) seed
	}{
		{"role", finalizers.ResourceTypeRoles, finalizers.RoleCleanup, rolesPath, storedRoleID, roleMD, adminCaller(), func(t *testing.T) seed {
			return namedRole(t, storedRoleID, takenRoleName, roledata.RoleStatusActive)
		}},
		{"group", finalizers.ResourceTypeGroups, finalizers.GroupCleanup, groupsPath, storedGroupID, groupMD, adminCaller(), func(t *testing.T) seed {
			return namedGroup(t, storedGroupID, takenGroupName)
		}},
		// An administrator's users list is never cached; a service's is.
		{"user", finalizers.ResourceTypeUsers, finalizers.UserCleanup, usersPath, plainUser, v1alpha1.UserMetadata, xauthz.Identity{Internal: true},
			func(t *testing.T) seed {
				return adminUser(t, plainUser, nil, nil, false)
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, terminating(withFinalizer(tt.record(t), tt.finalizer)))
			h := newRouter(t)
			if w := serveOn(t, h, tt.reader, http.MethodGet, tt.listPath, constants.EmptyString); !strings.Contains(w.Body.String(), tt.id) {
				t.Fatalf("list while terminating = %d %s, want the record", w.Code, w.Body.String())
			}

			path := strings.NewReplacer(restconstants.TypeParam, tt.kind, restconstants.IDParam, tt.id).
				Replace(router.Pattern(cleanupendpoints.RemoveFinalizer))
			removed := serveOn(t, h, xauthz.Identity{Internal: true}, http.MethodDelete, path, fieldBody(keyName, tt.finalizer))
			if removed.Code != http.StatusOK {
				t.Fatalf("finalizer removal = %d %s", removed.Code, removed.Body.String())
			}
			// The API server drops the record with its last finalizer; the fake keeps it.
			if err := client.Tracker().Delete(gvrOf(tt.md), tt.md.Namespace, tt.id); err != nil {
				t.Fatal(err)
			}

			if w := serveOn(t, h, tt.reader, http.MethodGet, tt.listPath, constants.EmptyString); strings.Contains(w.Body.String(), tt.id) {
				t.Fatalf("list after the removal = %d %s, still holds the record", w.Code, w.Body.String())
			}
		})
	}
}
