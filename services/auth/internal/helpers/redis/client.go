package redis

import (
	"context"
	"sync"

	redisv9 "github.com/redis/go-redis/v9"
	rediscore "github.com/telark/telark/internal/x-ware/redis/core"
	redisinit "github.com/telark/telark/internal/x-ware/redis/init"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
)

var (
	redisClient *redisv9.Client
	redisMu     sync.RWMutex
)

func GetClient() *redisv9.Client {
	redisMu.RLock()
	defer redisMu.RUnlock()
	return redisClient
}

func buildRedisClient() (*redisv9.Client, error) {
	manager := rediscore.NewRedisManager()
	wrapped, err := manager.GetClient()
	if err != nil {
		return nil, err
	}
	return wrapped.Client, nil
}

var lg = constants.GetLogger(constants.LoggerPrefixRedis)

func NewRedisClientWithRetry(ctx context.Context) *redisv9.Client {
	redisMu.RLock()
	if redisClient != nil {
		c := redisClient
		redisMu.RUnlock()
		return c
	}
	redisMu.RUnlock()

	retryInterval := config.RedisDialRetryInterval()
	maxWait := config.RedisDialMaxWait()
	pingTimeout := config.RedisPingTimeout()

	redisMu.Lock()
	defer redisMu.Unlock()

	if redisClient != nil {
		return redisClient
	}

	redisClient = redisinit.NewClientWithRetry(ctx, buildRedisClient, redisinit.RetryConfig{
		RetryInterval: retryInterval,
		MaxWait:       maxWait,
		PingTimeout:   pingTimeout,
	}, lg)
	return redisClient
}
