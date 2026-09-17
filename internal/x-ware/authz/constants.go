package authz

import (
	"errors"

	"github.com/telark/data/constants"
	dataerrors "github.com/telark/data/errors"
	"github.com/telark/rest/clients/shared"
)

// Stripped from every request before routing: they name an identity, so a
// caller-supplied value would let any client impersonate any user.
var spoofableHeaders = []string{
	constants.HeaderUserID,
	constants.HeaderUsername,
	constants.HeaderEmail,
}

// A resolver that reached its backend and was refused answers with one of
// these. Any other error means the backend could not answer, and an outage
// must never be reported as a revoked session or a missing permission.
var (
	ErrNotFound       = shared.ErrNotFound
	ErrSessionExpired = errors.New(string(dataerrors.ErrAuthzSessionExpired))
	ErrUserNotActive  = errors.New(string(dataerrors.ErrAuthzUserNotActive))
	verdicts          = []error{ErrNotFound, ErrSessionExpired, ErrUserNotActive}
)

const (
	errNilResolver      = "authz: Resolver is required"
	errNilRequirements  = "authz: Requirements is required"
	errNilRouteKey      = "authz: RouteKey is required"
	errEmptyServiceAuth = "authz: ServiceToken is required"
	ruleKeySeparator    = "."
	ruleKeySuffix       = "deny"
	constantTimeEq      = 1
)
