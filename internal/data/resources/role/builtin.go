package role

import "github.com/telark/data/constants"

const builtinVersion = "v1.0.0"

const (
	builtinPriorityAdmin       = 10401
	builtinPriorityOwner       = 10365
	builtinPriorityContributor = 10325
	builtinPriorityReadOnly    = 10285
)

var builtinScopes = []string{
	ScopeApplications,
	ScopeGroups,
	ScopeUsers,
	ScopeRoles,
	ScopeSettings,
	ScopeProtectionPlans,
}

func builtinProtection() *Protection {
	return &Protection{
		PreventDeletion:     true,
		PreventModification: true,
		PreventScopeChanges: true,
		LockName:            true,
		LockCategory:        true,
	}
}

func builtinValidity() *Validity {
	return &Validity{Type: ValidityTypePermanent}
}

func uniformPermissions(level PermissionLevel) []ScopeAndPermissions {
	permissions := make([]ScopeAndPermissions, constants.DefaultInitValue, len(builtinScopes))
	for _, scope := range builtinScopes {
		permissions = append(permissions, ScopeAndPermissions{Scope: scope, Level: level})
	}
	return permissions
}

var BuiltinRoles = []RoleAsResource{
	{
		ID:                   constants.RoleIDAdmin,
		Name:                 "Admin",
		Description:          "Top-level authority with full access across all scopes",
		Version:              builtinVersion,
		Type:                 RoleTypeBuiltIn,
		Priority:             builtinPriorityAdmin,
		CategoryID:           constants.CategoryIDPlatform,
		Status:               RoleStatusActive,
		ScopesAndPermissions: []ScopeAndPermissions{{Scope: ScopeAll, Level: PermissionLevelAdmin}},
		Protection:           builtinProtection(),
		Validity:             builtinValidity(),
		CreationDate:         constants.BuiltinCreationDate,
	},
	{
		ID:                   constants.RoleIDOwner,
		Name:                 "Owner",
		Description:          "Full control of resources and can assign roles within assigned scope",
		Version:              builtinVersion,
		Type:                 RoleTypeBuiltIn,
		Priority:             builtinPriorityOwner,
		CategoryID:           constants.CategoryIDPlatform,
		Status:               RoleStatusActive,
		ScopesAndPermissions: uniformPermissions(PermissionLevelOwner),
		Protection:           builtinProtection(),
		Validity:             builtinValidity(),
		CreationDate:         constants.BuiltinCreationDate,
	},
	{
		ID:                   constants.RoleIDContributor,
		Name:                 "Contributor",
		Description:          "Can create and modify resources within assigned scope",
		Version:              builtinVersion,
		Type:                 RoleTypeBuiltIn,
		Priority:             builtinPriorityContributor,
		CategoryID:           constants.CategoryIDPlatform,
		Status:               RoleStatusActive,
		ScopesAndPermissions: uniformPermissions(PermissionLevelContributor),
		Protection:           builtinProtection(),
		Validity:             builtinValidity(),
		CreationDate:         constants.BuiltinCreationDate,
	},
	{
		ID:                   constants.RoleIDReadOnly,
		Name:                 "ReadOnly",
		Description:          "Can only view resources within assigned scope",
		Version:              builtinVersion,
		Type:                 RoleTypeBuiltIn,
		Priority:             builtinPriorityReadOnly,
		CategoryID:           constants.CategoryIDPlatform,
		Status:               RoleStatusActive,
		ScopesAndPermissions: uniformPermissions(PermissionLevelReadOnly),
		Protection:           builtinProtection(),
		Validity:             builtinValidity(),
		CreationDate:         constants.BuiltinCreationDate,
	},
}
