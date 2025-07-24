package core

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
)

func InitNatsClient(ctx context.Context, user, password string) (*NATSClient, error) {
	if user == "" || password == "" {
		return nil, fmt.Errorf(string(errors.ERROR_NATS_AUTH))
	}

	// Set Config
	config := NATSConfig{
		User:     user,
		Password: password,
		Port:     CLIENT,
	}

	var client *NATSClient

	operation := func() error {
		var err error
		client, err = newClient(config)
		return err
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second // adjust as needed

	err := backoff.RetryNotify(operation, backoff.WithContext(expBackoff, ctx), func(err error, d time.Duration) {
		fmt.Printf(string(errors.ERROR_NATS_CONNECTION_FAILED), d, err)
	})
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_NATS_FAILED_CON), err)
	}
	return client, nil
}

func newClient(NatsConfig NATSConfig) (*NATSClient, error) {
	url := GetNATSClientUrl()

	// Connect to NATS
	nc, err := nats.Connect(url, nats.UserInfo(NatsConfig.User, NatsConfig.Password))
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_NATS_FAILED_CON), err)
	}

	// Create JetStream Context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf(string(errors.ERROR_NATS_CREATE_JETSTREAM_CONTEXT), err)
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
