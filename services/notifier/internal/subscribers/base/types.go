package base

import (
	"context"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/resources/shared"
	"github.com/telark/notifier/internal/constants"
	"github.com/telark/rest/response"
	natscore "github.com/telark/x-ware/nats/core"
)

type (
	GenericResponseAdapter struct {
		Resp *response.GenericResponse
	}
	BaseSubscriber struct {
		natscore.BaseSubscriber
		processedMessages sync.Map
		resourceType      shared.Type
		handlerCallback   func(*nats.Msg, natscore.Action) error
		workers           []chan *nats.Msg
		fetchWG           sync.WaitGroup
		workerWG          sync.WaitGroup
		startOnce         sync.Once
		stopOnce          sync.Once
	}
)

func NewBaseSubscriber(group natscore.Group, resourceType shared.Type) *BaseSubscriber {
	workers := make([]chan *nats.Msg, applyWorkerCount())
	for i := range workers {
		workers[i] = make(chan *nats.Msg, constants.ApplyWorkerQueueSize)
	}
	return &BaseSubscriber{
		BaseSubscriber: natscore.BaseSubscriber{
			Group:          group,
			MaxRetries:     constants.DefaultMaxRetries,
			RetryDelay:     constants.RetryDelaySeconds * time.Second,
			ProcessTimeout: constants.ProcessTimeoutSeconds * time.Second,
		},
		resourceType: resourceType,
		workers:      workers,
	}
}

func (s *BaseSubscriber) GetResourceType() shared.Type {
	return s.resourceType
}

// ctx is part of the natscore.ResourceSubscriber contract; message handling is synchronous today.
func (s *BaseSubscriber) ProcessMessage(ctx context.Context, m *nats.Msg) error {
	_ = ctx
	return s.HandleMessage(m)
}

func (g *GenericResponseAdapter) GetStatus() int {
	if g.Resp == nil {
		return constants.DefaultInitValue
	}
	return g.Resp.Status
}

func (g *GenericResponseAdapter) GetMessage() string {
	if g.Resp == nil {
		return constants.EmptyString
	}
	return g.Resp.Message
}
