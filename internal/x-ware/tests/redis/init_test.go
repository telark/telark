package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
	redisinit "github.com/telark/x-ware/redis/init"
)

const (
	fastTimeout       = 20 * time.Millisecond
	shortTimeout      = 10 * time.Millisecond
	maxWaitWindow     = 50 * time.Millisecond
	concurrentCallers = 8
)

// 127.0.0.1:1 is reserved and refuses instantly, so every ping fails fast.
func dialUnreachable() (*redisv9.Client, error) {
	return redisv9.NewClient(&redisv9.Options{
		Addr:        "127.0.0.1:1",
		MaxRetries:  -1,
		DialTimeout: fastTimeout,
	}), nil
}

func TestRedisNewClientWithRetry_NilContextOrDial(t *testing.T) {
	redisinit.SetClient(nil)
	t.Cleanup(func() { redisinit.SetClient(nil) })

	// A nil context is the guard under test; context.TODO() would dial and block.
	var nilCtx context.Context
	if c := redisinit.NewClientWithRetry(nilCtx, dialUnreachable, redisinit.RetryConfig{}, nil); c != nil {
		t.Errorf("expected nil for a nil context, got %v", c)
	}
	if c := redisinit.NewClientWithRetry(t.Context(), nil, redisinit.RetryConfig{}, nil); c != nil {
		t.Errorf("expected nil for a nil dial, got %v", c)
	}
	if redisinit.Client() != nil {
		t.Error("the cached client must stay nil when the guards trip")
	}
}

func TestRedisNewClientWithRetry_GivesUpOnMaxWait(t *testing.T) {
	redisinit.SetClient(nil)
	t.Cleanup(func() { redisinit.SetClient(nil) })

	cfg := redisinit.RetryConfig{
		RetryInterval: time.Millisecond,
		MaxWait:       maxWaitWindow,
		PingTimeout:   fastTimeout,
	}

	if c := redisinit.NewClientWithRetry(t.Context(), dialUnreachable, cfg, nil); c != nil {
		t.Fatalf("expected nil when redis never becomes reachable, got %v", c)
	}
	if redisinit.Client() != nil {
		t.Error("no client may be cached after giving up")
	}
}

func TestRedisNewClientWithRetry_ConcurrentCallers(t *testing.T) {
	redisinit.SetClient(nil)
	t.Cleanup(func() { redisinit.SetClient(nil) })

	cfg := redisinit.RetryConfig{
		RetryInterval: time.Millisecond,
		MaxWait:       fastTimeout,
		PingTimeout:   shortTimeout,
	}

	var wg sync.WaitGroup
	for range concurrentCallers {
		wg.Go(func() {
			if c := redisinit.NewClientWithRetry(t.Context(), dialUnreachable, cfg, nil); c != nil {
				t.Errorf("expected nil when redis never becomes reachable, got %v", c)
			}
		})
		wg.Go(func() {
			_ = redisinit.Client()
		})
	}
	wg.Wait()

	if redisinit.Client() != nil {
		t.Error("no client may be cached after giving up")
	}
}

func TestRedisNewClientWithRetry_ReturnsCachedClient(t *testing.T) {
	cached := redisv9.NewClient(&redisv9.Options{Addr: "127.0.0.1:1"})
	redisinit.SetClient(cached)
	t.Cleanup(func() {
		redisinit.SetClient(nil)
		_ = cached.Close()
	})

	if c := redisinit.NewClientWithRetry(t.Context(), nil, redisinit.RetryConfig{}, nil); c != cached {
		t.Errorf("expected the cached client, got %v", c)
	}
}
