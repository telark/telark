package informers

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/services/discovery/internal/constants"
	applicationscore "github.com/telark/telark/services/discovery/internal/core/applications/core"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/diff"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/manifestdiff"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	"github.com/telark/telark/services/discovery/internal/discovery/listing"
	discoveryshared "github.com/telark/telark/services/discovery/internal/discovery/shared"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

const (
	testWindow              = 5 * time.Millisecond
	testMaxWait             = 50 * time.Millisecond
	testMaxEntries          = 10
	testSettle              = 500 * time.Millisecond
	testBusyAttempts        = 3
	testBackoffWindow       = 100 * time.Millisecond
	testBackoffDeadline     = 2 * time.Second
	testRetryDelayProbes    = 24
	testAppName             = "app"
	testKindDeployment      = "Deployment"
	testKeyKind             = "kind"
	testKeyAPIVersion       = "apiVersion"
	testKeyMetadata         = "metadata"
	testNamespaceA          = "ns-a"
	testResourceName        = "a"
	testRev1                = "r1"
	testRev2                = "r2"
	testRev3                = "r3"
	testSeq0                = "s0"
	testSeq1                = "s1"
	testSnapID1             = "snap-1"
	testBurstAnnotation     = "stress/burst"
	testCreatesAfterRewrite = 6
	testProgressDeadlineSec = 600
	testDeployKey           = "deploy/app"
	testKeyName             = "name"
	testKeyNamespace        = "namespace"
	testRollbackID          = "rbk-1"
	testDeletedVersion      = "10"
	testRecreatedVersion    = "20"
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
	c.schedule(testAppName, testDeployKey, &unstructured.Unstructured{Object: map[string]any{testKeyKind: testKindDeployment}})

	waitFor(t, func() bool {
		return calls.Load() >= testBusyAttempts && len(c.inMemBuf(testAppName)) == 0
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
	c.schedule(testAppName, testDeployKey, &unstructured.Unstructured{Object: map[string]any{testKeyKind: testKindDeployment}})

	waitFor(t, func() bool {
		return calls.Load() >= testBusyAttempts && len(c.inMemBuf(testAppName)) == 0
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
	m.coalesce.schedule(testAppName, testDeployKey, &unstructured.Unstructured{Object: map[string]any{testKeyKind: testKindDeployment}})

	waitFor(t, func() bool {
		return calls.Load() == testBusyAttempts && len(m.coalesce.inMemBuf(testAppName)) == 0
	})
	time.Sleep(testMaxWait)
	if got := calls.Load(); got != testBusyAttempts {
		t.Fatalf("not-found flush re-armed: %d exporter calls, want %d", got, testBusyAttempts)
	}
	if shortBuffers.Load() != constants.DefaultInitValue {
		t.Fatal("transport-error retry lost the detached buffer")
	}
}

func TestScheduleWithoutPreImageStillFlushes(t *testing.T) {
	var entries atomic.Int64
	c := newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		entries.Store(int64(len(buf)))
		return nil
	})
	c.schedule(testAppName, "configmap/cfg", nil)

	waitFor(t, func() bool { return entries.Load() == 1 })
}

func TestEventsDuringFlushAreKept(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, constants.DefaultAddValue)
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
	c.schedule(testAppName, "deploy/a", &unstructured.Unstructured{Object: map[string]any{testKeyKind: testKindDeployment}})
	<-started
	c.schedule(testAppName, "deploy/b", &unstructured.Unstructured{Object: map[string]any{testKeyKind: "Service"}})
	close(release)

	waitFor(t, func() bool { return calls.Load() == 2 && len(c.inMemBuf(testAppName)) == 0 })
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
		testDeployKey: {Object: map[string]any{testKeyKind: testKindDeployment}},
	}
	raw, err := encodeCoalescePayload(buf, time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	key := coalesceRedisKey(testAppName)
	if err := rdb.Set(context.Background(), key, raw, 0).Err(); err != nil {
		t.Fatal(err)
	}

	c.resumeOneKey(context.Background(), key)

	waitFor(t, func() bool { return calls.Load() == 1 && !mr.Exists(key) })
}

func newTestManager(getStored func(string) (*applicationmodel.Application, error)) *Manager {
	return &Manager{getStoredApp: getStored, flushLimiter: rate.NewLimiter(rate.Inf, constants.DefaultInitValue)}
}

type fakeSnapshotStore struct {
	creates, deletes atomic.Int32
}

func (f *fakeSnapshotStore) opts() applicationscore.GetApplicationsOptions {
	return applicationscore.GetApplicationsOptions{
		CreateSnapshot: func(id, _, _ string, _ int, _ any) (string, error) {
			f.creates.Add(constants.DefaultAddValue)
			return "/snapshots/" + id, nil
		},
		DeleteSnapshot: func(string, string, string, int) error {
			f.deletes.Add(constants.DefaultAddValue)
			return nil
		},
	}
}

func testPreImage(replicas int64) map[string][]unstructured.Unstructured {
	obj := func(kind, ns, name string) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]any{
			testKeyAPIVersion: "apps/v1",
			testKeyKind:       kind,
			testKeyMetadata:   map[string]any{testKeyName: name, testKeyNamespace: ns},
			"spec":            map[string]any{"replicas": replicas},
		}}
	}
	return map[string][]unstructured.Unstructured{
		testNamespaceA: {obj(testKindDeployment, testNamespaceA, testResourceName)},
		"ns-b":         {obj(testKindDeployment, "ns-b", "b")},
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
	failed := failedApp()
	landed := &applicationmodel.Application{
		Conditions: publishedApp().Conditions,
		History:    applicationmodel.ApplicationHistory{Generation: nextGen},
	}

	var first []applicationmodel.ApplicationSnapshot
	for attempt := range testBusyAttempts {
		snaps, err := m.writePreSnapshots(ctx, testAppName, stored, nextGen, testPreImage(constants.DefaultAddValue), &opts)
		if err != nil {
			t.Fatal(err)
		}
		if attempt == constants.DefaultInitValue {
			first = snaps
		} else if len(snaps) != len(first) || snaps[constants.DefaultInitValue].ID != first[constants.DefaultInitValue].ID {
			t.Fatalf("attempt %d wrote a new set instead of reusing the pending one", attempt)
		}
		outcome := failed
		if attempt == testBusyAttempts-constants.DefaultAddValue {
			outcome = landed
		}
		m.settlePendingSnapshots(ctx, testAppName, stored, outcome)
	}
	if got := store.creates.Load(); got != constants.TwoValue {
		t.Fatalf("CreateSnapshot called %d times across %d attempts, want once per namespace (2)", got, testBusyAttempts)
	}
	if mr.Exists(pendingSnapshotsKey(testAppName)) {
		t.Fatal("pending record survived a landed publish")
	}
	assertContentChangeRewritesPendingSet(t, m, store, stored, failed, nextGen+1)
}

