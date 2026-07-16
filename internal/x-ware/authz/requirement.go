package authz

import roledata "github.com/telark/data/resources/role"

var (
	// For endpoints infrastructure calls without a session, such as probes.
	Public = Requirement{Access: AccessPublic}
	// For endpoints only a peer may reach, usually because they run pre-session.
	Internal = Requirement{Access: AccessInternal}
	// For endpoints acting only on the callers own record. Always pair with a
	// handler guard that narrows the route.
	Authenticated = Requirement{Access: AccessAuthenticated}
)

func Read(scope string) Requirement {
	return Requirement{Scope: scope, MinLevel: roledata.PermissionLevelReadOnly}
}

func Write(scope string) Requirement {
	return Requirement{Scope: scope, MinLevel: roledata.PermissionLevelContributor}
}

func Own(scope string) Requirement {
	return Requirement{Scope: scope, MinLevel: roledata.PermissionLevelOwner}
}

func Administer(scope string) Requirement {
	return Requirement{Scope: scope, MinLevel: roledata.PermissionLevelAdmin}
}

// Marks a route a role can withhold on its own.
func Denyable(requirement Requirement, action string) Requirement {
	requirement.Rule = RuleKey(requirement.Scope, action)
	return requirement
}
