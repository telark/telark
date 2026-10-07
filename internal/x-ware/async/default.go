package async

import (
	"context"
	"time"

	"github.com/telark/telark/internal/data/logger"
)

const (
	defaultPoolSize     = 64
	defaultTaskTimeout  = 10 * time.Second
	defaultDrainTimeout = 5 * time.Second
	loggerPrefix        = "Async: "
)

var defaultPool *Pool

func Init() {
	defaultPool = New(Config{
		Size:         defaultPoolSize,
		TaskTimeout:  defaultTaskTimeout,
		DrainTimeout: defaultDrainTimeout,
		Logger:       logger.GetLogger(loggerPrefix),
	})
}

// fn must honor ctx.Done(): the deadline cannot stop a function that ignores it.
func Dispatch(fn func(context.Context)) {
	defaultPool.Dispatch(fn)
}

func Drain() {
	defaultPool.Drain()
}
