package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/telark/telark/internal/rest/base"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/auth/internal/constants"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/ratelimit"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	clientA      = "203.0.113.7"
	clientB      = "203.0.113.8"
	clientC      = "203.0.113.9"
	forgedEntry  = "198.51.100.66"
	nginxPeer    = "10.0.0.5"
	remoteAddr   = "192.0.2.1:41000"
	remoteClient = "192.0.2.1"
	entrySep     = ", "
	noHops       = "0"
	oneHop       = "1"
	oneAttempt   = "1"
	twoStarts    = "2"
	perClient    = 2
	repeats      = 3
	idParam      = "{id}"
	redisLoading = "LOADING Redis is loading the dataset in memory"
	counterField = "f"
	// What a unix-socket listener reports as the peer.
	unparsablePeer = "@"
)

var (
	redisServer *miniredis.Miniredis
	login       = router.Pattern(autheps.StartLogin)
	loginKey    = constants.RedisKeyPrefixRateLimit + router.Key(base.Post, autheps.StartLogin) + constants.ColonSeparator
)

// The middleware reads the cached Redis client, so the package shares one server and each test flushes it.
func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	redisServer = mr
	if err := os.Setenv("REDIS_HOST", mr.Host()); err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_PORT", mr.Port()); err != nil {
		panic(err)
	}
	redishelper.NewRedisClientWithRetry(context.Background())
	m.Run()
	mr.Close()
}

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// The starts and a finish behind the limit, the way main registers it on auth's router.
func limitedRouter(t *testing.T, env map[string]string) http.Handler {
	t.Helper()
	for key, value := range env {
		t.Setenv(key, value)
	}
	redisServer.FlushAll()
	rt := router.NewRouter([]router.Route{
		router.CreateRoute(base.Post, autheps.StartLogin, ok),
		router.CreateRoute(base.Post, autheps.StartRegister, ok),
		router.CreateRoute(base.Post, bootstrapeps.StartAssertion, ok),
		router.CreateRoute(base.Post, autheps.OIDCGoogleNonce, ok),
		router.CreateRoute(base.Post, autheps.FinishLogin, ok),
	})
	rt.Use(ratelimit.New())
	return rt
}

func start(handler http.Handler, path string, forwarded ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, nil)
	r.RemoteAddr = remoteAddr
	for _, value := range forwarded {
		r.Header.Add(constants.HeaderForwardedFor, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)
	return rec
}

// A client gets RATE_LIMIT_PER_CLIENT starts a minute on each start route, then a 429 that says when to come back.
// Other clients and other starts go on, and the next window starts afresh.
func TestPerClientLimit(t *testing.T) {
	handler := limitedRouter(t, map[string]string{constants.EnvRateLimitPerClient: strconv.Itoa(perClient)})
	for range perClient {
		testutil.Equal(t, "start within the limit", start(handler, login, clientA).Code, http.StatusOK)
	}
	refused := start(handler, login, clientA)
	testutil.Equal(t, "start over the limit", refused.Code, http.StatusTooManyRequests)
	testutil.Equal(t, "Retry-After", refused.Header().Get(constants.HeaderRetryAfter), strconv.Itoa(constants.RateLimitWindowSec))
	testutil.Equal(t, "says why", strings.Contains(refused.Body.String(), string(constants.ErrTooManyAttempts)), true)
	testutil.Equal(t, "another client", start(handler, login, clientB).Code, http.StatusOK)
	testutil.Equal(t, "another start", start(handler, router.Pattern(autheps.StartRegister), clientA).Code, http.StatusOK)

	redisServer.FastForward(time.Duration(constants.RateLimitWindowSec) * time.Second)
	testutil.Equal(t, "next window", start(handler, login, clientA).Code, http.StatusOK)
}

// The approval start is one route whatever the request id, so request ids don't add attempts.
func TestApprovalStartIsLimitedPerRoute(t *testing.T) {
	handler := limitedRouter(t, map[string]string{constants.EnvRateLimitPerClient: oneAttempt})
	approval := router.Pattern(bootstrapeps.StartAssertion)
	testutil.Equal(t, "first request", start(handler, strings.Replace(approval, idParam, "req-1", 1), clientA).Code,
		http.StatusOK)
	testutil.Equal(t, "second request", start(handler, strings.Replace(approval, idParam, "req-2", 1), clientA).Code,
		http.StatusTooManyRequests)
}

