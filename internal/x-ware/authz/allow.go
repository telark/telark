package authz

import (
	"slices"

	dataconstants "github.com/telark/data/constants"
	roledata "github.com/telark/data/resources/role"
)

// Deny is checked before level, so withholding one action does not depend on
// the level the role holds.
func Allows(id Identity, req Requirement) bool {
	if isRuleDenied(id.Grants, req) {
		return false
	}

	granted, ok := grantedLevel(id.Grants, req.Scope)
	if !ok {
		return false
	}

	return granted.Covers(req.MinLevel)
}

// A rule denies one action, not the whole scope: a role denied
// "applications.deleteapplication" keeps every other application action.
func isRuleDenied(grants Grants, req Requirement) bool {
	if req.Rule == dataconstants.EmptyString {
		return false
	}

	return slices.Contains(grants.Denied[req.Scope], req.Rule) ||
		slices.Contains(grants.Denied[roledata.ScopeAll], req.Rule)
}

func grantedLevel(grants Grants, scope string) (roledata.PermissionLevel, bool) {
	if level, ok := grants.Levels[scope]; ok {
		return level, true
	}

	level, ok := grants.Levels[roledata.ScopeAll]
	return level, ok
}

// Roles are additive: the strongest level any role grants for a scope wins.
func MergeLevel(levels map[string]roledata.PermissionLevel, scope string, candidate roledata.PermissionLevel) {
	if candidate.Rank() == dataconstants.DefaultInitValue {
		return
	}

	current, exists := levels[scope]
	if !exists || candidate.Rank() > current.Rank() {
		levels[scope] = candidate
	}
}
