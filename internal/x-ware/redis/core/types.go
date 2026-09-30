package core

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

type (
	RedisClient struct {
		Client *redis.Client
	}
	RedisConfig struct {
		Host     string
		Port     string
		Password string
		DB       int
	}
	RedisManager struct {
		client      *RedisClient
		mu          sync.RWMutex
		ctx         context.Context //nolint:containedctx
		cancel      context.CancelFunc
		isConnected bool
	}
	BasicConfig struct {
		Host     string
		Port     string
		Password string
		DB       int
	}
	BufferConfig struct {
		ReadBufferSize  int
		WriteBufferSize int
	}
	PoolConfig struct {
		PoolSize     int
		MinIdleConns int
		MaxIdleConns int
	}
	TimeoutConfig struct {
		ConnMaxIdleTime int
		ConnMaxLifetime int
	}
)
