package rollbackctl

import (
	"context"
	"errors"
	"testing"

	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/handlers/rollback"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

// A blown process deadline is itself a common reason a rollback failed, so the
// caller's context is usually already done by the time the failure is recorded.
// Recording on that context patched nothing and the error was discarded, leaving
// the entry in_progress for the stale sweep to relabel.
func TestRecordWithRetryOutlivesCancelledCallerAndReturnsLastError(t *testing.T) {
	t.Parallel()
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
}

// The sweep is the last resort for entries nothing ever wrote back to. Replacing
// a reason that is already stored would destroy the only copy of it.
func TestStaleSweepErrorMsgKeepsRecordedReason(t *testing.T) {
	t.Parallel()
	testutil.Equal(t, "no reason stored",
		rollback.StaleSweepErrorMsg(constants.EmptyString), string(constants.ErrRollbackInterruptedRestart))
	testutil.Equal(t, "blank reason stored",
		rollback.StaleSweepErrorMsg("   "), string(constants.ErrRollbackInterruptedRestart))
	testutil.Equal(t, "reason already stored",
		rollback.StaleSweepErrorMsg("denied by admission webhook"), constants.EmptyString)
}
