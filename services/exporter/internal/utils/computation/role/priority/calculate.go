package priority

import (
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	roleconstants "github.com/telark/exporter/internal/utils/computation/role/constants"
)

func Calculate(role *roledata.RoleAsResource) int {
	if len(role.ScopesAndPermissions) == constants.DefaultInitValue {
		return constants.DefaultInitValue
	}

	// Find the maximum permission level weight
	maxWeight := constants.DefaultInitValue
	for _, scope := range role.ScopesAndPermissions {
		weight := roleconstants.PermissionLevelWeights[string(scope.Level)]
		if weight > maxWeight {
			maxWeight = weight
		}
	}

	// Calculate base priority: (maxLevelWeight / divisor) + scopeCount
	scopeCount := len(role.ScopesAndPermissions)
	basePriority := int(float64(maxWeight)/roleconstants.PriorityCalculationDivisor) + scopeCount

	// Built-in roles get priority boost to ensure they rank higher than custom roles
	if role.Type == roledata.RoleTypeBuiltIn {
		return basePriority + roleconstants.BuiltInRolePriorityBoost
	}

	return basePriority
}
