package init

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/x-ware/constants"
	"github.com/telark/x-ware/shared"
)

const (
	defaultRetryIntervalSeconds = 5
	defaultPingTimeoutSeconds   = 3
)

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

func NewClientWithRetry(
	ctx context.Context,
	dial func() (*redis.Client, error),
	cfg RetryConfig,
	lg shared.Logger,
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
	if ctx == nil || dial == nil {
		return nil
	}
	if cfg.RetryInterval <= constants.ZeroValue {
		cfg.RetryInterval = defaultRetryIntervalSeconds * time.Second
	}
	if cfg.PingTimeout <= constants.ZeroValue {
		cfg.PingTimeout = defaultPingTimeoutSeconds * time.Second
	}

	clientV = shared.ConnectWithRetry(ctx, shared.RetryPolicy[*redis.Client]{
		Dial:    dial,
		Healthy: func(c *redis.Client) bool { return pingWithTimeout(ctx, c, cfg.PingTimeout) == nil },
		OnFailure: func(c *redis.Client, err error) {
			if err == nil {
				_ = c.Close()
			}
		},
		Interval:   cfg.RetryInterval,
		MaxWait:    cfg.MaxWait,
		Log:        lg,
		MaxWaitMsg: string(constants.ErrRedisInitMaxWaitExceeded),
		RetryMsg:   string(constants.ErrRedisInitRetrying),
	})
	return clientV
}
