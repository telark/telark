package core

import (
	"context"
	"fmt"

	"github.com/plsyro/x-ware/shared"
	"github.com/redis/go-redis/v9"
)

func InitClient(ctx context.Context) (*RedisClient, error) {
	host, err := shared.GetEnvString(shared.EnvConfig{
		Key:          ENV_REDIS_HOST,
		DefaultValue: DEFAULT_HOST,
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	port, err := shared.GetEnvString(shared.EnvConfig{
		Key:          ENV_REDIS_PORT,
		DefaultValue: DEFAULT_PORT,
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	password, err := shared.GetEnvString(shared.EnvConfig{
		Key:          ENV_REDIS_PASSWORD,
		DefaultValue: DEFAULT_PASSWORD,
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	db, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          ENV_REDIS_DB,
		DefaultValue: fmt.Sprintf("%d", DEFAULT_DB),
		Required:     false,
	})
	if err != nil {
		return nil, err
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
