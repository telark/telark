package authz

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	dataconstants "github.com/telark/data/constants"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/x-ware/authz"
)

const (
	testServiceToken = "svc-secret"
	testSessionToken = "st-valid"
	testUserID       = "u-00001-0000-0001"
	testRouteKey     = "user:patch"
	testPublicKey    = "status:liveness"
	testInternalKey  = "session:create"
	testAuthedKey    = "permissions:get"
	spoofedUserID    = "u-99999-9999-9999"
	msgStatusWant    = "status = %d, want %d"
)

var testRule = authz.RuleKey(roledata.ScopeUsers, "edituser")

type stubResolver struct {
	userID    string
	userErr   error
	grants    authz.Grants
	grantsErr error
}

func (s stubResolver) UserIDForToken(token string) (string, error) {
	if s.userErr != nil {
		return dataconstants.EmptyString, s.userErr
	}
	if token != testSessionToken {
		return dataconstants.EmptyString, authz.ErrNotFound
	}
	return s.userID, nil
}

func (s stubResolver) GrantsForUser(string) (authz.Grants, error) {
	if s.grantsErr != nil {
		return authz.Grants{}, s.grantsErr
	}
	return s.grants, nil
}

var errStub = &stubError{}

type stubError struct{}

func (*stubError) Error() string { return "stub" }

func levels(scope string, level roledata.PermissionLevel) map[string]roledata.PermissionLevel {
	return map[string]roledata.PermissionLevel{scope: level}
}

func newTestConfig(resolver authz.Resolver) authz.Config {
	return authz.Config{
		Resolver: resolver,
		Requirements: map[string]authz.Requirement{
			testRouteKey: {
				Scope:    roledata.ScopeUsers,
				MinLevel: roledata.PermissionLevelContributor,
				Rule:     testRule,
			},
			testPublicKey:   {Access: authz.AccessPublic},
			testInternalKey: {Access: authz.AccessInternal},
			testAuthedKey:   {Access: authz.AccessAuthenticated},
		},
		RouteKey:     func(r *http.Request) string { return r.Header.Get("X-Test-Route") },
		ServiceToken: testServiceToken,
	}
}

// captured records what the wrapped handler actually saw, so tests can assert
// on the identity the middleware forwarded rather than only on status codes.
type captured struct {
	called   bool
	userID   string
	identity authz.Identity
}

func serve(t *testing.T, config authz.Config, r *http.Request) (*httptest.ResponseRecorder, *captured) {
	t.Helper()

	got := &captured{}
	mw, err := authz.New(config)
	if err != nil {
		t.Fatalf("authz.New() error = %v", err)
	}

	next := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		got.called = true
		got.userID = req.Header.Get(dataconstants.HeaderUserID)
		got.identity, _ = authz.FromContext(req.Context())
	})

	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, r)
	return rec, got
}

func request(routeKey string) *http.Request {
	r := httptest.NewRequest(http.MethodPatch, "/v1/users/u-1", nil)
	r.Header.Set("X-Test-Route", routeKey)
	return r
}

func TestNewRejectsIncompleteConfig(t *testing.T) {
	base := newTestConfig(stubResolver{})

	tests := map[string]func(*authz.Config){
		"nil resolver":     func(c *authz.Config) { c.Resolver = nil },
		"nil requirements": func(c *authz.Config) { c.Requirements = nil },
		"nil route key":    func(c *authz.Config) { c.RouteKey = nil },
		"empty token":      func(c *authz.Config) { c.ServiceToken = dataconstants.EmptyString },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := base
			mutate(&config)
			if _, err := authz.New(config); err == nil {
				t.Fatal("expected error for incomplete config, got nil")
			}
		})
	}
}

func TestUnmappedRouteIsDenied(t *testing.T) {
	config := newTestConfig(stubResolver{})
	r := request("route:never-registered")
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, got := serve(t, config, r)

	if rec.Code != http.StatusForbidden {
		t.Errorf(msgStatusWant, rec.Code, http.StatusForbidden)
	}
	if got.called {
		t.Error("handler ran for an unmapped route; default-deny is broken")
	}
}

