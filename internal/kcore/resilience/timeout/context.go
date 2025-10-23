package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/plsyro/kcore/constants"
)

func ContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func ContextWithTimeoutCause(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeoutCause(context.Background(), timeout, errors.New(string(constants.ErrTimeout)))
}