func assertContentChangeRewritesPendingSet(
	t *testing.T, m *Manager, store *fakeSnapshotStore, stored, failed *applicationmodel.Application, gen int,
) {
	t.Helper()
	ctx := context.Background()
	opts := store.opts()
	if _, err := m.writePreSnapshots(ctx, testAppName, stored, gen, testPreImage(constants.DefaultAddValue), &opts); err != nil {
		t.Fatal(err)
	}
	m.settlePendingSnapshots(ctx, testAppName, stored, failed)
	if _, err := m.writePreSnapshots(ctx, testAppName, stored, gen, testPreImage(constants.TwoValue), &opts); err != nil {
		t.Fatal(err)
	}
	if got := store.deletes.Load(); got != constants.TwoValue {
		t.Fatalf("content change deleted %d old snapshots, want 2", got)
	}
	if got := store.creates.Load(); got != testCreatesAfterRewrite {
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

	snaps, err := m.writePreSnapshots(
		ctx, testAppName, &applicationmodel.Application{}, constants.TwoValue, testPreImage(constants.DefaultAddValue), &opts,
	)
	if err != nil {
		t.Fatal(err)
	}
	stored := &applicationmodel.Application{Snapshots: snaps}
	if _, err := m.writePreSnapshots(ctx, testAppName, stored, constants.ThreeValue, testPreImage(constants.TwoValue), &opts); err != nil {
		t.Fatal(err)
	}
	if got := store.deletes.Load(); got != constants.DefaultInitValue {
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
	c.schedule(testAppName, testDeployKey, &unstructured.Unstructured{Object: map[string]any{testKeyKind: testKindDeployment}})

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
	if last < constants.InformerFlushMaxRetryDelay/constants.TwoValue {
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
		return c.attempts[testAppName]
	}
	c.schedule(testAppName, testDeployKey, &unstructured.Unstructured{Object: map[string]any{testKeyKind: testKindDeployment}})

	waitFor(t, func() bool { return attempts() == testBusyAttempts-1 })
	close(release)
	waitFor(t, func() bool { return attempts() == 0 && len(c.inMemBuf(testAppName)) == 0 })
}

func TestFlushRateLimitedRearmsWithBufferIntactAndNoExporterCall(t *testing.T) {
	var exporterCalls, flushes atomic.Int32
	m := newTestManager(func(string) (*applicationmodel.Application, error) {
		exporterCalls.Add(1)
		return nil, restshared.ErrNotFound
	})
	m.flushLimiter = rate.NewLimiter(rate.Every(time.Hour), constants.DefaultAddValue)
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
		_, ok := m.coalesce.timers[testAppName]
		return ok
	}
	m.coalesce.schedule(testAppName, testDeployKey, &unstructured.Unstructured{Object: map[string]any{testKeyKind: testKindDeployment}})

	waitFor(t, func() bool {
		return flushes.Load() == 1 && len(m.coalesce.inMemBuf(testAppName)) == 1 && rearmed()
	})
	if got := exporterCalls.Load(); got != constants.DefaultInitValue {
		t.Fatalf("rate-limited flush reached the exporter %d times", got)
	}
}

func testDeployment(burst string, observedGeneration int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		testKeyAPIVersion: "apps/v1",
		testKeyKind:       testKindDeployment,
		testKeyMetadata: map[string]any{
			testKeyName:      testResourceName,
			testKeyNamespace: testNamespaceA,
			"annotations":    map[string]any{testBurstAnnotation: burst},
		},
		// A non-curated integer: a post-image decoded to float64 would diff it against the live int64.
		"spec":   map[string]any{"replicas": int64(constants.DefaultInitValue), "progressDeadlineSeconds": int64(testProgressDeadlineSec)},
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
	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, &unstructured.Unstructured{}, constants.DefaultInitValue, cache.Indexers{})
	m.informers = map[string]cache.SharedIndexInformer{testNamespaceA: inf}
	ctx := context.Background()
	r1, r2 := testDeployment(testRev1, constants.DefaultAddValue), testDeployment(testRev2, constants.DefaultAddValue)
	key := resourceKey(r1)
	published := publishedApp()

	if err := inf.GetIndexer().Add(r2); err != nil {
		t.Fatal(err)
	}
	m.rememberFlushedManifests(ctx, testAppName, published, []manifestdiff.ManifestPair{{Old: r1, New: r2}})

	// The status write for r2 lands before the stale window flushes; it must not un-record r2.
	if err := inf.GetIndexer().Update(testDeployment(testRev2, constants.TwoValue)); err != nil {
		t.Fatal(err)
	}
	if err := m.flushApp(testAppName, map[string]*unstructured.Unstructured{key: r1}); err != nil {
		t.Fatal(err)
	}
	if got := exporterCalls.Load(); got != constants.DefaultInitValue {
		t.Fatalf("already-recorded pre-image reached the exporter %d times", got)
	}

	if err := inf.GetIndexer().Update(testDeployment(testRev3, constants.TwoValue)); err != nil {
		t.Fatal(err)
	}
	err := m.flushApp(testAppName, map[string]*unstructured.Unstructured{key: r2})
	if !errors.Is(err, errFlushStoredMissing) || exporterCalls.Load() != constants.DefaultAddValue {
		t.Fatalf("real change did not flush: err=%v, exporter calls=%d", err, exporterCalls.Load())
	}
}

