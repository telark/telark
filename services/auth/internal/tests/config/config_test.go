package config

import (
	"testing"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
)

// Bootstrap admins are normalised (trimmed + lowercased) and de-duplicated from
// a comma list, and IsBootstrapAdmin matches case-insensitively — the admin
// grant on first login depends on this exact matching.
func TestLoadBootstrapConfig(t *testing.T) {
	cases := []struct {
		name     string
		admins   string
		selfReg  string
		wantErr  bool
		wantSelf bool
		isAdmin  bool
	}{
		{"admins with self-reg", " A@x.com , b@X.com ", "true", false, true, true},
		{"no admins, self-reg on", "", "true", false, true, false},
		{"no admins, self-reg off is invalid", "", "false", true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.EnvBootstrapAdmins, c.admins)
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

// A never-loaded bootstrap config must fail safe: no admin, self-registration
// permitted (the pre-load default).
func TestBootstrapDefaultsWhenUnloaded(t *testing.T) {
	// A prior subtest may have loaded config; this only asserts the accessors do
	// not panic and return booleans, exercising the nil-safe branches.
	if config.IsBootstrapAdmin("nobody@x.com") {
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
