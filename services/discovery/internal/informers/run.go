package informers

import (
	"context"
	"sync"

	applicationscore "github.com/telark/telark/services/discovery/internal/core/applications/core"
	appsnapshot "github.com/telark/telark/services/discovery/internal/core/applications/snapshot"
)

var globalM *Manager

var informerMu sync.Mutex

var informerCancel context.CancelFunc

func Run(ctx context.Context, cfg Config) {
	informerMu.Lock()
	if informerCancel != nil {
		informerCancel()
	}
	runCtx, cancel := context.WithCancel(ctx)
	informerCancel = cancel
	m := newManager(cfg)
	globalM = m
	appsnapshot.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return m.getCachedManifest(kind, name, ns)
	}
	applicationscore.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return m.getCachedManifest(kind, name, ns)
	}
	informerMu.Unlock()
	go func() {
		defer cancel()
		m.run(runCtx)
	}()
}

func Global() *Manager {
	informerMu.Lock()
	defer informerMu.Unlock()
	return globalM
}

// Force sync flushes the buffered pre-image first so its snapshot is not lost; a failed flush
// (not leader, exporter down) still clears the buffer so the sync proceeds on authoritative state.
func CancelAppCoalesce(appName string) {
	informerMu.Lock()
	m := globalM
	informerMu.Unlock()
	if m == nil {
		return
	}
	m.coalesce.fireFlush(appName)
	m.coalesce.clearBufferRedis(appName)
}
