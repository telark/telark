package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/resources/finalizers"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notificationsclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/auth/cmd"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	flagEmail  = "--email"
	flagEnroll = "--enroll"
	// Pinned: a break-glass token opens the bootstrap account, so its lifetime is the security design.
	breakGlassTTL = 10 * time.Minute
	bootstrapPath = "/api/v1/bootstrap"
	usersPath     = "/api/v1/users"
)

// Shared by the package: auth binds its Redis client once per process, and the tests read the tokens stored there.
var redisServer *miniredis.Miniredis

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
	// Enrollment tokens are stored signed with it.
	if err := os.Setenv(dataconstants.EnvServiceToken, "test-service-token"); err != nil {
		panic(err)
	}
	m.Run()
	mr.Close()
}

// Both key kinds open a registration: a user's own link and an invite. An entry that doesn't verify names nobody.
func enrollTokenOwners() []string {
	var owners []string
	for _, key := range redisServer.Keys() {
		if strings.HasPrefix(key, constants.RedisKeyPrefixEnrollToken) || strings.HasPrefix(key, constants.RedisKeyPrefixInvite) {
			entry, _ := redisServer.Get(key)
			owner, _ := redishelper.VerifyEntry(key, entry)
			owners = append(owners, owner)
		}
	}
	return owners
}

func TestBackfillRunFailsClosed(t *testing.T) {
	lg := constants.GetLogger(constants.LoggerPrefixCleanup)
	if err := cmd.RunBackFill(config.LoadBackfillConfig(), lg); err == nil {
		t.Fatal("backfill Run should fail with no backend")
	}
}

// A record the exporter refuses to patch is logged, and the backfill goes on to the next type.
func TestBackfillGoesOnPastARefusedPatch(t *testing.T) {
	patches := constants.DefaultInitValue
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": []map[string]any{{"name": "bare"}}}})
			return
		}
		patches++
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":409}`))
	}))
	if err := cmd.RunBackFill(config.BackfillConfig{}, constants.GetLogger(constants.LoggerPrefixCleanup)); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "patches", patches, len(cleanupctrl.RegisteredResourceTypes()))
}

func TestBreakGlassRun(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, "nobody@example.com")
	if code := cmd.RunBreakGlass(nil); code != constants.ExitCodeError {
		t.Fatalf("break-glass with no email = %d, want %d", code, constants.ExitCodeError)
	}
	if code := cmd.RunBreakGlass([]string{"--email", "nobody@example.com"}); code != constants.ExitCodeError {
		t.Fatalf("break-glass with unreachable backend = %d, want %d", code, constants.ExitCodeError)
	}
}

func TestBreakGlassMarksBootstrapAdmin(t *testing.T) {
	const email = "admin@x.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	adminRole := constants.BuiltInRoleAdmin
	user := userresource.User{ID: "u-1", Email: email, RoleRefs: []*string{&adminRole}}
	var patches []map[string]any
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			patches = append(patches, body)
			_, _ = w.Write([]byte(`{"status":200}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": user})
	}))

	testutil.Equal(t, "exit", cmd.RunBreakGlass([]string{"--email", email}), constants.DefaultInitValue)
	testutil.Equal(t, "patches", len(patches), constants.DefaultIncrementValue)
	first := patches[constants.DefaultInitValue]
	testutil.Equal(t, "marker", first[constants.UserFieldBootstrap], true)
	testutil.Equal(t, "roles left alone (Admin already held)", first[constants.SpecFieldRoleRefs], nil)

	user.Bootstrap = true
	testutil.Equal(t, "exit when complete", cmd.RunBreakGlass([]string{"--email", email}), constants.DefaultInitValue)
	testutil.Equal(t, "patches when complete", len(patches), constants.DefaultIncrementValue)
}

// Every account break-glass promotes loses a Google identity bound while it was an
// ordinary account, and keeps its passkeys.
func TestBreakGlassStripsGoogleFromPromotedAccounts(t *testing.T) {
	const email = "test@example.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	passkey := &userresource.UserIdentity{Provider: constants.IdentityProviderPasskey, Subject: "cred"}
	google := &userresource.UserIdentity{Provider: constants.IdentityProviderGoogle, Subject: "sub"}
	cases := []struct {
		name  string
		email string
		want  []*userresource.UserIdentity
	}{
		{"bootstrap admin", email, []*userresource.UserIdentity{passkey}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user := userresource.User{ID: "u-1", Email: c.email, Identities: []*userresource.UserIdentity{google, passkey}}
			var patches []map[string]any
			testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					patches = append(patches, body)
					_, _ = w.Write([]byte(`{"status":200}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": user})
			}))

			testutil.Equal(t, "exit", cmd.RunBreakGlass([]string{"--email", c.email}), constants.DefaultInitValue)
			testutil.Equal(t, "patches", len(patches), constants.DefaultIncrementValue)
			got, _ := json.Marshal(patches[constants.DefaultInitValue][constants.UserFieldIdentities])
			want, _ := json.Marshal(c.want)
			testutil.Equal(t, "identities", string(got), string(want))
		})
	}
}

