package async

import (
	"context"
	"time"

	xasync "github.com/telark/telark/internal/x-ware/async"
	"github.com/telark/telark/services/discovery/internal/constants"
)

const (
	poolSize     = 64
	taskTimeout  = 10 * time.Second
	drainTimeout = 5 * time.Second
	loggerPrefix = "Async: "
)

var asyncPool *xasync.Pool

func Init() {
	asyncPool = xasync.New(xasync.Config{
		Size:         poolSize,
		TaskTimeout:  taskTimeout,
		DrainTimeout: drainTimeout,
		Logger:       constants.GetLogger(loggerPrefix),
	})
}

// fn must honor ctx.Done(): the deadline cannot stop a function that ignores it.
func Dispatch(fn func(context.Context)) {
	asyncPool.Dispatch(fn)
}

func Drain() {
	asyncPool.Drain()
}
