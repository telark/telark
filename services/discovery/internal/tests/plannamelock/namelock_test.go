package plannamelock

import (
	"context"
	"errors"
	"testing"

	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const planName = "Prod Guard"

// The regression: two parallel creates with the same name both passed the list-then-create
// uniqueness check. The lock is keyed on the normalized name, so a case or padding variant
// of a name in flight is refused too, and it is free again once released.
func TestNameLocksSerializeSameName(t *testing.T) {
	locks := protection.NewNameLocks(nil)
	release, err := locks.Acquire(context.Background(), planName)
	testutil.Equal(t, "first acquire", err, nil)

	_, err = locks.Acquire(context.Background(), "  prod guard ")
	if !errors.Is(err, protection.ErrNameInFlight) {
		t.Fatalf("second acquire = %v, want ErrNameInFlight", err)
	}
	_, err = locks.Acquire(context.Background(), "Staging")
	testutil.Equal(t, "other name", err, nil)

	release()
	_, err = locks.Acquire(context.Background(), planName)
	testutil.Equal(t, "after release", err, nil)
}
