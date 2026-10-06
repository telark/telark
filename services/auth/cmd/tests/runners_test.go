package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/finalizers"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notificationsclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/auth/cmd"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
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
	m.Run()
	mr.Close()
}

// Both key kinds open a registration: a user's own link and an invite.
func enrollTokenOwners() []string {
	var owners []string
	for _, key := range redisServer.Keys() {
		if strings.HasPrefix(key, constants.RedisKeyPrefixEnrollToken) || strings.HasPrefix(key, constants.RedisKeyPrefixInvite) {
			owner, _ := redisServer.Get(key)
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

func TestBreakGlassEnrollCreatesAdmin(t *testing.T) {
	const email = "root@x.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	redisServer.FlushAll()
	var created *userresource.User
	creates := constants.DefaultInitValue
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			creates++
			var user userresource.User
			_ = json.NewDecoder(r.Body).Decode(&user)
			user.ID = "u-root"
			created = &user
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": user})
		case r.Method == http.MethodPatch:
			_, _ = w.Write([]byte(`{"status":200}`))
		case created == nil:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":404}`))
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": created})
		}
	}))

	testutil.Equal(t, "exit without --enroll", cmd.RunBreakGlass([]string{"--email", email}), constants.ExitCodeError)
	testutil.Equal(t, "exit with --enroll", cmd.RunBreakGlass([]string{"--email", email, "--enroll"}), constants.DefaultInitValue)
	if created == nil || !created.Bootstrap || !authhelper.HasAdminRole(created.RoleRefs) {
		t.Fatalf("created = %+v, want Admin with the bootstrap marker", created)
	}
	testutil.Equal(t, "exit once it exists", cmd.RunBreakGlass([]string{"--email", email, "--enroll"}), constants.DefaultInitValue)
	testutil.Equal(t, "creates", creates, constants.DefaultIncrementValue)
	testutil.Equal(t, "enrollment token owners", strings.Join(enrollTokenOwners(), ","), "u-root")
}

// The operator reads the token on stdout, as the last field the command prints.
func enroll(t *testing.T, email string) (token, out string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = writer
	code := cmd.RunBreakGlass([]string{"--email", email, "--enroll"})
	os.Stdout = stdout
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	printed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "exit", code, constants.DefaultInitValue)
	fields := strings.Fields(string(printed))
	return fields[len(fields)-constants.DefaultIncrementValue], string(printed)
}

// --enroll issues an invite like a link from Members: a new run replaces the previous token, the record shows the
// link with no issuing user, and an account that has a passkey hears of it when the token is created and used.
func TestBreakGlassEnrollIssuesAnInvite(t *testing.T) {
	const email = "test@example.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	adminRole := constants.BuiltInRoleAdmin
	created, used := notificationsclient.TypeEnrollLinkCreated, notificationsclient.TypeEnrollLinkUsed
	cases := []struct {
		name     string
		passkeys int
		notices  []string
	}{
		{"account without a passkey", constants.DefaultInitValue, nil},
		{"account with a passkey", constants.DefaultIncrementValue, []string{created, created, used}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			redisServer.FlushAll()
			fake := &testutil.FakeExporter{
				Users:    map[string]*userresource.User{"u-1": {ID: "u-1", Email: email, Bootstrap: true, RoleRefs: []*string{&adminRole}}},
				Passkeys: map[string]int{"u-1": c.passkeys},
			}
			testutil.StubBackend(t, fake)

			replaced, _ := enroll(t, email)
			token, out := enroll(t, email)
			if _, _, err := authhelper.ResolveEnrollToken(replaced); err == nil {
				t.Fatal("the replaced token still opens a registration")
			}
			testutil.Equal(t, "live tokens", strings.Join(enrollTokenOwners(), ","), "u-1")
			testutil.Equal(t, "lifetime", redisServer.TTL(constants.RedisKeyPrefixInviteOf+"u-1"), config.EnrollInviteTTL())
			invite := fake.User("u-1").Status.Invite
			if invite == nil || invite.IssuedBy != constants.EmptyString || !strings.Contains(out, invite.ExpiresAt) {
				t.Fatalf("stored invite = %+v, want the printed expiry and no issuer", invite)
			}

			// The steps of the registration the token opens, without the WebAuthn ceremony.
			owner, isInvite, err := authhelper.ResolveEnrollToken(token)
			if err != nil || owner != "u-1" || !isInvite {
				t.Fatalf("token resolves to %q, invite %v (%v), want the invite of u-1", owner, isInvite, err)
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
