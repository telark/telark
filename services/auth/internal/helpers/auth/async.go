package auth

import (
	"sync"
	"time"

	"github.com/telark/auth/internal/constants"
)

const asyncWorkerDelta = 1

type asyncWorkerPool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

var asyncPool *asyncWorkerPool

var asyncLg = constants.GetLogger(constants.LoggerPrefixAsync)

func InitAsyncWorker() {
	asyncPool = &asyncWorkerPool{
		sem: make(chan struct{}, constants.RedisAsyncWorkerPoolSize),
	}
}

func Dispatch(fn func()) {
	if asyncPool == nil {
		return
	}
	select {
	case asyncPool.sem <- struct{}{}:
		asyncPool.wg.Add(asyncWorkerDelta)
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
			case <-time.After(constants.RedisAsyncWorkerTimeout):
				asyncLg.Warn(string(constants.WarnAsyncWorkerFailed))
			}
		}()
	default:
		asyncLg.Warn(string(constants.WarnAsyncWorkerFull))
	}
}

func DrainAsyncWorker() {
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
	case <-time.After(constants.RedisAsyncDrainTimeout):
		asyncLg.Warn(string(constants.WarnAsyncWorkerDrainTimeout))
	}
}
