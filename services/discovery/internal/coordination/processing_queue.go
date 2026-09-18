package coordination

import (
	"container/heap"
	"context"
	"sync"

	"github.com/telark/discovery/internal/constants"
)

type processingTask struct {
	priority    int
	bypassSlots bool
	run         func(context.Context)
	done        chan struct{}
}

type taskHeap []*processingTask

func (h taskHeap) Len() int { return len(h) }
func (h taskHeap) Less(i, j int) bool {
	return h[i].priority > h[j].priority
}
func (h taskHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *taskHeap) Push(x any) {
	*h = append(*h, x.(*processingTask))
}

func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	lastIndex := n - constants.DefaultAddValue
	item := old[lastIndex]
	*h = old[:lastIndex]
	return item
}

type processingQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	tasks   taskHeap
	started bool
	slots   chan struct{}
}

var (
	queueOnce sync.Once
	queueInst *processingQueue
)

func getProcessingQueue() *processingQueue {
	queueOnce.Do(func() {
		q := &processingQueue{
			slots: make(chan struct{}, constants.CoordinationBackgroundProcessingSlots),
		}
		q.cond = sync.NewCond(&q.mu)
		heap.Init(&q.tasks)
		queueInst = q
	})
	return queueInst
}

func startProcessingQueue(ctx context.Context) {
	q := getProcessingQueue()
	q.mu.Lock()
	if q.started {
		q.mu.Unlock()
		return
	}
	q.started = true
	q.mu.Unlock()
	go func() {
		<-ctx.Done()
		q.cond.Broadcast()
	}()
	go q.dispatch(ctx)
}

func submitBackgroundTask(ctx context.Context, run func(context.Context)) {
	getProcessingQueue().submit(ctx, &processingTask{
		priority:    constants.DefaultInitValue,
		bypassSlots: false,
		run:         run,
	})
}

func (q *processingQueue) submit(ctx context.Context, task *processingTask) {
	if ctx.Err() != nil {
		if task.done != nil {
			close(task.done)
		}
		return
	}
	q.mu.Lock()
	heap.Push(&q.tasks, task)
	q.mu.Unlock()
	q.cond.Signal()
}

func (q *processingQueue) dispatch(ctx context.Context) {
	for {
		task := q.nextTask(ctx)
		if task == nil {
			return
		}
		q.runTask(ctx, task)
	}
}

func (q *processingQueue) nextTask(ctx context.Context) *processingTask {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.tasks.Len() == constants.DefaultInitValue {
		if ctx.Err() != nil {
			return nil
		}
		q.cond.Wait()
	}
	item := heap.Pop(&q.tasks)
	task, ok := item.(*processingTask)
	if !ok {
		return nil
	}
	return task
}

func (q *processingQueue) runTask(ctx context.Context, task *processingTask) {
	if task.bypassSlots {
		go executeTask(ctx, task, nil)
		return
	}
	// Acquired off the dispatch loop so bypass-priority tasks still run immediately
	// however many background workers hold slots.
	go func() {
		select {
		case q.slots <- struct{}{}:
			executeTask(ctx, task, q.slots)
		case <-ctx.Done():
			if task.done != nil {
				close(task.done)
			}
		}
	}()
}

func executeTask(ctx context.Context, task *processingTask, slots chan struct{}) {
	defer func() {
		if slots != nil {
			<-slots
		}
		if task.done != nil {
			close(task.done)
		}
	}()
	task.run(ctx)
}
