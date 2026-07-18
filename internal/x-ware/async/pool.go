package async

import (
	"context"
	"sync"
	"time"
)

const (
	warnTaskTimedOut  = "async task timed out"
	warnPoolFull      = "async pool full; task dropped"
	warnDrainTimedOut = "async drain timed out"
	asyncDelta        = 1
)

type Logger interface {
	Warn(message string)
}

type Config struct {
	Size         int
	TaskTimeout  time.Duration
	DrainTimeout time.Duration
	Logger       Logger
}

type Pool struct {
	config Config
	sem    chan struct{}
	wg     sync.WaitGroup
}

func New(config Config) *Pool {
	return &Pool{
		config: config,
		sem:    make(chan struct{}, config.Size),
	}
}

// Dispatch runs fn under a deadline in a pooled goroutine, dropping the task
// when the pool is full rather than queueing without bound.
func (p *Pool) Dispatch(fn func(context.Context)) {
	if p == nil {
		return
	}

	select {
	case p.sem <- struct{}{}:
		p.wg.Add(asyncDelta)
		go p.run(fn)
	default:
		p.warn(warnPoolFull)
	}
}

func (p *Pool) run(fn func(context.Context)) {
	defer p.wg.Done()
	defer func() { <-p.sem }()

	ctx, cancel := context.WithTimeout(context.Background(), p.config.TaskTimeout)
	defer cancel()

	fn(ctx)

	if ctx.Err() == context.DeadlineExceeded {
		p.warn(warnTaskTimedOut)
	}
}

// Drain waits for in-flight work, giving up after DrainTimeout so shutdown
// cannot hang on a task that ignores its context.
func (p *Pool) Drain() {
	if p == nil {
		return
	}

	drained := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
	case <-time.After(p.config.DrainTimeout):
		p.warn(warnDrainTimedOut)
	}
}

func (p *Pool) warn(message string) {
	if p.config.Logger != nil {
		p.config.Logger.Warn(message)
	}
}
