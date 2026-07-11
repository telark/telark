package redis

import (
	"context"
	"sync"

	redisv9 "github.com/redis/go-redis/v9"
	cb "github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	rediscore "github.com/telark/x-ware/redis/core"
	redisinit "github.com/telark/x-ware/redis/init"
)

var (
	redisClient *redisv9.Client
	redisMu     sync.RWMutex
)

func SetBootstrapReady() {
	redisinit.SetBootstrapReady()
}

func IsBootstrapReady() bool {
	return redisinit.IsBootstrapReady()
}

func ResetBootstrapReady() {
	redisinit.ResetBootstrapReady()
}

func NewRedisClient() *redisv9.Client {
	redisMu.RLock()
	defer redisMu.RUnlock()
	return redisClient
}

func buildRedisClient() (*redisv9.Client, error) {
	var wrapped *rediscore.RedisClient
	err := cb.ExecuteRedis(func() error {
		redisManager := rediscore.NewRedisManager()
		var clientErr error
		wrapped, clientErr = redisManager.GetClient()
		return clientErr
	})
	if err != nil {
		return nil, err
	}
	return wrapped.Client, nil
}

func NewRedisClientWithRetry(ctx context.Context) *redisv9.Client {
	lg := constants.GetLogger(constants.LoggerPrefixRedis)

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

	// Dial without holding the write lock so concurrent readers are not blocked
	// while the retry loop waits for Redis to come up. Last writer wins on
	// install; the rare duplicate-client cost is acceptable vs. multi-minute
	// readlock starvation during a Redis outage.
	dialed := redisinit.NewClientWithRetry(ctx, buildRedisClient, redisinit.RetryConfig{
		RetryInterval: retryInterval,
		MaxWait:       maxWait,
		PingTimeout:   pingTimeout,
	}, lg)

	redisMu.Lock()
	defer redisMu.Unlock()
	if redisClient != nil {
		return redisClient
	}
	redisClient = dialed
	return redisClient
}
