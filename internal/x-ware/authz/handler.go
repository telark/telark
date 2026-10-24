package authz

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"slices"

	dataconstants "github.com/telark/telark/internal/data/constants"
	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/response"
)

type middleware struct {
	config        Config
	sessionCookie string
}

type denial struct {
	status  int
	message string
	code    string
}

func New(config Config) (func(http.Handler) http.Handler, error) {
	if err := validate(config); err != nil {
		return nil, err
	}

	m := &middleware{config: config, sessionCookie: SessionCookieName()}
	return m.wrap, nil
}

func validate(config Config) error {
	switch {
	case config.Resolver == nil:
		return errors.New(errNilResolver)
	case config.Requirements == nil:
		return errors.New(errNilRequirements)
	case config.RouteKey == nil:
		return errors.New(errNilRouteKey)
	case config.ServiceToken == dataconstants.EmptyString:
		return errors.New(errEmptyServiceAuth)
	default:
		return nil
	}
}

func (m *middleware) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claimedUserID := r.Header.Get(dataconstants.HeaderUserID)
		stripSpoofableHeaders(r)
		refusal, duplicate := sessionFromCookie(r, m.sessionCookie)
		if refusal != nil {
			reject(w, *refusal)
			return
		}

		requirement, found := m.config.Requirements[m.config.RouteKey(r)]
		if !found {
			reject(w, denial{status: http.StatusForbidden, message: string(dataerrors.ErrAuthzForbidden)})
			return
		}

		if requirement.Access == AccessPublic {
			next.ServeHTTP(w, r)
			return
		}
		if duplicate {
			reject(w, denial{status: http.StatusUnauthorized, message: string(dataerrors.ErrAuthzSessionCookieDuplicate),
				code: refusalSessionCookieDuplicate})
			return
		}

		identity, rejection := m.identify(r, requirement, claimedUserID)
		if rejection != nil {
			reject(w, *rejection)
			return
		}

		next.ServeHTTP(w, applyIdentity(r, identity))
	})
}

func (m *middleware) identify(r *http.Request, req Requirement, claimedUserID string) (Identity, *denial) {
	if serviceToken := r.Header.Get(dataconstants.HeaderServiceToken); serviceToken != dataconstants.EmptyString {
		if !m.serviceTokenMatches(serviceToken) {
			return Identity{}, &denial{status: http.StatusUnauthorized, message: string(dataerrors.ErrAuthzInvalidServiceToken)}
		}
		return Identity{UserID: claimedUserID, Internal: true}, nil
	}

	if req.Access == AccessInternal {
		return Identity{}, &denial{status: http.StatusUnauthorized, message: string(dataerrors.ErrAuthzInvalidServiceToken)}
	}

	return m.identifyUser(r, req)
}

func (m *middleware) identifyUser(r *http.Request, req Requirement) (Identity, *denial) {
	sessionToken := r.Header.Get(dataconstants.HeaderSessionToken)
	if sessionToken == dataconstants.EmptyString {
		return Identity{}, &denial{status: http.StatusUnauthorized, message: string(dataerrors.ErrAuthzMissingSessionToken)}
	}

	userID, err := m.config.Resolver.UserIDForToken(sessionToken)
	if err != nil {
		return Identity{}, denialFor(err, http.StatusUnauthorized, dataerrors.ErrAuthzInvalidSession)
	}

	grants, err := m.config.Resolver.GrantsForUser(userID)
	if err != nil {
		return Identity{}, denialFor(err, http.StatusForbidden, dataerrors.ErrAuthzForbidden)
	}

	identity := Identity{UserID: userID, Grants: grants}
	if req.Access == AccessAuthenticated {
		return identity, nil
	}

	if !Allows(identity, req) {
		return Identity{}, &denial{status: http.StatusForbidden, message: string(dataerrors.ErrAuthzForbidden)}
	}

	return identity, nil
}

// A long-lived request re-checks its session with this, under the same rule as denialFor:
// a verdict (expired, deleted, suspended) ends it, an outage does not.
func SessionEnded(resolver Resolver, token string) bool {
	userID, err := resolver.UserIDForToken(token)
	if err == nil {
		_, err = resolver.GrantsForUser(userID)
	}
	return err != nil && isVerdict(err)
}

func isVerdict(err error) bool {
	return slices.ContainsFunc(verdicts, func(verdict error) bool { return errors.Is(err, verdict) })
}

// Only a backend that answered gets to deny; one that could not answer is an
// outage, and an outage must never log a user out.
func denialFor(err error, status int, message dataerrors.Error) *denial {
	if !isVerdict(err) {
		return &denial{status: http.StatusServiceUnavailable, message: string(dataerrors.ErrAuthzResolverUnavailable)}
	}
	// The caller holds this account's own session, so naming the suspension leaks nothing.
	if errors.Is(err, ErrUserSuspended) {
		return &denial{status: status, message: string(dataerrors.ErrAuthzUserSuspended), code: refusalAccountSuspended}
	}
	return &denial{status: status, message: string(message)}
}

func (m *middleware) serviceTokenMatches(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(m.config.ServiceToken)) == constantTimeEq
}

func stripSpoofableHeaders(r *http.Request) {
	for _, header := range spoofableHeaders {
		r.Header.Del(header)
	}
}

// A browser sends its session as the cookie, an API client as the header, which wins. A cookie travels with any
// request the browser makes, so a change that carries no header credential also needs X-Requested-With, which a
// page on another origin can't add: that stops cross-site forms, sibling subdomains and sign-in CSRF alike. Only the
// install's name counts (SessionCookieName), and more than one such cookie counts as none (duplicate): a sibling
// subdomain can plant a plain one, and servers differ on which they read.
func sessionFromCookie(r *http.Request, name string) (refusal *denial, duplicate bool) {
	if r.Header.Get(dataconstants.HeaderSessionToken) != dataconstants.EmptyString ||
		r.Header.Get(dataconstants.HeaderServiceToken) != dataconstants.EmptyString {
		return nil, false
	}
	if !slices.Contains(safeMethods, r.Method) && r.Header.Get(dataconstants.HeaderRequestedWith) == dataconstants.EmptyString {
		return &denial{status: http.StatusForbidden, message: string(dataerrors.ErrAuthzCSRFHeaderMissing), code: refusalCSRFHeader}, false
	}
	cookies := r.CookiesNamed(name)
	if len(cookies) > singleCookie {
		return nil, true
	}
	if len(cookies) == singleCookie && cookies[firstCookie].Value != dataconstants.EmptyString {
		r.Header.Set(dataconstants.HeaderSessionToken, cookies[firstCookie].Value)
	}
	return nil, false
}

func applyIdentity(r *http.Request, identity Identity) *http.Request {
	if identity.UserID != dataconstants.EmptyString {
		r.Header.Set(dataconstants.HeaderUserID, identity.UserID)
	}
	return r.WithContext(WithIdentity(r.Context(), identity))
}

func reject(w http.ResponseWriter, d denial) {
	response.SendRefusalResponse(w, &response.RefusalResponse{
		GenericResponse: *response.NewGenericResponse(d.status, operationFor(d.status), nil, d.message),
		Code:            d.code,
	})
}

func operationFor(status int) response.OperationStatus {
	switch status {
	case http.StatusUnauthorized:
		return response.OperationUnauthorized
	case http.StatusForbidden:
		return response.OperationForbidden
	case http.StatusServiceUnavailable:
		return response.OperationUnavailable
	default:
		return response.OperationError
	}
}
