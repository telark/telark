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
		ctx         context.Context //nolint:containedctx // Context is used for client lifecycle management
		cancel      context.CancelFunc
		isConnected bool
	}
)
