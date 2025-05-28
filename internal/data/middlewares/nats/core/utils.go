package core

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/resources/common"
)

func (s *BaseSubscriber) ValidateMessage(m *nats.Msg) error {
	if m == nil || len(m.Data) == 0 || m.Data == nil {
		return fmt.Errorf("%s", errors.ERROR_INVALID_MESSAGE)
	}
	return nil
}

func (s *BaseSubscriber) AcknowledgeMessage(ctx context.Context, m *nats.Msg) error {
	return m.Ack()
}

func NewMessage(topic, name, scope string, resourceType common.Type, data interface{}) *Message {
	return &Message{
		Topic:        topic,
		ResourceName: name,
		ResourceType: resourceType,
		Scope:        scope,
		Data:         data,
	}
}
