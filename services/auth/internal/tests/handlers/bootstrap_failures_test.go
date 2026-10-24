package handlers

import (
	"crypto/ecdsa"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/gorilla/mux"

	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/constants"
	authhandler "github.com/telark/telark/services/auth/internal/handlers/auth"
	bootstraphandler "github.com/telark/telark/services/auth/internal/handlers/bootstrap"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	memberEmail   = "test+member@example.com"
	statusSuffix  = "/" + string(bootstrapeps.GetBootstrap)
	sessionSuffix = "/sessions"
	redisDown     = "LOADING Redis is loading the dataset in memory"
	unparsedTime  = "yesterday"
	failedReply   = `{"status":500}`
)

var errBodyLost = errors.New("connection reset by peer")

// An exporter that fails one call: it answers 500 to each request with method whose path ends in suffix, once a
// request whose path ends in after went through (from the first request when after is empty).
type failingRoute struct {
	inner  http.Handler
	method string
	suffix string
	after  string
	armed  atomic.Bool
}

func (f *failingRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == f.method && strings.HasSuffix(r.URL.Path, f.suffix) && (f.after == constants.EmptyString || f.armed.Load()) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(failedReply))
		return
	}
	f.inner.ServeHTTP(w, r)
	if f.after != constants.EmptyString && strings.HasSuffix(r.URL.Path, f.after) {
		f.armed.Store(true)
	}
}

// Serves fake, behind fail when set.
func serveFailing(t *testing.T, fake *testutil.FakeExporter, fail *failingRoute) {
	t.Helper()
	if fail == nil {
		testutil.StubBackend(t, fake)
		return
	}
	fail.inner = fake
	testutil.StubBackend(t, fail)
}

func brokenRedis(t *testing.T) {
	t.Helper()
	redisServer.SetError(redisDown)
	t.Cleanup(func() { redisServer.SetError(constants.EmptyString) })
}

type approvalCase struct {
	name string
	prep func(t *testing.T, fake *testutil.FakeExporter)
	fail *failingRoute
	run  func(t *testing.T, fake *testutil.FakeExporter) int
	want int
}

// Each case runs against the pinned account priv signs for, with its request pending.
func runApprovalCases(t *testing.T, priv *ecdsa.PrivateKey, cases []approvalCase) {
	t.Helper()
	initWebAuthn(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending)
			if c.prep != nil {
				c.prep(t, fake)
			}
			serveFailing(t, fake, c.fail)
			testutil.Equal(t, "status", c.run(t, fake), c.want)
		})
	}
}

// A start auth couldn't run is an outage (503) or a failure (500), and a refused one the caller's answer (400, 409).
func TestApprovalStartAnswersWhatWentWrong(t *testing.T) {
	startPath := approvalPath(bootstrapeps.StartAssertion, pendingRequest)
	start := func(t *testing.T, _ *testutil.FakeExporter) int {
		rec, _ := startApproval(t, pendingRequest, approveAction)
		return rec.Code
	}
	post := func(r *http.Request) int {
		rec := httptest.NewRecorder()
		routed(rec, r)
		return rec.Code
	}
	runApprovalCases(t, testutil.NewP256Key(t), []approvalCase{
		{name: "status unreadable", run: func(t *testing.T, fake *testutil.FakeExporter) int {
			fake.SetDown(true)
			return start(t, fake)
		}, want: http.StatusServiceUnavailable},
		{name: "account not pinned", prep: func(_ *testing.T, fake *testutil.FakeExporter) {
			fake.Bootstrap.PinnedAt = constants.EmptyString
		}, run: start, want: http.StatusConflict},
		{name: "passkeys unreadable", run: func(t *testing.T, fake *testutil.FakeExporter) int {
			fake.FailNextPasskeyRead()
			return start(t, fake)
		}, want: http.StatusServiceUnavailable},
		{name: "Redis down", prep: func(t *testing.T, _ *testutil.FakeExporter) { brokenRedis(t) }, run: start,
			want: http.StatusInternalServerError},
		{name: "from another site", run: func(*testing.T, *testutil.FakeExporter) int {
			r := jsonPost(startPath, `{"action":"`+approveAction+`"}`)
			r.Header.Set(constants.HeaderOrigin, "http://"+otherRPID)
			return post(r)
		}, want: http.StatusBadRequest},
		{name: "malformed body", run: func(*testing.T, *testutil.FakeExporter) int {
			return post(jsonPost(startPath, malformedJSON))
		}, want: http.StatusBadRequest},
	})
}

