package core

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
)

func InitClient(ctx context.Context, user, password string) (*NATSClient, error) {
	if user == "" || password == "" {
		return nil, fmt.Errorf("%s", errors.ErrNatsAuth)
	}

	config := NATSConfig{
		User:     user,
		Password: password,
		Port:     Client,
	}

	var client *NATSClient

	operation := func() error {
		var err error
		client, err = initJetStreamClient(config)
		return err
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = time.Duration(DefaultConnectionTimeout) * time.Second

	err := backoff.RetryNotify(operation, backoff.WithContext(expBackoff, ctx), func(err error, d time.Duration) {
		fmt.Printf(string(errors.ErrNatsConnectionFailedWithRetry), d, err)
	})
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrNatsConnectionFailed), err)
	}
	return client, nil
}

func initJetStreamClient(natsConfig NATSConfig) (*NATSClient, error) {
	url := GetNATSClientURL()

	nc, err := nats.Connect(url, nats.UserInfo(natsConfig.User, natsConfig.Password))
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrNatsConnectionFailed), err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf(string(errors.ErrNatsCreateJetstreamContext), err)
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
