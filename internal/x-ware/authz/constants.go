package authz

import (
	"errors"
	"net/http"

	"github.com/telark/telark/internal/data/constants"
	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/clients/shared"
)

// Stripped from every request before routing: they name an identity, so a
// caller-supplied value would let any client impersonate any user.
var spoofableHeaders = []string{
	constants.HeaderUserID,
	constants.HeaderUsername,
	constants.HeaderEmail,
}

// Methods that change nothing, so a request a page on another origin forges with them gets nothing done.
var safeMethods = []string{http.MethodGet, http.MethodHead, http.MethodOptions}

// The verdicts a resolver that reached its backend answers with; any other
// error is an outage, which must never read as a revoked session or permission.
var (
	ErrNotFound       = shared.ErrNotFound
	ErrGone           = shared.ErrGone
	ErrSessionExpired = errors.New(string(dataerrors.ErrAuthzSessionExpired))
	ErrUserNotActive  = errors.New(string(dataerrors.ErrAuthzUserNotActive))
	ErrUserSuspended  = errors.New(string(dataerrors.ErrAuthzUserSuspended))
	verdicts          = []error{ErrNotFound, ErrGone, ErrSessionExpired, ErrUserNotActive, ErrUserSuspended}
)

const (
	errNilResolver      = "authz: Resolver is required"
	errNilRequirements  = "authz: Requirements is required"
	errNilRouteKey      = "authz: RouteKey is required"
	errEmptyServiceAuth = "authz: ServiceToken is required"
	ruleKeySeparator    = "."
	ruleKeySuffix       = "deny"
	constantTimeEq      = 1
	// The code auth refuses a suspended account's sign-in with, so a client handles both alike.
	refusalAccountSuspended = "account_suspended"
	refusalCSRFHeader       = "csrf_header_missing"
	// The dashboard explains this one in a sentence of its own, so the name is part of the API.
	refusalSessionCookieDuplicate = "session_cookie_duplicate"
	singleCookie                  = 1
	firstCookie                   = 0
	signatureSeparator            = "."
	// Part of every Redis entry signature: changing it invalidates every signed entry.
	signatureLabel = "telark:authz-cache:v1:"
)
