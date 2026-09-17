package informers

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	testWindow       = 5 * time.Millisecond
	testMaxWait      = 50 * time.Millisecond
	testMaxEntries   = 10
	testSettle       = 500 * time.Millisecond
	testBusyAttempts = 3
)

func newTestCoalescer(flush func(string, map[string]*unstructured.Unstructured) error) *coalescer {
	return newCoalescer(testWindow, testMaxWait, testMaxEntries, nil, nil, nil, flush)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(testSettle)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(testWindow)
	}
	t.Fatal("condition not met before settle timeout")
}

func TestFireFlushRetriesWhileGenLockBusy(t *testing.T) {
	var calls atomic.Int32
	c := newTestCoalescer(func(string, map[string]*unstructured.Unstructured) error {
		if calls.Add(1) < testBusyAttempts {
			return errFlushGenLockBusy
		}
		return nil
	})
	c.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	waitFor(t, func() bool {
		return calls.Load() >= testBusyAttempts && len(c.inMemBuf("app")) == 0
	})
}

func TestFireFlushRetriesAndKeepsBufferOnTransientErrors(t *testing.T) {
	var calls atomic.Int32
	c := newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		if len(buf) != 1 {
			t.Errorf("flush must receive the detached buffer, got %d entries", len(buf))
		}
		if calls.Add(1) < testBusyAttempts {
			return errors.New("exporter unavailable")
		}
		return nil
	})
	c.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	waitFor(t, func() bool {
		return calls.Load() >= testBusyAttempts && len(c.inMemBuf("app")) == 0
	})
}

func TestScheduleWithoutPreImageStillFlushes(t *testing.T) {
	var entries atomic.Int32
	c := newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		entries.Store(int32(len(buf)))
		return nil
	})
	c.schedule("app", "configmap/cfg", nil)

	waitFor(t, func() bool { return entries.Load() == 1 })
}

func TestEventsDuringFlushAreKept(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	c := newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		if calls.Add(1) == 1 {
			started <- struct{}{}
			<-release
		}
		if _, first := buf["deploy/a"]; first && len(buf) != 1 {
			t.Errorf("first flush must only carry the pre-flush buffer, got %d entries", len(buf))
		}
		return nil
	})
	c.schedule("app", "deploy/a", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})
	<-started
	c.schedule("app", "deploy/b", &unstructured.Unstructured{Object: map[string]any{"kind": "Service"}})
	close(release)

	waitFor(t, func() bool { return calls.Load() == 2 && len(c.inMemBuf("app")) == 0 })
}

func TestResumeFlushesPastDeadlineBufferInsteadOfDropping(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	var calls atomic.Int32
	c := newCoalescer(testWindow, testMaxWait, testMaxEntries, rdb, nil, nil, func(string, map[string]*unstructured.Unstructured) error {
		calls.Add(1)
		return nil
	})
	buf := map[string]*unstructured.Unstructured{
		"deploy/app": {Object: map[string]any{"kind": "Deployment"}},
	}
	raw, err := encodeCoalescePayload(buf, time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	key := coalesceRedisKey("app")
	if err := rdb.Set(context.Background(), key, raw, 0).Err(); err != nil {
		t.Fatal(err)
	}

	c.resumeOneKey(context.Background(), key)

	waitFor(t, func() bool { return calls.Load() == 1 && !mr.Exists(key) })
}