func testApp() applicationmodel.Application {
	return applicationmodel.Application{
		Name:      testAppName,
		Resources: []applicationmodel.Resource{{Namespace: testNamespaceA, Kind: testKindDeployment, Name: testResourceName}},
	}
}

func testReconcileManager(t *testing.T, live *unstructured.Unstructured) (*Manager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	m := newTestManager(nil)
	m.cfg.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	inf := cache.NewSharedIndexInformer(&cache.ListWatch{}, &unstructured.Unstructured{}, constants.DefaultInitValue, cache.Indexers{})
	if err := inf.GetIndexer().Add(live); err != nil {
		t.Fatal(err)
	}
	m.informers = map[string]cache.SharedIndexInformer{testNamespaceA: inf}
	return m, mr
}

func testConfigMap(value string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		testKeyAPIVersion: "v1",
		testKeyKind:       "ConfigMap",
		testKeyMetadata:   map[string]any{testKeyName: "cfg", testKeyNamespace: testNamespaceA},
		"data":            map[string]any{"key": value},
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
	r1 := testDeployment(testRev1, constants.DefaultAddValue)
	r2 := testDeployment(testRev2, constants.DefaultAddValue)
	r3 := testDeployment(testRev3, constants.DefaultAddValue)
	m, mr := testReconcileManager(t, r3)
	newest := applicationmodel.ApplicationSnapshot{Generation: constants.TwoValue, ID: "snap-2", Namespace: testNamespaceA}
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		return &applicationmodel.Application{Snapshots: []applicationmodel.ApplicationSnapshot{
			{Generation: constants.DefaultAddValue, ID: testSnapID1, Namespace: testNamespaceA}, newest,
		}}, nil
	}
	m.getSnapshotManifest = func(_ context.Context, id, _, _ string, _ int) ([]unstructured.Unstructured, error) {
		if id != newest.ID {
			t.Errorf("pre-image read from snapshot %s, want the newest %s", id, newest.ID)
		}
		return []unstructured.Unstructured{*r2}, nil
	}
	flushed := make(chan map[string]*unstructured.Unstructured, constants.DefaultAddValue)
	m.coalesce = newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		flushed <- buf
		return nil
	})
	ctx := context.Background()
	published := publishedApp()
	m.rememberFlushedManifests(ctx, testAppName, published, []manifestdiff.ManifestPair{{Old: r1, New: r2}})
	cm2 := testConfigMap("v2")
	m.rememberFlushedManifests(ctx, testAppName, published, []manifestdiff.ManifestPair{{Old: testConfigMap("v1"), New: cm2}})
	fields, err := mr.HKeys(postKey(testAppName))
	fields = slices.DeleteFunc(fields, func(f string) bool { return f == constants.HistoryPostGenerationField })
	if err != nil || len(fields) != constants.DefaultAddValue || fields[constants.DefaultInitValue] != resourceKey(cm2) {
		t.Fatalf("post-image hash must hold only the last flush's changed resource, got %v (%v)", fields, err)
	}

	m.reconcileRecorded(ctx, []applicationmodel.Application{testApp()})

	buf := awaitFlush(t, flushed)
	pre := buf[resourceKey(r3)]
	if len(buf) != constants.DefaultAddValue || pre == nil || pre.GetAnnotations()[testBurstAnnotation] != testRev2 {
		t.Fatalf("flush pre-image must be the recorded r2, got %v", buf)
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: pre, New: r3}}); len(got) != constants.DefaultAddValue {
		t.Fatalf("expected exactly one history entry (r2 -> r3), got %d: %v", len(got), got)
	}
}

// The Deployment is what the last flush changed: the newest snapshot holds r1
// for it, the recorded post-image r2. The entry must read exactly r2 -> r3,
// built from the post-image alone: neither the exporter nor the snapshot store is asked.
func TestReconcilePrefersRecordedPostImageOverSnapshot(t *testing.T) {
	r1 := testDeployment(testRev1, constants.DefaultAddValue)
	r2 := testDeployment(testRev2, constants.DefaultAddValue)
	r3 := testDeployment(testRev3, constants.TwoValue)
	m, _ := testReconcileManager(t, r3)
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		t.Error("stored application consulted")
		return nil, restshared.ErrNotFound
	}
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		t.Error("snapshot store consulted")
		return nil, nil
	}
	flushed := make(chan map[string]*unstructured.Unstructured, constants.DefaultAddValue)
	m.coalesce = newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		flushed <- buf
		return nil
	})
	ctx := context.Background()
	published := publishedApp()
	m.rememberFlushedManifests(ctx, testAppName, published, []manifestdiff.ManifestPair{{Old: r1, New: r2}})

	m.reconcileRecorded(ctx, []applicationmodel.Application{testApp()})

	pre := awaitFlush(t, flushed)[resourceKey(r3)]
	if pre == nil || pre.GetKind() != testKindDeployment || pre.GetNamespace() != testNamespaceA || pre.GetName() != testResourceName {
		t.Fatalf("pre-image must be a whole object, got %v", pre)
	}
	got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: pre, New: r3}})
	if len(got) != constants.DefaultAddValue || !strings.Contains(got[0].Description, "r2 → r3") {
		t.Fatalf("expected exactly one entry reading r2 -> r3, got %v", got)
	}
}

