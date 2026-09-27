package authz

import (
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dataconstants "github.com/telark/data/constants"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/base"
	authendpoints "github.com/telark/rest/endpoints/auth"
	notificationsendpoints "github.com/telark/rest/endpoints/notifications"
	"github.com/telark/rest/router"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	testServiceToken = "service-token-for-tests"

	subjectUserID = "u-10000-0000-0001"

	sessionOfSubject  = "session-subject"
	sessionOfOutsider = "session-outsider"
	sessionOfReader   = "session-roles-reader"

	outsiderUserID = "u-10000-0000-0002"
	readerUserID   = "u-10000-0000-0003"
)

// Only the roles reader carries a scope; the subject of the lookup carries
// none, which is what separates an administrative read from a self read.
var sessionUsers = map[string]string{
	sessionOfSubject:  subjectUserID,
	sessionOfOutsider: outsiderUserID,
	sessionOfReader:   readerUserID,
}

var userGrants = map[string]xauthz.Grants{
	readerUserID: {
		Levels: map[string]roledata.PermissionLevel{
			roledata.ScopeRoles: roledata.PermissionLevelReadOnly,
		},
	},
}

type stubResolver struct{}

func (stubResolver) UserIDForToken(token string) (string, error) {
	userID, ok := sessionUsers[token]
	if !ok {
		return constants.EmptyString, errors.New("unknown session")
	}
	return userID, nil
}

func (stubResolver) GrantsForUser(userID string) (xauthz.Grants, error) {
	return userGrants[userID], nil
}

// Echoes the identity the middleware settled on, so a test can assert both the
// verdict and who the handler would have acted for.
func sentinel(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(html.EscapeString(r.Header.Get(dataconstants.HeaderUserID)))); err != nil {
		panic(err)
	}
}

// The real requirement table over stand-in handlers: the verdict under test is
// the middleware's, not whatever the production handler would go on to do.
func guardedRouter(endpoints ...base.Endpoint) (http.Handler, error) {
	routes := make([]router.Route, constants.DefaultInitValue, len(endpoints))
	for _, endpoint := range endpoints {
		routes = append(routes, router.CreateRoute(base.Get, endpoint, sentinel))
	}

	middleware, err := xauthz.New(xauthz.Config{
		Resolver:     stubResolver{},
		Requirements: authz.Requirements(),
		RouteKey:     router.KeyFromRequest,
		ServiceToken: testServiceToken,
	})
	if err != nil {
		return nil, err
	}

	rt := router.NewRouter(routes)
	rt.Use(middleware)
	return rt, nil
}

func pathFor(endpoint base.Endpoint) string {
	return strings.NewReplacer(
		"{"+constants.UserIDParam+"}", subjectUserID,
	).Replace(router.Pattern(endpoint))
}

// This route serves one subject's records off a header. Reading the requirement
// table proves what is declared; only a request proves what a caller gets.
func TestPerSubjectListRoutesRefuseForeignCallers(t *testing.T) {
	subjectEndpoints := []base.Endpoint{
		authendpoints.GetAllInternalPasskeysByUser,
	}

	handler, err := guardedRouter(subjectEndpoints...)
	if err != nil {
		t.Fatalf("building guarded router: %v", err)
	}

	cases := []struct {
		name     string
		endpoint base.Endpoint
		session  string
		internal bool
		want     int
	}{
		{"passkeys: outsider", authendpoints.GetAllInternalPasskeysByUser, sessionOfOutsider, false, http.StatusUnauthorized},
		{"passkeys: the subject", authendpoints.GetAllInternalPasskeysByUser, sessionOfSubject, false, http.StatusUnauthorized},
		{"passkeys: roles reader", authendpoints.GetAllInternalPasskeysByUser, sessionOfReader, false, http.StatusUnauthorized},
		{"passkeys: internal", authendpoints.GetAllInternalPasskeysByUser, constants.EmptyString, true, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, pathFor(tc.endpoint), nil)
			if tc.internal {
				request.Header.Set(dataconstants.HeaderServiceToken, testServiceToken)
				request.Header.Set(dataconstants.HeaderUserID, subjectUserID)
			} else {
				request.Header.Set(dataconstants.HeaderSessionToken, tc.session)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %q)", recorder.Code, tc.want, recorder.Body.String())
			}
		})
	}
}

// The passkey handlers read their subject straight from X-User-ID, which is only
// safe because the middleware drops a caller-supplied one and rewrites it from
// the session. A shared-package bump that stopped stripping would leak silently.
func TestCallerSuppliedUserIDHeaderIsReplaced(t *testing.T) {
	handler, err := guardedRouter(notificationsendpoints.List)
	if err != nil {
		t.Fatalf("building guarded router: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, router.Pattern(notificationsendpoints.List), nil)
	request.Header.Set(dataconstants.HeaderSessionToken, sessionOfOutsider)
	request.Header.Set(dataconstants.HeaderUserID, subjectUserID)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != outsiderUserID {
		t.Fatalf("handler saw user %q, want %q", got, outsiderUserID)
	}
}
