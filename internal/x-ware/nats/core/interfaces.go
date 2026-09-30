package core

import (
	"context"

	"github.com/nats-io/nats.go"
	resourceshared "github.com/telark/data/resources/shared"
)

type (
	MessageValidator interface {
		ValidateMessage(m *nats.Msg) error
	}
	MessageProcessor interface {
		ProcessMessage(ctx context.Context, m *nats.Msg) error
	}
	ResourceSubscriber interface {
		Subscribe(nc *NATSClient) error
		HandleMessage(m *nats.Msg) error
		GetResourceType() resourceshared.Type
		MessageValidator
		MessageProcessor
	}
	NatsManagerInterface interface {
		GetClient() (*NATSClient, error)
		IsConnected() bool
		Close() error
		Reconnect() error
		GetConnectionStatus() (bool, error)
	}
)
