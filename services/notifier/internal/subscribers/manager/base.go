package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/telark/data/errors"
	"github.com/telark/data/logger"
	"github.com/telark/data/messages"
	"github.com/telark/notifier/internal/constants"
	"github.com/telark/notifier/internal/subscribers/applications"
	natscore "github.com/telark/x-ware/nats/core"
	natsinit "github.com/telark/x-ware/nats/init"
	natstreams "github.com/telark/x-ware/nats/streams"
)

type Manager struct {
	natsManager natscore.NatsManagerInterface
	subscribers []natscore.ResourceSubscriber
	mu          sync.RWMutex
	ctx         context.Context //nolint:containedctx
	cancel      context.CancelFunc
}

type ctxSubscriber interface {
	SubscribeWithContext(context.Context, *natscore.NATSClient) error
	Drain()
}

func NewManager() *Manager {
	return NewManagerWith(natscore.NewNatsManager(), []natscore.ResourceSubscriber{
		applications.NewApplicationSubscriber(),
	})
}

// Injection seam: tests drive Start against an embedded NATS server.
func NewManagerWith(nm natscore.NatsManagerInterface, subs []natscore.ResourceSubscriber) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		natsManager: nm,
		subscribers: subs,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (m *Manager) Start() error {
	lg := logger.GetLogger(constants.PrefixManagerSubscriber)
	nc := natsinit.NewClientWithRetry(
		m.ctx,
		func() (*natscore.NATSClient, error) { return m.natsManager.GetClient() },
		natsinit.RetryConfig{
			RetryInterval: time.Duration(constants.NatsInitRetryIntervalSeconds) * time.Second,
			MaxWait:       time.Duration(constants.NatsInitMaxWaitSeconds) * time.Second,
		},
		lg,
	)
	if nc == nil {
		return fmt.Errorf(string(errors.ErrNatsConnectionFailed), errors.ErrNatsClientNotAvailable)
	}

	if nc.JetStream == nil {
		return fmt.Errorf("%s", errors.ErrNatsJetstreamNotInitialized)
	}

	if err := natstreams.CreateStreams(nc); err != nil {
		return fmt.Errorf(string(errors.ErrNatsFailedCreateStreams), err)
	}

	return m.startSubscribers(m.ctx, nc)
}

func (m *Manager) startSubscribers(parentCtx context.Context, nc *natscore.NATSClient) error {
	if nc == nil {
		return fmt.Errorf("%s", errors.ErrNatsClientNotAvailable)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(m.subscribers))

	for _, subscriber := range m.subscribers {
		wg.Go(func() {
			if cs, ok := any(subscriber).(ctxSubscriber); ok {
				if err := cs.SubscribeWithContext(parentCtx, nc); err != nil {
					errChan <- fmt.Errorf(string(errors.ErrNatsSubscriberManager), err)
				}
				return
			}
			if err := subscriber.Subscribe(nc); err != nil {
				errChan <- fmt.Errorf(string(errors.ErrNatsSubscriberManager), err)
			}
		})
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	logger.GetLogger(constants.PrefixManagerSubscriber).Info(string(messages.SuccessNatsAllSubscribersStarted))
	return nil
}

func (m *Manager) IsConnected() bool {
	if m.natsManager == nil {
		return false
	}
	return m.natsManager.IsConnected()
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	// Cancel stops the fetch loops, Drain acks what was already fetched, and
	// only then does the connection go away.
	for _, sub := range m.subscribers {
		if cs, ok := any(sub).(ctxSubscriber); ok {
			cs.Drain()
		}
	}

	if m.natsManager != nil {
		_ = m.natsManager.Close()
	}
}
