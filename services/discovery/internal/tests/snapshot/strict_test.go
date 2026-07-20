package snapshot

import (
	"context"
	"testing"
	"time"

	appresource "github.com/telark/data/resources/application"
	historyshared "github.com/telark/discovery/internal/core/applications/history/shared"
	"github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/tests/testutil"
)

// With the manifest cache populated (so no cluster fetch happens), the strict
// builder groups resources by namespace and seals one snapshot per namespace,
// stamping each entry with the requested generation.
func TestBuildSnapshotEntriesStrict(t *testing.T) {
	snapshot.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": name, "namespace": ns},
		}, true
	}
	t.Cleanup(func() { snapshot.GetManifestFromCache = nil })

	created := 0
	createSnap := func(id, _ string, _ string, _ int, _ any) (string, error) {
		created++
		return "path/" + id, nil
	}
	stored := &appresource.Application{
		Resources: []appresource.Resource{
			{Namespace: "prod", Kind: "Deployment", Name: "web"},
			{Namespace: "prod", Kind: "Service", Name: "web-svc"},
		},
	}

	entries, err := snapshot.BuildSnapshotEntriesStrict(
		context.Background(), createSnap, stored, 2,
		appresource.ChangeClassDeployment, historyshared.SeverityHigh, time.Now(),
	)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "one namespace entry", len(entries), 1)
	testutil.Equal(t, "generation", entries[0].Generation, 2)
	testutil.Equal(t, "namespace", entries[0].Namespace, "prod")
	if created == 0 {
		t.Fatal("createSnapshot was never invoked")
	}
}

// The non-strict builder seals one entry per namespace from the cached manifests
// and returns an empty (non-error) slice when the creator is nil.
func TestBuildSnapshotEntries(t *testing.T) {
	snapshot.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return map[string]any{"kind": kind, "metadata": map[string]any{"name": name, "namespace": ns}}, true
	}
	t.Cleanup(func() { snapshot.GetManifestFromCache = nil })

	createSnap := func(id, _ string, _ string, _ int, _ any) (string, error) { return "path/" + id, nil }
	stored := &appresource.Application{
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "web"}},
	}
	entries := snapshot.BuildSnapshotEntries(
		context.Background(), createSnap, stored, 5,
		appresource.ChangeClassDeployment, historyshared.SeverityHigh, time.Now(),
	)
	testutil.Equal(t, "entries", len(entries), 1)
	testutil.Equal(t, "generation", entries[0].Generation, 5)

	empty := snapshot.BuildSnapshotEntries(context.Background(), nil, stored, 5, "c", historyshared.SeverityHigh, time.Now())
	testutil.Equal(t, "nil creator empty", len(empty), 0)
}

// The strict builder rejects a nil creator and a nil stored application.
func TestBuildSnapshotEntriesStrictGuards(t *testing.T) {
	ctx := context.Background()
	stored := &appresource.Application{}
	createSnap := func(_ string, _ string, _ string, _ int, _ any) (string, error) { return "p", nil }

	if _, err := snapshot.BuildSnapshotEntriesStrict(ctx, nil, stored, 1, "c", historyshared.SeverityHigh, time.Now()); err == nil {
		t.Fatal("nil createSnapshot should error")
	}
	if _, err := snapshot.BuildSnapshotEntriesStrict(ctx, createSnap, nil, 1, "c", historyshared.SeverityHigh, time.Now()); err == nil {
		t.Fatal("nil stored application should error")
	}
}
