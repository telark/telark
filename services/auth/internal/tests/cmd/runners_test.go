package cmd

import (
	"testing"

	"github.com/telark/auth/internal/cmd/backfill"
	"github.com/telark/auth/internal/cmd/breakglass"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
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