func testStoredApp(snapID string) *applicationmodel.Application {
	return &applicationmodel.Application{
		History:   applicationmodel.ApplicationHistory{Generation: constants.DefaultAddValue},
		Snapshots: []applicationmodel.ApplicationSnapshot{{Generation: constants.DefaultAddValue, ID: snapID, Namespace: testNamespaceA}},
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
		exporterCalls.Add(constants.DefaultAddValue)
		return nil, restshared.ErrNotFound
	}
	err := m.flushApp(testAppName, buf)
	if !errors.Is(err, errFlushStoredMissing) || exporterCalls.Load() != constants.DefaultAddValue {
		t.Fatalf("flush dropped as already recorded: err=%v exporter calls=%d", err, exporterCalls.Load())
	}
}

// The newest snapshot already holds the live state: the baseline comes from
// it and nothing is published; a fully recorded app then costs nothing.
func TestReconcileBaselinesUnrecordedAppsWithoutPublishing(t *testing.T) {
	r3 := testDeployment(testRev3, constants.DefaultAddValue)
	m, mr := testReconcileManager(t, r3)
	var exporterCalls, snapshotReads atomic.Int32
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		exporterCalls.Add(constants.DefaultAddValue)
		return testStoredApp(testSnapID1), nil
	}
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		snapshotReads.Add(constants.DefaultAddValue)
		return []unstructured.Unstructured{*r3}, nil
	}
	m.coalesce = rejectFlush(t)
	ctx := context.Background()
	apps := []applicationmodel.Application{testApp()}

	m.reconcileRecorded(ctx, apps)
	key := recordedKey(testAppName)
	if got := mr.HGet(key, resourceKey(r3)); got != manifestdiff.Fingerprint(r3) {
		t.Fatalf("first-seen app not baselined: got %q", got)
	}
	if mr.TTL(key) <= constants.DefaultInitValue {
		t.Fatal("baseline written without a TTL")
	}
	if exporterCalls.Load() != constants.DefaultAddValue || snapshotReads.Load() != constants.DefaultAddValue {
		t.Fatalf("baseline read the store %d times and the snapshot %d times, want once each",
			exporterCalls.Load(), snapshotReads.Load())
	}

	m.reconcileRecorded(ctx, apps)
	time.Sleep(testMaxWait)
	if exporterCalls.Load() != constants.DefaultAddValue || snapshotReads.Load() != constants.DefaultAddValue ||
		len(m.coalesce.inMemBuf(testAppName)) != constants.DefaultInitValue {
		t.Fatalf("live == recorded still did work: exporter=%d snapshots=%d buffered=%d",
			exporterCalls.Load(), snapshotReads.Load(), len(m.coalesce.inMemBuf(testAppName)))
	}
}

// Replays stress-12: apps rediscovered at gen 1 with snapshot s0, one annotate
// round, and the tick landing between the change and its flush. Baselined from
// the live object, the recorded fingerprint equaled s1 and the flush dropped
// its s0 pre-image as already recorded: 30 of 100 apps lost the change.
func TestReconcileBaselinesFromSnapshotNotLiveObject(t *testing.T) {
	s0, s1 := testDeployment(testSeq0, constants.DefaultAddValue), testDeployment(testSeq1, constants.DefaultAddValue)
	m, mr := testReconcileManager(t, s1)
	m.getStoredApp = func(string) (*applicationmodel.Application, error) { return testStoredApp(testSnapID1), nil }
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		return []unstructured.Unstructured{*s0}, nil
	}
	flushed := make(chan map[string]*unstructured.Unstructured, constants.DefaultAddValue)
	m.coalesce = newTestCoalescer(func(_ string, buf map[string]*unstructured.Unstructured) error {
		flushed <- buf
		return nil
	})
	key := resourceKey(s1)

	m.reconcileRecorded(context.Background(), []applicationmodel.Application{testApp()})

	if got := mr.HGet(recordedKey(testAppName), key); got != manifestdiff.Fingerprint(s0) {
		t.Fatalf("baseline must be the snapshot state s0, got %q", got)
	}
	// Without any event (a blind window) the tick itself synthesizes s0 -> s1.
	pre := awaitFlush(t, flushed)[key]
	if pre == nil || pre.GetAnnotations()[testBurstAnnotation] != testSeq0 {
		t.Fatalf("synthesized flush pre-image must be s0, got %v", pre)
	}
	// The event's own flush, carrying s0, must publish rather than be dropped.
	assertFlushReachesExporter(t, m, map[string]*unstructured.Unstructured{key: s0})
}

func TestReconcileLeavesUnpublishedAppUnrecorded(t *testing.T) {
	s0, s1 := testDeployment(testSeq0, constants.DefaultAddValue), testDeployment(testSeq1, constants.DefaultAddValue)
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

			if mr.Exists(recordedKey(testAppName)) {
				t.Fatalf("never-published app baselined: %q", mr.HGet(recordedKey(testAppName), key))
			}
			assertFlushReachesExporter(t, m, map[string]*unstructured.Unstructured{key: s0})
		})
	}
}

