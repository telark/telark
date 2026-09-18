package informers

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	applicationscore "github.com/telark/discovery/internal/core/applications/core"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	restshared "github.com/telark/rest/clients/shared"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

const (
	testWindow           = 5 * time.Millisecond
	testMaxWait          = 50 * time.Millisecond
	testMaxEntries       = 10
	testSettle           = 500 * time.Millisecond
	testBusyAttempts     = 3
	testBackoffWindow    = 100 * time.Millisecond
	testBackoffDeadline  = 2 * time.Second
	testRetryDelayProbes = 24
)

func newTestCoalescer(flush func(string, map[string]*unstructured.Unstructured) error) *coalescer {
	return newCoalescer(testWindow, testMaxWait, testMaxEntries, nil, nil, nil, flush)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(testSettle)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(testWindow)
	}
	t.Fatal("condition not met before settle timeout")
}

func TestFireFlushRetriesWhileGenLockBusy(t *testing.T) {
	var calls atomic.Int32
	c := newTestCoalescer(func(string, map[string]*unstructured.Unstructured) error {
		if calls.Add(1) < testBusyAttempts {
			return errFlushGenLockBusy
		}
		return nil
	})
	c.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	waitFor(t, func() bool {
		return calls.Load() >= testBusyAttempts && len(c.inMemBuf("app")) == 0
	})
}

func TestFireFlushRetriesAndKeepsBufferOnTransientErrors(t *testing.T) {
	var calls atomic.Int32
	c := newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		if len(buf) != 1 {
			t.Errorf("flush must receive the detached buffer, got %d entries", len(buf))
		}
		if calls.Add(1) < testBusyAttempts {
			return errors.New("exporter unavailable")
		}
		return nil
	})
	c.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	waitFor(t, func() bool {
		return calls.Load() >= testBusyAttempts && len(c.inMemBuf("app")) == 0
	})
}

func TestFlushRearmsOnTransportErrorButDropsWhenStoredNotFound(t *testing.T) {
	var calls, shortBuffers atomic.Int32
	m := newTestManager(func(string) (*applicationmodel.Application, error) {
		if calls.Add(1) < testBusyAttempts {
			return nil, errors.New("exporter unavailable")
		}
		return nil, restshared.ErrNotFound
	})
	m.coalesce = newTestCoalescer(func(app string, buf map[string]*unstructured.Unstructured) error {
		if len(buf) != 1 {
			shortBuffers.Add(1)
		}
		return m.flushApp(app, buf)
	})
	m.coalesce.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	waitFor(t, func() bool {
		return calls.Load() == testBusyAttempts && len(m.coalesce.inMemBuf("app")) == 0
	})
	time.Sleep(testMaxWait)
	if got := calls.Load(); got != testBusyAttempts {
		t.Fatalf("not-found flush re-armed: %d exporter calls, want %d", got, testBusyAttempts)
	}
	if shortBuffers.Load() != 0 {
		t.Fatal("transport-error retry lost the detached buffer")
	}
}

func TestScheduleWithoutPreImageStillFlushes(t *testing.T) {
	var entries atomic.Int32
	c := newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		entries.Store(int32(len(buf)))
		return nil
	})
	c.schedule("app", "configmap/cfg", nil)

	waitFor(t, func() bool { return entries.Load() == 1 })
}

func TestEventsDuringFlushAreKept(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	c := newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		if calls.Add(1) == 1 {
			started <- struct{}{}
			<-release
		}
		if _, first := buf["deploy/a"]; first && len(buf) != 1 {
			t.Errorf("first flush must only carry the pre-flush buffer, got %d entries", len(buf))
		}
		return nil
	})
	c.schedule("app", "deploy/a", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})
	<-started
	c.schedule("app", "deploy/b", &unstructured.Unstructured{Object: map[string]any{"kind": "Service"}})
	close(release)

	waitFor(t, func() bool { return calls.Load() == 2 && len(c.inMemBuf("app")) == 0 })
}

