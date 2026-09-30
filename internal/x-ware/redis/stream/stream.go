package stream

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/x-ware/constants"
)

type StreamClient struct {
	redis *redis.Client
}

func NewStreamClient(r *redis.Client) *StreamClient {
	return &StreamClient{redis: r}
}

func (c *StreamClient) EnsureConsumerGroup(ctx context.Context, stream, group string) error {
	err := c.redis.XGroupCreateMkStream(ctx, stream, group, streamStartID).Err()
	if err != nil && strings.Contains(strings.ToLower(err.Error()), busyGroupErr) {
		return nil
	}
	return err
}

func (c *StreamClient) Publish(ctx context.Context, stream string, fields map[string]any) (string, error) {
	return c.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: fields,
	}).Result()
}

func (c *StreamClient) PublishWithMaxLen(
	ctx context.Context,
	stream string,
	fields map[string]any,
	maxLen int64,
) (string, error) {
	return c.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: fields,
		MaxLen: maxLen,
		Approx: true,
	}).Result()
}

func (c *StreamClient) TrimMinID(ctx context.Context, stream, minID string) error {
	return c.redis.XTrimMinIDApprox(ctx, stream, minID, 0).Err()
}

func (c *StreamClient) Consume(
	ctx context.Context,
	stream, group, consumer string,
	count int64,
	blockDuration time.Duration,
) ([]redis.XMessage, error) {
	results, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, StreamConsumerGroupDeliveryID},
		Count:    count,
		Block:    blockDuration,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	if len(results) == constants.EmptySliceLength {
		return nil, nil
	}
	return results[constants.FirstIndex].Messages, nil
}

func (c *StreamClient) Ack(ctx context.Context, stream, group, messageID string) error {
	return c.redis.XAck(ctx, stream, group, messageID).Err()
}

func (c *StreamClient) DeleteConsumer(ctx context.Context, stream, group, consumer string) error {
	return c.redis.XGroupDelConsumer(ctx, stream, group, consumer).Err()
}

func (c *StreamClient) ListConsumers(ctx context.Context, stream, group string) ([]redis.XInfoConsumer, error) {
	return c.redis.XInfoConsumers(ctx, stream, group).Result()
}

func (c *StreamClient) ClaimStale(
	ctx context.Context,
	stream, group, consumer string,
	minIdle time.Duration,
	count int64,
) ([]redis.XMessage, error) {
	msgs, _, err := c.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    autoClaimStartID,
		Count:    count,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return msgs, nil
}
