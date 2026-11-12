package writes

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	categorydata "github.com/telark/telark/internal/data/classification/category"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/services/exporter/internal/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const (
	otherCategoryID = "cat-0000b-0000-0001"
	firstReferrer   = "referrer-1"
	secondReferrer  = "referrer-2"
	verbList        = "list"
	oneUser         = 1
	twoUsers        = 2
)

var (
	groupMD = v1alpha1.GroupMetadata
	roleMD  = v1alpha1.AccessRoleMetadata
	planMD  = v1alpha1.ProtectionPlanMetadata
)

func twoCategories(scope string) seed {
	md := v1alpha1.CategoryMetadata
	entry := func(id string) map[string]any {
		return map[string]any{keyID: id, keyName: id, keyScope: scope, keyType: string(categorydata.CategoryTypeCustom)}
	}
	spec := map[string]any{constants.FieldCategories: []any{entry(customCategoryID), entry(otherCategoryID)}}
	return crSeed(md, object(md, constants.CategoriesCRDName, spec, nil))
}

func referrer(md base.Metadata, name, field string, ref any) seed {
	return crSeed(md, object(md, name, map[string]any{field: ref}, nil))
}

func oneReferrer(md base.Metadata, field string, ref any) []seed {
	return []seed{referrer(md, firstReferrer, field, ref)}
}

func terminatingReferrer() []seed {
	seeds := oneReferrer(groupMD, constants.FieldCategoryRef, customCategoryID)
	now := metav1.Now()
	seeds[constants.DefaultInitValue].obj.SetDeletionTimestamp(&now)
	return seeds
}

func deleteCustomCategory(t *testing.T) (int, string) {
	t.Helper()
	w := serviceCall(t, http.MethodDelete, categoriesPath+pathSep+customCategoryID, constants.EmptyString)
	return w.Code, w.Body.String()
}

func TestDeleteCategoryRefusedWhileInUse(t *testing.T) {
	cases := []struct {
		name, scope string
		referrers   []seed
		users       int
	}{
		{"group", roledata.ScopeGroups, oneReferrer(groupMD, constants.FieldCategoryRef, customCategoryID), oneUser},
		{"two groups", roledata.ScopeGroups, []seed{
			referrer(groupMD, firstReferrer, constants.FieldCategoryRef, customCategoryID),
			referrer(groupMD, secondReferrer, constants.FieldCategoryRef, customCategoryID),
		}, twoUsers},
		{"role", roledata.ScopeRoles, oneReferrer(roleMD, constants.FieldCategoryRef, customCategoryID), oneUser},
		{"plan kind", categorydata.ScopePlanKinds, oneReferrer(planMD, constants.FieldKindRef, customCategoryID), oneUser},
		{"plan environment", categorydata.ScopePlanEnvironments, oneReferrer(planMD, constants.FieldEnvironmentRef, customCategoryID), oneUser},
		{"plan tag", categorydata.ScopePlanTags, oneReferrer(planMD, constants.FieldTagRefs, []any{otherCategoryID, customCategoryID}), oneUser},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(c.referrers, twoCategories(c.scope))...)
			code, body := deleteCustomCategory(t)
			want := fmt.Sprintf(string(constants.ErrCategoryInUse), c.users)
			if code != http.StatusConflict || !strings.Contains(body, want) {
				t.Fatalf("delete = %d %s, want 409 with %q", code, body, want)
			}
			if got := writes(t, client); len(got) != constants.DefaultInitValue {
				t.Fatalf("writes = %+v, want none", got)
			}
		})
	}
}

func TestDeleteCategoryAllowedWhenUnused(t *testing.T) {
	cases := []struct {
		name, scope string
		referrers   []seed
	}{
		{"no references", roledata.ScopeGroups, nil},
		{"referrer being deleted", roledata.ScopeGroups, terminatingReferrer()},
		{"references another category", roledata.ScopeGroups, oneReferrer(groupMD, constants.FieldCategoryRef, otherCategoryID)},
		{"same id in another plan field", categorydata.ScopePlanKinds, oneReferrer(planMD, constants.FieldEnvironmentRef, customCategoryID)},
		{"tags hold other ids", categorydata.ScopePlanTags, oneReferrer(planMD, constants.FieldTagRefs, []any{otherCategoryID})},
		{"scope nothing references", roledata.ScopeApplications, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(c.referrers, twoCategories(c.scope))...)
			if code, body := deleteCustomCategory(t); code != http.StatusOK {
				t.Fatalf("delete = %d %s, want 200", code, body)
			}
			got := writes(t, client)
			if len(got) != constants.DefaultIncrementValue || got[constants.DefaultInitValue].resource != v1alpha1.CategoryMetadata.Plural {
				t.Fatalf("writes = %+v, want one category write", got)
			}
		})
	}
}

// A delete that can't see the references must not go through.
func TestDeleteCategoryRefusedWhenReferencesCannotBeRead(t *testing.T) {
	client := installFake(t, twoCategories(roledata.ScopeGroups))
	client.PrependReactor(verbList, groupMD.Plural, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("api down")
	})
	if code, body := deleteCustomCategory(t); code != http.StatusInternalServerError {
		t.Fatalf("delete = %d %s, want 500", code, body)
	}
	if got := writes(t, client); len(got) != constants.DefaultInitValue {
		t.Fatalf("writes = %+v, want none", got)
	}
}
