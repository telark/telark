package async

import (
	"sync"
	"time"

	"github.com/telark/exporter/internal/constants"
)

const (
	poolSize        = 64
	taskTimeout     = 10 * time.Second
	drainTimeout    = 5 * time.Second
	asyncDelta      = 1
	loggerPrefix    = "Async: "
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

func Dispatch(fn func()) {
	if asyncPool == nil {
		return
	}
	select {
	case asyncPool.sem <- struct{}{}:
		asyncPool.wg.Add(asyncDelta)
		go func() {
			defer asyncPool.wg.Done()
			defer func() { <-asyncPool.sem }()

			done := make(chan struct{})
			go func() {
				defer close(done)
				fn()
			}()

			select {
			case <-done:
			case <-time.After(taskTimeout):
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
