package concurrency

import (
	"sync"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
)

const contendingGoroutines = 50

func TestGetLockReturnsSameInstancePerKey(t *testing.T) {
	a := concurrency.GetLock("resource-1")
	b := concurrency.GetLock("resource-1")
	if a != b {
		t.Error("same key returned different locks")
	}
	c := concurrency.GetLock("resource-2")
	if a == c {
		t.Error("different keys shared a lock")
	}
}

func TestGetLockEmptyKeyIsEphemeral(t *testing.T) {
	// An empty key must never be shared, so each call gets a throwaway lock.
	first := concurrency.GetLock(constants.EmptyString)
	second := concurrency.GetLock(constants.EmptyString)
	if first == second {
		t.Error("empty key returned a shared lock")
	}
}

func TestGetLockActuallyLocks(t *testing.T) {
	lock := concurrency.GetLock("guarded")
	var counter int
	var wg sync.WaitGroup
	for range contendingGoroutines {
		wg.Go(func() {
			lock.Lock()
			counter++
			lock.Unlock()
		})
	}
	wg.Wait()
	if counter != contendingGoroutines {
		t.Errorf("counter = %d, want %d", counter, contendingGoroutines)
	}
}