func TestReconcileBoundsBackfillPerTick(t *testing.T) {
	const total = constants.InformerReconcileBackfillPerTick + 1
	m, _ := testReconcileManager(t, testDeployment(testSeq0, constants.DefaultAddValue))
	apps := make([]applicationmodel.Application, constants.DefaultInitValue, total)
	for i := range total {
		obj := testDeployment(testSeq0, constants.DefaultAddValue)
		obj.SetName(testResourceName + strconv.Itoa(i))
		if err := m.informers[testNamespaceA].GetIndexer().Add(obj); err != nil {
			t.Fatal(err)
		}
		apps = append(apps, applicationmodel.Application{
			Name:      obj.GetName(),
			Resources: []applicationmodel.Resource{{Namespace: testNamespaceA, Kind: testKindDeployment, Name: obj.GetName()}},
		})
	}
	var storedReads atomic.Int32
	m.getStoredApp = func(name string) (*applicationmodel.Application, error) {
		storedReads.Add(constants.DefaultAddValue)
		return testStoredApp(name), nil
	}
	m.getSnapshotManifest = func(_ context.Context, id, _, _ string, _ int) ([]unstructured.Unstructured, error) {
		obj := testDeployment(testSeq0, constants.DefaultAddValue)
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
// The rollback's own writes are no longer dropped: the flush carries the marker to
// the diff, which records them as the rollback entry with the pre-rollback objects
// as its snapshot; a marker the controller did not write (old "1") is ignored.
func TestRollbackMarkerFlushRecordsRestoredState(t *testing.T) {
	s0, s1 := testDeployment(testSeq0, constants.DefaultAddValue), testDeployment(testSeq1, constants.DefaultAddValue)
	m, mr := testReconcileManager(t, s1)
	m.coalesce = rejectFlush(t)
	ctx := context.Background()
	key := resourceKey(s1)
	published := publishedApp()
	m.rememberFlushedManifests(ctx, testAppName, published, []manifestdiff.ManifestPair{{Old: s1, New: s0}})
	if err := mr.Set(constants.KeyPrefixRollbackApplying+testAppName, "1"); err != nil {
		t.Fatal(err)
	}
	if m.rollbackMarker(ctx, testAppName) != nil {
		t.Fatal("legacy marker decoded as a rollback")
	}
	raw, err := json.Marshal(diff.RollbackMarker{ID: testRollbackID, TriggeredBy: testAppName})
	if err != nil {
		t.Fatal(err)
	}
	if err := mr.Set(constants.KeyPrefixRollbackApplying+testAppName, string(raw)); err != nil {
		t.Fatal(err)
	}
	if got := m.rollbackMarker(ctx, testAppName); got == nil || got.ID != testRollbackID {
		t.Fatalf("marker not decoded: %+v", got)
	}
	assertFlushReachesExporter(t, m, map[string]*unstructured.Unstructured{key: s0})
}

// A readiness move on a manifest the last flush already recorded is the health
// change the update handler let through: it is flushed against the recorded
// manifest (live) rather than dropped with the pre-patch object or diffed twice.
func TestRecordedPreImageKeepsReadinessMovesOnly(t *testing.T) {
	old := testDeployment(testRev1, constants.DefaultAddValue)
	if recordedPreImage(old, testDeployment(testRev1, constants.TwoValue)) != nil {
		t.Fatal("status-only write kept as a pre-image")
	}
	live := withStatus(old, "readyReplicas", constants.DefaultAddValue)
	pre := recordedPreImage(old, live)
	if pre == nil || manifestdiff.Fingerprint(pre) != manifestdiff.Fingerprint(live) {
		t.Fatalf("readiness move dropped: %v", pre)
	}
}

func withStatus(u *unstructured.Unstructured, field string, value int64) *unstructured.Unstructured {
	out := u.DeepCopy()
	_ = unstructured.SetNestedField(out.Object, value, "status", field)
	return out
}

func TestStatusOnlyUpdateIsResyncArtifact(t *testing.T) {
	old, statusWrite := testDeployment(testRev3, constants.TwoValue), testDeployment(testRev3, constants.ThreeValue)
	if !isResyncArtifact(old, statusWrite) {
		t.Fatal("observedGeneration write treated as a change")
	}
	if isResyncArtifact(old, withStatus(old, "readyReplicas", constants.DefaultAddValue)) {
		t.Fatal("readiness change treated as an artifact")
	}
	if isResyncArtifact(testDeployment(testRev2, constants.TwoValue), old) {
		t.Fatal("annotation change treated as an artifact")
	}
	m := newTestManager(nil)
	m.coalesce = newTestCoalescer(func(string, map[string]*unstructured.Unstructured) error {
		t.Error("status-only update reached the flush")
		return nil
	})
	m.onUpdate(old, statusWrite)
	time.Sleep(testMaxWait)
	if len(m.coalesce.inMemBuf(testAppName)) != constants.DefaultInitValue {
		t.Fatal("status-only update was buffered")
	}
}

// Generation 2 was flushed before post-images carried a generation stamp (or a
// rollback cleared them): snapshot 2 holds r1, live is r2, and nothing says
// whether r2 is the change already published or one that went unobserved. The
// tick must record live and publish nothing rather than re-publish r1 -> r2.
func TestReconcileBaselinesLiveWhenNewestGenerationHasNoPostStamp(t *testing.T) {
	r1, r2 := testDeployment(testRev1, constants.DefaultAddValue), testDeployment(testRev2, constants.DefaultAddValue)
	m, mr := testReconcileManager(t, r2)
	var snapshotReads atomic.Int32
	m.getStoredApp = func(string) (*applicationmodel.Application, error) {
		return &applicationmodel.Application{
			History: applicationmodel.ApplicationHistory{Generation: constants.TwoValue},
			Snapshots: []applicationmodel.ApplicationSnapshot{
				{Generation: constants.DefaultAddValue, ID: testSnapID1, Namespace: testNamespaceA},
				{Generation: constants.TwoValue, ID: "snap-2", Namespace: testNamespaceA},
			},
		}, nil
	}
	m.getSnapshotManifest = func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		snapshotReads.Add(constants.DefaultAddValue)
		return []unstructured.Unstructured{*r1}, nil
	}
	m.coalesce = rejectFlush(t)
	ctx := context.Background()
	if err := m.cfg.RDB.HSet(ctx, recordedKey(testAppName), resourceKey(r1), "stale-fingerprint").Err(); err != nil {
		t.Fatal(err)
	}

	m.reconcileRecorded(ctx, []applicationmodel.Application{testApp()})
	time.Sleep(testMaxWait)

	if got := mr.HGet(recordedKey(testAppName), resourceKey(r2)); got != manifestdiff.Fingerprint(r2) {
		t.Fatalf("live must be recorded as the baseline, got %q", got)
	}
	if snapshotReads.Load() != constants.DefaultInitValue || len(m.coalesce.inMemBuf(testAppName)) != constants.DefaultInitValue {
		t.Fatalf("stale snapshot must not be read (%d) nor flushed (%d buffered)", snapshotReads.Load(), len(m.coalesce.inMemBuf(testAppName)))
	}

	published := &applicationmodel.Application{
		Conditions: publishedApp().Conditions,
		History:    applicationmodel.ApplicationHistory{Generation: constants.TwoValue},
	}
	m.rememberFlushedManifests(ctx, testAppName, published, []manifestdiff.ManifestPair{{Old: testConfigMap("v1"), New: testConfigMap("v2")}})
	if _, gen := m.postImages(ctx, testAppName); gen != constants.TwoValue {
		t.Fatalf("post hash must carry the flushed generation, got %d", gen)
	}
}

func publishedApp() *applicationmodel.Application {
	app := &applicationmodel.Application{}
	applicationscore.MarkPublished(app)
	return app
}

func failedApp() *applicationmodel.Application {
	app := &applicationmodel.Application{}
	applicationscore.MarkPublishFailed(app)
	return app
}

// Each flush hands its buffer to before first, which may block it or end it as a no-inputs drop:
// reaching that drop through the real flush waits on NATS.
func newDropTestCoalescer(window time.Duration, before func(map[string]*unstructured.Unstructured) (noInputs bool)) *coalescer {
	m := newTestManager(func(string) (*applicationmodel.Application, error) { return nil, restshared.ErrNotFound })
	m.coalesce = newCoalescer(window, window, testMaxEntries, nil, nil, nil,
		func(app string, buf map[string]*unstructured.Unstructured) error {
			if before(buf) {
				return errFlushNoInputs
			}
			return m.flushApp(app, buf)
		})
	return m.coalesce
}

func testWorkload(version string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetKind(testKindDeployment)
	u.SetResourceVersion(version)
	return u
}

// A recreate landing while the delete's flush found no inputs (or no stored app yet) was wiped
// with that flush, and its timer with it; it now flushes on its own, carrying the delete's pre-image.
func TestEventsDuringDroppedFlushInheritItsPreImages(t *testing.T) {
	for name, noInputs := range map[string]bool{"stored missing": false, "no inputs": true} {
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			started := make(chan struct{}, constants.DefaultAddValue)
			next := make(chan map[string]*unstructured.Unstructured, constants.DefaultAddValue)
			first := true
			c := newDropTestCoalescer(testWindow, func(buf map[string]*unstructured.Unstructured) bool {
				if !first {
					next <- buf
					return false
				}
				first = false
				started <- struct{}{}
				<-release
				return noInputs
			})
			c.schedule(testAppName, testDeployKey, testWorkload(testDeletedVersion))
			<-started
			c.schedule(testAppName, testDeployKey, nil)
			close(release)

			select {
			case buf := <-next:
				if got := buf[testDeployKey]; got == nil || got.GetResourceVersion() != testDeletedVersion {
					t.Fatalf("recreate flushed without the deleted pre-image: %v", got)
				}
			case <-time.After(testSettle):
				t.Fatal("recreate never flushed")
			}
		})
	}
}

