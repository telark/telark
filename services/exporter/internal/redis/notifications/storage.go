package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/telark/exporter/constants"
	exprdb "github.com/telark/exporter/redis"
	notiftypes "github.com/telark/exporter/types/notifications"
	"github.com/redis/go-redis/v9"
)

const (
	UnreadTTL    = 30 * 24 * time.Hour
	ReadTTL      = 7 * 24 * time.Hour
	DedupTTL     = 7 * 24 * time.Hour
	PerUserCap   = 500
	OpTimeout    = 3 * time.Second
	listMaxLimit = 200
	base10       = 10
	int64BitSize = 64
	fieldReadAt  = "readAt"
)

type Storage struct {
	rdb *redis.Client
}

var ErrRedisUnavailable = errors.New("notifications: redis client not available")

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
	if targetID != "" {
		existingID, err := s.rdb.Get(ctx, dedupKey(n.UserID, n.Type, targetID)).Result()
		if err == nil && existingID != "" {
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
	existing, err := s.rdb.HGetAll(ctx, itemK).Result()
	if err != nil {
		return nil, false, fmt.Errorf("hgetall existing: %w", err)
	}
	if len(existing) == constants.DefaultInitValue || existing[fieldReadAt] != constants.EmptyString {
		return nil, false, nil
	}

	now := time.Now().UTC()
	createdMs := now.UnixMilli()
	metaJSON, err := marshalMeta(n.Metadata)
	if err != nil {
		return nil, false, err
	}

	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, itemK, map[string]any{
		"title":     n.Title,
		"message":   n.Message,
		"severity":  n.Severity,
		"metadata":  metaJSON,
		"createdAt": createdMs,
	})
	pipe.Expire(ctx, itemK, UnreadTTL)
	pipe.ZAdd(ctx, itemsKey(n.UserID), redis.Z{Score: float64(createdMs), Member: id})
	if targetID := metaString(n.Metadata, notiftypes.MetaKeyTargetID); targetID != constants.EmptyString {
		pipe.Set(ctx, dedupKey(n.UserID, n.Type, targetID), id, DedupTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("dedup update pipeline: %w", err)
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
		"id":        id,
		"userId":    n.UserID,
		"type":      n.Type,
		"title":     n.Title,
		"message":   n.Message,
		"severity":  n.Severity,
		"metadata":  metaJSON,
		"createdAt": createdMs,
	})
	pipe.Expire(ctx, itemKey(id), UnreadTTL)
	pipe.ZAdd(ctx, itemsKey(n.UserID), redis.Z{Score: float64(createdMs), Member: id})
	if targetID != constants.EmptyString {
		pipe.Set(ctx, dedupKey(n.UserID, n.Type, targetID), id, DedupTTL)
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
	if limit > listMaxLimit {
		limit = listMaxLimit
	}

	maxScore := "+inf"
	if cursor != constants.EmptyString {
		maxScore = "(" + cursor
	}
	ids, err := s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     itemsKey(userID),
		Start:   "-inf",
		Stop:    maxScore,
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
	fields, err := s.rdb.HGetAll(ctx, itemK).Result()
	if err != nil {
		return fmt.Errorf("hgetall: %w", err)
	}
	if len(fields) == constants.DefaultInitValue || fields["userId"] != userID {
		return nil
	}
	if fields[fieldReadAt] != constants.EmptyString {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, itemK, fieldReadAt, now)
	pipe.Expire(ctx, itemK, ReadTTL)
	pipe.Decr(ctx, unreadCountKey(userID))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("markread pipeline: %w", err)
	}
	return nil
}

func (s *Storage) MarkAllRead(ctx context.Context, userID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	ids, err := s.rdb.ZRange(ctx, itemsKey(userID), 0, -1).Result()
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

	ids, err := s.rdb.ZRange(ctx, itemsKey(userID), 0, -1).Result()
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
	dedupPattern := fmt.Sprintf("%suser:%s:dedup:*", keyPrefix, userID)
	s.deleteByPattern(ctx, dedupPattern)
	return nil
}

func (s *Storage) deleteByPattern(ctx context.Context, pattern string) {
	iter := s.rdb.Scan(ctx, 0, pattern, 100).Iterator()
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
	createdMs, _ := strconv.ParseInt(fields["createdAt"], base10, int64BitSize)
	n := notiftypes.Notification{
		ID:        fields["id"],
		UserID:    fields["userId"],
		Type:      fields["type"],
		Title:     fields["title"],
		Message:   fields["message"],
		Severity:  fields["severity"],
		CreatedAt: time.UnixMilli(createdMs).UTC(),
	}
	if metaRaw := fields["metadata"]; metaRaw != constants.EmptyString {
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
