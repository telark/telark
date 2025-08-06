package core

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v5"
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
			return fmt.Errorf(string(ErrFailedCloseRedisClient), err)
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
	operation := func() (*RedisClient, error) {
		client, err := InitClient()
		if err != nil {
			return nil, err
		}

		if err := client.Client.Ping(rm.ctx).Err(); err != nil {
			return nil, fmt.Errorf(string(ErrRedisPingError), err)
		}

		return client, nil
	}

	client, err := backoff.Retry(
		rm.ctx, operation,
		backoff.WithMaxElapsedTime(time.Duration(DefaultTimeout)*time.Second))
	if err != nil {
		return nil, fmt.Errorf(string(ErrFailedInitRedisClient), err)
	}

	return client, nil
}

func (rm *RedisManager) Reconnect() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.client != nil && rm.client.Client != nil {
		if err := rm.client.Client.Close(); err != nil {
			return fmt.Errorf(string(ErrFailedCloseRedisClient), err)
		}
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

// GetPoolStats returns detailed connection pool statistics
func (rm *RedisManager) GetPoolStats() (*PoolStats, error) {
	if rm.client == nil || rm.client.Client == nil {
		return nil, fmt.Errorf(string(ErrStringFormat), ErrRedisClientNotConnected)
	}

	stats := rm.client.Client.PoolStats()
	return &PoolStats{
		TotalConns: stats.TotalConns,
		IdleConns:  stats.IdleConns,
		StaleConns: stats.StaleConns,
		WaitCount:  stats.WaitCount,
	}, nil
}

func (rm *RedisManager) HealthCheck(ctx context.Context) error {
	if rm.client == nil || rm.client.Client == nil {
		return fmt.Errorf(string(ErrStringFormat), ErrRedisClientNotConnected)
	}

	// Perform a ping to check basic connectivity
	if err := rm.client.Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf(string(ErrRedisHealthCheckPingError), err)
	}

	// Check if we can perform basic operations
	if err := rm.client.Client.Set(ctx, healthCheckKey, "ok", time.Second).Err(); err != nil {
		return fmt.Errorf(string(ErrRedisHealthCheckSetError), err)
	}

	if err := rm.client.Client.Del(ctx, healthCheckKey).Err(); err != nil {
		return fmt.Errorf(string(ErrRedisHealthCheckDelError), err)
	}

	return nil
}

func (rm *RedisManager) GetConnectionInfo() (*ConnectionInfo, error) {
	if rm.client == nil || rm.client.Client == nil {
		return nil, fmt.Errorf(string(ErrStringFormat), ErrRedisClientNotConnected)
	}

	clientInfo, err := rm.client.Client.ClientInfo(rm.ctx).Result()
	if err != nil {
		return nil, fmt.Errorf(string(ErrRedisHealthCheckGetClientInfoError), err)
	}

	clientID, err := rm.client.Client.ClientID(rm.ctx).Result()
	if err != nil {
		return nil, fmt.Errorf(string(ErrRedisHealthCheckGetClientIDError), err)
	}

	clientName, err := rm.client.Client.ClientGetName(rm.ctx).Result()
	if err != nil {
		clientName = "" // Client name might not be set
	}

	return &ConnectionInfo{
		ClientName:              clientName,
		ClientID:                clientID,
		ConnectedAt:             int64(clientInfo.Age.Seconds()),
		LastCommandAt:           int64(clientInfo.Idle.Seconds()),
		Database:                clientInfo.DB,
		Flags:                   fmt.Sprintf("%v", clientInfo.Flags),
		Subscriptions:           clientInfo.Sub,
		PatternSubscriptions:    clientInfo.PSub,
		Channels:                DefaultInitValue,
		BlockedCommands:         DefaultInitValue,
		BlockedCommandsDuration: DefaultInitValue,
	}, nil
}
