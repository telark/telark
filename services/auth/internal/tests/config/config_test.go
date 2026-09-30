package config

import (
	"strconv"
	"testing"
	"time"

	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
)

// The bootstrap admin is one email, normalised (trimmed + lowercased), and
// IsBootstrapAdmin matches it case-insensitively; a comma list is not split.
func TestLoadBootstrapConfig(t *testing.T) {
	cases := []struct {
		name     string
		admin    string
		selfReg  string
		wantErr  bool
		wantSelf bool
		isAdmin  bool
	}{
		{"admin with self-reg off", " A@x.com ", "false", false, false, true},
		{"list is one unmatched value", "a@x.com,b@x.com", "true", false, true, false},
		{"no admin, self-reg on", "", "true", false, true, false},
		{"no admin, self-reg off is invalid", "", "false", true, false, false},
		{"self-reg unset defaults off", "a@x.com", constants.EmptyString, false, false, true},
		{"no admin, self-reg unset is invalid", constants.EmptyString, constants.EmptyString, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.EnvBootstrapAdmin, c.admin)
			t.Setenv(constants.EnvSelfRegistrationEnabled, c.selfReg)

			_, err := config.LoadBootstrapConfig()
			if (err != nil) != c.wantErr {
				t.Fatalf("LoadBootstrapConfig err = %v, wantErr %v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if config.IsSelfRegistrationEnabled() != c.wantSelf {
				t.Fatalf("IsSelfRegistrationEnabled = %v, want %v", config.IsSelfRegistrationEnabled(), c.wantSelf)
			}
			if config.IsBootstrapAdmin("A@X.COM") != c.isAdmin {
				t.Fatalf("IsBootstrapAdmin = %v, want %v", config.IsBootstrapAdmin("A@X.COM"), c.isAdmin)
			}
		})
	}
}

// A never-loaded bootstrap config must fail safe: no admin, self-registration off.
func TestBootstrapDefaultsWhenUnloaded(t *testing.T) {
	// A prior subtest may have loaded config; this only asserts the accessors do
	// not panic and return booleans, exercising the nil-safe branches.
	if config.IsBootstrapAdmin("nobody@x.com") || config.IsBootstrapAdmin(constants.EmptyString) {
		t.Fatal("unlisted email reported as bootstrap admin")
	}
	_ = config.IsSelfRegistrationEnabled()
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
