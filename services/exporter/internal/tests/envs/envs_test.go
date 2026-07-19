package envs

import (
	"testing"

	"github.com/telark/exporter/internal/managers/envs"
)

// Env names are validated against an allow-list pattern, then the value is
// checked for presence and sanitised — a malformed name or an unset var is a
// hard error, never a silent empty.
func TestLoadAndValidateEnv(t *testing.T) {
	t.Run("valid set var", func(t *testing.T) {
		t.Setenv("TEST_VAR", "value")
		got, err := envs.LoadAndValidateEnv("TEST_VAR")
		if err != nil || got != "value" {
			t.Fatalf("LoadAndValidateEnv = (%q, %v), want (value, nil)", got, err)
		}
	})
	t.Run("invalid name rejected", func(t *testing.T) {
		if _, err := envs.LoadAndValidateEnv("bad-name!"); err == nil {
			t.Fatal("malformed env name accepted")
		}
	})
	t.Run("unset var rejected", func(t *testing.T) {
		if _, err := envs.LoadAndValidateEnv("DEFINITELY_UNSET_VAR_XYZ"); err == nil {
			t.Fatal("unset env var accepted")
		}
	})
}

// The snapshots path + version config resolve to non-empty/positive defaults so
// the exporter never writes snapshots to an empty path.
func TestSnapshotsConfig(t *testing.T) {
	if p := envs.InitSnapshotsPath(); p == "" {
		t.Fatal("InitSnapshotsPath is empty")
	}
	if envs.GetSnapshotsPath() == "" {
		t.Fatal("GetSnapshotsPath is empty")
	}
	if envs.InitSnapshotsMaxVersions() <= 0 {
		t.Fatalf("InitSnapshotsMaxVersions = %d, want > 0", envs.InitSnapshotsMaxVersions())
	}
}
