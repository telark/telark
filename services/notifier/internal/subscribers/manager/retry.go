package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/data/errors"
	"github.com/telark/notifier/internal/constants"
)

type retryLogger interface {
	Error(string)
	Warn(string)
}

// NewClientWithRetry gives up for good after NatsInitMaxWaitSeconds; without this outer loop
// a NATS outage leaves the service running with no subscribers until someone restarts it by hand.
func RetryStart(
	ctx context.Context,
	start func() error,
	interval time.Duration,
	lg retryLogger,
) error {
	if interval <= constants.DefaultInitValue {
		interval = constants.NatsStartRetrySeconds * time.Second
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := start()
		if err == nil {
			return nil
		}

		if lg != nil {
			lg.Error(fmt.Sprintf(string(errors.ErrNatsSubscriberManager), err))
			lg.Warn(fmt.Sprintf(string(constants.InfoNatsStartRetrying), int(interval.Seconds())))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
