package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/exporter/internal/constants"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
)

const (
	UnreadTTL    = 30 * 24 * time.Hour
	ReadTTL      = 7 * 24 * time.Hour
	DedupTTL     = 7 * 24 * time.Hour
	PerUserCap   = 500
	OpTimeout    = 3 * time.Second
	base10       = 10
	int64BitSize = 64
	scanBatch    = 100
	watchRetries = 5

	fieldID        = "id"
	fieldUserID    = "userId"
	fieldType      = "type"
	fieldTitle     = "title"
	fieldMessage   = "message"
	fieldSeverity  = "severity"
	fieldMetadata  = "metadata"
	fieldCreatedAt = "createdAt"
	fieldReadAt    = "readAt"

	// A ZSET range of 0..-1 is every member, whatever the set holds.
	rangeStart = 0
	rangeEnd   = -1

	scoreMax          = "+inf"
	scoreMin          = "-inf"
	scoreExclusive    = "("
	dedupKeyAllSuffix = "*"
)

type Storage struct {
	rdb *redis.Client
}

var ErrRedisUnavailable = errors.New("notifications: redis client not available")

var ErrNotificationNotFound = errors.New(string(constants.ErrNotificationNotFound))

func NewStorage() (*Storage, error) {
	rdb := exprdb.Get()
	if rdb == nil {
		return nil, ErrRedisUnavailable
	}
	return &Storage{rdb: rdb}, nil
}

func (s *Storage) Emit(ctx context.Context, n notiftypes.Notification) (*notiftypes.Notification, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	targetID := metaString(n.Metadata, notiftypes.MetaKeyTargetID)
	if targetID != constants.EmptyString {
		existingID, err := s.rdb.Get(ctx, dedupKey(n, targetID)).Result()
		if err == nil && existingID != constants.EmptyString {
			updated, ok, uerr := s.tryUpdateUnread(ctx, n, existingID)
			if uerr != nil {
				return nil, uerr
			}
			if ok {
				return updated, nil
			}
		} else if err != nil && !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("dedup lookup: %w", err)
		}
	}

	return s.createNew(ctx, n, targetID)
}

func (s *Storage) tryUpdateUnread(
	ctx context.Context,
	n notiftypes.Notification,
	id string,
) (*notiftypes.Notification, bool, error) {
	itemK := itemKey(id)
	now := time.Now().UTC()
	createdMs := now.UnixMilli()
	metaJSON, err := marshalMeta(n.Metadata)
	if err != nil {
		return nil, false, err
	}

	unread := false
	err = s.updateOwned(ctx, n.UserID, id, func(pipe redis.Pipeliner, fields map[string]string) {
		unread = fields[fieldReadAt] == constants.EmptyString
		if !unread {
			return
		}
		pipe.HSet(ctx, itemK, map[string]any{
			fieldTitle:     n.Title,
			fieldMessage:   n.Message,
			fieldSeverity:  n.Severity,
			fieldMetadata:  metaJSON,
			fieldCreatedAt: createdMs,
		})
		pipe.Expire(ctx, itemK, UnreadTTL)
		pipe.ZAdd(ctx, itemsKey(n.UserID), redis.Z{Score: float64(createdMs), Member: id})
		if targetID := metaString(n.Metadata, notiftypes.MetaKeyTargetID); targetID != constants.EmptyString {
			pipe.Set(ctx, dedupKey(n, targetID), id, DedupTTL)
		}
	})
	if errors.Is(err, ErrNotificationNotFound) || (err == nil && !unread) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("dedup update: %w", err)
	}

	updated := n
	updated.ID = id
	updated.CreatedAt = now
	return &updated, true, nil
}

func (s *Storage) createNew(
	ctx context.Context,
	n notiftypes.Notification,
	targetID string,
) (*notiftypes.Notification, error) {
	id := newID()
	now := time.Now().UTC()
	createdMs := now.UnixMilli()
	metaJSON, err := marshalMeta(n.Metadata)
	if err != nil {
		return nil, err
	}

	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, itemKey(id), map[string]any{
		fieldID:        id,
		fieldUserID:    n.UserID,
		fieldType:      n.Type,
		fieldTitle:     n.Title,
		fieldMessage:   n.Message,
		fieldSeverity:  n.Severity,
		fieldMetadata:  metaJSON,
		fieldCreatedAt: createdMs,
	})
	pipe.Expire(ctx, itemKey(id), UnreadTTL)
	pipe.ZAdd(ctx, itemsKey(n.UserID), redis.Z{Score: float64(createdMs), Member: id})
	if targetID != constants.EmptyString {
		pipe.Set(ctx, dedupKey(n, targetID), id, DedupTTL)
	}
	pipe.Incr(ctx, unreadCountKey(n.UserID))
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("emit pipeline: %w", err)
	}

	if err := s.enforceCap(ctx, n.UserID); err != nil {
		return nil, err
	}

	created := n
	created.ID = id
	created.CreatedAt = now
	return &created, nil
}

