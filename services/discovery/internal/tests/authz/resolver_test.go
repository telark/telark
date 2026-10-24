package authz

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/base"
	clustereps "github.com/telark/telark/internal/rest/endpoints/cluster"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const loaderCallsUncached = 4

type countingResolver struct {
	calls int
	err   error
}

func (c *countingResolver) UserIDForToken(token string) (string, error) {
	c.calls++
	return token, c.err
}

func (c *countingResolver) GrantsForUser(string) (xauthz.Grants, error) {
	c.calls++
	return xauthz.Grants{}, c.err
}

// A repeat lookup inside the TTL is served from memory; the loader runs once
// per distinct token and once per distinct user.
func TestResolverCachesWithinTTL(t *testing.T) {
	inner := &countingResolver{}
	r := authz.NewCachedResolver(inner)

	for range constants.TwoValue {
		if _, err := r.UserIDForToken("tok"); err != nil {
			t.Fatal(err)
		}
		if _, err := r.GrantsForUser("u1"); err != nil {
			t.Fatal(err)
		}
	}

	testutil.Equal(t, "loader calls", inner.calls, constants.TwoValue)
}

// A failure is handed back as-is, so the middleware can still tell a verdict
// from an outage, and it is never pinned in the cache.
func TestResolverPassesErrorsThroughUncached(t *testing.T) {
	inner := &countingResolver{err: fmt.Errorf("%w: gone", xauthz.ErrNotFound)}
	r := authz.NewCachedResolver(inner)

	for range constants.TwoValue {
		if _, err := r.UserIDForToken("tok"); !errors.Is(err, xauthz.ErrNotFound) {
			t.Fatalf("UserIDForToken error = %v, want ErrNotFound", err)
		}
		if _, err := r.GrantsForUser("u1"); !errors.Is(err, xauthz.ErrNotFound) {
			t.Fatalf("GrantsForUser error = %v, want ErrNotFound", err)
		}
	}

	testutil.Equal(t, "loader calls", inner.calls, loaderCallsUncached)
}

const (
	liveToken  = "live-session-token"
	liveUserID = "u-1"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A session's name, which its owner can read, is no credential: presented as X-Session-Token it is not found, which
// x-ware answers with 401. The exporter knows a session only by the name the token hashes to.
func TestSessionNameIsNoCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(dataconstants.HeaderSessionToken) != authdata.SessionName(liveToken) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": http.StatusOK, "data": map[string]any{
			"userId": liveUserID, "expiresTimestamp": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}})
	}))
	t.Cleanup(srv.Close)
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		req := r.Clone(r.Context())
		req.URL.Scheme, req.URL.Host = "http", srv.Listener.Addr().String()
		return prev.RoundTrip(req)
	})
	t.Cleanup(func() { http.DefaultTransport = prev })

	resolver := authz.NewResolver()
	if user, err := resolver.UserIDForToken(liveToken); err != nil || user != liveUserID {
		t.Fatalf("token = (%q, %v), want %s", user, err, liveUserID)
	}
	if _, err := resolver.UserIDForToken(authdata.SessionName(liveToken)); !errors.Is(err, xauthz.ErrNotFound) {
		t.Fatalf("the session's name = %v, want not found", err)
	}
}

// Two session cookies count as no session: a Public route still runs, and one that needs a session answers 401 with
// the code the dashboard explains.
func TestDuplicateSessionCookies(t *testing.T) {
	middleware, err := xauthz.New(xauthz.Config{Resolver: &countingResolver{}, Requirements: authz.Requirements(),
		RouteKey: router.KeyFromRequest, ServiceToken: "service-token"})
	if err != nil {
		t.Fatal(err)
	}
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	rt := router.NewRouter([]router.Route{
		router.CreateRoute(base.Get, constants.StatusLivenessEp, ok),
		router.CreateRoute(base.Get, clustereps.GetAllNamespaces, ok),
	})
	rt.Use(middleware)
	for endpoint, want := range map[base.Endpoint]int{
		constants.StatusLivenessEp: http.StatusOK, clustereps.GetAllNamespaces: http.StatusUnauthorized,
	} {
		r := httptest.NewRequest(http.MethodGet, router.Pattern(endpoint), nil)
		for _, value := range []string{"real", "planted"} {
			r.AddCookie(&http.Cookie{Name: dataconstants.CookieSession, Value: value, Secure: true, HttpOnly: true,
				SameSite: http.SameSiteStrictMode})
		}
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, r)
		if rec.Code != want || (want == http.StatusUnauthorized) != strings.Contains(rec.Body.String(), "session_cookie_duplicate") {
			t.Fatalf("%s = %d %s, want %d", endpoint, rec.Code, rec.Body.String(), want)
		}
	}
}
