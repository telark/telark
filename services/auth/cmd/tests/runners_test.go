package tests

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/resources/finalizers"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/cmd"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

func TestBackfillRunFailsClosed(t *testing.T) {
	lg := constants.GetLogger(constants.LoggerPrefixCleanup)
	if err := cmd.RunBackFill(config.LoadBackfillConfig(), lg); err == nil {
		t.Fatal("backfill Run should fail with no backend")
	}
}

func TestBreakGlassRun(t *testing.T) {
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
		{"other account", "other@example.com", []*userresource.UserIdentity{passkey}},
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
	testutil.RedisEnv(t)
	var created *userresource.User
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			var user userresource.User
			_ = json.NewDecoder(r.Body).Decode(&user)
			user.ID = "u-root"
			created = &user
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": user})
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
