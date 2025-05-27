package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/common-pkg/global"
	"github.com/plsyro/data-pkg/errors"
)

type NATSClient struct {
	Conn          *nats.Conn
	JetStream     nats.JetStreamContext
	StreamManager *StreamManager
}

func NewNATSClient(NatsConfig NATSConfig) (*NATSClient, error) {
	url := fmt.Sprintf("nats://%s-nats-service:%d", global.BaseNamespace, NatsConfig.Port)

	// Connect to NATS
	nc, err := nats.Connect(url, nats.UserInfo(NatsConfig.User, NatsConfig.Password))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errors.ERROR_NATS_CONNECTION, err)
	}

	// Create JetStream Context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("%s: %w", errors.ERROR_NATS_CONNECTION, err)
	}

	// Create StreamManager
	streamManager := NewStreamManager(js)

	return &NATSClient{
		Conn:          nc,
		JetStream:     js,
		StreamManager: streamManager,
	}, nil
}

func (c *NATSClient) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}
