package core

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
)

func NewRedisManager() RedisManagerInterface {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisManager{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (rm *RedisManager) GetClient() (*RedisClient, error) {
	rm.mu.RLock()
	if rm.client != nil && rm.isConnected {
		rm.mu.RUnlock()
		return rm.client, nil
	}
	rm.mu.RUnlock()

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.client != nil && rm.isConnected {
		return rm.client, nil
	}

	client, err := rm.InitRedisClient()
	if err != nil {
		return nil, err
	}

	rm.client = client
	rm.isConnected = true
	return client, nil
}

func (rm *RedisManager) IsConnected() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.client == nil {
		return false
	}

	return rm.isConnected
}

func (rm *RedisManager) Close() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.client != nil && rm.client.Client != nil {
		if err := rm.client.Client.Close(); err != nil {
			return fmt.Errorf(string(ERROR_FAILED_CLOSE_REDIS_CLIENT), err)
		}
		rm.client = nil
		rm.isConnected = false
	}

	if rm.cancel != nil {
		rm.cancel()
	}

	return nil
}

func (rm *RedisManager) InitRedisClient() (*RedisClient, error) {
	var client *RedisClient
	var err error

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = 1 * time.Second
	expBackoff.MaxInterval = 30 * time.Second
	expBackoff.MaxElapsedTime = time.Duration(DefaultTimeout) * time.Second

	operation := func() error {
		client, err = InitClient(rm.ctx)
		if err != nil {
			return err
		}

		if err := client.Client.Ping(rm.ctx).Err(); err != nil {
			return fmt.Errorf(string(ERROR_REDIS_PING_ERROR), err)
		}

		return nil
	}

	if err := backoff.Retry(operation, expBackoff); err != nil {
		return nil, fmt.Errorf(string(ERROR_FAILED_INIT_REDIS_CLIENT), err)
	}

	return client, nil
}

func (rm *RedisManager) Reconnect() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.client != nil && rm.client.Client != nil {
		rm.client.Client.Close()
		rm.client = nil
		rm.isConnected = false
	}

	client, err := rm.InitRedisClient()
	if err != nil {
		return err
	}

	rm.client = client
	rm.isConnected = true
	return nil
}

func (rm *RedisManager) GetConnectionStatus() (bool, error) {
	if rm.client == nil || rm.client.Client == nil {
		return false, nil
	}

	err := rm.client.Client.Ping(rm.ctx).Err()
	return err == nil, err
}
