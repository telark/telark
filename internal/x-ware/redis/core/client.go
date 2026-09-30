package core

import (
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/x-ware/shared"
)

func getBasicConfig() (*BasicConfig, error) {
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

	return &BasicConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
	}, nil
}

func getBufferConfig() (*BufferConfig, error) {
	readBufferSize, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisReadBufferSize,
		DefaultValue: strconv.Itoa(DefaultReadBufferSize),
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	writeBufferSize, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisWriteBufferSize,
		DefaultValue: strconv.Itoa(DefaultWriteBufferSize),
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	return &BufferConfig{
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
	}, nil
}

func getPoolConfig() (*PoolConfig, error) {
	poolSize, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisPoolSize,
		DefaultValue: strconv.Itoa(DefaultPoolSize),
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	minIdleConns, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisMinIdleConns,
		DefaultValue: strconv.Itoa(DefaultMinIdleConns),
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	maxIdleConns, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisMaxIdleConns,
		DefaultValue: strconv.Itoa(DefaultMaxIdleConns),
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	return &PoolConfig{
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		MaxIdleConns: maxIdleConns,
	}, nil
}

func getTimeoutConfig() (*TimeoutConfig, error) {
	connMaxIdleTime, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisConnMaxIdleTime,
		DefaultValue: strconv.Itoa(DefaultConnMaxIdleTime),
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	connMaxLifetime, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          EnvRedisConnMaxLifetime,
		DefaultValue: strconv.Itoa(DefaultConnMaxLifetime),
		Required:     false,
	})
	if err != nil {
		return nil, err
	}

	return &TimeoutConfig{
		ConnMaxIdleTime: connMaxIdleTime,
		ConnMaxLifetime: connMaxLifetime,
	}, nil
}

func InitClient() (*RedisClient, error) {
	basicConfig, err := getBasicConfig()
	if err != nil {
		return nil, err
	}

	bufferConfig, err := getBufferConfig()
	if err != nil {
		return nil, err
	}

	poolConfig, err := getPoolConfig()
	if err != nil {
		return nil, err
	}

	timeoutConfig, err := getTimeoutConfig()
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(&redis.Options{
		Addr:                  fmt.Sprintf("%s:%s", basicConfig.Host, basicConfig.Port),
		Password:              basicConfig.Password,
		DB:                    basicConfig.DB,
		ReadBufferSize:        bufferConfig.ReadBufferSize,
		WriteBufferSize:       bufferConfig.WriteBufferSize,
		PoolSize:              poolConfig.PoolSize,
		MinIdleConns:          poolConfig.MinIdleConns,
		MaxIdleConns:          poolConfig.MaxIdleConns,
		ConnMaxIdleTime:       time.Duration(timeoutConfig.ConnMaxIdleTime) * time.Minute,
		ConnMaxLifetime:       time.Duration(timeoutConfig.ConnMaxLifetime) * time.Minute,
		ContextTimeoutEnabled: true,
		DialTimeout:           5 * time.Second,
		ReadTimeout:           3 * time.Second,
		WriteTimeout:          3 * time.Second,
		PoolTimeout:           4 * time.Second,
	})

	return &RedisClient{
		Client: client,
	}, nil
}