func TestResumeFlushesPastDeadlineBufferInsteadOfDropping(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	var calls atomic.Int32
	c := newCoalescer(testWindow, testMaxWait, testMaxEntries, rdb, nil, nil, func(string, map[string]*unstructured.Unstructured) error {
		calls.Add(1)
		return nil
	})
	buf := map[string]*unstructured.Unstructured{
		"deploy/app": {Object: map[string]any{"kind": "Deployment"}},
	}
	raw, err := encodeCoalescePayload(buf, time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	key := coalesceRedisKey("app")
	if err := rdb.Set(context.Background(), key, raw, 0).Err(); err != nil {
		t.Fatal(err)
	}

	c.resumeOneKey(context.Background(), key)

	waitFor(t, func() bool { return calls.Load() == 1 && !mr.Exists(key) })
}

func newTestManager(getStored func(string) (*applicationmodel.Application, error)) *Manager {
	return &Manager{getStoredApp: getStored, flushLimiter: rate.NewLimiter(rate.Inf, 0)}
}

type fakeSnapshotStore struct {
	creates, deletes atomic.Int32
}

func (f *fakeSnapshotStore) opts() applicationscore.GetApplicationsOptions {
	return applicationscore.GetApplicationsOptions{
		CreateSnapshot: func(id, scope, namespace string, generation int, manifest any) (string, error) {
			f.creates.Add(1)
			return "/snapshots/" + id, nil
		},
		DeleteSnapshot: func(id, scope, namespace string, generation int) error {
			f.deletes.Add(1)
			return nil
		},
	}
}

func testPreImage(replicas int64) map[string][]unstructured.Unstructured {
	obj := func(kind, ns, name string) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": name, "namespace": ns},
			"spec":       map[string]any{"replicas": replicas},
		}}
	}
	return map[string][]unstructured.Unstructured{
		"ns-a": {obj("Deployment", "ns-a", "a")},
		"ns-b": {obj("Deployment", "ns-b", "b")},
	}
}

func TestWritePreSnapshotsReusesPendingSetUntilPublishLands(t *testing.T) {
	mr := miniredis.RunT(t)
	m := newTestManager(nil)
	m.cfg.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := &fakeSnapshotStore{}
	opts := store.opts()
	ctx := context.Background()
	stored := &applicationmodel.Application{}
	const nextGen = 2
	failed := &applicationmodel.Application{CRStatus: applicationmodel.CRStatusFailed}
	landed := &applicationmodel.Application{
		CRStatus: applicationmodel.CRStatusPublished,
		History:  applicationmodel.ApplicationHistory{Generation: nextGen},
	}

	var first []applicationmodel.ApplicationSnapshot
	for attempt := range testBusyAttempts {
		snaps, err := m.writePreSnapshots(ctx, "app", stored, nextGen, testPreImage(1), &opts)
		if err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			first = snaps
		} else if len(snaps) != len(first) || snaps[0].ID != first[0].ID {
			t.Fatalf("attempt %d wrote a new set instead of reusing the pending one", attempt)
		}
		outcome := failed
		if attempt == testBusyAttempts-1 {
			outcome = landed
		}
		m.settlePendingSnapshots(ctx, "app", stored, outcome)
	}
	if got := store.creates.Load(); got != 2 {
		t.Fatalf("CreateSnapshot called %d times across %d attempts, want once per namespace (2)", got, testBusyAttempts)
	}
	if mr.Exists(pendingSnapshotsKey("app")) {
		t.Fatal("pending record survived a landed publish")
	}

	if _, err := m.writePreSnapshots(ctx, "app", stored, nextGen+1, testPreImage(1), &opts); err != nil {
		t.Fatal(err)
	}
	m.settlePendingSnapshots(ctx, "app", stored, failed)
	if _, err := m.writePreSnapshots(ctx, "app", stored, nextGen+1, testPreImage(2), &opts); err != nil {
		t.Fatal(err)
	}
	if got := store.deletes.Load(); got != 2 {
		t.Fatalf("content change deleted %d old snapshots, want 2", got)
	}
	if got := store.creates.Load(); got != 6 {
		t.Fatalf("CreateSnapshot called %d times, want 6 (2 reused set + 2 stale + 2 rewritten)", got)
	}
}