func TestMissingSessionTokenIsUnauthorized(t *testing.T) {
	rec, got := serve(t, newTestConfig(stubResolver{}), request(testRouteKey))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf(msgStatusWant, rec.Code, http.StatusUnauthorized)
	}
	if got.called {
		t.Error("handler ran without a session token")
	}
}

func TestInvalidSessionTokenIsUnauthorized(t *testing.T) {
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderSessionToken, "st-bogus")

	rec, got := serve(t, newTestConfig(stubResolver{userID: testUserID}), r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf(msgStatusWant, rec.Code, http.StatusUnauthorized)
	}
	if got.called {
		t.Error("handler ran with an invalid session token")
	}
}

// Only a backend that answered may deny; one that could not answer is reported
// as unavailable so the client retries instead of logging the user out.
func TestResolverErrorsAreVerdictsOrOutages(t *testing.T) {
	tests := map[string]struct {
		resolver stubResolver
		want     int
	}{
		"session not found":       {stubResolver{userErr: authz.ErrNotFound}, http.StatusUnauthorized},
		"session expired":         {stubResolver{userErr: authz.ErrSessionExpired}, http.StatusUnauthorized},
		"session backend down":    {stubResolver{userErr: errStub}, http.StatusServiceUnavailable},
		"user not found":          {stubResolver{userID: testUserID, grantsErr: authz.ErrNotFound}, http.StatusForbidden},
		"user not active":         {stubResolver{userID: testUserID, grantsErr: authz.ErrUserNotActive}, http.StatusForbidden},
		"user not found, wrapped": {stubResolver{userID: testUserID, grantsErr: fmt.Errorf("%w: u-1", authz.ErrNotFound)}, http.StatusForbidden},
		"grants backend down":     {stubResolver{userID: testUserID, grantsErr: errStub}, http.StatusServiceUnavailable},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := request(testRouteKey)
			r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

			rec, got := serve(t, newTestConfig(tc.resolver), r)

			if rec.Code != tc.want {
				t.Errorf(msgStatusWant, rec.Code, tc.want)
			}
			if got.called {
				t.Error("handler ran despite a resolver failure")
			}
		})
	}
}

func TestInsufficientLevelIsForbidden(t *testing.T) {
	resolver := stubResolver{
		userID: testUserID,
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelReadOnly)},
	}
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, got := serve(t, newTestConfig(resolver), r)

	if rec.Code != http.StatusForbidden {
		t.Errorf(msgStatusWant, rec.Code, http.StatusForbidden)
	}
	if got.called {
		t.Error("ReadOnly identity reached a Contributor route")
	}
}

func TestSufficientLevelIsAllowed(t *testing.T) {
	resolver := stubResolver{
		userID: testUserID,
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)},
	}
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, got := serve(t, newTestConfig(resolver), r)

	if rec.Code != http.StatusOK {
		t.Errorf(msgStatusWant, rec.Code, http.StatusOK)
	}
	if !got.called {
		t.Fatal("handler did not run for an authorized request")
	}
	if got.identity.UserID != testUserID {
		t.Errorf("identity user = %q, want %q", got.identity.UserID, testUserID)
	}
}

