package async

import (
	"context"
	"sync"
	"time"

	"github.com/telark/discovery/internal/constants"
)

const (
	poolSize     = 64
	taskTimeout  = 10 * time.Second
	drainTimeout = 5 * time.Second
	asyncDelta   = 1
	loggerPrefix = "Async: "
)

type pool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

var (
	asyncPool *pool
	asyncLg   = constants.GetLogger(loggerPrefix)
)

func Init() {
	asyncPool = &pool{
		sem: make(chan struct{}, poolSize),
	}
}

// Dispatch runs fn under a 10s deadline inside the bounded async pool. fn must
// honor ctx.Done() so it returns when the envelope expires; otherwise the
// goroutine would outlive its slot and leak.
func Dispatch(fn func(context.Context)) {
	if asyncPool == nil {
		return
	}
	select {
	case asyncPool.sem <- struct{}{}:
		asyncPool.wg.Add(asyncDelta)
		go func() {
			defer asyncPool.wg.Done()
			defer func() { <-asyncPool.sem }()

			ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
			defer cancel()
			fn(ctx)
			if ctx.Err() == context.DeadlineExceeded {
				asyncLg.Warn("async task timed out")
			}
		}()
	default:
		asyncLg.Warn("async pool full; task dropped")
	}
}

func Drain() {
	if asyncPool == nil {
		return
	}
	drained := make(chan struct{})
	go func() {
		asyncPool.wg.Wait()
		close(drained)
	}()
	select {
	case <-drained:
	case <-time.After(drainTimeout):
		asyncLg.Warn("async drain timed out")
	}
}
