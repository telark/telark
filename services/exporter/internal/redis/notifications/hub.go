package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const (
	// The insights stream's limit: every tab and device of one user, with room to spare.
	StreamsPerUser   = 8
	changedReadBlock = 5 * time.Second
	changedReadCount = 100
	changedReadRetry = 2 * time.Second
	// Read after this when the change stream is empty or absent: every entry is then new.
	changedStreamStart  = "0-0"
	changedStreamNewest = "+"
	changedStreamOldest = "-"
	newestOnly          = 1
)

var (
	lg = constants.GetLogger(constants.PrefixNotifications)

	ErrTooManyStreams = errors.New("notifications: too many open streams for this user")
	ErrHubClosed      = errors.New("notifications: stream hub closed")
)

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{rdb: rdb, subs: map[string]map[chan struct{}]struct{}{}}
}

// The channel holds one wake-up, so a burst of changes reaches the stream as one.
func (h *Hub) Subscribe(userID string) (<-chan struct{}, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case h.closed:
		return nil, nil, ErrHubClosed
	case h.rdb == nil:
		return nil, nil, ErrRedisUnavailable
	case len(h.subs[userID]) >= StreamsPerUser:
		return nil, nil, ErrTooManyStreams
	}
	ch := make(chan struct{}, constants.DefaultChannelBufferSize)
	if h.subs[userID] == nil {
		h.subs[userID] = map[chan struct{}]struct{}{}
	}
	h.subs[userID][ch] = struct{}{}
	return ch, func() { h.unsubscribe(userID, ch) }, nil
}

func (h *Hub) unsubscribe(userID string, ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs[userID], ch)
	if len(h.subs[userID]) == constants.DefaultInitValue {
		delete(h.subs, userID)
	}
}

// Ends every open stream, so a server shutdown doesn't wait for them.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for _, chans := range h.subs {
		for ch := range chans {
			close(ch)
		}
	}
	clear(h.subs)
}

// Reads the change stream until ctx ends, from its newest entry on: a stream refetches
// when it opens, so older entries tell it nothing. After a Redis error it resumes where
// it stopped and wakes every open stream once, since entries may have been trimmed meanwhile.
func (h *Hub) Run(ctx context.Context) {
	if h.rdb == nil {
		return
	}
	lastID := constants.EmptyString
	failed := false
	for ctx.Err() == nil {
		var err error
		if lastID == constants.EmptyString {
			lastID, err = h.newestID(ctx)
		}
		if err == nil {
			lastID, err = h.read(ctx, lastID)
		}
		switch {
		case err == nil && failed:
			failed = false
			h.wakeAll()
		case err != nil && ctx.Err() == nil:
			if !failed {
				lg.Warn(fmt.Sprintf(string(constants.WarnNotifStreamReadFailed), err))
			}
			failed = true
			wait(ctx, changedReadRetry)
		default:
		}
	}
}

func (h *Hub) newestID(ctx context.Context) (string, error) {
	newest, err := h.rdb.XRevRangeN(ctx, changedStream, changedStreamNewest, changedStreamOldest, newestOnly).Result()
	if err != nil {
		return constants.EmptyString, err
	}
	if len(newest) == constants.DefaultInitValue {
		return changedStreamStart, nil
	}
	return newest[constants.DefaultInitValue].ID, nil
}

func (h *Hub) read(ctx context.Context, after string) (string, error) {
	streams, err := h.rdb.XRead(ctx, &redis.XReadArgs{
		Streams: []string{changedStream, after},
		Count:   changedReadCount,
		Block:   changedReadBlock,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return after, nil
	}
	if err != nil {
		return after, err
	}
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			after = msg.ID
			if userID, ok := msg.Values[changedFieldUser].(string); ok {
				h.wake(userID)
			}
		}
	}
	return after, nil
}

func (h *Hub) wake(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[userID] {
		notify(ch)
	}
}

func (h *Hub) wakeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, chans := range h.subs {
		for ch := range chans {
			notify(ch)
		}
	}
}

func notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func wait(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