func TestWildcardScopeGrantsAccess(t *testing.T) {
	resolver := stubResolver{
		userID: testUserID,
		grants: authz.Grants{Levels: levels(roledata.ScopeAll, roledata.PermissionLevelAdmin)},
	}
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, _ := serve(t, newTestConfig(resolver), r)

	if rec.Code != http.StatusOK {
		t.Errorf("admin via ScopeAll: status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDenyRuleBeatsGrantedLevel(t *testing.T) {
	resolver := stubResolver{
		userID: testUserID,
		grants: authz.Grants{
			Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelOwner),
			Denied: map[string][]string{roledata.ScopeUsers: {testRule}},
		},
	}
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, got := serve(t, newTestConfig(resolver), r)

	if rec.Code != http.StatusForbidden {
		t.Errorf(msgStatusWant, rec.Code, http.StatusForbidden)
	}
	if got.called {
		t.Error("deny rule did not override the granted level")
	}
}

// A rule withholds one action, not the scope. Denying another action must
// leave this route reachable.
func TestUnrelatedDenyRuleDoesNotBlockScope(t *testing.T) {
	resolver := stubResolver{
		userID: testUserID,
		grants: authz.Grants{
			Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelOwner),
			Denied: map[string][]string{
				roledata.ScopeUsers: {authz.RuleKey(roledata.ScopeUsers, "deleteuser")},
			},
		},
	}
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, got := serve(t, newTestConfig(resolver), r)

	if rec.Code != http.StatusOK {
		t.Errorf(msgStatusWant, rec.Code, http.StatusOK)
	}
	if !got.called {
		t.Error("a deny rule for a different action blocked this one")
	}
}

func TestRuleKeyMatchesDashboardFormat(t *testing.T) {
	if got, want := authz.RuleKey("applications", "deleteapplication"), "applications.deleteapplication.deny"; got != want {
		t.Errorf("RuleKey() = %q, want %q", got, want)
	}
	if got, want := authz.RuleKey("Applications", "DeleteApplication"), "applications.deleteapplication.deny"; got != want {
		t.Errorf("RuleKey() = %q, want %q", got, want)
	}
}

// The spoof guard is the whole reason handlers may trust X-User-ID.
func TestClientSuppliedUserIDIsOverwritten(t *testing.T) {
	resolver := stubResolver{
		userID: testUserID,
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)},
	}
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)
	r.Header.Set(dataconstants.HeaderUserID, spoofedUserID)
	r.Header.Set(dataconstants.HeaderUsername, "attacker")
	r.Header.Set(dataconstants.HeaderEmail, "attacker@example.com")

	_, got := serve(t, newTestConfig(resolver), r)

	if got.userID != testUserID {
		t.Errorf("forwarded user = %q, want %q (spoofed header honored)", got.userID, testUserID)
	}
}

func TestSpoofedHeadersStrippedOnDeniedRequest(t *testing.T) {
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderUserID, spoofedUserID)

	rec, got := serve(t, newTestConfig(stubResolver{}), r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf(msgStatusWant, rec.Code, http.StatusUnauthorized)
	}
	if got.called {
		t.Error("spoofed X-User-ID alone reached the handler")
	}
}

func TestPublicRouteSkipsAuthentication(t *testing.T) {
	rec, got := serve(t, newTestConfig(stubResolver{}), request(testPublicKey))

	if rec.Code != http.StatusOK {
		t.Errorf(msgStatusWant, rec.Code, http.StatusOK)
	}
	if !got.called {
		t.Error("public route did not reach the handler; probes would fail")
	}
}

func TestPublicRouteStillStripsSpoofedHeaders(t *testing.T) {
	r := request(testPublicKey)
	r.Header.Set(dataconstants.HeaderUserID, spoofedUserID)

	_, got := serve(t, newTestConfig(stubResolver{}), r)

	if got.userID != dataconstants.EmptyString {
		t.Errorf("public route forwarded spoofed user %q", got.userID)
	}
}

