package init

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/plsyro/x-ware/constants"
	natscore "github.com/plsyro/x-ware/nats/core"
)

type logger interface {
	Error(string)
	Info(string)
}

var (
	clientMu sync.RWMutex
	clientV  *natscore.NATSClient

	bootstrap sync.Once
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
	return maxWait > 0 && time.Since(start) >= maxWait
}

func sleepRetry(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

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
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 5 * time.Second
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
