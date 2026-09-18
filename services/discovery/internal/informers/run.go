package informers

import (
	"context"
	"sync"

	applicationscore "github.com/telark/discovery/internal/core/applications/core"
	appsnapshot "github.com/telark/discovery/internal/core/applications/snapshot"
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

// CancelAppCoalesce flushes any buffered pre-state for appName, then clears
// the in-memory and Redis buffers. Called by force-sync to preserve the
// pre-update snapshot before discarding the coalescing window: if force-sync
// runs while a change is buffered, the pre-snapshot would otherwise be lost.
// If the flush fails (not leader, exporter unavailable) the buffer is still
// cleared so force-sync can proceed with authoritative state.
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