func TestWritePreSnapshotsForgetsSetTheStoreAlreadyReferences(t *testing.T) {
	mr := miniredis.RunT(t)
	m := newTestManager(nil)
	m.cfg.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := &fakeSnapshotStore{}
	opts := store.opts()
	ctx := context.Background()

	snaps, err := m.writePreSnapshots(ctx, "app", &applicationmodel.Application{}, 2, testPreImage(1), &opts)
	if err != nil {
		t.Fatal(err)
	}
	stored := &applicationmodel.Application{Snapshots: snaps}
	if _, err := m.writePreSnapshots(ctx, "app", stored, 3, testPreImage(2), &opts); err != nil {
		t.Fatal(err)
	}
	if got := store.deletes.Load(); got != 0 {
		t.Fatalf("a set the store references was deleted %d times", got)
	}
}

func TestFlushRetryDelayGrowsPerAttemptAndCaps(t *testing.T) {
	fired := make(chan time.Time, testBusyAttempts)
	var calls atomic.Int32
	c := newCoalescer(testBackoffWindow, testMaxWait, testMaxEntries, nil, nil, nil,
		func(string, map[string]*unstructured.Unstructured) error {
			if calls.Add(1) > testBusyAttempts {
				return nil
			}
			fired <- time.Now()
			return errors.New("exporter unavailable")
		})
	c.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	var at [testBusyAttempts]time.Time
	for i := range at {
		select {
		case at[i] = <-fired:
		case <-time.After(testBackoffDeadline):
			t.Fatalf("flush attempt %d never fired", i)
		}
	}
	if first, second := at[1].Sub(at[0]), at[2].Sub(at[1]); second <= first {
		t.Fatalf("second re-arm delay %s must exceed first %s", second, first)
	}
	var last time.Duration
	for range testRetryDelayProbes {
		last = c.retryDelay("capped", testBackoffWindow)
		if last > constants.InformerFlushMaxRetryDelay {
			t.Fatalf("retry delay %s exceeds cap %s", last, constants.InformerFlushMaxRetryDelay)
		}
	}
	if last < constants.InformerFlushMaxRetryDelay/2 {
		t.Fatalf("retry delay never reached the cap: %s", last)
	}
}

func TestFlushSuccessResetsRetryAttempts(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	c := newTestCoalescer(func(string, map[string]*unstructured.Unstructured) error {
		if calls.Add(1) < testBusyAttempts {
			return errors.New("exporter unavailable")
		}
		<-release
		return nil
	})
	attempts := func() int {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.attempts["app"]
	}
	c.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	waitFor(t, func() bool { return attempts() == testBusyAttempts-1 })
	close(release)
	waitFor(t, func() bool { return attempts() == 0 && len(c.inMemBuf("app")) == 0 })
}

func TestFlushRateLimitedRearmsWithBufferIntactAndNoExporterCall(t *testing.T) {
	var exporterCalls, flushes atomic.Int32
	m := newTestManager(func(string) (*applicationmodel.Application, error) {
		exporterCalls.Add(1)
		return nil, restshared.ErrNotFound
	})
	m.flushLimiter = rate.NewLimiter(rate.Every(time.Hour), 1)
	m.flushLimiter.Allow()
	m.coalesce = newCoalescer(testBackoffWindow, testMaxWait, testMaxEntries, nil, nil, nil,
		func(app string, buf map[string]*unstructured.Unstructured) error {
			if flushes.Add(1) > 1 {
				return errFlushStoredMissing
			}
			return m.flushApp(app, buf)
		})
	rearmed := func() bool {
		m.coalesce.mu.Lock()
		defer m.coalesce.mu.Unlock()
		_, ok := m.coalesce.timers["app"]
		return ok
	}
	m.coalesce.schedule("app", "deploy/app", &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}})

	waitFor(t, func() bool {
		return flushes.Load() == 1 && len(m.coalesce.inMemBuf("app")) == 1 && rearmed()
	})
	if got := exporterCalls.Load(); got != 0 {
		t.Fatalf("rate-limited flush reached the exporter %d times", got)
	}
}

