package informers

import (
	"context"
	"sync"

	"github.com/telark/telark/services/discovery/internal/constants"
	applicationscore "github.com/telark/telark/services/discovery/internal/core/applications/core"
	appsnapshot "github.com/telark/telark/services/discovery/internal/core/applications/snapshot"
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

// Test seam: installs, as Run would, a manager whose informer cache holds objs
// and hands back that cache so a test can update or delete them.
func UseCacheForTest(objs ...*unstructured.Unstructured) cache.Indexer {
	inf := cache.NewSharedIndexInformerWithOptions(
		&cache.ListWatch{}, &unstructured.Unstructured{}, cache.SharedIndexInformerOptions{Indexers: appIndexers()},
	)
	for _, obj := range objs {
		_ = inf.GetIndexer().Add(obj)
	}
	informerMu.Lock()
	globalM = &Manager{informers: map[string]cache.SharedIndexInformer{constants.EmptyString: inf}}
	informerMu.Unlock()
	return inf.GetIndexer()
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
