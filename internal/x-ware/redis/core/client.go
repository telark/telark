package core

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

func InitClient(ctx context.Context) (*RedisClient, error) {
	host := os.Getenv(ENV_REDIS_HOST)
	if host == "" {
		host = DEFAULT_HOST
	}

	port := os.Getenv(ENV_REDIS_PORT)
	if port == "" {
		port = DEFAULT_PORT
	}

	password := os.Getenv(ENV_REDIS_PASSWORD)
	if password == "" {
		password = DEFAULT_PASSWORD
	}

	dbStr := os.Getenv(ENV_REDIS_DB)
	db := DEFAULT_DB
	if dbStr != "" {
		if dbInt, err := strconv.Atoi(dbStr); err == nil {
			db = dbInt
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	return &RedisClient{
		Client: client,
	}, nil
}
