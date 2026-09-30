package forcesync

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/config"
)

type (
	JobExecutor func(ctx context.Context, replicaID, appName string) error
	Job         struct {
		JobID       string
		AppName     string
		RequestedBy string
		RequestedAt string
		Reason      string
	}
	EnqueueRequest struct {
		AppName     string
		RequestedBy string
		Reason      string
	}
	EnqueueResult struct {
		JobID    string
		Phase    string
		Status   string
		Enqueued bool
	}
	StreamOps struct {
		client *xwareredis.StreamClient
		cfg    config.ForceSyncConfig
	}
	Dedup struct {
		rdb *redis.Client
		ttl time.Duration
	}
	Manager struct {
		cfg       config.ForceSyncConfig
		stream    *StreamOps
		dedup     *Dedup
		exporter  *clients.ExporterClient
		executor  JobExecutor
		replicaID string
		mu        sync.Mutex
		cancel    context.CancelFunc
		wg        sync.WaitGroup
		active    bool
	}
	Maintenance struct {
		cfg    config.ForceSyncConfig
		stream *StreamOps
	}
	Ingress struct {
		stream   *StreamOps
		dedup    *Dedup
		exporter *clients.ExporterClient
	}
	LeaderLoop struct {
		election    *xwareredis.ElectionClient
		manager     *Manager
		maintenance *Maintenance
		cfg         config.CoordinationConfig
		cancel      context.CancelFunc
		active      bool
	}
)
