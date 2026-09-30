package diffpure

import (
	"context"
	"slices"
	"testing"
	"time"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	historyshared "github.com/telark/discovery/internal/core/applications/history/shared"
	"github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	healthyStatus   = "healthy"
	downStatus      = "down"
	stuckApp        = "stuck"
	rediscovered    = "reborn"
	staleFloor      = "5"
	singleSnapshot  = 1
	httpPort        = 8080
	outcomeField    = "outcome"
	generationField = "generation"
)

const (
	deletedImage     = "registry.k8s.io/pause:3.9"
	recreatedImage   = "registry.k8s.io/pause:3.10"
	preImageSnapshot = "snap-pre"
)

func serveManifestsFromCache(t *testing.T) {
	t.Helper()
	snapshot.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": name, "namespace": ns},
		}, true
	}
	t.Cleanup(func() { snapshot.GetManifestFromCache = nil })
}

// An app whose readiness recovered while no informer pre-image reached the flush
// (ollama: incident state set, health stuck on "down") was deferred on every tick
// for a day. The manifests are as recorded, so the tick snapshots live and authors
// the recovery instead of leaving the app down.
func TestDiffApplicationsHealthOnlyChangeSnapshotsLiveWithoutPreImage(t *testing.T) {
	serveManifestsFromCache(t)
	created := constants.DefaultInitValue
	rdb := newRedis(t)
	ctx := context.Background()
	incidentKey := constants.KeyPrefixIncidentState + stuckApp
	if err := rdb.Set(ctx, incidentKey, constants.IncidentStateValueIncident, constants.IncidentStateTTL).Err(); err != nil {
		t.Fatal(err)
	}
	base := appresource.Application{
		Name:      stuckApp,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		History:   appresource.ApplicationHistory{Generation: constants.ThreeValue},
	}
	stored := base
	stored.Health = appresource.Health{Status: downStatus}
	fresh := base
	fresh.Health = appresource.Health{Status: healthyStatus}

	history, snaps, outcome := diff.DiffApplications(
		ctx, noopBaseline, recordingCreate(&created), emptyManifest, rdb, &stored, fresh, nil,
	)
	testutil.Equal(t, outcomeField, outcome, diff.OutcomeAuthored)
	testutil.Equal(t, generationField, history.Generation, sealedGeneration)
	testutil.Equal(t, "live snapshot written", created, singleSnapshot)
	testutil.Equal(t, "snapshot for the recovery generation", hasGeneration(snaps, sealedGeneration), true)
	entry := diff.LastChangeLogEntry(history)
	testutil.Equal(t, "recovery recorded", entry != nil && entry.IsRecovery, true)
	state, err := rdb.Get(ctx, incidentKey).Result()
	testutil.Equal(t, "incident state read", err, nil)
	testutil.Equal(t, "incident closed", state, constants.IncidentStateValueHealthy)
}

// A readiness move is nobody's write: the entry carries the detection time, not the
// annotation of the change that started the rollout.
func TestDiffApplicationsHealthOnlyEntryIsUnattributed(t *testing.T) {
	serveManifestsFromCache(t)
	created := constants.DefaultInitValue
	rdb := newRedis(t)
	ctx := context.Background()
	incidentKey := constants.KeyPrefixIncidentState + stuckApp
	if err := rdb.Set(ctx, incidentKey, constants.IncidentStateValueIncident, constants.IncidentStateTTL).Err(); err != nil {
		t.Fatal(err)
	}
	base := appresource.Application{
		Name:      stuckApp,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		History: appresource.ApplicationHistory{
			Generation:     constants.ThreeValue,
			LastModifiedBy: annotatedUser,
			LastModifiedAt: time.Now().Add(-annotationAge).UTC().Format(time.RFC3339),
		},
	}
	stored := base
	stored.Health = appresource.Health{Status: downStatus}
	fresh := base
	fresh.Health = appresource.Health{Status: healthyStatus}

	history, _, outcome := diff.DiffApplications(ctx, noopBaseline, recordingCreate(&created), emptyManifest, rdb, &stored, fresh, nil)
	testutil.Equal(t, outcomeField, outcome, diff.OutcomeAuthored)
	entry := diff.LastChangeLogEntry(history)
	testutil.Equal(t, "changedBy", entry.ChangedBy, constants.EmptyString)
	detected, err := time.Parse(time.RFC3339, entry.DetectedAt)
	testutil.Equal(t, "detectedAt parses", err, nil)
	testutil.Equal(t, "detectedAt is the detection time", time.Since(detected) < time.Minute, true)
}