func testDeployment(burst string, observedGeneration int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":        "a",
			"namespace":   "ns-a",
			"annotations": map[string]any{"stress/burst": burst},
		},
		// A non-curated integer: a post-image decoded to float64 would diff it against the live int64.
		"spec":   map[string]any{"replicas": int64(0), "progressDeadlineSeconds": int64(600)},
		"status": map[string]any{"observedGeneration": observedGeneration},
	}}
}

// Replays the burst: the flush opened by a controller status write read a
// cache the r2 patch had already reached and published r1 -> r2; the patch's
// own event then re-opened the window with the pre-r2 object.
func TestFlushDropsPreImageTheLastFlushAlreadyRecorded(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	var exporterCalls atomic.Int32
	m := newTestManager(func(string) (*applicationmodel.Application, error) {
		exporterCalls.Add(1)
		return nil, restshared.ErrNotFound
	})
	m.cfg.RDB = rdb
	m.coalesce = newCoalescer(testWindow, testMaxWait, testMaxEntries, rdb, nil, nil, nil)
	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, &unstructured.Unstructured{}, 0, cache.Indexers{})
	m.informers = map[string]cache.SharedIndexInformer{"ns-a": inf}
	ctx := context.Background()
	r1, r2 := testDeployment("r1", 1), testDeployment("r2", 1)
	key := resourceKey(r1)
	published := &applicationmodel.Application{CRStatus: applicationmodel.CRStatusPublished}

	if err := inf.GetIndexer().Add(r2); err != nil {
		t.Fatal(err)
	}
	m.rememberFlushedManifests(ctx, "app", published, []manifestdiff.ManifestPair{{Old: r1, New: r2}})

	// The status write for r2 lands before the stale window flushes; it must not un-record r2.
	if err := inf.GetIndexer().Update(testDeployment("r2", 2)); err != nil {
		t.Fatal(err)
	}
	if err := m.flushApp("app", map[string]*unstructured.Unstructured{key: r1}); err != nil {
		t.Fatal(err)
	}
	if got := exporterCalls.Load(); got != 0 {
		t.Fatalf("already-recorded pre-image reached the exporter %d times", got)
	}

	if err := inf.GetIndexer().Update(testDeployment("r3", 2)); err != nil {
		t.Fatal(err)
	}
	err := m.flushApp("app", map[string]*unstructured.Unstructured{key: r2})
	if !errors.Is(err, errFlushStoredMissing) || exporterCalls.Load() != 1 {
		t.Fatalf("real change did not flush: err=%v, exporter calls=%d", err, exporterCalls.Load())
	}
}

func testApp() applicationmodel.Application {
	return applicationmodel.Application{
		Name:      "app",
		Resources: []applicationmodel.Resource{{Namespace: "ns-a", Kind: "Deployment", Name: "a"}},
	}
}

func testReconcileManager(t *testing.T, live *unstructured.Unstructured) (*Manager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	m := newTestManager(nil)
	m.cfg.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, &unstructured.Unstructured{}, 0, cache.Indexers{})
	if err := inf.GetIndexer().Add(live); err != nil {
		t.Fatal(err)
	}
	m.informers = map[string]cache.SharedIndexInformer{"ns-a": inf}
	return m, mr
}

func testConfigMap(value string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cfg", "namespace": "ns-a"},
		"data":       map[string]any{"key": value},
	}}
}

func awaitFlush(t *testing.T, flushed <-chan map[string]*unstructured.Unstructured) map[string]*unstructured.Unstructured {
	t.Helper()
	select {
	case buf := <-flushed:
		return buf
	case <-time.After(testSettle):
		t.Fatal("missed change never reached the flush")
		return nil
	}
}

