package builtin

import (
	"slices"
	"testing"

	"github.com/telark/data/classification/category"
	"github.com/telark/data/constants"
	"github.com/telark/data/resources/role"
)

func TestBuiltinRoleCategoriesResolve(t *testing.T) {
	known := make(map[string]category.Category, len(category.BuiltinCategories))
	for _, c := range category.BuiltinCategories {
		if _, dup := known[c.ID]; dup {
			t.Fatalf("duplicate builtin category ID %s", c.ID)
		}
		known[c.ID] = c
	}

	for _, r := range role.BuiltinRoles {
		c, ok := known[r.CategoryID]
		if !ok {
			t.Errorf("role %s references unknown category %s", r.Name, r.CategoryID)
			continue
		}
		if c.Scope != role.ScopeRoles {
			t.Errorf("role %s sits in category %s scoped %s, want %s", r.Name, c.Name, c.Scope, role.ScopeRoles)
		}
	}
}

func TestBuiltinRoleScopes(t *testing.T) {
	scopes := []string{
		role.ScopeApplications,
		role.ScopeGroups,
		role.ScopeUsers,
		role.ScopeRoles,
		role.ScopeSettings,
		role.ScopeProtectionPlans,
	}
	uniform := map[string]role.PermissionLevel{
		constants.RoleIDOwner:       role.PermissionLevelOwner,
		constants.RoleIDContributor: role.PermissionLevelContributor,
		constants.RoleIDReadOnly:    role.PermissionLevelReadOnly,
	}

	if len(role.BuiltinRoles) != len(uniform)+constants.SingleItem {
		t.Fatalf("got %d builtin roles, want %d", len(role.BuiltinRoles), len(uniform)+constants.SingleItem)
	}

	for _, r := range role.BuiltinRoles {
		granted := make(map[string]role.PermissionLevel, len(r.ScopesAndPermissions))
		for _, sp := range r.ScopesAndPermissions {
			granted[sp.Scope] = sp.Level
		}

		if r.ID == constants.RoleIDAdmin {
			if len(granted) != constants.SingleItem || granted[role.ScopeAll] != role.PermissionLevelAdmin {
				t.Errorf("Admin grants %v, want only %s=%s", granted, role.ScopeAll, role.PermissionLevelAdmin)
			}
			continue
		}

		want, ok := uniform[r.ID]
		if !ok {
			t.Errorf("unexpected builtin role %s (%s)", r.Name, r.ID)
			continue
		}
		if len(granted) != len(scopes) {
			t.Errorf("role %s grants %d scopes, want %d", r.Name, len(granted), len(scopes))
		}
		for _, scope := range scopes {
			if granted[scope] != want {
				t.Errorf("role %s grants %s=%q, want %q", r.Name, scope, granted[scope], want)
			}
		}
	}
}

func TestWithBuiltinsKeepsUserCategoriesAndRestoresBuiltins(t *testing.T) {
	userCategory := category.Category{
		ID:           "cat-user-0001",
		Name:         "Team Falcon",
		Scope:        role.ScopeGroups,
		Type:         category.CategoryTypeCustom,
		CreationDate: "2026-01-01T00:00:00Z",
	}
	tampered := category.BuiltinCategories[constants.DefaultInitValue]
	tampered.Name = "Renamed By Hand"
	tampered.Scope = role.ScopeRoles

	merged := category.WithBuiltins([]category.Category{tampered, userCategory})

	if len(merged) != len(category.BuiltinCategories)+constants.SingleItem {
		t.Fatalf("got %d categories, want %d", len(merged), len(category.BuiltinCategories)+constants.SingleItem)
	}

	for _, want := range category.BuiltinCategories {
		idx := slices.IndexFunc(merged, func(c category.Category) bool { return c.ID == want.ID })
		if idx < constants.DefaultInitValue {
			t.Errorf("built-in %s missing after merge", want.ID)
			continue
		}
		if merged[idx] != want {
			t.Errorf("built-in %s = %+v, want %+v", want.ID, merged[idx], want)
		}
	}

	if !slices.Contains(merged, userCategory) {
		t.Errorf("user category %s was dropped by the merge", userCategory.ID)
	}
}

func TestWithBuiltinsOnEmptySeedsOnlyBuiltins(t *testing.T) {
	merged := category.WithBuiltins(nil)
	if !slices.Equal(merged, category.BuiltinCategories) {
		t.Errorf("got %+v, want the built-in set", merged)
	}
}

func TestBuiltinRolesAreProtected(t *testing.T) {
	for _, r := range role.BuiltinRoles {
		if r.Protection == nil {
			t.Errorf("role %s has no protection", r.Name)
			continue
		}
		if !r.Protection.PreventDeletion || !r.Protection.PreventModification || !r.Protection.PreventScopeChanges {
			t.Errorf("role %s is not fully protected: %+v", r.Name, *r.Protection)
		}
		if r.Validity == nil || r.Validity.Type != role.ValidityTypePermanent {
			t.Errorf("role %s is not permanent", r.Name)
		}
	}
}

func TestBuiltinRoleProtectionNotShared(t *testing.T) {
	seen := make(map[*role.Protection]string, len(role.BuiltinRoles))
	for _, r := range role.BuiltinRoles {
		if other, dup := seen[r.Protection]; dup {
			t.Errorf("roles %s and %s share one Protection pointer", other, r.Name)
		}
		seen[r.Protection] = r.Name
	}
}
