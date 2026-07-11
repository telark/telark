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
		group             natscore.Group
		logger            any
		handlerCallback   func(*nats.Msg, natscore.Action) error
	}
)

func NewBaseSubscriber(group natscore.Group, resourceType shared.Type, logger any) *BaseSubscriber {
	return &BaseSubscriber{
		BaseSubscriber: natscore.BaseSubscriber{
			Group:          group,
			MaxRetries:     constants.DefaultMaxRetries,
			RetryDelay:     constants.RetryDelaySeconds * time.Second,
			ProcessTimeout: constants.ProcessTimeoutSeconds * time.Second,
		},
		resourceType: resourceType,
		group:        group,
		logger:       logger,
	}
}

func (s *BaseSubscriber) GetResourceType() shared.Type {
	return s.resourceType
}

func (s *BaseSubscriber) ProcessMessage(ctx context.Context, m *nats.Msg) error {
	_ = ctx // context currently unused
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
		return ""
	}
	return g.Resp.Message
}
