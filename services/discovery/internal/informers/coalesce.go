package informers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type coalesceRedisPayload struct {
	Deadline int64                      `json:"dl"`
	Entries  map[string]json.RawMessage `json:"e"`
}

type coalescer struct {
	mu            sync.Mutex
	window        time.Duration
	maxWait       time.Duration // maximum time before a flush is forced regardless of new events
	maxEntries    int
	rdb           *redis.Client
	ctxFn         func() context.Context
	leaderFn      func(context.Context) bool
	buf           map[string]map[string]*unstructured.Unstructured
	bufFirstEvent map[string]time.Time // time of the first event in each pending window
	timers        map[string]*time.Timer
	flush         func(string, map[string]*unstructured.Unstructured) error
}

func newCoalescer(
	window time.Duration,
	maxWait time.Duration,
	maxEntries int,
	rdb *redis.Client,
	ctxFn func() context.Context,
	leaderFn func(context.Context) bool,
	flush func(string, map[string]*unstructured.Unstructured) error,
) *coalescer {
	return &coalescer{
		window:        window,
		maxWait:       maxWait,
		maxEntries:    maxEntries,
		rdb:           rdb,
		ctxFn:         ctxFn,
		leaderFn:      leaderFn,
		buf:           make(map[string]map[string]*unstructured.Unstructured),
		bufFirstEvent: make(map[string]time.Time),
		timers:        make(map[string]*time.Timer),
		flush:         flush,
	}
}

func (c *coalescer) ctx() context.Context {
	if c.ctxFn != nil {
		return c.ctxFn()
	}
	return context.Background()
}

func coalesceRedisKey(appName string) string {
	return constants.KeyPrefixCoalesceBuffer + appName
}

func encodeCoalescePayload(buf map[string]*unstructured.Unstructured, deadline int64) ([]byte, error) {
	e := make(map[string]json.RawMessage, len(buf))
	for k, v := range buf {
		if v == nil {
			continue
		}
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		e[k] = json.RawMessage(b)
	}
	p := coalesceRedisPayload{Deadline: deadline, Entries: e}
	return json.Marshal(p)
}

func decodeCoalescePayload(raw string) (map[string]*unstructured.Unstructured, int64, error) {
	var p coalesceRedisPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, int64(constants.DefaultInitValue), err
	}
	out := make(map[string]*unstructured.Unstructured, len(p.Entries))
	for k, v := range p.Entries {
		var u unstructured.Unstructured
		if err := json.Unmarshal(v, &u); err != nil {
			continue
		}
		out[k] = &u
	}
	return out, p.Deadline, nil
}

// persistBufferLocked snapshots and writes the current buffer for appName to
// Redis. Must be called with c.mu held: keeping the SET under the lock
// serializes concurrent schedule() calls so the Redis copy never lags the
// in-memory buffer (a stale Redis value would steer the next flush onto a
// missing entry — see loadFlushBuffer fallback). Only one attempt is made;
// transient Redis failures self-heal on the next schedule() because the
// in-memory buffer is also written back here.
func (c *coalescer) persistBufferLocked(appName string) error {
	if c.rdb == nil {
		return nil
	}
	if c.leaderFn != nil && !c.leaderFn(c.ctx()) {
		return nil
	}
	buf := c.buf[appName]
	if len(buf) == constants.DefaultInitValue {
		return nil
	}
	raw, err := encodeCoalescePayload(buf, time.Now().Add(c.window).Unix())
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx(), coalesceRedisKey(appName), raw, constants.CoalesceBufferPersistTTL).Err()
}

func (c *coalescer) shouldSkipEntry(buf map[string]*unstructured.Unstructured, key string) bool {
	if len(buf) < c.maxEntries {
		return false
	}
	_, exists := buf[key]
	return !exists
}

func (c *coalescer) schedule(appName, key string, oldObj *unstructured.Unstructured) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buf[appName] == nil {
		c.buf[appName] = make(map[string]*unstructured.Unstructured)
	}
	buf := c.buf[appName]
	if c.shouldSkipEntry(buf, key) {
		return
	}
	if _, exists := buf[key]; !exists {
		// nil is an added resource: no pre-image, but the flush still runs.
		buf[key] = oldObj.DeepCopy()
	}
	// Best-effort Redis persist — timer is always set regardless of result so
	// events are never silently dropped due to transient Redis unavailability.
	_ = c.persistBufferLocked(appName)

	now := time.Now()
	first, hasFirst := c.bufFirstEvent[appName]
	if !hasFirst {
		c.bufFirstEvent[appName] = now
		first = now
	}

	// Cap the debounce delay so that a continuously-busy app is forced to flush
	// once maxWait elapses from the first pending event, preventing indefinite delay.
	maxWaitRemaining := c.maxWait - now.Sub(first)
	delay := c.window
	if maxWaitRemaining <= constants.DefaultInitValue {
		// Max wait exceeded: force flush on next scheduler tick.
		delay = time.Millisecond
		delete(c.bufFirstEvent, appName)
	} else if delay > maxWaitRemaining {
		delay = maxWaitRemaining
	}

	if t, ok := c.timers[appName]; ok {
		t.Stop()
	}
	c.timers[appName] = time.AfterFunc(delay, func() {
		c.fireFlush(appName)
	})
}

