package init

import (
	"context"
	"sync"
	"time"

	"github.com/telark/x-ware/constants"
	natscore "github.com/telark/x-ware/nats/core"
	"github.com/telark/x-ware/shared"
)

const defaultRetryIntervalSeconds = 5

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

func NewClientWithRetry(
	ctx context.Context,
	dial func() (*natscore.NATSClient, error),
	cfg RetryConfig,
	lg shared.Logger,
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

	c := shared.ConnectWithRetry(ctx, shared.RetryPolicy[*natscore.NATSClient]{
		Dial:    dial,
		Healthy: isHealthy,
		OnFailure: func(c *natscore.NATSClient, _ error) {
			if c != nil {
				c.Close()
			}
		},
		Interval:   cfg.RetryInterval,
		MaxWait:    cfg.MaxWait,
		Log:        lg,
		MaxWaitMsg: string(constants.ErrNatsInitMaxWaitExceeded),
		RetryMsg:   string(constants.ErrNatsInitRetrying),
	})
	// Giving up must leave a stale-but-unhealthy clientV in place.
	if c == nil {
		return nil
	}
	clientV = c
	return clientV
}
