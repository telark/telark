package authz

import (
	"crypto/subtle"
	"errors"
	"net/http"

	dataconstants "github.com/telark/data/constants"
	dataerrors "github.com/telark/data/errors"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

type middleware struct {
	config Config
}

type denial struct {
	status  int
	message string
}

func New(config Config) (func(http.Handler) http.Handler, error) {
	if err := validate(config); err != nil {
		return nil, err
	}

	m := &middleware{config: config}
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

		requirement, found := m.config.Requirements[m.config.RouteKey(r)]
		if !found {
			reject(w, denial{http.StatusForbidden, string(dataerrors.ErrAuthzForbidden)})
			return
		}

		if requirement.Access == AccessPublic {
			next.ServeHTTP(w, r)
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
			return Identity{}, &denial{http.StatusUnauthorized, string(dataerrors.ErrAuthzInvalidServiceToken)}
		}
		return Identity{UserID: claimedUserID, Internal: true}, nil
	}

	if req.Access == AccessInternal {
		return Identity{}, &denial{http.StatusUnauthorized, string(dataerrors.ErrAuthzInvalidServiceToken)}
	}

	return m.identifyUser(r, req)
}

func (m *middleware) identifyUser(r *http.Request, req Requirement) (Identity, *denial) {
	sessionToken := r.Header.Get(dataconstants.HeaderSessionToken)
	if sessionToken == dataconstants.EmptyString {
		return Identity{}, &denial{http.StatusUnauthorized, string(dataerrors.ErrAuthzMissingSessionToken)}
	}

	userID, err := m.config.Resolver.UserIDForToken(sessionToken)
	if err != nil {
		return Identity{}, &denial{http.StatusUnauthorized, string(dataerrors.ErrAuthzInvalidSession)}
	}

	grants, err := m.config.Resolver.GrantsForUser(userID)
	if err != nil {
		return Identity{}, &denial{http.StatusInternalServerError, string(dataerrors.ErrAuthzGrantsUnavailable)}
	}

	identity := Identity{UserID: userID, Grants: grants}
	if req.Access == AccessAuthenticated {
		return identity, nil
	}

	if !Allows(identity, req) {
		return Identity{}, &denial{http.StatusForbidden, string(dataerrors.ErrAuthzForbidden)}
	}

	return identity, nil
}

func (m *middleware) serviceTokenMatches(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(m.config.ServiceToken)) == constantTimeEq
}

func stripSpoofableHeaders(r *http.Request) {
	for _, header := range spoofableHeaders {
		r.Header.Del(header)
	}
}

func applyIdentity(r *http.Request, identity Identity) *http.Request {
	if identity.UserID != dataconstants.EmptyString {
		r.Header.Set(dataconstants.HeaderUserID, identity.UserID)
	}
	return r.WithContext(WithIdentity(r.Context(), identity))
}

func reject(w http.ResponseWriter, d denial) {
	responseutils.SendResponse(w, d.status, operationFor(d.status), d.message, nil)
}

func operationFor(status int) response.OperationStatus {
	switch status {
	case http.StatusUnauthorized:
		return response.OperationUnauthorized
	case http.StatusForbidden:
		return response.OperationForbidden
	default:
		return response.OperationError
	}
}
