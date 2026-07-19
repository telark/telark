package init

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/telark/x-ware/constants"
	natscore "github.com/telark/x-ware/nats/core"
)

const defaultRetryIntervalSeconds = 5

type logger interface {
	Error(string)
	Info(string)
}

var (
	clientMu sync.RWMutex
	clientV  *natscore.NATSClient
)

func Client() *natscore.NATSClient {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return clientV
}

func SetClient(c *natscore.NATSClient) {
	clientMu.Lock()
	defer clientMu.Unlock()
	clientV = c
}

func isHealthy(c *natscore.NATSClient) bool {
	return c != nil && c.Conn != nil && c.Conn.IsConnected() && !c.Conn.IsClosed()
}

func maxWaitExceeded(start time.Time, maxWait time.Duration) bool {
	return maxWait > constants.ZeroValue && time.Since(start) >= maxWait
}

func sleepRetry(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

//nolint:gocyclo // connection retry is an inherent state machine; splitting hurts readability
func NewClientWithRetry(
	ctx context.Context,
	dial func() (*natscore.NATSClient, error),
	cfg RetryConfig,
	lg logger,
) *natscore.NATSClient {
	clientMu.RLock()
	if isHealthy(clientV) {
		c := clientV
		clientMu.RUnlock()
		return c
	}
	clientMu.RUnlock()

	clientMu.Lock()
	defer clientMu.Unlock()

	if isHealthy(clientV) {
		return clientV
	}
	if ctx == nil || dial == nil {
		return nil
	}
	if cfg.RetryInterval <= constants.ZeroValue {
		cfg.RetryInterval = defaultRetryIntervalSeconds * time.Second
	}

	start := time.Now()
	for {
		if ctx.Err() != nil {
			return nil
		}
		if maxWaitExceeded(start, cfg.MaxWait) {
			if lg != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrNatsInitMaxWaitExceeded), int(cfg.MaxWait.Seconds())))
			}
			return nil
		}

		c, err := dial()
		if err == nil && isHealthy(c) {
			clientV = c
			return clientV
		}

		if c != nil {
			c.Close()
		}
		if lg != nil {
			lg.Info(fmt.Sprintf(
				string(constants.ErrNatsInitRetrying),
				int(cfg.RetryInterval.Seconds()),
				int(time.Since(start).Seconds()),
			))
		}
		if !sleepRetry(ctx, cfg.RetryInterval) {
			return nil
		}
	}
}
