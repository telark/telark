package cleanup

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/auth/internal/config"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
)

type (
	Job struct {
		JobID        string
		ResourceType string
		ResourceID   string
		RequestedBy  string
		EnqueuedAt   string
		Attempts     int
	}
)

func (j Job) requeued(attempts int) Job {
	next := j
	next.EnqueuedAt = time.Now().UTC().Format(time.RFC3339)
	next.Attempts = attempts
	return next
}

func (j Job) dlq(attempts int) Job {
	next := j
	next.Attempts = attempts
	return next
}

type (
	EnqueueRequest struct {
		ResourceType string
		ResourceID   string
		RequestedBy  string
	}

	EnqueueResult struct {
		JobID    string
		Enqueued bool
	}

	StreamOps struct {
		client        *xwareredis.StreamClient
		streamKey     string
		dlqStream     string
		group         string
		maxLen        int64
		xclaimMinIdle time.Duration
	}

	Dedup struct {
		rdb *redis.Client
		ttl time.Duration
	}

	Manager struct {
		cfg          config.CleanupConfig
		resourceType string
		stream       *StreamOps
		dedup        *Dedup
		reconciler   *cleanupctrl.Reconciler
		replicaID    string
		mu           sync.Mutex
		cancel       context.CancelFunc
		wg           sync.WaitGroup
		active       bool
	}

	Sweeper struct {
		cfg          config.CleanupConfig
		resourceType string
		ingress      *Ingress
	}

	Ingress struct {
		streams map[string]*StreamOps
		dedup   *Dedup
	}

	LeaderLoop struct {
		election  *xwareredis.ElectionClient
		managers  []*Manager
		sweepers  []*Sweeper
		renewTick time.Duration
		cancel    context.CancelFunc
		active    bool
	}
	System struct {
		Ingress    *Ingress
		LeaderLoop *LeaderLoop
	}
)
