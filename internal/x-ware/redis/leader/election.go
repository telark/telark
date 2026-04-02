package leader

import (
	"context"
	"fmt"
	"time"

	"github.com/plsyro/x-ware/redis/lock"
	"github.com/redis/go-redis/v9"
)

// New creates a new LeaderElector with the given Redis client and configuration.
func New(client *redis.Client, cfg LeaderConfig) (*LeaderElector, error) {
	if client == nil {
		return nil, fmt.Errorf("%s", ErrNilRedisClient)
	}
	if cfg.LeaseKey == "" {
		cfg.LeaseKey = DefaultLeaseKey
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = DefaultLeaseTTL
	}
	if cfg.RenewInterval <= 0 {
		cfg.RenewInterval = DefaultRenewInterval
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = DefaultRetryInterval
	}

	return &LeaderElector{
		client:   client,
		lock:     lock.New(client),
		config:   cfg,
		holderID: lock.GenerateHolderID(),
	}, nil
}

func (le *LeaderElector) Run(ctx context.Context) <-chan LeaderEvent {
	ch := make(chan LeaderEvent, 1)
	go le.electionLoop(ctx, ch)
	return ch
}

// returns whether this elector currently holds the leader lease.
func (le *LeaderElector) IsLeader() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.isLeader
}

// returns the current fence token, or empty if not leader.
func (le *LeaderElector) CurrentFenceToken() string {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.fenceToken
}

func (le *LeaderElector) electionLoop(ctx context.Context, ch chan<- LeaderEvent) {
	defer func() {
		// On shutdown, release lease if we are leader
		le.mu.RLock()
		wasLeader := le.isLeader
		le.mu.RUnlock()
		if wasLeader {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = le.lock.Release(releaseCtx, le.config.LeaseKey, le.holderID)
			cancel()
			le.setLeaderStatus(false, "")
		}
		close(ch)
	}()

	for {
		if ctx.Err() != nil {
			return
		}

		token, acquired, err := le.lock.Acquire(ctx, le.config.LeaseKey, le.holderID, le.config.LeaseTTL)
		if err != nil {
			le.sleepOrDone(ctx, le.config.RetryInterval)
			continue
		}

		if !acquired {
			// Not leader — standby mode
			le.mu.RLock()
			wasLeader := le.isLeader
			le.mu.RUnlock()
			if wasLeader {
				le.setLeaderStatus(false, "")
				le.emit(ch, LeaderEvent{IsLeader: false, Timestamp: time.Now()})
			}
			le.sleepOrDone(ctx, le.config.RetryInterval)
			continue
		}

		// Became leader
		le.setLeaderStatus(true, token)
		le.emit(ch, LeaderEvent{IsLeader: true, FenceToken: token, Timestamp: time.Now()})

		// Start renewal loop
		le.renewLoop(ctx, ch)

		// If we exit renewLoop, we lost leadership
		if ctx.Err() != nil {
			return
		}
	}
}

func (le *LeaderElector) renewLoop(ctx context.Context, ch chan<- LeaderEvent) {
	ticker := time.NewTicker(le.config.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			extended, err := le.lock.Extend(ctx, le.config.LeaseKey, le.holderID, le.config.LeaseTTL)
			if err != nil || !extended {
				// Lost leadership — demote to standby
				le.setLeaderStatus(false, "")
				le.emit(ch, LeaderEvent{IsLeader: false, Timestamp: time.Now()})
				return
			}
		}
	}
}

func (le *LeaderElector) setLeaderStatus(isLeader bool, token string) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.isLeader = isLeader
	le.fenceToken = token
}

func (le *LeaderElector) emit(ch chan<- LeaderEvent, event LeaderEvent) {
	select {
	case ch <- event:
	default:
		// Drop if channel is full to avoid blocking
	}
}

func (le *LeaderElector) sleepOrDone(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
