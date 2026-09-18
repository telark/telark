package informers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	historyshared "github.com/telark/discovery/internal/core/applications/history/shared"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
	kcoredynamic "github.com/telark/kcore/informers/dynamic"
	kcorefactory "github.com/telark/kcore/informers/factory"
	"github.com/telark/kcore/k8sclient"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

//nolint:containedctx // lifecycle root for flush and event handlers
type Manager struct {
	cfg                 Config
	rootCtxMu           sync.RWMutex
	rootCtx             context.Context
	informersMu         sync.RWMutex
	informers           map[string]cache.SharedIndexInformer
	nsRunMu             sync.Mutex
	nsCancels           map[string]context.CancelFunc
	watchGVRs           []schema.GroupVersionResource
	coalesce            *coalescer
	getStoredApp        func(string) (*applicationmodel.Application, error)
	getSnapshotManifest snapshotManifestFn
	flushLimiter        *rate.Limiter
	kubeClient          *kubernetes.Clientset
	appNamesMu          sync.RWMutex
	appNames            map[string]struct{}
	nsPrewarmMu         sync.Mutex
	nsPrewarmTimers     map[string]*time.Timer
	reconcileMu         sync.Mutex
}

func newManager(cfg Config) *Manager {
	flushRate := config.InformerFlushRatePerSec()
	m := &Manager{
		cfg:                 cfg,
		informers:           make(map[string]cache.SharedIndexInformer),
		nsCancels:           make(map[string]context.CancelFunc),
		nsPrewarmTimers:     make(map[string]*time.Timer),
		getStoredApp:        clients.NewExporterClient().GetApplicationByNameFresh,
		getSnapshotManifest: clients.NewSnapshotClient().GetSnapshotManifest,
		flushLimiter:        rate.NewLimiter(rate.Limit(flushRate), flushRate),
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
		func(app string, buf map[string]*unstructured.Unstructured) error {
			return m.flushApp(app, buf)
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
	m.refreshKnownApps(ctx)
	watchKinds := watchKindSet()
	gvrs, ok := m.resolveWatchGVRs(ctx, watchKinds)
	if !ok {
		return
	}
	m.watchGVRs = gvrs
	go m.watchLeaderResume(ctx)
	nsTick := time.NewTicker(time.Duration(constants.GlobalConfigExcludedPollSec) * time.Second)
	defer nsTick.Stop()
	appsTick := time.NewTicker(constants.InformerKnownAppsRefresh)
	defer appsTick.Stop()
	m.reconcileNamespaceWatchers(ctx, dyn, resync)
	for {
		select {
		case <-ctx.Done():
			m.coalesce.stopAllTimers()
			m.stopAllNamespaceWatchers()
			return
		case <-nsTick.C:
			m.reconcileNamespaceWatchers(ctx, dyn, resync)
		case <-appsTick.C:
			m.refreshKnownApps(ctx)
		}
	}
}

// A failed refresh keeps the previous set; nil (no apps yet) treats every app as known.
func (m *Manager) refreshKnownApps(ctx context.Context) {
	apps, err := m.cfg.DiscoverApps(ctx, m.cfg.RDB)
	if err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnInformersDiscoverAppsFailed), err),
		)
		return
	}
	m.appNamesMu.Lock()
	m.appNames = knownAppNames(apps)
	m.appNamesMu.Unlock()
	m.reconcileInBackground(ctx, apps)
}

// One pass at a time, off the run loop: a drift-heavy pass waits on the
// exporter per app and must neither hold the tick nor pile up behind it.
func (m *Manager) reconcileInBackground(ctx context.Context, apps []applicationmodel.Application) {
	if !m.isLeader(ctx) || !m.informersSynced() || !m.reconcileMu.TryLock() {
		return
	}
	go func() {
		defer m.reconcileMu.Unlock()
		m.reconcileRecorded(ctx, apps)
	}()
}

func (m *Manager) attachInformer(
	factory dynamicinformer.DynamicSharedInformerFactory,
	ns string,
	gvr schema.GroupVersionResource,
) cache.SharedIndexInformer {
	inf := factory.ForResource(gvr).Informer()
	_ = inf.SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnInformerWatchError), ns, gvr.Resource, err),
		)
	})
	key := informerIndexKey(ns, gvr)
	m.informersMu.Lock()
	m.informers[key] = inf
	m.informersMu.Unlock()
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if !inf.HasSynced() {
				return
			}
			timedHandler(ns, gvr.Resource, constants.InformerEventAdd, func() { m.onAdd(obj) })
		},
		UpdateFunc: func(oldObj, newObj any) {
			if !inf.HasSynced() {
				return
			}
			timedHandler(ns, gvr.Resource, constants.InformerEventUpdate, func() { m.onUpdate(oldObj, newObj) })
		},
		DeleteFunc: func(obj any) {
			if !inf.HasSynced() {
				return
			}
			timedHandler(ns, gvr.Resource, constants.InformerEventDelete, func() { m.onDelete(obj) })
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
				// A failover lands on synced informers: catch up now rather than
				// at the next tick. At startup the tick runs it once synced.
				if m.informersSynced() {
					m.refreshKnownApps(ctx)
				}
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
	// A fresh ADD has no pre-image; the flush diffs it against the stored
	// resource list. A replayed one (startup, resync) is already there.
	if eventLag(u) <= constants.InformerAddFreshWindow {
		m.coalesce.schedule(app, resourceKey(u), nil)
	}
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
		app, oldU.GetKind(), oldU.GetNamespace(), oldU.GetName(), repv, eventLag(newU),
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
	m.appNamesMu.RLock()
	defer m.appNamesMu.RUnlock()
	if m.appNames == nil {
		return true
	}
	_, ok := m.appNames[app]
	return ok
}

