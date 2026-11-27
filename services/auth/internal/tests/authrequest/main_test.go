package authrequest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	dataconstants "github.com/telark/telark/internal/data/constants"
	requestdata "github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/services/auth/internal/authrequest"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
	"golang.org/x/crypto/nacl/box"
)

const (
	adminMailbox  = "test@example.com"
	otherMailbox  = "other@example.com"
	nextMailbox   = "next@example.com"
	pinnedUser    = "u-1"
	nextUser      = "u-2"
	requester     = "system:serviceaccount:telark:ops-admin"
	firstRequest  = "enroll-1"
	secondRequest = "enroll-2"
	dashboard     = "https://telark.example.com"
	serviceToken  = "test-service-token"
	envRedisHost  = "REDIS_HOST"
	envRedisPort  = "REDIS_PORT"
	// Pinned: a request's token opens the bootstrap account, so its lifetime is the security design.
	linkTTL = 10 * time.Minute
	// How long a test waits for a request to reach a phase (the longest run takes under 14 minutes), and how often it looks.
	runLimit  = 30 * time.Minute
	lookEvery = 100 * time.Millisecond
	// Stands in a wanted reason for an error text the exporter client words.
	anyText       = "\x00"
	requestRoutes = "/api/v1/internal/auth/requests/"
	bootstrapPath = "/api/v1/bootstrap"
	usersPath     = "/api/v1/users"
	passkeysPath  = "/api/v1/internal/auth/passkeys"
	byEmailPath   = "/users/by-email/"
	fragmentSep   = "#"
	tokenParam    = "enroll"
	listSep       = ","
	statusKey     = "status"
	takenOver     = "another-run"
	// Before 2000, where a bubble's clock starts.
	longAgo      = "1999-01-01T00:00:00Z"
	phasesLabel  = "phases"
	callsLabel   = "exporter calls"
	othersLabel  = "calls outside the request routes"
	tokensLabel  = "live links"
	lockLabel    = "run lock"
	thawedLabel  = "thawed"
	requestLabel = "requestId"
)

// Shared by the package: auth binds its Redis client once per process, and the tests read the tokens stored there.
var redisServer *miniredis.Miniredis

func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	redisServer = mr
	if err := os.Setenv(envRedisHost, mr.Host()); err != nil {
		panic(err)
	}
	if err := os.Setenv(envRedisPort, mr.Port()); err != nil {
		panic(err)
	}
	// Enrollment tokens are stored signed with it.
	if err := os.Setenv(dataconstants.EnvServiceToken, serviceToken); err != nil {
		panic(err)
	}
	// Bound outside any synctest bubble: a client a bubble created would panic when the next bubble used it.
	if redishelper.NewRedisClientWithRetry(context.Background()) == nil {
		panic("redis client unavailable")
	}
	m.Run()
	mr.Close()
}

// The loop reads the bootstrap admin from auth's loaded config, as it does once auth started.
func newBench(t *testing.T, admin string, fake *testutil.FakeExporter) *bench {
	t.Helper()
	t.Setenv(constants.EnvBootstrapAdmin, admin)
	if _, err := config.LoadBootstrapConfig(); err != nil {
		t.Fatal(err)
	}
	redisServer.FlushAll()
	public, private, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b := &bench{fake: fake, backend: fake, admin: admin, public: public, private: private, done: make(chan struct{})}
	testutil.StubBackend(t, b)
	return b
}

func (b *bench) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, requestRoutes) {
		b.others.Add(constants.DefaultIncrementValue)
	}
	b.backend.ServeHTTP(w, r)
}

func (b *bench) request(id, email string) requestdata.AuthRequest {
	return requestdata.AuthRequest{ID: id, Spec: requestdata.Spec{
		Email: email, RequestedBy: requester, RecipientKey: base64.StdEncoding.EncodeToString(b.public[:]),
	}}
}

// With no request given, one for the bootstrap admin's email. The test's end cancels the loop if the test didn't.
func (b *bench) begin(t *testing.T, requests ...requestdata.AuthRequest) {
	t.Helper()
	if len(requests) == constants.DefaultInitValue {
		requests = []requestdata.AuthRequest{b.request(firstRequest, b.admin)}
	}
	b.fake.AuthRequests = requests
	ctx, cancel := context.WithCancel(t.Context())
	b.cancel = cancel
	go func() {
		authrequest.Start(ctx)
		close(b.done)
	}()
}

func (b *bench) halt() {
	b.cancel()
	<-b.done
}

func (b *bench) writes(id string) []requestdata.Status {
	var statuses []requestdata.Status
	for _, write := range b.fake.StatusWrites() {
		if write.ID == id {
			statuses = append(statuses, write.Status)
		}
	}
	return statuses
}

