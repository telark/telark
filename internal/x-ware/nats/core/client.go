package core

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/nats-io/nats.go"
	"github.com/plsyro/data/errors"
)

func InitClient(ctx context.Context, user, password string) (
	*NATSClient,
	error,
) {
	if user == "" || password == "" {
		return nil, fmt.Errorf("%s", errors.ErrNatsAuth)
	}

	config := natsConfig{
		User:     user,
		Password: password,
		Port:     Client,
	}

	var client *NATSClient

	operation := func() (*NATSClient, error) {
		return initJetStreamClient(config)
	}

	client, err := backoff.Retry(
		ctx, operation,
		backoff.WithMaxElapsedTime(
			time.Duration(DefaultConnectionTimeout)*time.Second,
		),
	)
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrNatsConnectionFailed), err)
	}
	return client, nil
}

func initJetStreamClient(natsConfig natsConfig) (*NATSClient, error) {
	url := GetNATSClientURL()

	nc, err := nats.Connect(url,
		nats.UserInfo(natsConfig.User, natsConfig.Password))
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrNatsConnectionFailed), err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf(string(errors.ErrNatsCreateJetstreamContext),
			err)
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