// A manifest change the tick sees without a pre-image waits for the informer flush;
// when the same change set is still there on the next tick (a resource that joined
// while discovery was down), the live state is recorded so the CR converges.
func TestDiffApplicationsDeferredChangeConvergesAfterTicks(t *testing.T) {
	serveManifestsFromCache(t)
	created := constants.DefaultInitValue
	rdb := newRedis(t)
	ctx := context.Background()
	base := appresource.Application{
		Name:      stuckApp,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		History:   appresource.ApplicationHistory{Generation: constants.ThreeValue, LastModifiedBy: annotatedUser},
	}
	stored := base
	fresh := base
	fresh.Images = []string{diffImage}
	ports := base
	ports.Ports = []int{httpPort}

	_, _, outcome := diff.DiffApplications(ctx, noopBaseline, recordingCreate(&created), emptyManifest, rdb, &stored, fresh, nil)
	testutil.Equal(t, "first tick", outcome, diff.OutcomeDeferred)
	_, _, outcome = diff.DiffApplications(ctx, noopBaseline, recordingCreate(&created), emptyManifest, rdb, &stored, ports, nil)
	testutil.Equal(t, "another change set restarts the wait", outcome, diff.OutcomeDeferred)
	_, _, outcome = diff.DiffApplications(ctx, noopBaseline, recordingCreate(&created), emptyManifest, rdb, &stored, fresh, nil)
	testutil.Equal(t, "second tick of the images change", outcome, diff.OutcomeDeferred)
	testutil.Equal(t, "nothing written while deferred", created, constants.DefaultInitValue)

	history, snaps, outcome := diff.DiffApplications(ctx, noopBaseline, recordingCreate(&created), emptyManifest, rdb, &stored, fresh, nil)
	testutil.Equal(t, "converged", outcome, diff.OutcomeAuthored)
	testutil.Equal(t, generationField, history.Generation, sealedGeneration)
	testutil.Equal(t, "live snapshot written", created, singleSnapshot)
	testutil.Equal(t, "snapshot for the generation", hasGeneration(snaps, history.Generation), true)
	entry := diff.LastChangeLogEntry(history)
	testutil.Equal(t, "unattributed", entry.ChangedBy, constants.EmptyString)
	n, err := rdb.Exists(ctx, constants.KeyPrefixHistoryDeferred+stuckApp).Result()
	testutil.Equal(t, "counter read", err, nil)
	testutil.Equal(t, "counter cleared", n, int64(constants.DefaultInitValue))
}

// The pre-image set is written before the change is classified; the entry the
// store keeps must carry the class of the change it precedes, not a placeholder.
func TestDiffApplicationsStampsPrewrittenSnapshotsWithEntryClass(t *testing.T) {
	created := constants.DefaultInitValue
	stored := &appresource.Application{
		Name:      "sealed",
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{diffImage},
		History:   appresource.ApplicationHistory{Generation: constants.ThreeValue},
	}
	fresh := *stored
	fresh.Images = []string{"nginx:2.0"}
	opts := &diff.DiffOptions{
		PrewrittenGeneration: sealedGeneration,
		PrewrittenSnapshots: []appresource.ApplicationSnapshot{{
			Generation: sealedGeneration, ID: "s4", Namespace: diffNamespace, Path: "prod/s4.json",
			ChangeClass: appresource.ChangeClassTopology, Severity: historyshared.SeverityLow,
		}},
	}
	history, snaps, _ := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest, newRedis(t), stored, fresh, opts,
	)
	entry := diff.LastChangeLogEntry(history)
	if entry == nil {
		t.Fatal("no entry authored")
	}
	for _, s := range snaps {
		if s.Generation != sealedGeneration {
			continue
		}
		testutil.Equal(t, "snapshot class", s.ChangeClass, entry.ChangeClass)
		testutil.Equal(t, "snapshot severity", s.Severity, entry.Severity)
	}
}