func (c *coalescer) fireFlush(appName string) {
	c.mu.Lock()
	if t, ok := c.timers[appName]; ok {
		t.Stop()
		delete(c.timers, appName)
	}
	delete(c.bufFirstEvent, appName)
	// Detached: events landing while this flush runs open a fresh buffer instead
	// of being wiped together with the flushed one.
	flushing := c.buf[appName]
	delete(c.buf, appName)
	c.mu.Unlock()
	err := c.flush(appName, flushing)
	if err == nil {
		c.clearBufferAfterSuccessfulFlush(appName)
		return
	}
	if errors.Is(err, errFlushStoredMissing) || errors.Is(err, errFlushNoInputs) {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnInformersFlushFailed), appName, err))
		return
	}
	c.restoreBuffer(appName, flushing)
	if errors.Is(err, errFlushNotLeader) {
		return
	}
	delay := c.window
	if errors.Is(err, errFlushStoredStale) {
		delay = constants.InformerFlushStaleRetry
	}
	constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
		fmt.Sprintf(string(constants.WarnInformersFlushRetry), appName, err, delay))
	c.rearm(appName, delay)
}

// restoreBuffer puts a detached buffer back; its objects predate anything
// buffered meanwhile, so they win as the pre-image.
func (c *coalescer) restoreBuffer(appName string, flushing map[string]*unstructured.Unstructured) {
	if len(flushing) == constants.DefaultInitValue {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buf[appName] == nil {
		c.buf[appName] = make(map[string]*unstructured.Unstructured, len(flushing))
	}
	for key, obj := range flushing {
		c.buf[appName][key] = obj
	}
	_ = c.persistBufferLocked(appName)
}

// rearm schedules another flush attempt after delay, keeping the buffer intact.
func (c *coalescer) rearm(appName string, delay time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.timers[appName]; exists {
		return
	}
	c.timers[appName] = time.AfterFunc(delay, func() {
		c.fireFlush(appName)
	})
}

func (c *coalescer) loadBufferFromRedisForFlush(appName string) (map[string]*unstructured.Unstructured, error) {
	if c.rdb == nil {
		return nil, nil
	}
	raw, err := c.rdb.Get(c.ctx(), coalesceRedisKey(appName)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	redisBuf, _, decErr := decodeCoalescePayload(raw)
	if decErr != nil {
		return nil, decErr
	}
	if len(redisBuf) == constants.DefaultInitValue {
		return nil, nil
	}
	return redisBuf, nil
}

func (c *coalescer) clearBufferAfterSuccessfulFlush(appName string) {
	c.mu.Lock()
	if len(c.buf[appName]) > constants.DefaultInitValue {
		_ = c.persistBufferLocked(appName)
		c.mu.Unlock()
		return
	}
	rdb := c.rdb
	c.mu.Unlock()
	if rdb != nil {
		_ = rdb.Del(c.ctx(), coalesceRedisKey(appName)).Err()
	}
}

func (c *coalescer) clearBufferRedis(appName string) {
	c.mu.Lock()
	delete(c.buf, appName)
	delete(c.bufFirstEvent, appName)
	if t, ok := c.timers[appName]; ok {
		t.Stop()
		delete(c.timers, appName)
	}
	rdb := c.rdb
	c.mu.Unlock()
	if rdb != nil {
		_ = rdb.Del(c.ctx(), coalesceRedisKey(appName)).Err()
	}
}

// inMemBuf returns the current in-memory buffer for appName without consuming it.
func (c *coalescer) inMemBuf(appName string) map[string]*unstructured.Unstructured {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf[appName]
}

func (c *coalescer) resumeFromRedis(ctx context.Context) {
	if c.rdb == nil {
		return
	}
	var cursor uint64
	pattern := constants.KeyPrefixCoalesceBuffer + constants.Wildcard
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, pattern, constants.DefaultQueueSize).Result()
		if err != nil {
			return
		}
		for i := range keys {
			c.resumeOneKey(ctx, keys[i])
		}
		cursor = next
		if cursor == uint64(constants.DefaultInitValue) {
			break
		}
	}
}

func (c *coalescer) stopAllTimers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, t := range c.timers {
		t.Stop()
		delete(c.timers, name)
	}
	clear(c.bufFirstEvent)
}

func (c *coalescer) resumeOneKey(ctx context.Context, key string) {
	appName := strings.TrimPrefix(key, constants.KeyPrefixCoalesceBuffer)
	if appName == constants.EmptyString {
		return
	}
	raw, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return
	}
	buf, deadline, err := decodeCoalescePayload(raw)
	if err != nil {
		return
	}
	rem := time.Until(time.Unix(deadline, int64(constants.DefaultInitValue)))
	if rem <= time.Duration(constants.DefaultInitValue) {
		// Deadline passed while no leader was running: flush on the next tick, never drop.
		rem = time.Millisecond
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.timers[appName]; exists {
		return
	}
	c.buf[appName] = buf
	c.timers[appName] = time.AfterFunc(rem, func() {
		c.fireFlush(appName)
	})
}
