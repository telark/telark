package diffpure

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/diff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	diffAppName    = "shop"
	diffNamespace  = "prod"
	diffImage      = "nginx:1.0"
	kindDeployment = "Deployment"
	workloadAPI    = "api"
)

func noopBaseline(string, string, string) appresource.MetricsBaseline {
	return appresource.MetricsBaseline{}
}

func recordingCreate(created *int) func(id, scope, namespace string, generation int, manifest any) (string, error) {
	return func(string, string, string, int, any) (string, error) {
		*created++
		return "snap-path", nil
	}
}

func emptyManifest(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
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
	created := constants.DefaultInitValue
	fresh := appresource.Application{
		Name:      diffAppName,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{diffImage},
	}
	history, _, _ := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), nil, fresh, nil,
	)
	if history.Generation < constants.DefaultAddValue {
		t.Fatalf("first-seen history generation = %d, want >= 1", history.Generation)
	}
}

// Two identical applications produce no new change log entries.
func TestDiffApplicationsNoChange(t *testing.T) {
	created := constants.DefaultInitValue
	app := appresource.Application{
		Name:      diffAppName,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{diffImage},
		History:   appresource.ApplicationHistory{Generation: constants.TwoValue},
	}
	stored := app
	history, _, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), &stored, app, nil,
	)
	if changed != diff.OutcomeNoChange {
		t.Fatal("identical applications reported a change")
	}
	if history.Generation < constants.TwoValue {
		t.Fatalf("no-change generation regressed to %d", history.Generation)
	}
}

// A changed image drives the change-handling path. Without a pre-image the
// generation cannot be sealed, so the diff keeps the stored history and reports
// the change as deferred: the caller must not publish the fresh application.
func TestDiffApplicationsWithChange(t *testing.T) {
	created := constants.DefaultInitValue
	stored := &appresource.Application{
		Name:      diffAppName,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{diffImage},
		History:   appresource.ApplicationHistory{Generation: constants.ThreeValue},
	}
	fresh := appresource.Application{
		Name:      diffAppName,
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{"nginx:2.0"},
	}
	history, _, outcome := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), stored, fresh, nil,
	)
	if outcome != diff.OutcomeDeferred {
		t.Fatalf("change without pre-image reported %v, want deferred", outcome)
	}
	if history.Generation != constants.ThreeValue {
		t.Fatalf("deferred change moved generation to %d", history.Generation)
	}
}
