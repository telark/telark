package constants

// PermissionLevelWeights defines the global weights for each permission level
// These weights are used to calculate role priority
var PermissionLevelWeights = map[string]int{
	"ReadOnly":    700,
	"Contributor": 800,
	"Owner":       900,
	"Admin":       1000,
}

// BuiltInRolePriorityBoost is added to built-in roles to ensure they have higher priority than custom roles
// This ensures built-in roles always take precedence over custom roles with the same permission level
const BuiltInRolePriorityBoost = 10000

// PriorityCalculationDivisor is used in the priority calculation formula
// Formula: priority = (maxLevelWeight / PriorityCalculationDivisor) + scopeCount
// This matches the example: Admin (1000) on 1 scope = (1000/2.5) + 1 = 400 + 1 = 401
const PriorityCalculationDivisor = 2.5
