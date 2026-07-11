package worker

import (
	"sync"

	"github.com/telark/kcore/constants"
)

type WorkerPool struct {
	workers int
	sem     chan struct{}
	wg      sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
	if workers <= constants.EmptySliceLength {
		workers = constants.DefaultWorkerPoolSize
	}
	return &WorkerPool{
		workers: workers,
		sem:     make(chan struct{}, workers),
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
		task()
	}()
}

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

func (wp *WorkerPool) Close() {
	close(wp.sem)
}