// Replays the chaos phase: r2 was recorded, r3 landed inside a leader's
// initial LIST (no MODIFIED, no pre-image) and the controller's status write
// diffed empty. A later flush changed only a ConfigMap, so the Deployment has
// no post-image left and the tick must flush r2 -> r3 from the stored snapshot.
func TestReconcileFlushesChangeMissedWhileUnobserved(t *testing.T) {
	r1, r2, r3 := testDeployment("r1", 1), testDeployment("r2", 1), testDeployment("r3", 1)
	m, mr := testReconcileManager(t, r3)
	newest := applicationmodel.ApplicationSnapshot{Generation: 2, ID: "snap-2", Namespace: "ns-a"}
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		return &applicationmodel.Application{Snapshots: []applicationmodel.ApplicationSnapshot{
			{Generation: 1, ID: "snap-1", Namespace: "ns-a"}, newest,
		}}, nil
	}
	m.getSnapshotManifest = func(_ context.Context, id, _, _ string, _ int) ([]unstructured.Unstructured, error) {
		if id != newest.ID {
			t.Errorf("pre-image read from snapshot %s, want the newest %s", id, newest.ID)
		}
		return []unstructured.Unstructured{*r2}, nil
	}
	flushed := make(chan map[string]*unstructured.Unstructured, 1)
	m.coalesce = newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		flushed <- buf
		return nil
	})
	ctx := context.Background()
	published := &applicationmodel.Application{CRStatus: applicationmodel.CRStatusPublished}
	m.rememberFlushedManifests(ctx, "app", published, []manifestdiff.ManifestPair{{Old: r1, New: r2}})
	cm2 := testConfigMap("v2")
	m.rememberFlushedManifests(ctx, "app", published, []manifestdiff.ManifestPair{{Old: testConfigMap("v1"), New: cm2}})
	fields, err := mr.HKeys(postKey("app"))
	fields = slices.DeleteFunc(fields, func(f string) bool { return f == constants.HistoryPostGenerationField })
	if err != nil || len(fields) != 1 || fields[0] != resourceKey(cm2) {
		t.Fatalf("post-image hash must hold only the last flush's changed resource, got %v (%v)", fields, err)
	}

	m.reconcileRecorded(ctx, []applicationmodel.Application{testApp()})

	buf := awaitFlush(t, flushed)
	pre := buf[resourceKey(r3)]
	if len(buf) != 1 || pre == nil || pre.GetAnnotations()["stress/burst"] != "r2" {
		t.Fatalf("flush pre-image must be the recorded r2, got %v", buf)
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: pre, New: r3}}); len(got) != 1 {
		t.Fatalf("expected exactly one history entry (r2 -> r3), got %d: %v", len(got), got)
	}
}

// The Deployment is what the last flush changed: the newest snapshot holds r1
// for it, the recorded post-image r2. The entry must read exactly r2 -> r3,
// built from the post-image alone: neither the exporter nor the snapshot store is asked.
func TestReconcilePrefersRecordedPostImageOverSnapshot(t *testing.T) {
	r1, r2, r3 := testDeployment("r1", 1), testDeployment("r2", 1), testDeployment("r3", 2)
	m, _ := testReconcileManager(t, r3)
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		t.Error("stored application consulted")
		return nil, restshared.ErrNotFound
	}
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		t.Error("snapshot store consulted")
		return nil, nil
	}
	flushed := make(chan map[string]*unstructured.Unstructured, 1)
	m.coalesce = newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		flushed <- buf
		return nil
	})
	ctx := context.Background()
	published := &applicationmodel.Application{CRStatus: applicationmodel.CRStatusPublished}
	m.rememberFlushedManifests(ctx, "app", published, []manifestdiff.ManifestPair{{Old: r1, New: r2}})

	m.reconcileRecorded(ctx, []applicationmodel.Application{testApp()})

	pre := awaitFlush(t, flushed)[resourceKey(r3)]
	if pre == nil || pre.GetKind() != "Deployment" || pre.GetNamespace() != "ns-a" || pre.GetName() != "a" {
		t.Fatalf("pre-image must be a whole object, got %v", pre)
	}
	got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: pre, New: r3}})
	if len(got) != 1 || !strings.Contains(got[0].Description, "r2 → r3") {
		t.Fatalf("expected exactly one entry reading r2 -> r3, got %v", got)
	}
}

func testStoredApp(snapID string) *applicationmodel.Application {
	return &applicationmodel.Application{
		History:   applicationmodel.ApplicationHistory{Generation: 1},
		Snapshots: []applicationmodel.ApplicationSnapshot{{Generation: 1, ID: snapID, Namespace: "ns-a"}},
	}
}

