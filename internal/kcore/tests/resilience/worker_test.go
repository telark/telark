package resilience

import (
	"sync/atomic"
	"testing"

	"github.com/telark/kcore/resilience/worker"
)

const singleWorker = 1

func TestPanickingTaskIsRecoveredAndFreesItsSlot(t *testing.T) {
	pool := worker.NewWorkerPool(singleWorker)

	pool.Submit(func() { panic("poison task") })
	pool.Wait()

	var ran atomic.Bool
	pool.Submit(func() { ran.Store(true) })
	pool.Wait()

	if !ran.Load() {
		t.Fatal("the slot of a panicking task was never released")
	}
}
