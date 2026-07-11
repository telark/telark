package informers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/clients"
	"github.com/telark/discovery/config"
	"github.com/telark/discovery/constants"
	gcfghelper "github.com/telark/discovery/helpers/globalconfig"
	redishelper "github.com/telark/discovery/helpers/redis"
	kcoredynamic "github.com/telark/kcore/informers/dynamic"
	kcorefactory "github.com/telark/kcore/informers/factory"
	"github.com/telark/kcore/k8sclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

//nolint:containedctx // lifecycle root for flush and event handlers
type Manager struct {
	cfg         Config
	rootCtxMu   sync.RWMutex
	rootCtx     context.Context
	informersMu sync.RWMutex
	informers   map[string]cache.SharedIndexInformer
	nsRunMu     sync.Mutex
	nsCancels   map[string]context.CancelFunc
	watchGVRs   []schema.GroupVersionResource
	coalesce    *coalescer
	exporter    *clients.ExporterClient
	kubeClient  *kubernetes.Clientset
	appNames    map[string]struct{}
}

func newManager(cfg Config) *Manager {
	m := &Manager{
		cfg:       cfg,
		informers: make(map[string]cache.SharedIndexInformer),
		nsCancels: make(map[string]context.CancelFunc),
		exporter:  clients.NewExporterClient(),
	}
	kc, _ := k8sclient.InitKubernetesClient()
	m.kubeClient = kc
	m.coalesce = newCoalescer(
		time.Duration(config.CoalesceWindowSec())*time.Second,
		time.Duration(config.CoalesceMaxWaitSec())*time.Second,
		config.CoalesceBufferMaxEntries(),
		cfg.RDB,
		func() context.Context { return m.ctxOrBackground() },
		cfg.LeaderFn,
		func(app string) error {
			return m.flushApp(app)
		},
	)
	return m
}

func (m *Manager) run(ctx context.Context) {
	m.rootCtxMu.Lock()
	m.rootCtx = ctx
	m.rootCtxMu.Unlock()
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	lg.Info(string(constants.InfoInformersFactoryStart))
	dyn, err := k8sclient.InitDynamicClient()
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnInformersInitFailed), err))
		return
	}
	base := time.Duration(config.InformerResyncSec()) * time.Second
	resync := kcorefactory.JitteredResync(base, config.InformerResyncJitterFraction())
	if m.cfg.DiscoverApps == nil {
		return
	}
	apps, err := m.cfg.DiscoverApps(ctx, m.cfg.RDB)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnInformersDiscoverAppsFailed), err))
		apps = nil
	}
	m.appNames = knownAppNames(apps)
	watchKinds := kindSetFromApps(apps)
	gvrs, err := kcoredynamic.GVRsForKinds(watchKinds, m.kubeClient.Discovery())
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnInformersGVRFailed), err))
		return
	}
	m.watchGVRs = gvrs
	go m.watchLeaderResume(ctx)
	nsTick := time.NewTicker(time.Duration(constants.GlobalConfigExcludedPollSec) * time.Second)
	defer nsTick.Stop()
	m.reconcileNamespaceWatchers(ctx, dyn, resync)
	for {
		select {
		case <-ctx.Done():
			m.coalesce.stopAllTimers()
			m.stopAllNamespaceWatchers()
			return
		case <-nsTick.C:
			m.reconcileNamespaceWatchers(ctx, dyn, resync)
		}
	}
}

func (m *Manager) attachInformer(
	factory dynamicinformer.DynamicSharedInformerFactory,
	ns string,
	gvr schema.GroupVersionResource,
) cache.SharedIndexInformer {
	inf := factory.ForResource(gvr).Informer()
	key := informerIndexKey(ns, gvr)
	m.informersMu.Lock()
	m.informers[key] = inf
	m.informersMu.Unlock()
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if !inf.HasSynced() {
				return
			}
			m.onAdd(obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			if !inf.HasSynced() {
				return
			}
			m.onUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj any) {
			if !inf.HasSynced() {
				return
			}
			m.onDelete(obj)
		},
	})
	return inf
}

func (m *Manager) watchLeaderResume(ctx context.Context) {
	wasLeader := false
	ticker := time.NewTicker(constants.DiscoveryLeaderGatePoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := m.isLeader(ctx)
			if now && !wasLeader {
				m.coalesce.resumeFromRedis(ctx)
			} else if !now && wasLeader {
				m.coalesce.stopAllTimers()
			}
			wasLeader = now
		}
	}
}

