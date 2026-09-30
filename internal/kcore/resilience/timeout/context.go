package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/telark/kcore/constants"
)

func ContextWithTimeoutCause(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeoutCause(context.Background(), timeout, errors.New(string(constants.ErrTimeout)))
}