func (s *Storage) enforceCap(ctx context.Context, userID string) error {
	count, err := s.rdb.ZCard(ctx, itemsKey(userID)).Result()
	if err != nil {
		return fmt.Errorf("zcard: %w", err)
	}
	if count <= PerUserCap {
		return nil
	}
	excess := count - PerUserCap
	popped, err := s.rdb.ZPopMin(ctx, itemsKey(userID), excess).Result()
	if err != nil {
		return fmt.Errorf("zpopmin: %w", err)
	}
	for _, z := range popped {
		id, ok := z.Member.(string)
		if !ok {
			continue
		}
		readAt, _ := s.rdb.HGet(ctx, itemKey(id), fieldReadAt).Result()
		s.rdb.Del(ctx, itemKey(id))
		if readAt == constants.EmptyString {
			s.rdb.Decr(ctx, unreadCountKey(userID))
		}
	}
	return nil
}

func (s *Storage) List(
	ctx context.Context,
	userID string,
	limit int,
	cursor string,
) (*notiftypes.ListResponse, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	if limit <= constants.DefaultInitValue {
		limit = notiftypes.DefaultListLimit
	}
	if limit > notiftypes.MaxListLimit {
		limit = notiftypes.MaxListLimit
	}

	maxScore := scoreMax
	if cursor != constants.EmptyString {
		maxScore = scoreExclusive + cursor
	}
	// BYSCORE combined with REV requires Start to carry the max bound and Stop
	// the min — reversed from the ascending case. Passing them the ascending
	// way here made every query empty, since -inf is never >= +inf in REV mode.
	ids, err := s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     itemsKey(userID),
		Start:   maxScore,
		Stop:    scoreMin,
		ByScore: true,
		Rev:     true,
		Offset:  constants.DefaultInitValue,
		Count:   int64(limit),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrangebyscore: %w", err)
	}

	items := make([]notiftypes.Notification, constants.DefaultInitValue, len(ids))
	var lastScore int64
	for _, id := range ids {
		fields, err := s.rdb.HGetAll(ctx, itemKey(id)).Result()
		if err != nil || len(fields) == constants.DefaultInitValue {
			continue
		}
		n := hashToNotification(fields)
		items = append(items, n)
		lastScore = n.CreatedAt.UnixMilli()
	}

	unread, err := s.unreadCount(ctx, userID)
	if err != nil {
		return nil, err
	}

	nextCursor := constants.EmptyString
	if len(ids) == limit && lastScore != constants.DefaultInitValue {
		nextCursor = strconv.FormatInt(lastScore, base10)
	}

	return &notiftypes.ListResponse{
		Items:       items,
		NextCursor:  nextCursor,
		UnreadCount: unread,
	}, nil
}

func (s *Storage) MarkRead(ctx context.Context, userID, notificationID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	itemK := itemKey(notificationID)
	return s.updateOwned(ctx, userID, notificationID, func(pipe redis.Pipeliner, fields map[string]string) {
		if fields[fieldReadAt] != constants.EmptyString {
			return
		}
		pipe.HSet(ctx, itemK, fieldReadAt, time.Now().UTC().Format(time.RFC3339))
		pipe.Expire(ctx, itemK, ReadTTL)
		pipe.Decr(ctx, unreadCountKey(userID))
	})
}

func (s *Storage) Delete(ctx context.Context, userID, notificationID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	return s.updateOwned(ctx, userID, notificationID, func(pipe redis.Pipeliner, fields map[string]string) {
		pipe.Del(ctx, itemKey(notificationID))
		pipe.ZRem(ctx, itemsKey(userID), notificationID)
		if fields[fieldReadAt] == constants.EmptyString {
			pipe.Decr(ctx, unreadCountKey(userID))
		}
	})
}

