package informers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
	kcorefactory "github.com/telark/kcore/informers/factory"
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
	attempts      map[string]int // consecutive failed flushes, drives the retry backoff
	inflight      map[string]struct{}
	held          map[string]map[string]*unstructured.Unstructured // pre-images of a no-inputs drop, for one window
	heldTimers    map[string]*time.Timer
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
		attempts:      make(map[string]int),
		inflight:      make(map[string]struct{}),
		held:          make(map[string]map[string]*unstructured.Unstructured),
		heldTimers:    make(map[string]*time.Timer),
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

// Called with c.mu held: the SET under the lock keeps the Redis copy from lagging the
// in-memory buffer (loadFlushBuffer falls back to it). One attempt; the next schedule rewrites it.
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
	if dropped, ok := c.held[appName]; ok {
		maps.Copy(buf, dropped)
		c.releaseHeldLocked(appName)
	}
	// Best effort: the timer is armed whatever Redis answers, so an event is never dropped.
	_ = c.persistBufferLocked(appName)

	now := time.Now()
	first, hasFirst := c.bufFirstEvent[appName]
	if !hasFirst {
		c.bufFirstEvent[appName] = now
		first = now
	}

	// A continuously busy app is forced to flush once maxWait elapses from its first pending event.
	maxWaitRemaining := c.maxWait - now.Sub(first)
	delay := c.window
	if maxWaitRemaining <= constants.DefaultInitValue {
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
	c.inflight[appName] = struct{}{}
	c.mu.Unlock()
	err := c.flush(appName, flushing)
	c.mu.Lock()
	delete(c.inflight, appName)
	c.mu.Unlock()
	if err == nil {
		c.forgetRetries(appName)
		c.clearBufferAfterSuccessfulFlush(appName)
		return
	}
	if errors.Is(err, errFlushStoredMissing) || errors.Is(err, errFlushNoInputs) {
		c.forgetRetries(appName)
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnInformersFlushFailed), appName, err),
		)
		c.discardUnlessPending(appName, flushing, errors.Is(err, errFlushNoInputs))
		return
	}
	c.restoreBuffer(appName, flushing)
	if errors.Is(err, errFlushNotLeader) {
		return
	}
	base := c.window
	if errors.Is(err, errFlushStoredStale) {
		base = constants.InformerFlushStaleRetry
	}
	delay := c.retryDelay(appName, base)
	constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
		fmt.Sprintf(string(constants.WarnInformersFlushRetry), appName, err, delay),
	)
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

// Events landing during the flush, or up to one window after a no-inputs drop, inherit its pre-images:
// a delete and recreate straddling the drop otherwise diffs the recreated object against itself.
func (c *coalescer) discardUnlessPending(appName string, flushing map[string]*unstructured.Unstructured, hold bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if fresh := c.buf[appName]; len(fresh) > constants.DefaultInitValue {
		maps.Copy(fresh, flushing)
		_ = c.persistBufferLocked(appName)
		return
	}
	if hold && len(flushing) > constants.DefaultInitValue {
		c.holdLocked(appName, flushing)
	}
	if c.rdb != nil {
		_ = c.rdb.Del(c.ctx(), coalesceRedisKey(appName), pendingSnapshotsKey(appName)).Err()
	}
}

func (c *coalescer) holdLocked(appName string, dropped map[string]*unstructured.Unstructured) {
	c.releaseHeldLocked(appName)
	c.held[appName] = dropped
	var expiry *time.Timer
	expiry = time.AfterFunc(c.window, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		// A timer that fired while a newer hold replaced this one must not release it.
		if c.heldTimers[appName] == expiry {
			c.releaseHeldLocked(appName)
		}
	})
	c.heldTimers[appName] = expiry
}

func (c *coalescer) releaseHeldLocked(appName string) {
	if t, ok := c.heldTimers[appName]; ok {
		t.Stop()
	}
	delete(c.held, appName)
	delete(c.heldTimers, appName)
}

// retryDelay doubles per consecutive failure so a storm of dirty apps drains
// instead of re-hitting an overloaded exporter every window.
func (c *coalescer) retryDelay(appName string, base time.Duration) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	delay := min(base<<c.attempts[appName], constants.InformerFlushMaxRetryDelay)
	if delay < constants.InformerFlushMaxRetryDelay {
		c.attempts[appName]++
	}
	jittered := kcorefactory.JitteredResync(delay, constants.InformerFlushRetryJitterFraction)
	return min(jittered, constants.InformerFlushMaxRetryDelay)
}

func (c *coalescer) forgetRetries(appName string) {
	c.mu.Lock()
	delete(c.attempts, appName)
	c.mu.Unlock()
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
	c.releaseHeldLocked(appName)
	rdb := c.rdb
	c.mu.Unlock()
	if rdb != nil {
		_ = rdb.Del(c.ctx(), coalesceRedisKey(appName), pendingSnapshotsKey(appName)).Err()
	}
}

// inMemBuf returns the current in-memory buffer for appName without consuming it.
func (c *coalescer) inMemBuf(appName string) map[string]*unstructured.Unstructured {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf[appName]
}

// pending reports buffered events, an armed flush or one in flight: the
// reconcile leaves such an app to that flush, which records its own result.
func (c *coalescer) pending(appName string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, armed := c.timers[appName]
	_, running := c.inflight[appName]
	return armed || running || len(c.buf[appName]) > constants.DefaultInitValue
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
