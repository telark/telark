package priority

import (
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/services/exporter/internal/constants"
	roleconstants "github.com/telark/telark/services/exporter/internal/utils/compute/role/constants"
)

func Calculate(role *roledata.AccessRole) int {
	if len(role.ScopesAndPermissions) == constants.DefaultInitValue {
		return constants.DefaultInitValue
	}

	maxWeight := constants.DefaultInitValue
	for _, scope := range role.ScopesAndPermissions {
		weight := roleconstants.PermissionLevelWeights[string(scope.Level)]
		if weight > maxWeight {
			maxWeight = weight
		}
	}

	scopeCount := len(role.ScopesAndPermissions)
	basePriority := int(float64(maxWeight)/roleconstants.PriorityCalculationDivisor) + scopeCount

	if role.Type == roledata.RoleTypeBuiltIn {
		return basePriority + roleconstants.BuiltInRolePriorityBoost
	}

	return basePriority
}