func TestInternalRouteRequiresServiceToken(t *testing.T) {
	r := request(testInternalKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, got := serve(t, newTestConfig(stubResolver{userID: testUserID}), r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf(msgStatusWant, rec.Code, http.StatusUnauthorized)
	}
	if got.called {
		t.Error("a user session reached an internal-only route")
	}
}

func TestValidServiceTokenAllowsInternalRoute(t *testing.T) {
	r := request(testInternalKey)
	r.Header.Set(dataconstants.HeaderServiceToken, testServiceToken)

	rec, got := serve(t, newTestConfig(stubResolver{}), r)

	if rec.Code != http.StatusOK {
		t.Errorf(msgStatusWant, rec.Code, http.StatusOK)
	}
	if !got.identity.Internal {
		t.Error("identity not marked internal")
	}
}

func TestWrongServiceTokenIsUnauthorized(t *testing.T) {
	r := request(testInternalKey)
	r.Header.Set(dataconstants.HeaderServiceToken, "svc-wrong")

	rec, got := serve(t, newTestConfig(stubResolver{}), r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf(msgStatusWant, rec.Code, http.StatusUnauthorized)
	}
	if got.called {
		t.Error("handler ran with a wrong service token")
	}
}

// A peer service acts on a user's behalf, so its claimed X-User-ID is honored
// only once the service token proves the caller is a peer.
func TestInternalCallerKeepsClaimedUserID(t *testing.T) {
	r := request(testInternalKey)
	r.Header.Set(dataconstants.HeaderServiceToken, testServiceToken)
	r.Header.Set(dataconstants.HeaderUserID, testUserID)

	_, got := serve(t, newTestConfig(stubResolver{}), r)

	if got.userID != testUserID {
		t.Errorf("forwarded user = %q, want %q", got.userID, testUserID)
	}
}

func TestServiceTokenBypassesScopeCheck(t *testing.T) {
	r := request(testRouteKey)
	r.Header.Set(dataconstants.HeaderServiceToken, testServiceToken)

	rec, got := serve(t, newTestConfig(stubResolver{}), r)

	if rec.Code != http.StatusOK {
		t.Errorf(msgStatusWant, rec.Code, http.StatusOK)
	}
	if !got.identity.Internal {
		t.Error("identity not marked internal on a scoped route")
	}
}

// A user with no scopes at all must still reach their own permissions and
// notifications, or a custom role that omits the users scope locks them out.
func TestAuthenticatedRouteNeedsNoScope(t *testing.T) {
	resolver := stubResolver{userID: testUserID, grants: authz.Grants{}}
	r := request(testAuthedKey)
	r.Header.Set(dataconstants.HeaderSessionToken, testSessionToken)

	rec, got := serve(t, newTestConfig(resolver), r)

	if rec.Code != http.StatusOK {
		t.Errorf(msgStatusWant, rec.Code, http.StatusOK)
	}
	if got.identity.UserID != testUserID {
		t.Errorf("identity user = %q, want %q", got.identity.UserID, testUserID)
	}
}

func TestAuthenticatedRouteStillNeedsSession(t *testing.T) {
	rec, got := serve(t, newTestConfig(stubResolver{}), request(testAuthedKey))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf(msgStatusWant, rec.Code, http.StatusUnauthorized)
	}
	if got.called {
		t.Error("authenticated route ran without a session")
	}
}