func rejectFlush(t *testing.T) *coalescer {
	t.Helper()
	return newTestCoalescer(func(string, map[string]*unstructured.Unstructured) error {
		t.Error("flush scheduled although nothing changed")
		return nil
	})
}

// A flush that reaches the exporter proves the pre-image was not dropped as
// already recorded; the not-found answer keeps the test off the publish path.
func assertFlushReachesExporter(t *testing.T, m *Manager, buf map[string]*unstructured.Unstructured) {
	t.Helper()
	var exporterCalls atomic.Int32
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		exporterCalls.Add(1)
		return nil, restshared.ErrNotFound
	}
	err := m.flushApp("app", buf)
	if !errors.Is(err, errFlushStoredMissing) || exporterCalls.Load() != 1 {
		t.Fatalf("flush dropped as already recorded: err=%v exporter calls=%d", err, exporterCalls.Load())
	}
}

// The newest snapshot already holds the live state: the baseline comes from
// it and nothing is published; a fully recorded app then costs nothing.
func TestReconcileBaselinesUnrecordedAppsWithoutPublishing(t *testing.T) {
	r3 := testDeployment("r3", 1)
	m, mr := testReconcileManager(t, r3)
	var exporterCalls, snapshotReads atomic.Int32
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		exporterCalls.Add(1)
		return testStoredApp("snap-1"), nil
	}
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		snapshotReads.Add(1)
		return []unstructured.Unstructured{*r3}, nil
	}
	m.coalesce = rejectFlush(t)
	ctx := context.Background()
	apps := []applicationmodel.Application{testApp()}

	m.reconcileRecorded(ctx, apps)
	key := recordedKey("app")
	if got := mr.HGet(key, resourceKey(r3)); got != manifestdiff.Fingerprint(r3) {
		t.Fatalf("first-seen app not baselined: got %q", got)
	}
	if mr.TTL(key) <= 0 {
		t.Fatal("baseline written without a TTL")
	}
	if exporterCalls.Load() != 1 || snapshotReads.Load() != 1 {
		t.Fatalf("baseline read the store %d times and the snapshot %d times, want once each",
			exporterCalls.Load(), snapshotReads.Load())
	}

	m.reconcileRecorded(ctx, apps)
	time.Sleep(testMaxWait)
	if exporterCalls.Load() != 1 || snapshotReads.Load() != 1 || len(m.coalesce.inMemBuf("app")) != 0 {
		t.Fatalf("live == recorded still did work: exporter=%d snapshots=%d buffered=%d",
			exporterCalls.Load(), snapshotReads.Load(), len(m.coalesce.inMemBuf("app")))
	}
}

// Replays stress-12: apps rediscovered at gen 1 with snapshot s0, one annotate
// round, and the tick landing between the change and its flush. Baselined from
// the live object, the recorded fingerprint equalled s1 and the flush dropped
// its s0 pre-image as already recorded: 30 of 100 apps lost the change.
func TestReconcileBaselinesFromSnapshotNotLiveObject(t *testing.T) {
	s0, s1 := testDeployment("s0", 1), testDeployment("s1", 1)
	m, mr := testReconcileManager(t, s1)
	m.getStoredApp = func(string) (*applicationmodel.Application, error) { return testStoredApp("snap-1"), nil }
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		return []unstructured.Unstructured{*s0}, nil
	}
	flushed := make(chan map[string]*unstructured.Unstructured, 1)
	m.coalesce = newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		flushed <- buf
		return nil
	})
	key := resourceKey(s1)

	m.reconcileRecorded(context.Background(), []applicationmodel.Application{testApp()})

	if got := mr.HGet(recordedKey("app"), key); got != manifestdiff.Fingerprint(s0) {
		t.Fatalf("baseline must be the snapshot state s0, got %q", got)
	}
	// Without any event (a blind window) the tick itself synthesizes s0 -> s1.
	pre := awaitFlush(t, flushed)[key]
	if pre == nil || pre.GetAnnotations()["stress/burst"] != "s0" {
		t.Fatalf("synthesized flush pre-image must be s0, got %v", pre)
	}
	// The event's own flush, carrying s0, must publish rather than be dropped.
	assertFlushReachesExporter(t, m, map[string]*unstructured.Unstructured{key: s0})
}