// A CR deleted through the exporter leaves discovery's per-app keys behind; the
// re-discovered app then deferred every flush as "behind published 5" for 10 min
// and inherited the old incident state. Generation 1 starts from nothing.
func TestDiffApplicationsNewAppForgetsPreviousIncarnation(t *testing.T) {
	rdb := newRedis(t)
	ctx := context.Background()
	stale := []string{
		constants.KeyPrefixHistoryFloor + rediscovered,
		constants.KeyPrefixIncidentState + rediscovered,
		constants.KeyPrefixHistoryRecorded + rediscovered,
		constants.KeyPrefixHistoryPost + rediscovered,
	}
	for _, key := range stale {
		if err := rdb.Set(ctx, key, staleFloor, constants.HistoryFloorTTL).Err(); err != nil {
			t.Fatal(err)
		}
	}
	created := constants.DefaultInitValue
	fresh := appresource.Application{Name: rediscovered}

	history, _, outcome := diff.DiffApplications(
		ctx, noopBaseline, recordingCreate(&created), emptyManifest, rdb, nil, fresh, nil,
	)
	testutil.Equal(t, outcomeField, outcome, diff.OutcomeAuthored)
	testutil.Equal(t, generationField, history.Generation, constants.DefaultAddValue)
	for _, key := range stale {
		n, err := rdb.Exists(ctx, key).Result()
		testutil.Equal(t, key+" read", err, nil)
		testutil.Equal(t, key+" purged", n, int64(constants.DefaultInitValue))
	}
}

func deploymentRunning(image string) unstructured.Unstructured {
	u := unstructured.Unstructured{Object: map[string]any{}}
	u.SetKind(kindDeployment)
	u.SetName(workloadAPI)
	u.SetNamespace(diffNamespace)
	_ = unstructured.SetNestedField(u.Object, int64(constants.DefaultAddValue),
		constants.K8sObjectFieldSpec, constants.K8sObjectFieldReplicas)
	_ = unstructured.SetNestedSlice(u.Object, []any{map[string]any{constants.K8sObjectFieldImage: image}},
		constants.K8sObjectFieldSpec, constants.K8sObjectFieldTemplate, constants.K8sObjectFieldSpec, constants.K8sObjectFieldContainers)
	return u
}

func flushOpts(generation int, preImage unstructured.Unstructured, pairs []manifestdiff.ManifestPair) (
	*diff.DiffOptions,
	func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error),
) {
	opts := &diff.DiffOptions{
		FromCoalescingFlush:  true,
		PrewrittenGeneration: generation,
		PrewrittenSnapshots: []appresource.ApplicationSnapshot{
			{Generation: generation, ID: preImageSnapshot, Namespace: diffNamespace, Path: preImageSnapshot},
		},
		ManifestPairs: pairs,
	}
	manifest := func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		return []unstructured.Unstructured{preImage}, nil
	}
	return opts, manifest
}

// A recreate whose delete pre-image never reached the flush snapshots the live object; its
// image matching fresh read as a change undone within the window, the no-change publish moved
// the CR to the new image, and the reconcile's flush from the older snapshot then found nothing.
func TestFlushWithoutPreImageRecordsTheImageChangeOnce(t *testing.T) {
	ctx := context.Background()
	rdb := newRedis(t)
	created := constants.DefaultInitValue
	health := appresource.Health{TotalReplicas: constants.DefaultAddValue}
	stored := appresource.Application{
		Name:      diffAppName,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{deletedImage},
		Health:    health,
		History:   appresource.ApplicationHistory{Generation: constants.DefaultAddValue},
	}
	fresh := stored
	fresh.Images = []string{recreatedImage}
	fresh.History = appresource.ApplicationHistory{}

	opts, live := flushOpts(constants.TwoValue, deploymentRunning(recreatedImage), nil)
	history, snaps, _ := diff.DiffApplications(ctx, noopBaseline, recordingCreate(&created), live, rdb, &stored, fresh, opts)
	published := fresh
	published.History, published.Snapshots = history, snaps

	old, cur := deploymentRunning(deletedImage), deploymentRunning(recreatedImage)
	opts, fromSnapshot := flushOpts(history.Generation+constants.DefaultAddValue, old,
		[]manifestdiff.ManifestPair{{Old: &old, New: &cur}})
	history, _, _ = diff.DiffApplications(ctx, noopBaseline, recordingCreate(&created), fromSnapshot, rdb, &published, fresh, opts)

	recorded := constants.DefaultInitValue
	for _, entry := range history.ChangeLog {
		recorded += len(slices.DeleteFunc(slices.Clone(entry.Changes), func(c appresource.ApplicationChange) bool {
			return c.Field != changes.ChangeFieldImage
		}))
	}
	testutil.Equal(t, "image changes recorded", recorded, constants.DefaultAddValue)
}
