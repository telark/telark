package forcesync

import (
	"context"
	"strings"
	"time"

	"github.com/telark/discovery/config"
	"github.com/telark/discovery/constants"
	xwareredis "github.com/telark/x-ware/redis/stream"
	"github.com/redis/go-redis/v9"
)

func IsNoGroupError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), constants.ForceSyncNoGroupErrorSubstr)
}

func NewStreamOps(client *xwareredis.StreamClient, cfg config.ForceSyncConfig) *StreamOps {
	return &StreamOps{client: client, cfg: cfg}
}

func (s *StreamOps) EnsureGroup(ctx context.Context) error {
	return s.client.EnsureConsumerGroup(ctx, s.cfg.StreamKey, s.cfg.ConsumerGroup)
}

func (s *StreamOps) Enqueue(ctx context.Context, fields map[string]any) (string, error) {
	id, err := s.client.PublishWithMaxLen(ctx, s.cfg.StreamKey, fields, s.cfg.StreamMaxLen)
	if err == nil || !IsNoGroupError(err) {
		return id, err
	}
	if recoverErr := s.recoverGroup(ctx); recoverErr != nil {
		return constants.EmptyString, recoverErr
	}
	return s.client.PublishWithMaxLen(ctx, s.cfg.StreamKey, fields, s.cfg.StreamMaxLen)
}

func (s *StreamOps) Read(ctx context.Context, consumer string) ([]redis.XMessage, error) {
	msgs, err := s.client.Consume(
		ctx, s.cfg.StreamKey, s.cfg.ConsumerGroup,
		consumer, readBatchCount, readBlockDuration,
	)
	if err == nil || !IsNoGroupError(err) {
		return msgs, err
	}
	if recoverErr := s.recoverGroup(ctx); recoverErr != nil {
		return nil, recoverErr
	}
	return s.client.Consume(
		ctx, s.cfg.StreamKey, s.cfg.ConsumerGroup,
		consumer, readBatchCount, readBlockDuration,
	)
}

func (s *StreamOps) recoverGroup(ctx context.Context) error {
	backoff := time.NewTimer(constants.ForceSyncNoGroupBackoff)
	defer backoff.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-backoff.C:
	}
	return s.EnsureGroup(ctx)
}

func (s *StreamOps) Ack(ctx context.Context, entryID string) error {
	return s.client.Ack(ctx, s.cfg.StreamKey, s.cfg.ConsumerGroup, entryID)
}

func (s *StreamOps) Reclaim(ctx context.Context, consumer string) ([]redis.XMessage, error) {
	return s.client.ClaimStale(
		ctx,
		s.cfg.StreamKey,
		s.cfg.ConsumerGroup,
		consumer,
		s.cfg.PELIdleReclaim,
		reclaimMaxCount,
	)
}

func (s *StreamOps) TrimByAge(ctx context.Context, now time.Time) error {
	cutoff := now.Add(-s.cfg.AckRetention)
	minID := minIDForCutoff(cutoff)
	return s.client.TrimMinID(ctx, s.cfg.StreamKey, minID)
}
