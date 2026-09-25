package informers

import (
	"context"
	"sync"

	"github.com/telark/discovery/internal/constants"
	applicationscore "github.com/telark/discovery/internal/core/applications/core"
	appsnapshot "github.com/telark/discovery/internal/core/applications/snapshot"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
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

// Test seam: installs, as Run would, a manager whose informer cache holds objs.
func UseCacheForTest(objs ...*unstructured.Unstructured) {
	inf := cache.NewSharedIndexInformerWithOptions(&cache.ListWatch{}, &unstructured.Unstructured{}, cache.SharedIndexInformerOptions{})
	for _, obj := range objs {
		_ = inf.GetIndexer().Add(obj)
	}
	informerMu.Lock()
	globalM = &Manager{informers: map[string]cache.SharedIndexInformer{constants.EmptyString: inf}}
	informerMu.Unlock()
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