// A fresh install: the exporter knows no account until break-glass creates one.
type freshInstall struct {
	created *userresource.User
	creates int
}

func (f *freshInstall) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == bootstrapPath:
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	case r.Method == http.MethodPost:
		f.creates++
		var user userresource.User
		_ = json.NewDecoder(r.Body).Decode(&user)
		user.ID = "u-root"
		f.created = &user
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": user})
	case r.Method == http.MethodPatch:
		_, _ = w.Write([]byte(`{"status":200}`))
	case f.created == nil:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":404}`))
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{"data": f.created})
	}
}

func TestBreakGlassEnrollCreatesAdmin(t *testing.T) {
	const email = "root@x.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	redisServer.FlushAll()
	fresh := &freshInstall{}
	testutil.StubBackend(t, fresh)

	testutil.Equal(t, "exit without --enroll", cmd.RunBreakGlass([]string{"--email", email}), constants.ExitCodeError)
	testutil.Equal(t, "exit with --enroll", cmd.RunBreakGlass([]string{"--email", email, "--enroll"}), constants.DefaultInitValue)
	if fresh.created == nil || !fresh.created.Bootstrap || !authhelper.HasAdminRole(fresh.created.RoleRefs) {
		t.Fatalf("created = %+v, want Admin with the bootstrap marker", fresh.created)
	}
	testutil.Equal(t, "exit once it exists", cmd.RunBreakGlass([]string{"--email", email, "--enroll"}), constants.DefaultInitValue)
	testutil.Equal(t, "creates", fresh.creates, constants.DefaultIncrementValue)
	testutil.Equal(t, "enrollment token owners", strings.Join(enrollTokenOwners(), ","), "u-root")
}

// stdout holds only the steps and the link the dashboard's Members dialog would build. The 404 a fresh install answers
// the first lookup with is the expected "no account yet", so it shows nowhere as an exporter error.
func TestBreakGlassOutput(t *testing.T) {
	const email = "test@example.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	adminRole := constants.BuiltInRoleAdmin
	cases := []struct {
		name   string
		origin string
		fresh  bool
		args   []string
		want   []string
		link   string
	}{
		{"fresh install, dashboard exposed", "https://telark.example.com", true, []string{flagEnroll},
			[]string{"Created test@example.com with the Admin role", "Register a passkey for test@example.com"},
			"https://telark.example.com/register"},
		{"existing admin, port-forward install", constants.EmptyString, false, []string{flagEnroll},
			[]string{"Nothing to change", "Replace <dashboard-url>"}, "<dashboard-url>/register"},
		{"existing admin without --enroll", constants.EmptyString, false, nil,
			[]string{"Nothing to change", cmd.BreakGlassSignIn}, constants.EmptyString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.EnvRPOrigin, c.origin)
			redisServer.FlushAll()
			var backend http.Handler = &freshInstall{}
			if !c.fresh {
				backend = &testutil.FakeExporter{Users: map[string]*userresource.User{
					"u-root": {ID: "u-root", Email: email, Bootstrap: true, RoleRefs: []*string{&adminRole}},
				}}
			}
			testutil.StubBackend(t, backend)

			code, out, logged := runBreakGlass(t, append([]string{flagEmail, email}, c.args...)...)
			testutil.Equal(t, "exit", code, constants.DefaultInitValue)
			testutil.Equal(t, "stderr", logged, constants.EmptyString)
			for _, want := range c.want {
				if !strings.Contains(out, want) {
					t.Fatalf("stdout %q lacks %q", out, want)
				}
			}
			if c.link == constants.EmptyString {
				testutil.Equal(t, "link printed", strings.Contains(out, "/register#"), false)
				return
			}
			base, fragment := enrollLink(t, out)
			token := linkToken(t, fragment)
			testutil.Equal(t, "link", base, c.link)
			testutil.Equal(t, "fragment", fragment, "enroll="+token+"&email=test%40example.com")
			if owner, _, err := authhelper.ResolveEnrollToken(token); err != nil || owner != "u-root" {
				t.Fatalf("token resolves to %q (%v), want u-root", owner, err)
			}
		})
	}
}