// The recreate's first event landing just after the delete's flush was dropped for lack of
// inputs opened a buffer without the deleted pre-image, so its flush snapshotted the live object.
func TestRecreateAfterNoInputsDropInheritsTheDeletedPreImage(t *testing.T) {
	cases := []struct {
		name     string
		noInputs bool
		recreate *unstructured.Unstructured
		inherits bool
	}{
		{name: "added after no inputs", noInputs: true, inherits: true},
		{name: "modified after no inputs", noInputs: true, recreate: testWorkload(testRecreatedVersion), inherits: true},
		// No stored app means no history to keep: nothing is held.
		{name: "added after stored missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var next map[string]*unstructured.Unstructured
			dropped := false
			c := newDropTestCoalescer(time.Hour, func(buf map[string]*unstructured.Unstructured) bool {
				if dropped {
					next = buf
					return true
				}
				dropped = true
				return tc.noInputs
			})
			c.schedule(testAppName, testDeployKey, testWorkload(testDeletedVersion))
			c.fireFlush(testAppName)
			c.schedule(testAppName, testDeployKey, tc.recreate)
			c.fireFlush(testAppName)
			pre := next[testDeployKey]
			if got := pre != nil && pre.GetResourceVersion() == testDeletedVersion; got != tc.inherits {
				t.Fatalf("deleted pre-image flushed = %v, want %v", got, tc.inherits)
			}
		})
	}
}

// Without Redis, held pre-images are kept in memory for one window only: a recreate arriving later flushes on its own.
func TestNoInputsDropHoldsPreImagesForOneWindow(t *testing.T) {
	flushed := make(chan map[string]*unstructured.Unstructured, constants.TwoValue)
	c := newDropTestCoalescer(testWindow, func(buf map[string]*unstructured.Unstructured) bool {
		flushed <- buf
		return true
	})
	c.schedule(testAppName, testDeployKey, testWorkload(testDeletedVersion))
	<-flushed
	time.Sleep(testSettle)
	c.schedule(testAppName, testDeployKey, nil)
	select {
	case buf := <-flushed:
		if buf[testDeployKey] != nil {
			t.Fatalf("expired pre-image still flushed: %v", buf[testDeployKey])
		}
	case <-time.After(testSettle):
		t.Fatal("recreate never flushed")
	}
}

func awaitDropped(t *testing.T, flushed <-chan map[string]*unstructured.Unstructured) map[string]*unstructured.Unstructured {
	t.Helper()
	select {
	case buf := <-flushed:
		return buf
	case <-time.After(testSettle):
		t.Fatal("no flush")
		return nil
	}
}

