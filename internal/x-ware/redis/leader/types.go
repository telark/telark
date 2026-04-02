package leader

import (
	"sync"
	"time"

	"github.com/plsyro/x-ware/redis/lock"
	"github.com/redis/go-redis/v9"
)

// LeaderEvent is emitted on leadership changes.
type LeaderEvent struct {
	IsLeader   bool
	FenceToken string
	Timestamp  time.Time
}

// LeaderConfig holds configuration for leader election.
type LeaderConfig struct {
	LeaseKey      string
	LeaseTTL      time.Duration
	RenewInterval time.Duration
	RetryInterval time.Duration
}

// LeaderElector manages leader election via Redis distributed locks.
type LeaderElector struct {
	client     *redis.Client
	lock       lock.DistributedLock
	config     LeaderConfig
	holderID   string
	isLeader   bool
	fenceToken string
	mu         sync.RWMutex
}