// Only a 404 means the account is missing. Any other failed lookup is reported as one and creates nothing,
// and a create the exporter refuses is reported with nothing on stdout.
func TestBreakGlassLookupFailure(t *testing.T) {
	const email = "test@example.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	cases := []struct {
		name       string
		status     int
		args       []string
		want       string
		creates    int
		statusRead bool
	}{
		{"exporter fails the lookup", http.StatusInternalServerError, []string{flagEnroll},
			"Could not look up test@example.com", constants.DefaultInitValue, true},
		{"no account without --enroll", http.StatusNotFound, nil, "No account uses test@example.com", constants.DefaultInitValue, true},
		{"exporter refuses the create", http.StatusNotFound, []string{flagEnroll}, "Could not create test@example.com: status 404",
			constants.DefaultIncrementValue, true},
		{"exporter fails the bootstrap read", http.StatusInternalServerError, []string{flagEnroll},
			"Could not read the bootstrap account", constants.DefaultInitValue, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			creates := constants.DefaultInitValue
			testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					creates++
				}
				// No account is pinned, so break-glass goes on to the lookup this test fails.
				if r.URL.Path == bootstrapPath && c.statusRead {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
					return
				}
				w.WriteHeader(c.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": c.status})
			}))

			code, out, logged := runBreakGlass(t, append([]string{flagEmail, email}, c.args...)...)
			testutil.Equal(t, "exit", code, constants.ExitCodeError)
			testutil.Equal(t, "creates", creates, c.creates)
			testutil.Equal(t, "stdout", out, constants.EmptyString)
			if !strings.Contains(logged, c.want) {
				t.Fatalf("stderr %q lacks %q", logged, c.want)
			}
		})
	}
}

// The operator reads the answer on stdout and diagnostics on stderr, so each is captured on its own.
func runBreakGlass(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errReader, errWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prevOut, prevErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWriter, errWriter
	code = cmd.RunBreakGlass(args)
	os.Stdout, os.Stderr = prevOut, prevErr
	for _, writer := range []*os.File{outWriter, errWriter} {
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	printed, err := io.ReadAll(outReader)
	if err != nil {
		t.Fatal(err)
	}
	logged, err := io.ReadAll(errReader)
	if err != nil {
		t.Fatal(err)
	}
	return code, string(printed), string(logged)
}

// The token rides in the enrollment link's fragment, which no server sees, as the dashboard's Members dialog builds it.
func enrollLink(t *testing.T, out string) (base, fragment string) {
	t.Helper()
	for _, field := range strings.Fields(out) {
		if before, after, found := strings.Cut(field, "/register#"); found {
			return before + "/register", after
		}
	}
	t.Fatalf("no enrollment link in %q", out)
	return constants.EmptyString, constants.EmptyString
}

func linkToken(t *testing.T, fragment string) string {
	t.Helper()
	values, err := url.ParseQuery(fragment)
	if err != nil {
		t.Fatal(err)
	}
	return values.Get("enroll")
}

func enroll(t *testing.T, email string) (token, out string) {
	t.Helper()
	code, out, _ := runBreakGlass(t, flagEmail, email, flagEnroll)
	testutil.Equal(t, "exit", code, constants.DefaultInitValue)
	_, fragment := enrollLink(t, out)
	return linkToken(t, fragment), out
}

// Before the pin, --enroll issues an invite like a link from Members: a new run replaces the previous token, the
// record shows the link with no issuing user, and an account promoted with a passkey hears of it when the token is
// created and used (the exporter pins that account as it marks it, so its next run opens a request). The token
// lives 10 minutes, whatever lifetime the chart gives member links.
func TestBreakGlassEnrollIssuesAnInvite(t *testing.T) {
	const email, replacingRuns = "test@example.com", 2
	t.Setenv(constants.EnvBootstrapAdmin, email)
	t.Setenv(constants.EnvEnrollInviteTTLSec, "86400")
	adminRole := constants.BuiltInRoleAdmin
	created, used := notificationsclient.TypeEnrollLinkCreated, notificationsclient.TypeEnrollLinkUsed
	cases := []struct {
		name     string
		marked   bool
		passkeys int
		runs     int
		notices  []string
	}{
		{"account without a passkey", true, constants.DefaultInitValue, replacingRuns, nil},
		{"account with a passkey, promoted", false, constants.DefaultIncrementValue, constants.DefaultIncrementValue, []string{created, used}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			redisServer.FlushAll()
			fake := &testutil.FakeExporter{
				Users:    map[string]*userresource.User{"u-1": {ID: "u-1", Email: email, Bootstrap: c.marked, RoleRefs: []*string{&adminRole}}},
				Passkeys: map[string]int{"u-1": c.passkeys},
			}
			testutil.StubBackend(t, fake)

			var tokens []string
			out := constants.EmptyString
			for range c.runs {
				var token string
				token, out = enroll(t, email)
				tokens = append(tokens, token)
			}
			token := tokens[len(tokens)-constants.DefaultIncrementValue]
			for _, replaced := range tokens[:len(tokens)-constants.DefaultIncrementValue] {
				if _, _, err := authhelper.ResolveEnrollToken(replaced); err == nil {
					t.Fatal("the replaced token still opens a registration")
				}
			}
			testutil.Equal(t, "live tokens", strings.Join(enrollTokenOwners(), ","), "u-1")
			testutil.Equal(t, "lifetime", redisServer.TTL(constants.RedisKeyPrefixInviteOf+"u-1"), breakGlassTTL)
			invite := fake.User("u-1").Status.Invite
			if invite == nil || invite.IssuedBy != constants.EmptyString || !strings.Contains(out, invite.ExpiresAt) {
				t.Fatalf("stored invite = %+v, want the printed expiry and no issuer", invite)
			}
			expectInviteWindow(t, invite, breakGlassTTL)

			// The steps of the registration the token opens, without the WebAuthn ceremony.
			owner, digest, err := authhelper.ResolveEnrollToken(token)
			if err != nil || owner != "u-1" {
				t.Fatalf("token resolves to %q (%v), want u-1", owner, err)
			}
			if err := authhelper.SpendEnrollToken(owner, digest); err != nil {
				t.Fatalf("spend token: %v", err)
			}
			if _, err := authhelper.CreatePasskey(owner, &authdata.Passkey{UserID: owner}); err != nil {
				t.Fatalf("store passkey: %v", err)
			}
			user := fake.User(owner)
			authhelper.CompleteInvite(&user, true)
			testutil.Equal(t, "notices", noticeTypes(t, fake.Notices(), owner), strings.Join(c.notices, ","))
		})
	}
}

