package worker

import (
	"fmt"
	"sync"

	globallogger "github.com/telark/data/logger"
	"github.com/telark/kcore/constants"
)

var logger = globallogger.NewCustomLogger(constants.LoggerPrefixWorkerPool)

type WorkerPool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
	if workers <= constants.EmptySliceLength {
		workers = constants.DefaultWorkerPoolSize
	}
	return &WorkerPool{
		sem: make(chan struct{}, workers),
	}
}

func (wp *WorkerPool) Submit(task func()) {
	wp.wg.Add(constants.WorkerPoolAddCount)
	wp.sem <- struct{}{}

	go func() {
		defer func() {
			<-wp.sem
			wp.wg.Done()
		}()
		// One task's panic must not take down the process that submitted it.
		defer func() {
			if r := recover(); r != nil {
				logger.Error(fmt.Sprintf(string(constants.ErrWorkerTaskPanicked), r))
			}
		}()
		task()
	}()
}

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

func (wp *WorkerPool) Close() {
	close(wp.sem)
}
