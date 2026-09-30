package shared

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/x-ware/constants"
)

type Logger interface {
	Error(string)
	Info(string)
}

type RetryPolicy[T any] struct {
	Dial       func() (T, error)
	Healthy    func(T) bool
	OnFailure  func(T, error)
	Interval   time.Duration
	MaxWait    time.Duration
	Log        Logger
	MaxWaitMsg string
	RetryMsg   string
}

// Returns the zero value of T when the context ends or MaxWait elapses; callers
// treat that as "not connected" rather than an error.
func ConnectWithRetry[T any](ctx context.Context, p RetryPolicy[T]) T {
	var zero T

	start := time.Now()
	for {
		if ctx.Err() != nil {
			return zero
		}
		if p.MaxWait > constants.ZeroValue && time.Since(start) >= p.MaxWait {
			p.logMaxWait()
			return zero
		}

		c, err := p.Dial()
		if err == nil && p.Healthy(c) {
			return c
		}

		p.OnFailure(c, err)
		p.logRetry(start)
		if !p.sleep(ctx) {
			return zero
		}
	}
}

func (p RetryPolicy[T]) sleep(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(p.Interval):
		return true
	}
}

func (p RetryPolicy[T]) logMaxWait() {
	if p.Log == nil {
		return
	}
	p.Log.Error(fmt.Sprintf(p.MaxWaitMsg, int(p.MaxWait.Seconds())))
}

func (p RetryPolicy[T]) logRetry(start time.Time) {
	if p.Log == nil {
		return
	}
	p.Log.Info(fmt.Sprintf(p.RetryMsg, int(p.Interval.Seconds()), int(time.Since(start).Seconds())))
}