// A finish auth couldn't run is an outage (503), a refused one the caller's answer (400, 401), and the exporter's
// verdict on a verified approval is relayed: its refusal as it is, its failure opaque.
func TestApprovalFinishAnswersWhatWentWrong(t *testing.T) {
	priv := testutil.NewP256Key(t)
	// Started while the exporter answers, finished once breakIt broke it.
	finishBroken := func(breakIt func(fake *testutil.FakeExporter)) func(*testing.T, *testutil.FakeExporter) int {
		return func(t *testing.T, fake *testutil.FakeExporter) int {
			_, challenge := startApproval(t, pendingRequest, approveAction)
			breakIt(fake)
			return finishApproval(t, pendingRequest, approvalBody(t, priv, flagVerified, challenge, approveAction))
		}
	}
	approve := func(t *testing.T, _ *testutil.FakeExporter) int {
		return decide(t, priv, flagVerified, approveAction, approveAction)
	}
	runApprovalCases(t, priv, []approvalCase{
		{name: "unknown action", run: func(t *testing.T, _ *testutil.FakeExporter) int {
			return finishApproval(t, pendingRequest, `{"action":"promote"}`)
		}, want: http.StatusBadRequest},
		{name: "body unreadable", run: func(*testing.T, *testutil.FakeExporter) int {
			r := mux.SetURLVars(httptest.NewRequest(http.MethodPost, testPath, iotest.ErrReader(errBodyLost)),
				map[string]string{constants.IDPathParam: pendingRequest})
			rec := httptest.NewRecorder()
			bootstraphandler.FinishAssertion(rec, r)
			return rec.Code
		}, want: http.StatusBadRequest},
		{name: "status unreadable", run: finishBroken(func(fake *testutil.FakeExporter) { fake.SetDown(true) }),
			want: http.StatusServiceUnavailable},
		{name: "passkeys unreadable", run: finishBroken((*testutil.FakeExporter).FailNextPasskeyRead),
			want: http.StatusServiceUnavailable},
		{name: "short challenge planted", run: func(t *testing.T, _ *testutil.FakeExporter) int {
			short := base64.RawURLEncoding.EncodeToString(make([]byte, constants.BootstrapChallengeRandomBytes))
			plant(t, constants.RedisKeyPrefixBootstrapAssert+pendingRequest+constants.ColonSeparator+testutil.ChallengeDigest(t, short),
				constants.RedisChallengeEntry)
			return finishApproval(t, pendingRequest, approvalBody(t, priv, flagVerified, short, approveAction))
		}, want: http.StatusUnauthorized},
		{name: "exporter refuses the approval", prep: func(_ *testing.T, fake *testutil.FakeExporter) {
			delete(fake.RequestAnswers, approveRoute)
		}, run: approve, want: http.StatusConflict},
		{name: "exporter fails the approval", fail: &failingRoute{method: http.MethodPost, suffix: "/" + approveRoute}, run: approve,
			want: http.StatusInternalServerError},
	})
}

// A ceremony names its request in the path: without one, neither step reads anything (400).
func TestApprovalNeedsTheRequestID(t *testing.T) {
	for name, handler := range map[string]handlerFunc{
		"start":  bootstraphandler.StartAssertion,
		"finish": bootstraphandler.FinishAssertion,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, jsonReq(`{"action":"`+approveAction+`"}`))
			testutil.Equal(t, "status", rec.Code, http.StatusBadRequest)
		})
	}
}

