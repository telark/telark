package role

import (
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
)

const unknownLevel roledata.PermissionLevel = "SuperUser"

// Levels rank ReadOnly < Contributor < Owner < Admin, and a level nobody defined grants nothing
// and requires something nobody has, so a typo in a stored role can never widen access.
func TestPermissionLevelCovers(t *testing.T) {
	ordered := []roledata.PermissionLevel{
		roledata.PermissionLevelReadOnly,
		roledata.PermissionLevelContributor,
		roledata.PermissionLevelOwner,
		roledata.PermissionLevelAdmin,
	}
	for i, held := range ordered {
		for j, required := range ordered {
			if got := held.Covers(required); got != (i >= j) {
				t.Errorf("%s.Covers(%s) = %v, want %v", held, required, got, i >= j)
			}
		}
		if held.Covers(unknownLevel) || unknownLevel.Covers(held) {
			t.Errorf("%s and an unknown level cover each other", held)
		}
	}
	if unknownLevel.Rank() >= roledata.PermissionLevelReadOnly.Rank() {
		t.Errorf("an unknown level ranks %d, at or above ReadOnly", unknownLevel.Rank())
	}
}
