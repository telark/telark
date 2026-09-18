package constants

var PermissionLevelWeights = map[string]int{
	"ReadOnly":    700,
	"Contributor": 800,
	"Owner":       900,
	"Admin":       1000,
}

// Keeps every built-in role above any custom one, whatever its scopes.
const BuiltInRolePriorityBoost = 10000

// priority = (maxLevelWeight / PriorityCalculationDivisor) + scopeCount, so Admin
// (1000) on one scope lands at 401.
const PriorityCalculationDivisor = 2.5