// Seen live (E-17): a recreate landing past the window, or after another leader took over, lost the
// deleted pre-image and its change entry; the hold lived one window in the memory of the dropping leader.
func TestNoInputsDropSurvivesTheWindowAndALeaderChange(t *testing.T) {
	for _, nextLeader := range []bool{false, true} {
		t.Run("nextLeader="+strconv.FormatBool(nextLeader), func(t *testing.T) {
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			flushed := make(chan map[string]*unstructured.Unstructured, constants.TwoValue)
			drop := func(buf map[string]*unstructured.Unstructured) bool {
				flushed <- buf
				return true
			}
			dropper := newDropTestCoalescer(testWindow, drop)
			dropper.rdb = rdb
			dropper.schedule(testAppName, testDeployKey, testWorkload(testDeletedVersion))
			awaitDropped(t, flushed)
			waitFor(t, func() bool { return mr.Exists(heldRedisKey(testAppName)) })
			testutil.Equal(t, "held drop expires", mr.TTL(heldRedisKey(testAppName)), constants.CoalesceHeldTTL)

			recreated := dropper
			if nextLeader {
				recreated = newDropTestCoalescer(testWindow, drop)
				recreated.rdb = rdb
				recreated.resumeFromRedis(context.Background())
			} else {
				time.Sleep(testMaxWait)
			}
			recreated.schedule(testAppName, testDeployKey, nil)
			if pre := awaitDropped(t, flushed)[testDeployKey]; pre == nil || pre.GetResourceVersion() != testDeletedVersion {
				t.Fatalf("recreate flushed without the deleted pre-image: %v", pre)
			}
		})
	}
}

// A force sync restarts from live state: the drop it clears must not reach a later flush.
func TestForceSyncForgetsTheHeldDrop(t *testing.T) {
	mr := miniredis.RunT(t)
	flushed := make(chan map[string]*unstructured.Unstructured, constants.TwoValue)
	c := newDropTestCoalescer(testWindow, func(buf map[string]*unstructured.Unstructured) bool {
		flushed <- buf
		return true
	})
	c.rdb = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c.schedule(testAppName, testDeployKey, testWorkload(testDeletedVersion))
	awaitDropped(t, flushed)
	waitFor(t, func() bool { return mr.Exists(heldRedisKey(testAppName)) })

	c.clearBufferRedis(testAppName)
	testutil.Equal(t, "drop gone from Redis", mr.Exists(heldRedisKey(testAppName)), false)
	c.schedule(testAppName, testDeployKey, nil)
	if pre := awaitDropped(t, flushed)[testDeployKey]; pre != nil {
		t.Fatalf("cleared drop still flushed: %v", pre)
	}
}

// Only this package can fill a manager's informers, so the app-index tests live here.
const (
	idxApp           = "rl-skew"
	idxJobNS         = "recs-lab"
	idxOtherNS       = "recs-lab-prod"
	idxNeighborApp   = "cart"
	idxNeighborNS    = "shop"
	idxThirdApp      = "billing"
	idxHiddenApp     = "coredns"
	idxExcludedNS    = "kube-system"
	idxMissingApp    = "ghost"
	idxLabelAppName  = "app.kubernetes.io/name"
	idxKeyLabels     = "labels"
	idxAPIVersionV1  = "v1"
	idxKindService   = "Service"
	idxKindSA        = "ServiceAccount"
	idxKindNetPolicy = "NetworkPolicy"
	idxSuffixService = "-svc"
	idxSuffixSA      = "-sa"
	idxSuffixNetPol  = "-np"
	idxObjectsPerNS  = 4
	idxWrites        = 2000
	idxBenchSmall    = 100
	idxBenchLarge    = 10000
	idxBenchFullPass = "fullpass-"
	idxBenchIndexed  = "indexed-"
)

func idxObject(kind, namespace, name, app string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		testKeyAPIVersion: idxAPIVersionV1,
		testKeyKind:       kind,
		testKeyMetadata: map[string]any{
			testKeyName:      name,
			testKeyNamespace: namespace,
			idxKeyLabels:     map[string]any{idxLabelAppName: app},
		},
	}}
}

func skewObjects(namespace string) []*unstructured.Unstructured {
	return []*unstructured.Unstructured{
		idxObject(testKindDeployment, namespace, idxApp, idxApp),
		idxObject(idxKindService, namespace, idxApp+idxSuffixService, idxApp),
		idxObject(idxKindSA, namespace, idxApp+idxSuffixSA, idxApp),
		idxObject(idxKindNetPolicy, namespace, idxApp+idxSuffixNetPol, idxApp),
	}
}

// Installs, as Run would, a manager whose informer holds the objects, and wires listing to it as main does.
// Discovery keeps the first excluded list it loads, so every test here shares idxExcludedNS.
func useCluster(tb testing.TB, extra ...*unstructured.Unstructured) cache.Indexer {
	tb.Helper()
	testutil.ExcludedNamespaces(tb, idxExcludedNS)
	inf := cache.NewSharedIndexInformerWithOptions(
		&cache.ListWatch{}, &unstructured.Unstructured{}, cache.SharedIndexInformerOptions{Indexers: appIndexers()},
	)
	objs := slices.Concat(skewObjects(idxJobNS), skewObjects(idxOtherNS), []*unstructured.Unstructured{
		idxObject(testKindDeployment, idxNeighborNS, idxNeighborApp, idxNeighborApp),
		idxObject(testKindDeployment, idxExcludedNS, idxHiddenApp, idxHiddenApp),
	}, extra)
	for _, obj := range objs {
		if err := inf.GetIndexer().Add(obj); err != nil {
			tb.Fatal(err)
		}
	}
	informerMu.Lock()
	globalM = &Manager{informers: map[string]cache.SharedIndexInformer{constants.EmptyString: inf}}
	informerMu.Unlock()
	listing.InformersCache, listing.AppNamespacesCache = TryListResourcesInNamespaces, AppNamespaces
	tb.Cleanup(func() {
		listing.InformersCache, listing.AppNamespacesCache = nil, nil
		informerMu.Lock()
		globalM = nil
		informerMu.Unlock()
	})
	return inf.GetIndexer()
}

