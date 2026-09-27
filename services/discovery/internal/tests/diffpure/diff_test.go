package diffpure

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	storedGeneration = 5
	lockAppName      = "app"
)

// A fresh history starts at generation 1 with an empty, non-nil change log and
// no drift.
func TestNewApplicationHistory(t *testing.T) {
	h := diff.NewApplicationHistory()
	testutil.Equal(t, "generation", h.Generation, constants.DefaultAddValue)
	testutil.Equal(t, "empty log", len(h.ChangeLog), constants.DefaultInitValue)
	testutil.Equal(t, "no drift", h.HasDrift, false)
}

// The last entry is nil for an empty log, otherwise the final element.
func TestLastChangeLogEntry(t *testing.T) {
	if diff.LastChangeLogEntry(diff.NewApplicationHistory()) != nil {
		t.Fatal("empty log should have no last entry")
	}
	h := appresource.ApplicationHistory{
		ChangeLog: []appresource.ChangeLogEntry{{Generation: constants.DefaultAddValue}, {Generation: constants.TwoValue}},
	}
	last := diff.LastChangeLogEntry(h)
	if last == nil || last.Generation != constants.TwoValue {
		t.Fatalf("last entry = %+v, want generation 2", last)
	}
}

// The current generation defaults to 1 for a missing or unset history, otherwise
// the stored value.
func TestCurrentGenerationOrDefault(t *testing.T) {
	testutil.Equal(t, "nil", diff.CurrentGenerationOrDefault(nil), constants.DefaultAddValue)
	testutil.Equal(t, "unset", diff.CurrentGenerationOrDefault(&appresource.Application{}), constants.DefaultAddValue)
	stored := &appresource.Application{History: appresource.ApplicationHistory{Generation: storedGeneration}}
	testutil.Equal(t, "set", diff.CurrentGenerationOrDefault(stored), storedGeneration)
}

// A nil client treats the lock as trivially acquired; a real client grants the
// lock once and refuses the second holder until release.
func TestGenProcessingLock(t *testing.T) {
	ctx := context.Background()
	if key, ok := diff.AcquireGenProcessingLock(ctx, nil, lockAppName, constants.DefaultAddValue); key != "" || !ok {
		t.Fatalf("nil client lock = %q,%v", key, ok)
	}
	diff.ReleaseGenProcessingLock(nil, "")

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	key, ok := diff.AcquireGenProcessingLock(ctx, rdb, lockAppName, constants.TwoValue)
	if key == constants.EmptyString || !ok {
		t.Fatalf("first acquire = %q,%v", key, ok)
	}
	if _, ok2 := diff.AcquireGenProcessingLock(ctx, rdb, lockAppName, constants.TwoValue); ok2 {
		t.Fatal("second acquire should fail while held")
	}
	diff.ReleaseGenProcessingLock(rdb, key)
}
