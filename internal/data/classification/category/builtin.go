package category

import (
	"slices"

	"github.com/telark/data/constants"
	"github.com/telark/data/resources/role"
)

var BuiltinCategories = []Category{
	{
		ID:           constants.CategoryIDEngineering,
		Name:         "Engineering",
		Scope:        role.ScopeGroups,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDOperations,
		Name:         "Operations",
		Scope:        role.ScopeGroups,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDQualityAssurance,
		Name:         "Quality Assurance",
		Scope:        role.ScopeGroups,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDSecurity,
		Name:         "Security",
		Scope:        role.ScopeGroups,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDPlatform,
		Name:         "Platform",
		Scope:        role.ScopeRoles,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDEnvProduction,
		Name:         "Production",
		Scope:        ScopePlanEnvironments,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDEnvStaging,
		Name:         "Staging",
		Scope:        ScopePlanEnvironments,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDEnvDevelopment,
		Name:         "Development",
		Scope:        ScopePlanEnvironments,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDTagCompliance,
		Name:         "Compliance",
		Scope:        ScopePlanTags,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDTagSecurity,
		Name:         "Security",
		Scope:        ScopePlanTags,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
	{
		ID:           constants.CategoryIDTagBaseline,
		Name:         "Baseline",
		Scope:        ScopePlanTags,
		Type:         CategoryTypeBuiltIn,
		CreationDate: constants.BuiltinCreationDate,
	},
}

// Built-ins and user-created categories share one resource. Built-ins are restored to
// their canonical form and everything else is carried across as-is.
func WithBuiltins(existing []Category) []Category {
	merged := slices.Clone(BuiltinCategories)
	for _, category := range existing {
		if !isBuiltin(category.ID) {
			merged = append(merged, category)
		}
	}
	return merged
}

func isBuiltin(id string) bool {
	return slices.ContainsFunc(BuiltinCategories, func(b Category) bool { return b.ID == id })
}