// The unread count follows each item's read state, so a change reads the item under WATCH and
// retries when a concurrent change to it commits first; otherwise both would count it.
func (s *Storage) updateOwned(
	ctx context.Context,
	userID, id string,
	queue func(redis.Pipeliner, map[string]string),
) error {
	itemK := itemKey(id)
	for range watchRetries {
		err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
			fields, err := tx.HGetAll(ctx, itemK).Result()
			if err != nil {
				return fmt.Errorf("hgetall: %w", err)
			}
			if len(fields) == constants.DefaultInitValue || fields[fieldUserID] != userID {
				return ErrNotificationNotFound
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				queue(pipe, fields)
				return nil
			})
			return err
		}, itemK)
		if !errors.Is(err, redis.TxFailedErr) {
			return err
		}
	}
	return fmt.Errorf("update %s: %w", id, redis.TxFailedErr)
}

func (s *Storage) MarkAllRead(ctx context.Context, userID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	ids, err := s.rdb.ZRange(ctx, itemsKey(userID), rangeStart, rangeEnd).Result()
	if err != nil {
		return fmt.Errorf("zrange: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	pipe := s.rdb.TxPipeline()
	for _, id := range ids {
		readAt, _ := s.rdb.HGet(ctx, itemKey(id), fieldReadAt).Result()
		if readAt != constants.EmptyString {
			continue
		}
		pipe.HSet(ctx, itemKey(id), fieldReadAt, now)
		pipe.Expire(ctx, itemKey(id), ReadTTL)
	}
	pipe.Set(ctx, unreadCountKey(userID), constants.DefaultInitValue, constants.DefaultInitValue)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("markallread pipeline: %w", err)
	}
	return nil
}

func (s *Storage) Clear(ctx context.Context, userID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	ids, err := s.rdb.ZRange(ctx, itemsKey(userID), rangeStart, rangeEnd).Result()
	if err != nil {
		return fmt.Errorf("zrange: %w", err)
	}
	pipe := s.rdb.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, itemKey(id))
	}
	pipe.Del(ctx, itemsKey(userID))
	pipe.Del(ctx, unreadCountKey(userID))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("clear pipeline: %w", err)
	}
	dedupPattern := fmt.Sprintf("%suser:%s:dedup:%s", keyPrefix, userID, dedupKeyAllSuffix)
	s.deleteByPattern(ctx, dedupPattern)
	return nil
}

func (s *Storage) deleteByPattern(ctx context.Context, pattern string) {
	iter := s.rdb.Scan(ctx, constants.DefaultInitValue, pattern, scanBatch).Iterator()
	for iter.Next(ctx) {
		s.rdb.Del(ctx, iter.Val())
	}
}

func (s *Storage) unreadCount(ctx context.Context, userID string) (int, error) {
	val, err := s.rdb.Get(ctx, unreadCountKey(userID)).Result()
	if errors.Is(err, redis.Nil) {
		return constants.DefaultInitValue, nil
	}
	if err != nil {
		return constants.DefaultInitValue, fmt.Errorf("get unread: %w", err)
	}
	n, _ := strconv.Atoi(val)
	if n < constants.DefaultInitValue {
		return constants.DefaultInitValue, nil
	}
	return n, nil
}

func metaString(meta map[string]any, key string) string {
	if meta == nil {
		return constants.EmptyString
	}
	v, ok := meta[key]
	if !ok {
		return constants.EmptyString
	}
	s, ok := v.(string)
	if !ok {
		return constants.EmptyString
	}
	return s
}

func marshalMeta(meta map[string]any) (string, error) {
	if meta == nil {
		return constants.EmptyString, nil
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return constants.EmptyString, fmt.Errorf("marshal metadata: %w", err)
	}
	return string(b), nil
}

func hashToNotification(fields map[string]string) notiftypes.Notification {
	createdMs, _ := strconv.ParseInt(fields[fieldCreatedAt], base10, int64BitSize)
	n := notiftypes.Notification{
		ID:        fields[fieldID],
		UserID:    fields[fieldUserID],
		Type:      fields[fieldType],
		Title:     fields[fieldTitle],
		Message:   fields[fieldMessage],
		Severity:  fields[fieldSeverity],
		CreatedAt: time.UnixMilli(createdMs).UTC(),
	}
	if metaRaw := fields[fieldMetadata]; metaRaw != constants.EmptyString {
		var meta map[string]any
		if err := json.Unmarshal([]byte(metaRaw), &meta); err == nil {
			n.Metadata = meta
		}
	}
	if readAt := fields[fieldReadAt]; readAt != constants.EmptyString {
		if t, err := time.Parse(time.RFC3339, readAt); err == nil {
			tu := t.UTC()
			n.ReadAt = &tu
		}
	}
	return n
}

func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, OpTimeout)
}