// The full cache pass the app index replaced; the reference every lookup must match.
func fullPass(ctx context.Context, idx cache.Indexer, app string) []string {
	if app == constants.EmptyString {
		return nil
	}
	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	found := make(map[string]struct{})
	for _, it := range idx.List() {
		u, ok := it.(*unstructured.Unstructured)
		if ok && derivation.AppKey(u.GetLabels()) == app && !slices.Contains(excluded, u.GetNamespace()) {
			found[u.GetNamespace()] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(found))
}

func assertMatchesFullPass(ctx context.Context, t *testing.T, idx cache.Indexer) {
	t.Helper()
	for _, app := range []string{idxApp, idxNeighborApp, idxThirdApp, idxHiddenApp, idxMissingApp, constants.EmptyString} {
		got, want := AppNamespaces(ctx, app), fullPass(ctx, idx, app)
		if !slices.Equal(got, want) {
			t.Fatalf("app %q: indexed %v, full pass %v", app, got, want)
		}
	}
}

// Every mutation the informer store sees (a label moving an object between apps,
// a delete emptying a namespace, a label losing its identity) must keep the
// index equal to a full pass, with an excluded namespace filtered on both sides.
func TestAppNamespacesMatchesFullPassAcrossCacheChanges(t *testing.T) {
	ctx := context.Background()
	idx := useCluster(t,
		idxObject(testKindDeployment, idxJobNS, idxNeighborApp, idxNeighborApp),
		idxObject(testKindDeployment, idxExcludedNS, idxThirdApp, idxThirdApp),
	)
	steps := []struct {
		name   string
		mutate func() error
		app    string
		want   []string
	}{
		{name: "initial", mutate: func() error { return nil },
			app: idxApp, want: []string{idxJobNS, idxOtherNS}},
		{name: "label moves an object to another app",
			mutate: func() error { return idx.Update(idxObject(testKindDeployment, idxJobNS, idxNeighborApp, idxThirdApp)) },
			app:    idxThirdApp, want: []string{idxJobNS}},
		{name: "deletes empty a namespace", mutate: func() error {
			for _, obj := range skewObjects(idxOtherNS) {
				if err := idx.Delete(obj); err != nil {
					return err
				}
			}
			return nil
		}, app: idxApp, want: []string{idxJobNS}},
		{name: "label loses its identity",
			mutate: func() error {
				return idx.Update(idxObject(testKindDeployment, idxJobNS, idxNeighborApp, constants.EmptyString))
			},
			app: idxThirdApp, want: nil},
	}
	for _, step := range steps {
		if err := step.mutate(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		got := AppNamespaces(ctx, step.app)
		if !slices.Equal(got, step.want) {
			t.Fatalf("%s: app %q = %v, want %v", step.name, step.app, got, step.want)
		}
		assertMatchesFullPass(ctx, t, idx)
	}
}

func TestAppNamespacesUnderConcurrentCacheWrites(t *testing.T) {
	ctx := context.Background()
	idx := useCluster(t)
	apps := []string{idxNeighborApp, idxThirdApp}
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range idxWrites {
			_ = idx.Update(idxObject(testKindDeployment, idxJobNS, idxNeighborApp, apps[i%constants.TwoValue]))
		}
	})
	wg.Go(func() {
		for range idxWrites {
			AppNamespaces(ctx, idxApp)
		}
	})
	wg.Wait()
	assertMatchesFullPass(ctx, t, idx)
}

func benchObjects(others int) []*unstructured.Unstructured {
	objs := make([]*unstructured.Unstructured, constants.DefaultInitValue, others)
	for i := range others {
		name := idxNeighborApp + strconv.Itoa(i)
		objs = append(objs, idxObject(testKindDeployment, idxNeighborNS, name, name))
	}
	return objs
}

// Per-call cost must not grow with the number of other apps in the cache.
func BenchmarkAppNamespaces(b *testing.B) {
	ctx := context.Background()
	for _, others := range []int{idxBenchSmall, idxBenchLarge} {
		idx := useCluster(b, benchObjects(others)...)
		b.Run(idxBenchIndexed+strconv.Itoa(others), func(b *testing.B) {
			for b.Loop() {
				AppNamespaces(ctx, idxApp)
			}
		})
		b.Run(idxBenchFullPass+strconv.Itoa(others), func(b *testing.B) {
			for b.Loop() {
				fullPass(ctx, idx, idxApp)
			}
		})
	}
}

// A per-app job carries one namespace of its app (F2: rl-skew in recs-lab and
// recs-lab-prod). Listing only that one published the app without the other.
func TestAppJobListsEveryNamespaceOfItsApp(t *testing.T) {
	useCluster(t)
	ctx := context.Background()

	refs, err := listing.Resources(ctx, listing.AppNamespaces(ctx, idxApp, []string{idxJobNS}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	perNamespace := make(map[string]int)
	for _, g := range derivation.GroupByWorkloadAnchor(discoveryshared.ToDerivationInputs(refs)) {
		if g.Group == idxApp {
			perNamespace[g.Namespace]++
		}
	}
	testutil.Equal(t, "namespaces", len(perNamespace), constants.TwoValue)
	testutil.Equal(t, idxJobNS, perNamespace[idxJobNS], idxObjectsPerNS)
	testutil.Equal(t, idxOtherNS, perNamespace[idxOtherNS], idxObjectsPerNS)
}

func TestAppNamespaces(t *testing.T) {
	cases := []struct {
		name  string
		app   string
		known []string
		want  []string
	}{
		{name: "adds the app's other namespace", app: idxApp,
			known: []string{idxJobNS}, want: []string{idxJobNS, idxOtherNS}},
		{name: "keeps known namespaces the cache lacks", app: idxNeighborApp,
			known: []string{idxJobNS}, want: []string{idxJobNS, idxNeighborNS}},
		{name: "never adds an excluded namespace", app: idxHiddenApp,
			known: []string{idxJobNS}, want: []string{idxJobNS}},
		{name: "unknown app adds nothing", app: constants.EmptyString,
			known: []string{idxJobNS}, want: []string{idxJobNS}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useCluster(t)
			got := listing.AppNamespaces(context.Background(), tc.app, tc.known)
			testutil.Equal(t, "namespaces", slices.Equal(got, tc.want), true)
		})
	}
}
