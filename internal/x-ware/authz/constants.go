package authz

import "github.com/telark/data/constants"

// Stripped from every request before routing: they name an identity, so a
// caller-supplied value would let any client impersonate any user.
var spoofableHeaders = []string{
	constants.HeaderUserID,
	constants.HeaderUsername,
	constants.HeaderEmail,
}

const (
	errNilResolver      = "authz: Resolver is required"
	errNilRequirements  = "authz: Requirements is required"
	errNilRouteKey      = "authz: RouteKey is required"
	errEmptyServiceAuth = "authz: ServiceToken is required"
	ruleKeySeparator    = "."
	ruleKeySuffix       = "deny"
	constantTimeEq      = 1
)
