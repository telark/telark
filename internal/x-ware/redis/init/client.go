package init

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/telark/x-ware/constants"
	"github.com/redis/go-redis/v9"
)

type logger interface {
	Error(string)
	Info(string)
}

var (
	clientMu sync.RWMutex
	clientV  *redis.Client

	bootstrap BootstrapState
)

func SetBootstrapReady() {
	bootstrap.SetReady()
}

func IsBootstrapReady() bool {
	return bootstrap.IsReady()
}

func ResetBootstrapReady() {
	bootstrap.Reset()
}

func Client() *redis.Client {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return clientV
}

func SetClient(c *redis.Client) {
	clientMu.Lock()
	defer clientMu.Unlock()
	clientV = c
}

func pingWithTimeout(ctx context.Context, client *redis.Client, timeout time.Duration) error {
	if client == nil {
		return context.Canceled
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return client.Ping(pingCtx).Err()
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
	dial func() (*redis.Client, error),
	cfg RetryConfig,
	lg logger,
) *redis.Client {
	clientMu.RLock()
	if clientV != nil {
		c := clientV
		clientMu.RUnlock()
		return c
	}
	clientMu.RUnlock()

	clientMu.Lock()
	defer clientMu.Unlock()

	if clientV != nil {
		return clientV
	}
	if ctx == nil {
		return nil
	}
	if dial == nil {
		return nil
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 5 * time.Second
	}
	if cfg.PingTimeout <= 0 {
		cfg.PingTimeout = 3 * time.Second
	}

	start := time.Now()
	for {
		if ctx.Err() != nil {
			return nil
		}
		if maxWaitExceeded(start, cfg.MaxWait) {
			if lg != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrRedisInitMaxWaitExceeded), int(cfg.MaxWait.Seconds())))
			}
			return nil
		}

		c, err := dial()
		if err != nil {
			if lg != nil {
				lg.Info(fmt.Sprintf(string(constants.ErrRedisInitRetrying),
					int(cfg.RetryInterval.Seconds()),
					int(time.Since(start).Seconds()),
				))
			}
			if !sleepRetry(ctx, cfg.RetryInterval) {
				return nil
			}
			continue
		}

		if err := pingWithTimeout(ctx, c, cfg.PingTimeout); err != nil {
			_ = c.Close()
			if lg != nil {
				lg.Info(fmt.Sprintf(string(constants.ErrRedisInitRetrying),
					int(cfg.RetryInterval.Seconds()),
					int(time.Since(start).Seconds()),
				))
			}
			if !sleepRetry(ctx, cfg.RetryInterval) {
				return nil
			}
			continue
		}

		clientV = c
		return clientV
	}
}