func (m *Manager) onAdd(obj any) {
	ctx := m.ctxOrBackground()
	if !m.isLeader(ctx) {
		return
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	if informerExcludedKind(u.GetKind()) {
		return
	}
	if namespaceExcluded(ctx, u.GetNamespace()) {
		return
	}
	ns := u.GetNamespace()
	if ns == constants.EmptyString {
		return
	}
	app := m.appGroupFromCache(ctx, u)
	if !m.isKnownAppName(app) {
		return
	}
	if app != constants.EmptyString && redishelper.ApplicationHasRedisState(ctx, m.cfg.RDB, app) {
		return
	}
	m.runPrewarmNS(ctx, ns)
}

func (m *Manager) onUpdate(oldObj, newObj any) {
	ctx := m.ctxOrBackground()
	if !m.isLeader(ctx) {
		return
	}
	oldU, ok := oldObj.(*unstructured.Unstructured)
	newU, ok2 := newObj.(*unstructured.Unstructured)
	if !ok || !ok2 || oldU == nil || newU == nil {
		return
	}
	if isResyncArtifact(oldU, newU) {
		return
	}
	if informerExcludedKind(newU.GetKind()) {
		return
	}
	if namespaceExcluded(ctx, newU.GetNamespace()) {
		return
	}
	app := m.appGroupFromCache(ctx, newU)
	if app == constants.EmptyString {
		return
	}
	if !m.isKnownAppName(app) {
		return
	}
	key := resourceKey(newU)
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	rep, repFound, _ := unstructured.NestedInt64(oldU.Object, "spec", "replicas")
	var repv any
	if repFound {
		repv = rep
	} else {
		repv = "-"
	}
	lg.Info(fmt.Sprintf(
		string(constants.InfoInformerOldObjectCaptured),
		app, oldU.GetKind(), oldU.GetNamespace(), oldU.GetName(), repv,
	))
	m.coalesce.schedule(app, key, oldU.DeepCopy())
}

func knownAppNames(apps []applicationmodel.Application) map[string]struct{} {
	if len(apps) == constants.DefaultInitValue {
		return nil
	}
	set := make(map[string]struct{}, len(apps))
	for i := range apps {
		if apps[i].Name != constants.EmptyString {
			set[apps[i].Name] = struct{}{}
		}
	}
	return set
}

func (m *Manager) isKnownAppName(app string) bool {
	if app == constants.EmptyString {
		return false
	}
	if m.appNames == nil {
		return true
	}
	_, ok := m.appNames[app]
	return ok
}

func isResyncArtifact(oldU, newU *unstructured.Unstructured) bool {
	if oldU == nil || newU == nil {
		return false
	}
	a := sanitizedUnstructuredForCompare(oldU)
	b := sanitizedUnstructuredForCompare(newU)
	return cmp.Equal(a, b)
}

func sanitizedUnstructuredForCompare(u *unstructured.Unstructured) map[string]any {
	c := u.DeepCopy()
	if c == nil || c.Object == nil {
		return nil
	}
	if md, ok := c.Object["metadata"].(map[string]any); ok && md != nil {
		delete(md, "resourceVersion")
		delete(md, "managedFields")
		delete(md, "generation")
		if len(md) > constants.DefaultInitValue {
			return c.Object
		}
		delete(c.Object, "metadata")
	}
	return c.Object
}

func (m *Manager) onDelete(obj any) {
	ctx := m.ctxOrBackground()
	if !m.isLeader(ctx) {
		return
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	if informerExcludedKind(u.GetKind()) {
		return
	}
	if namespaceExcluded(ctx, u.GetNamespace()) {
		return
	}
	ns := u.GetNamespace()
	if ns == constants.EmptyString {
		return
	}
	m.runPrewarmNS(ctx, ns)
}

func (m *Manager) runPrewarmNS(ctx context.Context, ns string) {
	if m.cfg.RunPrewarmNS != nil {
		_ = m.cfg.RunPrewarmNS(ctx, m.cfg.RDB, ns)
	}
}

func (m *Manager) ctxOrBackground() context.Context {
	m.rootCtxMu.RLock()
	ctx := m.rootCtx
	m.rootCtxMu.RUnlock()
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func (m *Manager) isLeader(ctx context.Context) bool {
	if m.cfg.LeaderFn == nil {
		return true
	}
	return m.cfg.LeaderFn(ctx)
}

func namespaceExcluded(ctx context.Context, ns string) bool {
	if ns == constants.EmptyString {
		return true
	}
	ex := gcfghelper.FetchExcludedNamespaces(ctx)
	for i := range ex {
		if ex[i] == ns {
			return true
		}
	}
	return false
}
