package redis

import (
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	mu     sync.RWMutex
	client *redis.Client
)

func Set(c *redis.Client) {
	mu.Lock()
	client = c
	mu.Unlock()
}

func Get() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()
	return client
}
