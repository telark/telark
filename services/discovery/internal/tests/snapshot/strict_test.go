package snapshot

import (
	"context"
	"testing"
	"time"

	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	historyshared "github.com/telark/telark/services/discovery/internal/core/applications/history/shared"
	"github.com/telark/telark/services/discovery/internal/core/applications/snapshot"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	snapNamespace  = "prod"
	keyNamespace   = "namespace"
	changeClass    = "c"
	sealGeneration = 5
)

// With the manifest cache populated (so no cluster fetch happens), the strict
// builder groups resources by namespace and seals one snapshot per namespace,
// stamping each entry with the requested generation.
func TestBuildSnapshotEntriesStrict(t *testing.T) {
	snapshot.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": name, keyNamespace: ns},
		}, true
	}
	t.Cleanup(func() { snapshot.GetManifestFromCache = nil })

	created := constants.DefaultInitValue
	createSnap := func(id, _ string, _ string, _ int, _ any) (string, error) {
		created++
		return "path/" + id, nil
	}
	stored := &appresource.Application{
		Resources: []appresource.Resource{
			{Namespace: snapNamespace, Kind: "Deployment", Name: "web"},
			{Namespace: snapNamespace, Kind: "Service", Name: "web-svc"},
		},
	}

	entries, err := snapshot.BuildSnapshotEntriesStrict(
		context.Background(), createSnap, stored, constants.TwoValue,
		appresource.ChangeClassDeployment, historyshared.SeverityHigh, time.Now(),
	)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "one namespace entry", len(entries), constants.DefaultAddValue)
	testutil.Equal(t, "generation", entries[0].Generation, constants.TwoValue)
	testutil.Equal(t, keyNamespace, entries[0].Namespace, snapNamespace)
	if created == constants.DefaultInitValue {
		t.Fatal("createSnapshot was never invoked")
	}
}

// The non-strict builder seals one entry per namespace from the cached manifests
// and returns an empty (non-error) slice when the creator is nil.
func TestBuildSnapshotEntries(t *testing.T) {
	snapshot.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return map[string]any{"kind": kind, "metadata": map[string]any{"name": name, keyNamespace: ns}}, true
	}
	t.Cleanup(func() { snapshot.GetManifestFromCache = nil })

	createSnap := func(id, _ string, _ string, _ int, _ any) (string, error) { return "path/" + id, nil }
	stored := &appresource.Application{
		Resources: []appresource.Resource{{Namespace: snapNamespace, Kind: "Deployment", Name: "web"}},
	}
	entries := snapshot.BuildSnapshotEntries(
		context.Background(), createSnap, stored, sealGeneration,
		appresource.ChangeClassDeployment, historyshared.SeverityHigh, time.Now(),
	)
	testutil.Equal(t, "entries", len(entries), constants.DefaultAddValue)
	testutil.Equal(t, "generation", entries[0].Generation, sealGeneration)

	empty := snapshot.BuildSnapshotEntries(context.Background(), nil, stored, sealGeneration, changeClass, historyshared.SeverityHigh, time.Now())
	testutil.Equal(t, "nil creator empty", len(empty), constants.DefaultInitValue)
}

// The strict builder rejects a nil creator and a nil stored application.
func TestBuildSnapshotEntriesStrictGuards(t *testing.T) {
	ctx := context.Background()
	stored := &appresource.Application{}
	createSnap := func(_ string, _ string, _ string, _ int, _ any) (string, error) { return "p", nil }

	_, err := snapshot.BuildSnapshotEntriesStrict(
		ctx, nil, stored, constants.DefaultAddValue, changeClass, historyshared.SeverityHigh, time.Now(),
	)
	if err == nil {
		t.Fatal("nil createSnapshot should error")
	}
	_, err = snapshot.BuildSnapshotEntriesStrict(
		ctx, createSnap, nil, constants.DefaultAddValue, changeClass, historyshared.SeverityHigh, time.Now(),
	)
	if err == nil {
		t.Fatal("nil stored application should error")
	}
}