// A controller status write leaves the diff roots untouched; the flush it
// would open publishes nothing unless the counters ComputeHealth reads or the
// owner references grouping follows moved with it.
func isResyncArtifact(oldU, newU *unstructured.Unstructured) bool {
	if oldU == nil || newU == nil {
		return false
	}
	if manifestdiff.Fingerprint(oldU) != manifestdiff.Fingerprint(newU) {
		return false
	}
	return cmp.Equal(observedState(oldU), observedState(newU))
}

func observedState(u *unstructured.Unstructured) []any {
	out := make([]any, constants.DefaultInitValue, len(constants.InformerHealthStatusFields)+constants.DefaultAddValue)
	for _, field := range constants.InformerHealthStatusFields {
		v, _, _ := unstructured.NestedFieldNoCopy(u.Object, historyshared.ManifestStatusKey, field)
		out = append(out, v)
	}
	return append(out, u.GetOwnerReferences())
}

func (m *Manager) onDelete(obj any) {
	ctx := m.ctxOrBackground()
	if !m.isLeader(ctx) {
		return
	}
	if tomb, isTomb := obj.(cache.DeletedFinalStateUnknown); isTomb {
		obj = tomb.Obj
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
		m.runPrewarmNS(ns)
		return
	}
	// The deleted object is the pre-image: the flush records the removal and
	// the snapshot keeps a copy a rollback can restore.
	m.coalesce.schedule(app, resourceKey(u), u.DeepCopy())
}

func (m *Manager) runPrewarmNS(ns string) {
	if m.cfg.RunPrewarmNS == nil {
		return
	}
	m.nsPrewarmMu.Lock()
	defer m.nsPrewarmMu.Unlock()
	if t, ok := m.nsPrewarmTimers[ns]; ok {
		t.Stop()
	}
	m.nsPrewarmTimers[ns] = time.AfterFunc(constants.InformerPrewarmDebounce, func() {
		m.nsPrewarmMu.Lock()
		delete(m.nsPrewarmTimers, ns)
		m.nsPrewarmMu.Unlock()
		_ = m.cfg.RunPrewarmNS(m.ctxOrBackground(), m.cfg.RDB, ns)
	})
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

func (m *Manager) informersSynced() bool {
	m.informersMu.RLock()
	defer m.informersMu.RUnlock()
	if len(m.informers) == constants.DefaultInitValue {
		return false
	}
	for _, inf := range m.informers {
		if !inf.HasSynced() {
			return false
		}
	}
	return true
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

// eventLag measures how long the API server change waited before this handler
// saw it, using the newest managedFields timestamp on the object.
func eventLag(u *unstructured.Unstructured) time.Duration {
	var newest time.Time
	for _, mf := range u.GetManagedFields() {
		if mf.Time != nil && mf.Time.After(newest) {
			newest = mf.Time.Time
		}
	}
	if newest.IsZero() {
		return time.Duration(constants.DefaultInitValue)
	}
	return time.Since(newest).Round(time.Second)
}

func timedHandler(ns, resource, event string, fn func()) {
	start := time.Now()
	fn()
	if took := time.Since(start); took > constants.InformerSlowHandlerThreshold {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(fmt.Sprintf(
			string(constants.WarnInformerSlowHandler), ns, resource, event, took.Round(time.Millisecond),
		))
	}
}

// resolveWatchGVRs keeps retrying API discovery until it succeeds or the
// context ends: giving up would leave the informers off until a restart.
func (m *Manager) resolveWatchGVRs(ctx context.Context, watchKinds map[string]struct{}) ([]schema.GroupVersionResource, bool) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	for {
		gvrs, err := kcoredynamic.GVRsForKinds(watchKinds, m.kubeClient.Discovery())
		if err == nil {
			return gvrs, true
		}
		lg.Warn(fmt.Sprintf(string(constants.WarnInformersGVRFailed), err))
		select {
		case <-ctx.Done():
			return nil, false
		case <-time.After(constants.InformersGVRRetryInterval):
		}
	}
}
