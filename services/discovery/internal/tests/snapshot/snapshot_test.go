package snapshot

import (
	"context"
	"testing"
	"time"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Generation membership is a straight scan.
func TestHasSnapshotGeneration(t *testing.T) {
	snaps := []appresource.ApplicationSnapshot{{Generation: 1}, {Generation: 3}}
	testutil.Equal(t, "present", snapshot.HasSnapshotGeneration(snaps, 3), true)
	testutil.Equal(t, "absent", snapshot.HasSnapshotGeneration(snaps, 2), false)
}

// Blank TakenAt timestamps are backfilled; a nil app is a no-op.
func TestNormalizeApplicationSnapshotTakenAt(t *testing.T) {
	app := &appresource.Application{Snapshots: []appresource.ApplicationSnapshot{{ID: "a"}, {ID: "b", TakenAt: "set"}}}
	snapshot.NormalizeApplicationSnapshotTakenAt(app)
	if app.Snapshots[0].TakenAt == "" {
		t.Fatal("blank timestamp was not backfilled")
	}
	testutil.Equal(t, "existing kept", app.Snapshots[1].TakenAt, "set")
	snapshot.NormalizeApplicationSnapshotTakenAt(nil)
}

// Merge concatenates, orders by (generation, namespace) and keeps only the most
// recent maxVersions entries.
func TestMergeSnapshots(t *testing.T) {
	existing := []appresource.ApplicationSnapshot{{Generation: 1, Namespace: "a"}, {Generation: 2, Namespace: "b"}}
	fresh := []appresource.ApplicationSnapshot{{Generation: 3, Namespace: "c"}}
	merged := snapshot.MergeSnapshots(existing, fresh, 2)
	testutil.Equal(t, "capped", len(merged), 2)
	testutil.Equal(t, "oldest dropped", merged[0].Generation, 2)
}

// Namespaces for a generation are unique and sorted; a non-positive generation
// yields nothing.
func TestNamespacesForGeneration(t *testing.T) {
	snaps := []appresource.ApplicationSnapshot{
		{Generation: 2, Namespace: "b"}, {Generation: 2, Namespace: "a"},
		{Generation: 2, Namespace: "a"}, {Generation: 3, Namespace: "z"},
	}
	got := snapshot.NamespacesForGeneration(snaps, 2)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("namespaces = %v, want [a b]", got)
	}
	testutil.Equal(t, "non-positive", len(snapshot.NamespacesForGeneration(snaps, 0)), 0)
}

// A snapshot id is prefixed and non-empty.
func TestNewSnapshotID(t *testing.T) {
	id, err := snapshot.NewSnapshotID()
	if err != nil || id == "" {
		t.Fatalf("NewSnapshotID = %q, %v", id, err)
	}
}

// PayloadFromUnstructured always returns the resources+note envelope, empty or
// populated.
func TestPayloadFromUnstructured(t *testing.T) {
	empty := snapshot.PayloadFromUnstructured(nil)
	if _, ok := empty["resources"]; !ok {
		t.Fatal("payload missing resources key")
	}
	obj := unstructured.Unstructured{Object: map[string]any{
		"kind":     "ConfigMap",
		"metadata": map[string]any{"name": "cfg", "namespace": "prod"},
	}}
	full := snapshot.PayloadFromUnstructured([]unstructured.Unstructured{obj})
	if _, ok := full["resources"]; !ok {
		t.Fatal("payload missing resources key")
	}
}

// DiscardSnapshots calls the deleter for each entry and logs (not returns)
// failures; a nil deleter is a no-op.
func TestDiscardSnapshots(t *testing.T) {
	snaps := []appresource.ApplicationSnapshot{{ID: "1"}, {ID: "2"}}
	var seen []string
	snapshot.DiscardSnapshots(snaps, func(id, scope, namespace string, generation int) error {
		seen = append(seen, id)
		return nil
	})
	testutil.Equal(t, "deleted all", len(seen), 2)

	snapshot.DiscardSnapshots(snaps, func(id, scope, namespace string, generation int) error {
		return errFake
	})
	snapshot.DiscardSnapshots(snaps, nil)
}

var errFake = fakeErr("boom")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

// Without a populated manifest cache no entries can be resolved, so the builder
// returns nothing — but it still walks namespace grouping and item collection.
func TestBuildSnapshotEntriesNoManifests(t *testing.T) {
	stored := &appresource.Application{
		Resources: []appresource.Resource{
			{Namespace: "prod", Kind: "Deployment", Name: "api"},
			{Namespace: "staging", Kind: "Service", Name: "svc"},
		},
	}
	created := false
	out := snapshot.BuildSnapshotEntries(
		context.Background(),
		func(id, scope, namespace string, generation int, manifestPayload any) (string, error) {
			created = true
			return "path", nil
		},
		stored, 1, "topology", appresource.SeverityCritical, time.Now(),
	)
	if len(out) != 0 || created {
		t.Fatalf("no cached manifests should yield no snapshots (got %d, created=%v)", len(out), created)
	}
	// A nil creator short-circuits.
	if got := snapshot.BuildSnapshotEntries(context.Background(), nil, stored, 1, "topology", "high", time.Now()); len(got) != 0 {
		t.Fatalf("nil creator returned %d entries", len(got))
	}
}
