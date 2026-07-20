package concurrency

import (
	"sync"
	"testing"

	"github.com/telark/exporter/internal/utils/concurrency"
)

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
	if concurrency.GetLock("") == concurrency.GetLock("") {
		t.Error("empty key returned a shared lock")
	}
}

func TestGetLockActuallyLocks(t *testing.T) {
	lock := concurrency.GetLock("guarded")
	var counter int
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock.Lock()
			counter++
			lock.Unlock()
		}()
	}
	wg.Wait()
	if counter != 50 {
		t.Errorf("counter = %d, want 50", counter)
	}
}