// A marked account with a passkey and no pin means the exporter couldn't store the pin: break-glass refuses rather
// than act at once, and changes nothing.
func TestBreakGlassRefusesAnEnrolledAccountWithoutAPin(t *testing.T) {
	const email = "test@example.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	redisServer.FlushAll()
	adminRole := constants.BuiltInRoleAdmin
	testutil.StubBackend(t, &testutil.FakeExporter{
		Users:    map[string]*userresource.User{"u-1": {ID: "u-1", Email: email, Bootstrap: true, RoleRefs: []*string{&adminRole}}},
		Passkeys: map[string]int{"u-1": constants.DefaultIncrementValue},
	})
	code, out, logged := runBreakGlass(t, flagEmail, email, flagEnroll)
	testutil.Equal(t, "refused", code, constants.ExitCodeError)
	testutil.Equal(t, "says why", strings.Contains(logged, "has a passkey but is not pinned"), true)
	testutil.Equal(t, "no link", strings.Contains(out, "#enroll="), false)
	testutil.Equal(t, "tokens", len(enrollTokenOwners()), constants.DefaultInitValue)
}

// Before the pin, an exporter that fails a step after the lookup stops the run with the reason on stderr and no link.
func TestBreakGlassReportsAFailedStep(t *testing.T) {
	const email, userID = "test@example.com", "u-1"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	adminRole := constants.BuiltInRoleAdmin
	marked := &userresource.User{ID: userID, Email: email, Bootstrap: true, RoleRefs: []*string{&adminRole}}
	member := &userresource.User{ID: userID, Email: email}
	gone := func(fake *testutil.FakeExporter) { fake.SetGone(userID) }
	cases := []struct {
		name  string
		user  *userresource.User
		setup func(fake *testutil.FakeExporter)
		says  string
	}{
		{"the passkey lookup fails", marked, (*testutil.FakeExporter).FailNextPasskeyRead, "Could not look up test@example.com"},
		{"the promotion is refused", member, gone, "Could not promote test@example.com: status 410"},
		{"the link's record is refused", marked, gone, "Could not create the enrollment link"},
		{"the created account never shows", nil, func(*testutil.FakeExporter) {}, "it was not found once created"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			redisServer.FlushAll()
			fake := &testutil.FakeExporter{Users: map[string]*userresource.User{}}
			if c.user != nil {
				fake.Users[userID] = c.user
			}
			c.setup(fake)
			// The exporter accepts any create, and its lookups never show the account.
			testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == usersPath {
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"status":201}`))
					return
				}
				fake.ServeHTTP(w, r)
			}))

			code, out, logged := runBreakGlass(t, flagEmail, email, flagEnroll)
			testutil.Equal(t, "exit", code, constants.ExitCodeError)
			testutil.Equal(t, "link printed", strings.Contains(out, "#enroll="), false)
			if !strings.Contains(logged, c.says) {
				t.Fatalf("stderr %q lacks %q", logged, c.says)
			}
		})
	}
}

// The record's window, from issue to expiry, is the token's lifetime.
func expectInviteWindow(t *testing.T, invite *userresource.Invite, ttl time.Duration) {
	t.Helper()
	issued, issuedErr := time.Parse(time.RFC3339, invite.IssuedAt)
	expires, expiresErr := time.Parse(time.RFC3339, invite.ExpiresAt)
	if issuedErr != nil || expiresErr != nil || expires.Sub(issued) != ttl {
		t.Fatalf("invite window %s..%s, want %s", invite.IssuedAt, invite.ExpiresAt, ttl)
	}
}

// Every notice goes to the account, and one about a new token names break-glass.
func noticeTypes(t *testing.T, notices []notificationsclient.Notification, owner string) string {
	t.Helper()
	types := make([]string, constants.DefaultInitValue, len(notices))
	for _, notice := range notices {
		testutil.Equal(t, "recipient", notice.UserID, owner)
		if notice.Type == notificationsclient.TypeEnrollLinkCreated && !strings.Contains(notice.Message, "break-glass") {
			t.Fatalf("creation notice %q does not name break-glass", notice.Message)
		}
		types = append(types, notice.Type)
	}
	return strings.Join(types, ",")
}

// Nothing but BOOTSTRAP_ADMIN may be created or promoted, so any other email, or none configured,
// is refused before the exporter or Redis is reached.
func TestBreakGlassRefusesAnyOtherEmail(t *testing.T) {
	const other = "other@example.com"
	readOnly := constants.BuiltInRoleReadOnly
	member := &userresource.User{ID: "u-member", Email: other, RoleRefs: []*string{&readOnly}}
	cases := []struct {
		name      string
		bootstrap string
		existing  *userresource.User
	}{
		{"no bootstrap admin configured", "", nil},
		{"unknown email on a fresh install", "test@example.com", nil},
		{"existing non-bootstrap account", "test@example.com", member},
	}
	for _, c := range cases {
		for _, args := range [][]string{{"--email", other, "--enroll"}, {"--email", other}} {
			t.Run(c.name+" "+strings.Join(args, " "), func(t *testing.T) {
				t.Setenv(constants.EnvBootstrapAdmin, c.bootstrap)
				redisServer.FlushAll()
				calls := constants.DefaultInitValue
				testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					if c.existing == nil {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"status":404}`))
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": c.existing})
				}))

				testutil.Equal(t, "exit", cmd.RunBreakGlass(args), constants.ExitCodeError)
				testutil.Equal(t, "exporter calls", calls, constants.DefaultInitValue)
				testutil.Equal(t, "enrollment tokens", len(enrollTokenOwners()), constants.DefaultInitValue)
			})
		}
	}
}

