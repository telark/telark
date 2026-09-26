package membership

import (
	"slices"
	"testing"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/membership"
)

const (
	userA  = "u-00001-0000-0001"
	userB  = "u-00002-0000-0002"
	userC  = "u-00003-0000-0003"
	groupX = "ug-00001-0000-0001"
	groupY = "ug-00002-0000-0002"
	groupZ = "ug-00003-0000-0003"
)

func TestWithMember(t *testing.T) {
	tests := []struct {
		name    string
		current []string
		present bool
		want    []string
		changed bool
	}{
		{"add missing", []string{userA}, true, []string{userA, userB}, true},
		{"add present", []string{userA, userB}, true, []string{userA, userB}, false},
		{"remove present", []string{userA, userB}, false, []string{userA}, true},
		{"remove missing", []string{userA}, false, []string{userA}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := membership.WithMember(tt.current, userB, tt.present)
			if changed != tt.changed || !slices.Equal(got, tt.want) {
				t.Fatalf("WithMember = %v, %v; want %v, %v", got, changed, tt.want, tt.changed)
			}
		})
	}
}

// The user side grants, so the repair must not change anyone's access: a
// group missing a user that names it gains that user, a group listing a user
// who does not name it loses that entry, and user lists only lose duplicates
// and groups that no longer exist.
func TestPlanAlignsGroupsToUsers(t *testing.T) {
	userGroups := map[string][]string{
		userA: {groupX, groupX, groupY},
		userB: {groupX, groupZ},
		userC: {},
	}
	groupMembers := map[string][]string{
		groupX: {userA, userC},
		groupY: {userA},
	}

	users, groups := membership.Plan(userGroups, groupMembers)

	if !slices.Equal(users[userA], []string{groupX, groupY}) {
		t.Errorf("userA = %v, want duplicates dropped", users[userA])
	}
	if !slices.Equal(users[userB], []string{groupX}) {
		t.Errorf("userB = %v, want the missing group dropped and groupX kept", users[userB])
	}
	if _, patched := users[userC]; patched {
		t.Error("userC needs no patch")
	}
	if !slices.Equal(groups[groupX], []string{userA, userB}) {
		t.Errorf("groupX = %v, want userC dropped and userB added", groups[groupX])
	}
	if _, patched := groups[groupY]; patched {
		t.Error("groupY needs no patch")
	}
	if len(groups) != constants.DefaultIncrementValue {
		t.Errorf("groups patched = %v, want only groupX", groups)
	}
}
