package core

import (
	"context"
	"fmt"
	"time"
)

const (
	DeduplicationValue  = "1"
	DeduplicationPrefix = "dedup:"
	UnknownEventType    = "unknown"
)

type EventType interface {
	GetEventType() string
}

func CreateDeduplicationKey(key string, event any) string {
	if eventWithType, ok := event.(EventType); ok {
		return fmt.Sprintf("%s%s:%s", DeduplicationPrefix, key, eventWithType.GetEventType())
	}
	return key
}

func IsDuplicateEvent(ctx context.Context, client *RedisClient, key string, event any) (bool, error) {
	if client == nil || client.Client == nil {
		return false, fmt.Errorf(string(ERROR_REDIS_CLIENT_NOT_CONNECTED))
	}

	dedupKey := CreateDeduplicationKey(key, event)
	exists, err := client.Client.Exists(ctx, dedupKey).Result()
	if err != nil {
		return false, fmt.Errorf(string(ERROR_REDIS_EXISTS_ERROR), err)
	}

	return exists > 0, nil
}

func MarkEventAsProcessed(ctx context.Context, client *RedisClient, key string, event any, window time.Duration) error {
	if client == nil || client.Client == nil {
		return fmt.Errorf(string(ERROR_REDIS_CLIENT_NOT_CONNECTED))
	}

	dedupKey := CreateDeduplicationKey(key, event)
	err := client.Client.Set(ctx, dedupKey, DeduplicationValue, window).Err()
	if err != nil {
		return fmt.Errorf(string(ERROR_REDIS_SET_ERROR), err)
	}

	return nil
}

func GetEventType(event any) string {
	if eventWithType, ok := event.(EventType); ok {
		return eventWithType.GetEventType()
	}
	return UnknownEventType
}