// The bootstrap account's sign-in reads the break-glass state before the session, where a state it can't read or a
// recovery it can't date refuses it (503, 500) with no session, and again after it, where nothing fails the sign-in.
// Any account's sign-in fails (500) when its session can't be saved.
func TestSignInWhenSomethingFails(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	priv := testutil.NewP256Key(t)
	member := func(_ *testing.T, fake *testutil.FakeExporter) {
		fake.Users[plainTargetID].Email = memberEmail
		key := fake.Credentials[bootstrapID][constants.DefaultInitValue]
		key.UserID = plainTargetID
		fake.Credentials[plainTargetID] = []authdata.Passkey{key}
	}
	recovery := func(opened string) func(*testing.T, *testutil.FakeExporter) {
		return func(_ *testing.T, fake *testutil.FakeExporter) {
			asRecovery(fake, time.Now().Add(-pastTheMark))
			if opened != constants.EmptyString {
				fake.Bootstrap.Requests[constants.DefaultInitValue].OpenedAt = opened
			}
		}
	}
	cases := []struct {
		name     string
		email    string
		prep     func(t *testing.T, fake *testutil.FakeExporter)
		fail     *failingRoute
		want     int
		sessions int
	}{
		{"an account without the bootstrap marker", memberEmail, member, nil, http.StatusOK, constants.DefaultIncrementValue},
		{"status unreadable", bootstrapEmail, nil, &failingRoute{method: http.MethodGet, suffix: statusSuffix},
			http.StatusServiceUnavailable, constants.DefaultInitValue},
		{"recovery opened at an unreadable time", bootstrapEmail, recovery(unparsedTime), nil,
			http.StatusInternalServerError, constants.DefaultInitValue},
		{"recovery, passkey without a creation time", bootstrapEmail, recovery(constants.EmptyString), nil,
			http.StatusOK, constants.DefaultIncrementValue},
		{"status unreadable after the session", bootstrapEmail, nil,
			&failingRoute{method: http.MethodGet, suffix: statusSuffix, after: sessionSuffix}, http.StatusOK, constants.DefaultIncrementValue},
		{"cancel fails", bootstrapEmail, nil, &failingRoute{method: http.MethodPost, suffix: "/" + cancelRoute},
			http.StatusOK, constants.DefaultIncrementValue},
		{"session not saved", bootstrapEmail, nil, &failingRoute{method: http.MethodPost, suffix: sessionSuffix},
			http.StatusInternalServerError, constants.DefaultInitValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending)
			if c.prep != nil {
				c.prep(t, fake)
			}
			serveFailing(t, fake, c.fail)
			rec := httptest.NewRecorder()
			authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, challengeFor(t, c.email),
				map[string]any{"email": c.email})))
			testutil.Equal(t, "login", rec.Code, c.want)
			testutil.Equal(t, "sessions", fake.SessionsIssued(), c.sessions)
		})
	}
}

// Each break-glass step on Redis fails closed when Redis can't do it as asked: a link index of another type, a freeze
// that lapsed or holds another type, a keyspace walk Redis refuses, or a link spent for an account it doesn't name.
func TestBreakGlassRedisStepsFailClosed(t *testing.T) {
	unreadable := func(key string) func(t *testing.T) {
		return func(t *testing.T) {
			redisServer.HSet(key, hashField, unreadableEntry)
			t.Cleanup(func() { redisServer.Del(key) })
		}
	}
	var link string
	cases := []struct {
		name string
		prep func(t *testing.T)
		step func() error
	}{
		{"drop links, invite index unreadable", unreadable(constants.RedisKeyPrefixInviteOf + plainTargetID),
			func() error { return authhelper.DropLinks(plainTargetID) }},
		{"drop links, session links unreadable", func(t *testing.T) {
			plant(t, constants.RedisKeyPrefixEnrollTokensOf+plainTargetID, unreadableEntry)
		}, func() error { return authhelper.DropLinks(plainTargetID) }},
		{"hold a link once the freeze lapsed", nil,
			func() error { return authhelper.FreezeForLink(plainTargetID, plantedToken, time.Minute) }},
		{"check a freeze of another type", unreadable(constants.RedisKeyPrefixFrozen + plainTargetID),
			func() error { return authhelper.RefuseFrozen(plainTargetID, constants.EmptyString) }},
		{"drop ceremonies, Redis down", brokenRedis, func() error { return webauthnhelper.DropCeremonies(plainTargetID) }},
		{"spend another account's link", func(t *testing.T) {
			token, _, err := authhelper.CreateEnrollToken(plainTargetID)
			if err != nil {
				t.Fatal(err)
			}
			link = token
		}, func() error { return authhelper.SpendEnrollToken(enrolledTargetID, tokenDigest(link)) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.prep != nil {
				c.prep(t)
			}
			if c.step() == nil {
				t.Fatal("the step went through, want it refused")
			}
		})
	}
}
