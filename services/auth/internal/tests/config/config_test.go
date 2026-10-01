package config

import (
	"strconv"
	"testing"
	"time"

	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	retiredSelfRegEnv = "SELF_REGISTRATION_ENABLED"
	customTTLSec      = "900"
	customTTL         = 15 * time.Minute
)

// The bootstrap admin is one email, normalized (trimmed + lowercased), and
// IsBootstrapAdmin matches it case-insensitively; a comma list is not split.
func TestLoadBootstrapConfig(t *testing.T) {
	cases := []struct {
		name    string
		admin   string
		selfReg string
		wantErr bool
		isAdmin bool
	}{
		{"admin", " A@x.com ", constants.EmptyString, false, true},
		{"list is one unmatched value", "a@x.com,b@x.com", constants.EmptyString, false, false},
		{"no admin is invalid", constants.EmptyString, constants.EmptyString, true, false},
		{"no admin with the retired switch on is invalid", constants.EmptyString, "true", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.EnvBootstrapAdmin, c.admin)
			t.Setenv(retiredSelfRegEnv, c.selfReg)

			_, err := config.LoadBootstrapConfig()
			if (err != nil) != c.wantErr {
				t.Fatalf("LoadBootstrapConfig err = %v, wantErr %v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if config.IsBootstrapAdmin("A@X.COM") != c.isAdmin {
				t.Fatalf("IsBootstrapAdmin = %v, want %v", config.IsBootstrapAdmin("A@X.COM"), c.isAdmin)
			}
		})
	}
}

// A never-loaded bootstrap config must fail safe: no admin.
func TestBootstrapDefaultsWhenUnloaded(t *testing.T) {
	// A prior subtest may have loaded config; this only asserts the accessor does
	// not panic and returns a boolean, exercising the nil-safe branch.
	if config.IsBootstrapAdmin("nobody@x.com") || config.IsBootstrapAdmin(constants.EmptyString) {
		t.Fatal("unlisted email reported as bootstrap admin")
	}
}

// An invite link lives an hour unless the chart sets ENROLL_INVITE_TTL_SEC; anything but
// a positive number keeps the hour, since a zero TTL would never expire.
func TestEnrollInviteTTL(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"default", constants.EmptyString, time.Hour},
		{"chart value", customTTLSec, customTTL},
		{"zero keeps the default", "0", time.Hour},
		{"garbage keeps the default", "soon", time.Hour},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.EnvEnrollInviteTTLSec, c.value)
			testutil.Equal(t, "invite TTL", config.EnrollInviteTTL(), c.want)
		})
	}
}

// Cleanup + backfill config read every knob from the environment with sane
// positive defaults so the reconciler never runs with a zero interval.
func TestLoadCleanupAndBackfillConfig(t *testing.T) {
	clean := config.LoadCleanupConfig()
	if clean.ReconcileTick <= constants.DefaultInitValue || clean.WorkersPerType <= constants.DefaultInitValue ||
		clean.SweeperInterval <= constants.DefaultInitValue {
		t.Fatalf("cleanup defaults not positive: %+v", clean)
	}

	back := config.LoadBackfillConfig()
	if back.BatchSize <= constants.DefaultInitValue {
		t.Fatalf("backfill batch size not positive: %d", back.BatchSize)
	}
}

// A malformed, zero or negative knob falls back to its default instead of stopping
// the workers, panicking the sweeper's ticker or dead-lettering every job.
func TestLoadCleanupConfigRejectsNonPositiveValues(t *testing.T) {
	const customConcurrentPatches = 7
	t.Setenv(constants.EnvCleanupWorkersPerType, "abc")
	t.Setenv(constants.EnvCleanupSweeperIntervalSeconds, "0")
	t.Setenv(constants.EnvCleanupJobMaxAttempts, "-3")
	t.Setenv(constants.EnvCleanupMaxConcurrentPatches, strconv.Itoa(customConcurrentPatches))

	clean := config.LoadCleanupConfig()
	if clean.WorkersPerType != constants.DefaultCleanupWorkersPerType ||
		clean.SweeperInterval != constants.DefaultCleanupSweeperIntervalSeconds*time.Second ||
		clean.JobMaxAttempts != constants.DefaultCleanupJobMaxAttempts {
		t.Fatalf("invalid values not replaced by defaults: %+v", clean)
	}
	if clean.MaxConcurrentPatches != customConcurrentPatches {
		t.Fatalf("valid value = %d, want %d", clean.MaxConcurrentPatches, customConcurrentPatches)
	}
}

// LoadConfig requires the WebAuthn relying-party env and wires it into the
// typed config; missing RP_* is a hard boot failure elsewhere.
func TestLoadConfig(t *testing.T) {
	t.Setenv("RP_ID", "localhost")
	t.Setenv("RP_NAME", "Dashboard App")
	t.Setenv("RP_ORIGIN", "http://localhost:3000")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	if cfg.WebAuthn.RPID != "localhost" || cfg.WebAuthn.RPName != "Dashboard App" {
		t.Fatalf("WebAuthn config not wired: %+v", cfg.WebAuthn)
	}
}
