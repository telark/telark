package resources

import (
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/telark/exporter/internal/constants"
	grouputil "github.com/telark/exporter/internal/utils/resources/group"
	userutil "github.com/telark/exporter/internal/utils/resources/user"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	roleOne  = "r-00001-0000-0001"
	roleTwo  = "r-00002-0000-0002"
	memberA  = "u-0000a-0000-000a"
	memberB  = "u-0000b-0000-000b"
	emailOld = "Jane.Doe@Example.com"
	emailNew = " jane.doe@example.com "
)

func TestExtractSpecsDedupeIDs(t *testing.T) {
	user, err := userutil.ExtractUserSpecFromRequestBody(map[string]any{
		constants.FieldRoleRefs:  []any{roleOne, roleOne, roleTwo},
		constants.FieldGroupRefs: []any{memberA, memberA},
	})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(user.RoleRefs) != wantReplacedIDs || len(user.GroupRefs) != constants.DefaultIncrementValue {
		t.Errorf("user ids not deduplicated: %d roles, %d groups", len(user.RoleRefs), len(user.GroupRefs))
	}

	group, err := grouputil.ExtractGroupSpecFromRequestBody(map[string]any{
		constants.FieldUserRefs: []any{memberA, memberB, memberA},
		constants.FieldRoleRefs: []any{},
	})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if !slices.Equal(group.UserRefs, []string{memberA, memberB}) || group.RoleRefs == nil {
		t.Errorf("group ids not deduplicated or initialized: %v %v", group.UserRefs, group.RoleRefs)
	}
}

// Two spellings of one mailbox are one email; an unchanged email never
// triggers the cluster uniqueness lookup.
func TestEmailNormalization(t *testing.T) {
	if userutil.NormalizeEmail(emailOld) != userutil.NormalizeEmail(emailNew) {
		t.Fatal("case and whitespace must not distinguish emails")
	}
	if !userutil.CheckEmailChangeAllowed(emailOld, emailNew, httptest.NewRecorder()) {
		t.Fatal("unchanged email was checked against the cluster")
	}
	if err := userutil.CheckEmailExists(constants.EmptyString); err != nil {
		t.Fatalf("empty email must never be a duplicate: %v", err)
	}
}

func TestStripMembers(t *testing.T) {
	group := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{
		constants.FieldUserRefs: []any{memberA, memberB},
	}}}
	grouputil.StripMembers(group, map[string]bool{memberB: true})
	members, _, _ := unstructured.NestedStringSlice(group.Object, constants.SpecField, constants.FieldUserRefs)
	if !slices.Equal(members, []string{memberA}) {
		t.Fatalf("members = %v, want hidden member stripped", members)
	}
	grouputil.StripMembers(group, nil)
	members, _, _ = unstructured.NestedStringSlice(group.Object, constants.SpecField, constants.FieldUserRefs)
	if !slices.Equal(members, []string{memberA}) {
		t.Fatalf("members = %v, want untouched with nothing to hide", members)
	}
}