func (b *bench) phases(id string) string {
	writes := b.writes(id)
	phases := make([]string, constants.DefaultInitValue, len(writes))
	for _, status := range writes {
		phases = append(phases, status.Phase)
	}
	return strings.Join(phases, listSep)
}

func (b *bench) await(t *testing.T, id, phase string) requestdata.Status {
	t.Helper()
	for deadline := time.Now().Add(runLimit); time.Now().Before(deadline); time.Sleep(lookEvery) {
		writes := b.writes(id)
		at := slices.IndexFunc(writes, func(status requestdata.Status) bool { return status.Phase == phase })
		if at >= constants.DefaultInitValue {
			return writes[at]
		}
	}
	t.Fatalf("request %s never reached %s: %s", id, phase, b.phases(id))
	return requestdata.Status{}
}

// The status the request's run ended with.
func (b *bench) last(t *testing.T, id string) requestdata.Status {
	t.Helper()
	for deadline := time.Now().Add(runLimit); time.Now().Before(deadline); time.Sleep(lookEvery) {
		writes := b.writes(id)
		if len(writes) == constants.DefaultInitValue {
			continue
		}
		last := requestdata.AuthRequest{Status: writes[len(writes)-constants.DefaultIncrementValue]}
		if !last.InFlight() {
			return last.Status
		}
	}
	t.Fatalf("request %s never ended: %s", id, b.phases(id))
	return requestdata.Status{}
}

func (b *bench) end(t *testing.T, id string) requestdata.Status {
	t.Helper()
	last := b.last(t, id)
	b.halt()
	return last
}

func (b *bench) run(t *testing.T) requestdata.Status {
	t.Helper()
	b.begin(t)
	return b.end(t, firstRequest)
}

func (b *bench) expect(t *testing.T, id string, want ending) {
	t.Helper()
	testutil.Equal(t, phasesLabel, b.phases(id), strings.Join(want.phases, listSep))
	writes := b.writes(id)
	got := writes[len(writes)-constants.DefaultIncrementValue].Reason
	before, after, wild := strings.Cut(want.reason, anyText)
	if !wild {
		testutil.Equal(t, "reason", got, want.reason)
		return
	}
	if !strings.HasPrefix(got, before) || !strings.HasSuffix(got, after) {
		t.Fatalf("reason = %q, want %q … %q", got, before, after)
	}
}

// Only the requester's private key opens the link the status carries.
func (b *bench) openLink(t *testing.T, status requestdata.Status) (link, token string) {
	t.Helper()
	sealed, err := base64.StdEncoding.DecodeString(status.SealedLink)
	if err != nil {
		t.Fatal(err)
	}
	opened, ok := box.OpenAnonymous(nil, sealed, b.public, b.private)
	if !ok {
		t.Fatalf("sealedLink %q does not open with the requester's key", status.SealedLink)
	}
	_, fragment, _ := strings.Cut(string(opened), fragmentSep)
	values, err := url.ParseQuery(fragment)
	if err != nil {
		t.Fatal(err)
	}
	return string(opened), values.Get(tokenParam)
}

func wantLink(origin, token, email string) string {
	return fmt.Sprintf(constants.AuthRequestEnrollURL, origin, url.QueryEscape(token), url.QueryEscape(email))
}

// Answers status to the calls of method whose path holds part; the fake answers the rest.
func refuse(fake http.Handler, method, part string, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method || !strings.Contains(r.URL.Path, part) {
			fake.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{statusKey: status})
	})
}

// Both key kinds open a registration: a user's own link and an invite. An entry that doesn't verify names nobody.
func enrollTokenOwners() string {
	var owners []string
	for _, key := range redisServer.Keys() {
		if strings.HasPrefix(key, constants.RedisKeyPrefixEnrollToken) || strings.HasPrefix(key, constants.RedisKeyPrefixInvite) {
			entry, _ := redisServer.Get(key)
			owner, _ := redishelper.VerifyEntry(key, entry)
			owners = append(owners, owner)
		}
	}
	return strings.Join(owners, listSep)
}

// The way auth's registration finish does, without the WebAuthn ceremony: the freeze checks with the link's digest, the
// token is spent and the passkey saved.
func register(t *testing.T, userID, token string) {
	t.Helper()
	owner, digest, err := authhelper.ResolveEnrollToken(token)
	if err != nil || owner != userID {
		t.Fatalf("token resolves to %q (%v), want %s", owner, err, userID)
	}
	if err := authhelper.RefuseFrozen(userID, digest); err != nil {
		t.Fatal(err)
	}
	if err := authhelper.SpendEnrollToken(userID, digest); err != nil {
		t.Fatal(err)
	}
	saveLinkPasskey(t, userID)
}
