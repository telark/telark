package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/common-pkg/global"
	"github.com/plsyro/data-pkg/errors"
)

type Port int

const (
	CLIENT     Port = 4222
	MONITORING Port = 8222
)

type NATSConfig struct {
	User     string
	Password string
	Port     Port
}

type NATSClient struct {
	Conn      *nats.Conn
	JetStream nats.JetStreamContext
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

	// Create streams for each scope
	scopes := []Scope{NAMESPACE, DEPLOYMENT}
	for _, scope := range scopes {
		streamName := fmt.Sprintf("%s_%s", global.BaseNamespace, scope)
		_, err = js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  []string{fmt.Sprintf("%s.%s.*", global.BaseNamespace, scope)},
			Storage:   nats.FileStorage,
			Retention: nats.WorkQueuePolicy,
			MaxAge:    24 * time.Hour,
		})
		if err != nil && err != nats.ErrStreamNameAlreadyInUse {
			nc.Close()
			return nil, fmt.Errorf("failed to create stream %s: %w", streamName, err)
		}
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
