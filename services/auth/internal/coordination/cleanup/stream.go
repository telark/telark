package cleanup

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/auth/internal/constants"
)

func NewStreamOps(
	client *xwareredis.StreamClient,
	resourceType string,
	maxLen int64,
	xclaimMinIdle time.Duration,
) *StreamOps {
	return &StreamOps{
		client:        client,
		streamKey:     streamKey(resourceType),
		dlqStream:     dlqStreamKey(resourceType),
		group:         constants.CleanupConsumerGroup,
		maxLen:        maxLen,
		xclaimMinIdle: xclaimMinIdle,
	}
}

func (s *StreamOps) EnsureGroup(ctx context.Context) error {
	return s.client.EnsureConsumerGroup(ctx, s.streamKey, s.group)
}

func (s *StreamOps) Enqueue(ctx context.Context, fields map[string]any) (string, error) {
	id, err := s.client.PublishWithMaxLen(ctx, s.streamKey, fields, s.maxLen)
	if err == nil || !isNoGroupError(err) {
		return id, err
	}
	if recoverErr := s.recoverGroup(ctx); recoverErr != nil {
		return constants.EmptyString, recoverErr
	}
	return s.client.PublishWithMaxLen(ctx, s.streamKey, fields, s.maxLen)
}

func (s *StreamOps) Read(ctx context.Context, consumer string) ([]redis.XMessage, error) {
	msgs, err := s.client.Consume(ctx, s.streamKey, s.group, consumer, readBatchCount, readBlockDuration)
	if err == nil || !isNoGroupError(err) {
		return msgs, err
	}
	if recoverErr := s.recoverGroup(ctx); recoverErr != nil {
		return nil, recoverErr
	}
	return s.client.Consume(ctx, s.streamKey, s.group, consumer, readBatchCount, readBlockDuration)
}

func (s *StreamOps) Ack(ctx context.Context, entryID string) error {
	return s.client.Ack(ctx, s.streamKey, s.group, entryID)
}

func (s *StreamOps) Reclaim(ctx context.Context, consumer string) ([]redis.XMessage, error) {
	return s.client.ClaimStale(ctx, s.streamKey, s.group, consumer, s.xclaimMinIdle, reclaimMaxCount)
}

func (s *StreamOps) PublishDLQ(ctx context.Context, fields map[string]any) error {
	_, err := s.client.PublishWithMaxLen(ctx, s.dlqStream, fields, constants.CleanupDLQMaxLen)
	return err
}

func (s *StreamOps) recoverGroup(ctx context.Context) error {
	timer := time.NewTimer(noGroupBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	return s.EnsureGroup(ctx)
}

func isNoGroupError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), noGroupSubstr)
}
