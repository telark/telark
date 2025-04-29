package nats

import (
	"fmt"

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

func NewNATSClient(NatsConfig NATSConfig) (*nats.Conn, error) {
	url := fmt.Sprintf("nats://%s-nats-service:%d", global.BaseNamespace, NatsConfig.Port)

	client, err := nats.Connect(url, nats.UserInfo(NatsConfig.User, NatsConfig.Password))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errors.ERROR_NATS_CONNECTION, err)
	}

	return client, nil
}