func TestRemoveFinalizers(t *testing.T) {
	views := []map[string]any{
		{"name": "held", "finalizers": []string{finalizers.UserCleanup, finalizers.GroupCleanup, finalizers.RoleCleanup}},
		{"name": "free"},
	}
	var removed []string
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": views}})
			return
		}
		removed = append(removed, r.Method+" "+r.URL.Path)
		_, _ = w.Write([]byte(`{"status":200}`))
	}))

	cmd.RemoveFinalizers(constants.GetLogger(constants.LoggerPrefixCleanup))

	testutil.Equal(t, "removals", len(removed), len(cleanupctrl.RegisteredResourceTypes()))
	for _, call := range removed {
		if !strings.HasPrefix(call, http.MethodDelete) || !strings.Contains(call, "held") {
			t.Fatalf("unexpected call %q", call)
		}
	}
}

// Best effort: a list that fails is logged, and the uninstall goes on to the next type.
func TestRemoveFinalizersGoesOnPastAFailedList(t *testing.T) {
	lists := constants.DefaultInitValue
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		lists++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":500}`))
	}))

	cmd.RemoveFinalizers(constants.GetLogger(constants.LoggerPrefixCleanup))

	testutil.Equal(t, "lists", lists, len(cleanupctrl.RegisteredResourceTypes()))
}
