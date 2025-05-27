package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/resources/common"
)

// MessageValidator interface for validating NATS messages
type MessageValidator interface {
	ValidateMessage(m *nats.Msg) error
}

// MessageProcessor interface for processing NATS messages
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, m *nats.Msg) error
}

// ResourceSubscriber interface for NATS subscribers
type ResourceSubscriber interface {
	Subscribe(nc *NATSClient) error
	HandleMessage(m *nats.Msg) error
	GetResourceType() common.Type
	MessageValidator
	MessageProcessor
}