func TestReconcileLeavesUnpublishedAppUnrecorded(t *testing.T) {
	s0, s1 := testDeployment("s0", 1), testDeployment("s1", 1)
	for name, stored := range map[string]*applicationmodel.Application{"not stored": nil, "no snapshot": {}} {
		t.Run(name, func(t *testing.T) {
			m, mr := testReconcileManager(t, s1)
			m.getStoredApp = func(string) (*applicationmodel.Application, error) {
				if stored == nil {
					return nil, restshared.ErrNotFound
				}
				return stored, nil
			}
			m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
				t.Error("snapshot store consulted")
				return nil, nil
			}
			m.coalesce = rejectFlush(t)
			key := resourceKey(s1)

			m.reconcileRecorded(context.Background(), []applicationmodel.Application{testApp()})

			if mr.Exists(recordedKey("app")) {
				t.Fatalf("never-published app baselined: %q", mr.HGet(recordedKey("app"), key))
			}
			assertFlushReachesExporter(t, m, map[string]*unstructured.Unstructured{key: s0})
		})
	}
}

func TestReconcileBoundsBackfillPerTick(t *testing.T) {
	const total = constants.InformerReconcileBackfillPerTick + 1
	m, _ := testReconcileManager(t, testDeployment("s0", 1))
	apps := make([]applicationmodel.Application, 0, total)
	for i := range total {
		obj := testDeployment("s0", 1)
		obj.SetName("a" + strconv.Itoa(i))
		if err := m.informers["ns-a"].GetIndexer().Add(obj); err != nil {
			t.Fatal(err)
		}
		apps = append(apps, applicationmodel.Application{
			Name:      obj.GetName(),
			Resources: []applicationmodel.Resource{{Namespace: "ns-a", Kind: "Deployment", Name: obj.GetName()}},
		})
	}
	var storedReads atomic.Int32
	m.getStoredApp = func(name string) (*applicationmodel.Application, error) {
		storedReads.Add(1)
		return testStoredApp(name), nil
	}
	m.getSnapshotManifest = func(_ context.Context, id, _, _ string, _ int) ([]unstructured.Unstructured, error) {
		obj := testDeployment("s0", 1)
		obj.SetName(id)
		return []unstructured.Unstructured{*obj}, nil
	}
	m.coalesce = rejectFlush(t)
	ctx := context.Background()

	m.reconcileRecorded(ctx, apps)
	if got := storedReads.Load(); got != constants.InformerReconcileBackfillPerTick {
		t.Fatalf("first tick backfilled %d apps, want the %d cap", got, constants.InformerReconcileBackfillPerTick)
	}
	m.reconcileRecorded(ctx, apps)
	if got := storedReads.Load(); got != total {
		t.Fatalf("second tick left the remaining app unrecorded: %d reads, want %d", got, total)
	}
	m.reconcileRecorded(ctx, apps)
	if got := storedReads.Load(); got != total {
		t.Fatalf("fully recorded apps still read the store: %d reads", got)
	}
}

// The rollback's own writes are dropped; the restored objects are what the
// history describes, so they are recorded as they stand and the next tick
// sees no drift.
func TestRollbackDropRecordsRestoredStateWithoutPublishing(t *testing.T) {
	s0, s1 := testDeployment("s0", 1), testDeployment("s1", 1)
	m, mr := testReconcileManager(t, s1)
	var exporterCalls atomic.Int32
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		exporterCalls.Add(1)
		return nil, restshared.ErrNotFound
	}
	m.coalesce = rejectFlush(t)
	ctx := context.Background()
	key := resourceKey(s1)
	published := &applicationmodel.Application{CRStatus: applicationmodel.CRStatusPublished}
	m.rememberFlushedManifests(ctx, "app", published, []manifestdiff.ManifestPair{{Old: s1, New: s0}})
	if err := mr.Set(constants.KeyPrefixRollbackApplying+"app", "1"); err != nil {
		t.Fatal(err)
	}

	if err := m.flushApp("app", map[string]*unstructured.Unstructured{key: s0}); err != nil {
		t.Fatal(err)
	}
	if exporterCalls.Load() != 0 {
		t.Fatal("rollback's own writes reached the exporter")
	}
	if got := mr.HGet(recordedKey("app"), key); got != manifestdiff.Fingerprint(s1) {
		t.Fatalf("restored state not recorded: got %q", got)
	}
	if mr.Exists(postKey("app")) {
		t.Fatal("stale post-image kept across the rollback")
	}

	m.reconcileRecorded(ctx, []applicationmodel.Application{testApp()})
	time.Sleep(testMaxWait)
	if exporterCalls.Load() != 0 {
		t.Fatal("tick treated the restored state as drift")
	}
}

