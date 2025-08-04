package core

import (
	"fmt"
	"strconv"

	"github.com/plsyro/x-ware/shared"
	"github.com/redis/go-redis/v9"
)

func InitClient() (*RedisClient, error) {
	host, err := shared.GetEnvString(shared.EnvConfig{
		Key:          EnvRedisHost,
		DefaultValue: DefaultHost,
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	port, err := shared.GetEnvString(shared.EnvConfig{
		Key:          EnvRedisPort,
		DefaultValue: DefaultPort,
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	password, err := shared.GetEnvString(shared.EnvConfig{
		Key:          EnvRedisPassword,
		DefaultValue: DefaultPassword,
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	db, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisDB,
		DefaultValue: strconv.Itoa(DefaultDB),
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
