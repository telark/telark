package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/telark/auth/internal/cmd/backfill"
	"github.com/telark/auth/internal/cmd/breakglass"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	"github.com/telark/auth/internal/tests/testutil"
	userresource "github.com/telark/data/resources/user"
)

// The backfill runner walks every registered resource type; with no backend the
// first list call fails and the run aborts with that error.
func TestBackfillRunFailsClosed(t *testing.T) {
	lg := constants.GetLogger(constants.LoggerPrefixCleanup)
	if err := backfill.Run(config.LoadBackfillConfig(), lg); err == nil {
		t.Fatal("backfill Run should fail with no backend")
	}
}

// break-glass rejects a missing email and, given one, fails when the user cannot
// be looked up — both return the error exit code.
func TestBreakGlassRun(t *testing.T) {
	if code := breakglass.Run(nil); code != constants.ExitCodeError {
		t.Fatalf("break-glass with no email = %d, want %d", code, constants.ExitCodeError)
	}
	if code := breakglass.Run([]string{"--email", "nobody@example.com"}); code != constants.ExitCodeError {
		t.Fatalf("break-glass with unreachable backend = %d, want %d", code, constants.ExitCodeError)
	}
}

// Run by an operator, break-glass is the trusted path that marks a bootstrap
// admin created before the marker existed: an Admin user whose email is in
// BOOTSTRAP_ADMINS gets `bootstrap: true`; a record already carrying it is left alone.
func TestBreakGlassMarksBootstrapAdmin(t *testing.T) {
	const email = "admin@x.com"
	t.Setenv(constants.EnvBootstrapAdmins, email)
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

	testutil.Equal(t, "exit", breakglass.Run([]string{"--email", email}), constants.DefaultInitValue)
	testutil.Equal(t, "patches", len(patches), constants.DefaultIncrementValue)
	first := patches[constants.DefaultInitValue]
	testutil.Equal[any](t, "marker", first[constants.UserFieldBootstrap], true)
	testutil.Equal[any](t, "roles left alone (Admin already held)", first[constants.SpecFieldRoleRefs], nil)

	user.Bootstrap = true
	testutil.Equal(t, "exit when complete", breakglass.Run([]string{"--email", email}), constants.DefaultInitValue)
	testutil.Equal(t, "patches when complete", len(patches), constants.DefaultIncrementValue)
}

// With --enroll the operator can bootstrap a missing account: the user is created
// with the Admin role and the bootstrap marker, and a one-time enrollment token
// is printed, which is the only unauthenticated way into that account.
func TestBreakGlassEnrollCreatesAdmin(t *testing.T) {
	const email = "root@x.com"
	t.Setenv(constants.EnvBootstrapAdmins, email)
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

	testutil.Equal(t, "exit without --enroll", breakglass.Run([]string{"--email", email}), constants.ExitCodeError)
	testutil.Equal(t, "exit with --enroll", breakglass.Run([]string{"--email", email, "--enroll"}), constants.DefaultInitValue)
	if created == nil || !created.Bootstrap || !authhelper.HasAdminRole(created.RoleRefs) {
		t.Fatalf("created = %+v, want Admin with the bootstrap marker", created)
	}
}
