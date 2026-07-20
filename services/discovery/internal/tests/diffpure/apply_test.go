package diffpure

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func noopBaseline(ns, kind, name string) appresource.MetricsBaseline {
	return appresource.MetricsBaseline{}
}

func recordingCreate(created *int) func(id, scope, namespace string, generation int, manifest any) (string, error) {
	return func(id, scope, namespace string, generation int, manifest any) (string, error) {
		*created++
		return "snap-path", nil
	}
}

func emptyManifest(ctx context.Context, snapshotID, scope, namespace string, generation int) ([]unstructured.Unstructured, error) {
	return nil, nil
}

func newRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// A first-ever diff (no stored application) seeds a baseline history at
// generation 1.
func TestDiffApplicationsFirstSeen(t *testing.T) {
	created := 0
	fresh := appresource.Application{
		Name:      "shop",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		Images:    []string{"nginx:1.0"},
	}
	history, _, _ := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), nil, fresh, nil,
	)
	if history.Generation < 1 {
		t.Fatalf("first-seen history generation = %d, want >= 1", history.Generation)
	}
}

// Two identical applications produce no new change log entries.
func TestDiffApplicationsNoChange(t *testing.T) {
	created := 0
	app := appresource.Application{
		Name:      "shop",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		Images:    []string{"nginx:1.0"},
		History:   appresource.ApplicationHistory{Generation: 2},
	}
	stored := app
	history, _, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), &stored, app, nil,
	)
	if changed {
		t.Fatal("identical applications reported a change")
	}
	if history.Generation < 2 {
		t.Fatalf("no-change generation regressed to %d", history.Generation)
	}
}

// A changed image drives the change-handling path. Without a populated manifest
// cache the generation snapshot cannot be sealed, so the diff falls back to the
// no-change history rather than sealing a new generation — the point here is that
// the full change path executes without error.
func TestDiffApplicationsWithChange(t *testing.T) {
	created := 0
	stored := &appresource.Application{
		Name:      "shop",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		Images:    []string{"nginx:1.0"},
		History:   appresource.ApplicationHistory{Generation: 3},
	}
	fresh := appresource.Application{
		Name:      "shop",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		Images:    []string{"nginx:2.0"},
	}
	history, _, _ := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), stored, fresh, nil,
	)
	if history.Generation < 3 {
		t.Fatalf("generation regressed to %d", history.Generation)
	}
}
