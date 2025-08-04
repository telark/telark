package core

import (
	"context"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	resourceShared "github.com/plsyro/data-pkg/resources/shared"
)

type (
	MessageHandler func(*nats.Msg) error
	Port           int
	Group          string
	Action         string
	NATSClient     struct {
		Conn      *nats.Conn
		JetStream nats.JetStreamContext
	}
	NATSConfig struct {
		User     string
		Password string
		Port     Port
	}
	Message struct {
		Topic        string              `json:"topic"`
		ResourceName string              `json:"resourceName"`
		ResourceType resourceShared.Type `json:"resourceType"`
		Scope        string              `json:"scope"`
		Data         any                 `json:"data"`
	}
	BaseSubscriber struct {
		Group          Group
		MaxRetries     int
		RetryDelay     time.Duration
		ProcessTimeout time.Duration
	}
	NatsManager struct {
		client      *NATSClient
		mu          sync.RWMutex
		ctx         context.Context //nolint:containedctx // Context is used for client lifecycle management
		cancel      context.CancelFunc
		isConnected bool
	}
)