func withStatus(u *unstructured.Unstructured, field string, value int64) *unstructured.Unstructured {
	out := u.DeepCopy()
	_ = unstructured.SetNestedField(out.Object, value, "status", field)
	return out
}

func TestStatusOnlyUpdateIsResyncArtifact(t *testing.T) {
	old, statusWrite := testDeployment("r3", 2), testDeployment("r3", 3)
	if !isResyncArtifact(old, statusWrite) {
		t.Fatal("observedGeneration write treated as a change")
	}
	if isResyncArtifact(old, withStatus(old, "readyReplicas", 1)) {
		t.Fatal("readiness change treated as an artifact")
	}
	if isResyncArtifact(testDeployment("r2", 2), old) {
		t.Fatal("annotation change treated as an artifact")
	}
	m := newTestManager(nil)
	m.coalesce = newTestCoalescer(func(string, map[string]*unstructured.Unstructured) error {
		t.Error("status-only update reached the flush")
		return nil
	})
	m.onUpdate(old, statusWrite)
	time.Sleep(testMaxWait)
	if len(m.coalesce.inMemBuf("app")) != 0 {
		t.Fatal("status-only update was buffered")
	}
}

// Generation 2 was flushed before post-images carried a generation stamp (or a
// rollback cleared them): snapshot 2 holds r1, live is r2, and nothing says
// whether r2 is the change already published or one that went unobserved. The
// tick must record live and publish nothing rather than re-publish r1 -> r2.
func TestReconcileBaselinesLiveWhenNewestGenerationHasNoPostStamp(t *testing.T) {
	r1, r2 := testDeployment("r1", 1), testDeployment("r2", 1)
	m, mr := testReconcileManager(t, r2)
	var snapshotReads atomic.Int32
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		return &applicationmodel.Application{
			History: applicationmodel.ApplicationHistory{Generation: 2},
			Snapshots: []applicationmodel.ApplicationSnapshot{
				{Generation: 1, ID: "snap-1", Namespace: "ns-a"}, {Generation: 2, ID: "snap-2", Namespace: "ns-a"},
			},
		}, nil
	}
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		snapshotReads.Add(1)
		return []unstructured.Unstructured{*r1}, nil
	}
	m.coalesce = rejectFlush(t)
	ctx := context.Background()
	if err := m.cfg.RDB.HSet(ctx, recordedKey("app"), resourceKey(r1), "stale-fingerprint").Err(); err != nil {
		t.Fatal(err)
	}

	m.reconcileRecorded(ctx, []applicationmodel.Application{testApp()})
	time.Sleep(testMaxWait)

	if got := mr.HGet(recordedKey("app"), resourceKey(r2)); got != manifestdiff.Fingerprint(r2) {
		t.Fatalf("live must be recorded as the baseline, got %q", got)
	}
	if snapshotReads.Load() != 0 || len(m.coalesce.inMemBuf("app")) != 0 {
		t.Fatalf("stale snapshot must not be read (%d) nor flushed (%d buffered)", snapshotReads.Load(), len(m.coalesce.inMemBuf("app")))
	}

	published := &applicationmodel.Application{CRStatus: applicationmodel.CRStatusPublished, History: applicationmodel.ApplicationHistory{Generation: 2}}
	m.rememberFlushedManifests(ctx, "app", published, []manifestdiff.ManifestPair{{Old: testConfigMap("v1"), New: testConfigMap("v2")}})
	if _, gen := m.postImages(ctx, "app"); gen != 2 {
		t.Fatalf("post hash must carry the flushed generation, got %d", gen)
	}
}
