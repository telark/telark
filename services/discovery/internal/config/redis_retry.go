package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/telark/discovery/internal/constants"
)

func RedisDialRetryInterval() time.Duration {
	return envDurationSec(constants.EnvRedisRetryIntervalSec,
		time.Duration(constants.DefaultRedisRetryIntervalSec)*time.Second)
}

func RedisDialMaxWait() time.Duration {
	return envRedisMaxWaitSec(constants.EnvRedisMaxWaitSec,
		time.Duration(constants.DefaultRedisMaxWaitSec)*time.Second)
}

func RedisPingTimeout() time.Duration {
	return envDurationSec(constants.EnvRedisPingTimeoutSec,
		time.Duration(constants.DefaultRedisPingTimeoutSec)*time.Second)
}

func envRedisMaxWaitSec(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if n == constants.RedisDialMaxWaitForeverSeconds {
		return constants.ZeroDuration
	}
	if n < constants.RedisDialMaxWaitForeverSeconds {
		return fallback
	}
	return time.Duration(n) * time.Second
}
