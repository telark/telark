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
	SubscribeWithContext(*natscore.NATSClient, context.Context) error
}

func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		natsManager: natscore.NewNatsManager(),
		subscribers: []natscore.ResourceSubscriber{
			applications.NewApplicationSubscriber(),
		},
		ctx:    ctx,
		cancel: cancel,
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

	if err := m.startSubscribers(m.ctx, nc); err != nil {
		logger.GetLogger(constants.PrefixManagerSubscriber).Error(fmt.Sprintf("%s: %v", errors.ErrNatsSubscriberManager, err))
	}

	logger.GetLogger(constants.PrefixNotifierService).Info(string(constants.InfoSubscribersStarted))
	return nil
}

func (m *Manager) startSubscribers(parentCtx context.Context, nc *natscore.NATSClient) error {
	if nc == nil {
		return fmt.Errorf("%s", errors.ErrNatsClientNotAvailable)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(m.subscribers))

	for _, subscriber := range m.subscribers {
		wg.Add(constants.DefaultAdd) //nolint:revive // WaitGroup pattern is intentional here
		go func(s natscore.ResourceSubscriber) {
			defer wg.Done()
			if cs, ok := any(s).(ctxSubscriber); ok {
				if err := cs.SubscribeWithContext(nc, parentCtx); err != nil {
					errChan <- fmt.Errorf(string(errors.ErrNatsSubscriberManager), err)
				}
				return
			}
			if err := s.Subscribe(nc); err != nil {
				errChan <- fmt.Errorf(string(errors.ErrNatsSubscriberManager), err)
			}
		}(subscriber)
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

	if m.natsManager != nil {
		_ = m.natsManager.Close()
	}
}
