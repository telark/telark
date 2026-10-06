package rollbackctl

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/handlers/rollback"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

// A blown process deadline is itself a common reason a rollback failed, so the
// caller's context is usually already done by the time the failure is recorded.
// Recording on that context patched nothing and the error was discarded, leaving
// the entry in_progress for the stale sweep to relabel.
func TestRecordWithRetryOutlivesCanceledCallerAndReturnsLastError(t *testing.T) {
	t.Parallel()
	// The bubble's fake clock runs the real retry intervals without waiting them out.
	synctest.Test(t, func(t *testing.T) {
		canceled, cancel := context.WithCancel(context.Background())
		cancel()

		attempts := constants.DefaultInitValue
		live := true
		wantErr := errors.New("patch rejected")

		err := rollback.RecordWithRetry(canceled, func(recordCtx context.Context) error {
			attempts++
			if recordCtx.Err() != nil {
				live = false
			}
			return wantErr
		})

		testutil.Equal(t, "record ran on a live context", live, true)
		testutil.Equal(t, "attempts", attempts, constants.RollbackRetryMaxAttempts)
		testutil.Equal(t, "last error returned for logging", errors.Is(err, wantErr), true)
	})
}

// The sweep is the last resort for entries nothing ever wrote back to. Replacing
// a reason that is already stored would destroy the only copy of it.
func TestFailStaleInProgressKeepsRecordedReason(t *testing.T) {
	const recorded = "denied by admission webhook"
	restart := string(constants.ErrRollbackInterruptedRestart)
	cases := []struct{ name, stored, want string }{
		{"no reason stored", constants.EmptyString, restart},
		{"blank reason stored", "   ", restart},
		{"reason already stored", recorded, recorded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, spec := installApplication(t, application.RollbackEntry{
				ID:          rollbackID,
				TriggeredAt: time.Now().Add(-staleAge),
				Status:      constants.RollbackStatusInProgress,
				Error:       c.stored,
			})
			if _, err := rollback.NewController(nil).FailStaleInProgress(context.Background(), shopApp, spec); err != nil {
				t.Fatal(err)
			}
			testutil.Equal(t, "stored reason", storedRollback(t, client).Error, c.want)
		})
	}
}
