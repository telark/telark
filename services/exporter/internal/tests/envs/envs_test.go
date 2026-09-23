package envs

import (
	"testing"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/managers/envs"
)

const wantRenderConcurrency = 4

// Env names are validated against an allow-list pattern, then the value is
// checked for presence and sanitized — a malformed name or an unset var is a
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
	if p := envs.InitSnapshotsPath(); p == constants.EmptyString {
		t.Fatal("InitSnapshotsPath is empty")
	}
	if envs.GetSnapshotsPath() == constants.EmptyString {
		t.Fatal("GetSnapshotsPath is empty")
	}
	if envs.InitSnapshotsMaxVersions() <= constants.DefaultInitValue {
		t.Fatalf("InitSnapshotsMaxVersions = %d, want > 0", envs.InitSnapshotsMaxVersions())
	}
}

// The render bound is read once at startup; a malformed value or one below
// the minimum keeps the default rather than lifting the bound.
func TestListRenderConcurrency(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"4", wantRenderConcurrency},
		{"0", constants.DefaultListRenderConcurrency},
		{"x", constants.DefaultListRenderConcurrency},
		{constants.EmptyString, constants.DefaultListRenderConcurrency},
	}
	for _, tc := range cases {
		t.Setenv(constants.ListRenderConcurrencyEnv, tc.raw)
		if got := envs.InitListRenderConcurrency(); got != tc.want || envs.GetListRenderConcurrency() != tc.want {
			t.Errorf("%q: InitListRenderConcurrency = %d, want %d", tc.raw, got, tc.want)
		}
	}
}
