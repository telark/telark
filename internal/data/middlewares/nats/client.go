package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
)

func NewNATSClient(NatsConfig NATSConfig) (*NATSClient, error) {
	url := GenerateNATSClientUrl()

	// Connect to NATS
	nc, err := nats.Connect(url, nats.UserInfo(NatsConfig.User, NatsConfig.Password))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errors.ERROR_NATS_FAILED_CON, err)
	}

	// Create JetStream Context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("%s: %w", errors.ERROR_NATS_CREATE_JETSTREAM_CONTEXT, err)
	}

	return &NATSClient{
		Conn:      nc,
		JetStream: js,
	}, nil
}

func (c *NATSClient) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}