// RATE_LIMIT_GLOBAL caps a start over all clients: past it every client waits for the next window, with no counter
// of its own written, and a client refused by its own limit spends none of the cap.
func TestRouteCapAcrossClients(t *testing.T) {
	handler := limitedRouter(t, map[string]string{
		constants.EnvRateLimitPerClient: oneAttempt,
		constants.EnvRateLimitGlobal:    twoStarts,
	})
	testutil.Equal(t, "first client", start(handler, login, clientA).Code, http.StatusOK)
	testutil.Equal(t, "first client again", start(handler, login, clientA).Code, http.StatusTooManyRequests)
	testutil.Equal(t, "second client", start(handler, login, clientB).Code, http.StatusOK)
	testutil.Equal(t, "third client", start(handler, login, clientC).Code, http.StatusTooManyRequests)
	testutil.Equal(t, "third client's counter", redisServer.Exists(loginKey+clientC), false)
	testutil.Equal(t, "another start", start(handler, router.Pattern(autheps.StartRegister), clientC).Code, http.StatusOK)
}

// The client is the X-Forwarded-For entry RATE_LIMIT_TRUSTED_HOPS from the right, the peer when there is none, and
// its /64 for IPv6.
func TestClientKey(t *testing.T) {
	cases := []struct {
		name      string
		hops      string
		forwarded []string
		client    string
	}{
		{"the ui's peer", noHops, []string{clientA + entrySep + nginxPeer}, nginxPeer},
		{"behind one proxy", oneHop, []string{forgedEntry + entrySep + clientA + entrySep + nginxPeer}, clientA},
		{"fewer entries than hops", oneHop, []string{nginxPeer}, nginxPeer},
		{"two headers", oneHop, []string{clientA, nginxPeer}, clientA},
		{"no header", noHops, nil, remoteClient},
		{"unparsable", noHops, []string{"unknown"}, remoteClient},
		{"IPv6", noHops, []string{"2001:db8:1:2:aaaa::1"}, "2001:db8:1:2::/64"},
		{"IPv4 in IPv6", noHops, []string{"::ffff:" + clientA}, clientA},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handler := limitedRouter(t, map[string]string{constants.EnvRateLimitTrustedHops: c.hops})
			testutil.Equal(t, "status", start(handler, login, c.forwarded...).Code, http.StatusOK)
			testutil.Equal(t, "counter of "+c.client, redisServer.Exists(loginKey+c.client), true)
		})
	}
}

// Entries left of the trusted ones are the client's own: changing them doesn't make it a new client.
func TestForgedEntriesShareTheClientLimit(t *testing.T) {
	handler := limitedRouter(t, map[string]string{
		constants.EnvRateLimitPerClient:   oneAttempt,
		constants.EnvRateLimitTrustedHops: oneHop,
	})
	testutil.Equal(t, "first start", start(handler, login, forgedEntry+entrySep+clientA+entrySep+nginxPeer).Code,
		http.StatusOK)
	testutil.Equal(t, "forged entry changed", start(handler, login, clientB+entrySep+clientA+entrySep+nginxPeer).Code,
		http.StatusTooManyRequests)
}

// The finishes and every other route pass untouched and write no counter.
func TestOtherRoutesAreNotLimited(t *testing.T) {
	handler := limitedRouter(t, map[string]string{constants.EnvRateLimitPerClient: oneAttempt})
	for range repeats {
		testutil.Equal(t, "finish", start(handler, router.Pattern(autheps.FinishLogin), clientA).Code, http.StatusOK)
	}
	testutil.Equal(t, "counters", len(redisServer.Keys()), constants.DefaultInitValue)
}

// A failing Redis, or a client counter it can't count, lets the starts through: each then fails at its own write,
// which reports the outage as one.
func TestFailsOpen(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"Redis down": func(t *testing.T) {
			redisServer.SetError(redisLoading)
			t.Cleanup(func() { redisServer.SetError(constants.EmptyString) })
		},
		"client counter of another type": func(*testing.T) { redisServer.HSet(loginKey+clientA, counterField, oneAttempt) },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			handler := limitedRouter(t, map[string]string{constants.EnvRateLimitPerClient: oneAttempt})
			breakIt(t)
			for range repeats {
				testutil.Equal(t, "start", start(handler, login, clientA).Code, http.StatusOK)
			}
		})
	}
}

// A peer address that doesn't parse, with no forwarded entry, is the client as it is.
func TestUnparsablePeerIsItsOwnClient(t *testing.T) {
	handler := limitedRouter(t, nil)
	r := httptest.NewRequest(http.MethodPost, login, nil)
	r.RemoteAddr = unparsablePeer
	handler.ServeHTTP(httptest.NewRecorder(), r)
	testutil.Equal(t, "counter", redisServer.Exists(loginKey+unparsablePeer), true)
}
