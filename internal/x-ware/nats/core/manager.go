package core

import (
	"context"
	"fmt"
	"os"
)

func NewNatsManager() NatsManagerInterface {
	ctx, cancel := context.WithCancel(context.Background())
	return &NatsManager{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (nm *NatsManager) GetClient() (*NATSClient, error) {
	nm.mu.RLock()
	if nm.client != nil && nm.isConnected {
		nm.mu.RUnlock()
		return nm.client, nil
	}
	nm.mu.RUnlock()

	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.client != nil && nm.isConnected {
		return nm.client, nil
	}

	client, err := nm.InitNatsClient()
	if err != nil {
		return nil, err
	}

	nm.client = client
	nm.isConnected = true
	return client, nil
}

func (nm *NatsManager) IsConnected() bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	if nm.client == nil {
		return false
	}

	return nm.isConnected && nm.client.Conn.IsConnected()
}

func (nm *NatsManager) Close() error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.client != nil {
		nm.client.Close()
		nm.client = nil
		nm.isConnected = false
	}

	if nm.cancel != nil {
		nm.cancel()
	}

	return nil
}

func (nm *NatsManager) InitNatsClient() (*NATSClient, error) {
	user := os.Getenv(ENV_NATS_USER)
	password := os.Getenv(ENV_NATS_PASSWORD)

	if user == "" {
		return nil, fmt.Errorf(string(ERROR_NATS_USER_REQUIRED))
	}
	if password == "" {
		return nil, fmt.Errorf(string(ERROR_NATS_PASSWORD_REQUIRED))
	}

	client, err := InitClient(nm.ctx, user, password)
	if err != nil {
		return nil, fmt.Errorf(string(ERROR_FAILED_INIT_NATS_CLIENT), err)
	}

	return client, nil
}

func (nm *NatsManager) Reconnect() error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.client != nil {
		nm.client.Close()
		nm.client = nil
		nm.isConnected = false
	}

	client, err := nm.InitNatsClient()
	if err != nil {
		return err
	}

	nm.client = client
	nm.isConnected = true
	return nil
}

func (nm *NatsManager) GetConnectionStatus() (bool, error) {
	if !nm.IsConnected() {
		return false, fmt.Errorf(string(ERROR_NATS_CLIENT_NOT_CONNECTED))
	}
	return true, nil
}