var allowsTests = []struct {
	name   string
	grants authz.Grants
	req    authz.Requirement
	want   bool
}{
	{
		name:   "no grants at all",
		grants: authz.Grants{},
		req:    authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelReadOnly},
		want:   false,
	},
	{
		name:   "unrelated scope only",
		grants: authz.Grants{Levels: levels(roledata.ScopeRoles, roledata.PermissionLevelAdmin)},
		req:    authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelReadOnly},
		want:   false,
	},
	{
		name:   "exact level matches",
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelContributor)},
		req:    authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelContributor},
		want:   true,
	},
	{
		name:   "higher level covers lower",
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelAdmin)},
		req:    authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelReadOnly},
		want:   true,
	},
	{
		name:   "lower level does not cover higher",
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelContributor)},
		req:    authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelOwner},
		want:   false,
	},
	{
		name: "exact scope wins over wildcard",
		grants: authz.Grants{Levels: map[string]roledata.PermissionLevel{
			roledata.ScopeAll:   roledata.PermissionLevelAdmin,
			roledata.ScopeUsers: roledata.PermissionLevelReadOnly,
		}},
		req:  authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelOwner},
		want: false,
	},
	{
		name:   "unknown granted level never allows",
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevel("Superuser"))},
		req:    authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelReadOnly},
		want:   false,
	},
	{
		name:   "unknown required level never allows",
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelAdmin)},
		req:    authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevel("Bogus")},
		want:   false,
	},
	{
		name:   "empty required level never allows",
		grants: authz.Grants{Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelAdmin)},
		req:    authz.Requirement{Scope: roledata.ScopeUsers},
		want:   false,
	},
	{
		name: "deny rule on the wildcard scope blocks the action",
		grants: authz.Grants{
			Levels: levels(roledata.ScopeAll, roledata.PermissionLevelAdmin),
			Denied: map[string][]string{roledata.ScopeAll: {testRule}},
		},
		req: authz.Requirement{
			Scope:    roledata.ScopeUsers,
			MinLevel: roledata.PermissionLevelReadOnly,
			Rule:     testRule,
		},
		want: false,
	},
	{
		name: "deny rule is ignored when the route declares none",
		grants: authz.Grants{
			Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelOwner),
			Denied: map[string][]string{roledata.ScopeUsers: {testRule}},
		},
		req:  authz.Requirement{Scope: roledata.ScopeUsers, MinLevel: roledata.PermissionLevelReadOnly},
		want: true,
	},
	{
		name: "deny rule for another scope does not apply",
		grants: authz.Grants{
			Levels: levels(roledata.ScopeUsers, roledata.PermissionLevelOwner),
			Denied: map[string][]string{roledata.ScopeRoles: {testRule}},
		},
		req: authz.Requirement{
			Scope:    roledata.ScopeUsers,
			MinLevel: roledata.PermissionLevelReadOnly,
			Rule:     testRule,
		},
		want: true,
	},
}

func TestAllows(t *testing.T) {
	for _, tt := range allowsTests {
		t.Run(tt.name, func(t *testing.T) {
			got := authz.Allows(authz.Identity{Grants: tt.grants}, tt.req)
			if got != tt.want {
				t.Errorf("authz.Allows() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeLevelKeepsStrongest(t *testing.T) {
	tests := []struct {
		name       string
		start      roledata.PermissionLevel
		candidate  roledata.PermissionLevel
		want       roledata.PermissionLevel
		startEmpty bool
	}{
		{
			name:       "first grant wins",
			candidate:  roledata.PermissionLevelReadOnly,
			want:       roledata.PermissionLevelReadOnly,
			startEmpty: true,
		},
		{
			name:      "stronger replaces",
			start:     roledata.PermissionLevelReadOnly,
			candidate: roledata.PermissionLevelOwner,
			want:      roledata.PermissionLevelOwner,
		},
		{
			name:      "weaker ignored",
			start:     roledata.PermissionLevelOwner,
			candidate: roledata.PermissionLevelReadOnly,
			want:      roledata.PermissionLevelOwner,
		},
		{
			name:      "unknown ignored",
			start:     roledata.PermissionLevelReadOnly,
			candidate: roledata.PermissionLevel("Nope"),
			want:      roledata.PermissionLevelReadOnly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := map[string]roledata.PermissionLevel{}
			if !tt.startEmpty {
				got[roledata.ScopeUsers] = tt.start
			}

			authz.MergeLevel(got, roledata.ScopeUsers, tt.candidate)

			if got[roledata.ScopeUsers] != tt.want {
				t.Errorf("level = %q, want %q", got[roledata.ScopeUsers], tt.want)
			}
		})
	}
}

func TestMergeLevelSkipsUnknownOnEmptyMap(t *testing.T) {
	got := map[string]roledata.PermissionLevel{}

	authz.MergeLevel(got, roledata.ScopeUsers, roledata.PermissionLevel("Bogus"))

	if _, exists := got[roledata.ScopeUsers]; exists {
		t.Error("unknown level created a grant")
	}
}
