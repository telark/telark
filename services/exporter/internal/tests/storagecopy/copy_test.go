package storagecopy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/storagecopy"
)

const (
	fileName = "plan.json"
	body     = "{}"
	filePerm = 0o600
)

func TestRun(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cases := map[string]struct {
		pairs  func(t *testing.T) []string
		code   int
		copied int
	}{
		"no pair":     {pairs: func(*testing.T) []string { return nil }, code: constants.ExitCodeFailure},
		"half a pair": {pairs: func(t *testing.T) []string { return []string{t.TempDir()} }, code: constants.ExitCodeFailure},
		"a missing source": {
			pairs: func(t *testing.T) []string { return []string{missing, t.TempDir()} }, code: constants.ExitCodeFailure,
		},
		"two pairs": {
			pairs:  func(t *testing.T) []string { return []string{source(t), t.TempDir(), source(t), t.TempDir()} },
			copied: constants.CopyTreePairSize,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			pairs := c.pairs(t)
			if code := storagecopy.Run(pairs); code != c.code {
				t.Fatalf("Run = %d, want %d", code, c.code)
			}
			copied := constants.DefaultInitValue
			for index := constants.DefaultIncrementValue; index < len(pairs); index += constants.CopyTreePairSize {
				if _, err := os.Stat(filepath.Join(pairs[index], fileName)); err == nil {
					copied++
				}
			}
			if copied != c.copied {
				t.Fatalf("copied trees = %d, want %d", copied, c.copied)
			}
		})
	}
}

func source(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, fileName), []byte(body), filePerm); err != nil {
		t.Fatal(err)
	}
	return directory
}
