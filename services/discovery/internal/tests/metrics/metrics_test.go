package metrics

import (
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/metrics"
	"github.com/telark/discovery/internal/tests/testutil"
)

const cpuRequest = "100m"

// DiffBaseline emits one change per differing request/limit field and nothing
// when the baselines match.
func TestDiffBaseline(t *testing.T) {
	same := appresource.MetricsBaseline{Requests: appresource.ResourceValues{CPU: cpuRequest}}
	testutil.Equal(t, "identical", len(metrics.DiffBaseline(same, same)), constants.DefaultInitValue)

	prev := appresource.MetricsBaseline{
		Requests: appresource.ResourceValues{CPU: cpuRequest, Memory: "128Mi"},
		Limits:   appresource.ResourceValues{CPU: "200m", Memory: "256Mi"},
	}
	curr := appresource.MetricsBaseline{
		Requests: appresource.ResourceValues{CPU: "150m", Memory: "128Mi"},
		Limits:   appresource.ResourceValues{CPU: "200m", Memory: "512Mi"},
	}
	got := metrics.DiffBaseline(prev, curr)
	testutil.Equal(t, "two fields changed", len(got), constants.TwoValue)
	for _, c := range got {
		if c.Description == "" {
			t.Errorf("baseline change %q has no description", c.Field)
		}
	}
}

// BaselineResourceChanges compares each fresh workload against its stored
// baseline using the injected reader, and only reports when the fingerprint
// moved.
func TestBaselineResourceChanges(t *testing.T) {
	stored := &appresource.Application{
		Metrics: appresource.ApplicationMetrics{
			Workloads: []appresource.WorkloadUsage{{
				Namespace: "prod", ResourceKind: "Deployment", ResourceName: "api",
				Baseline: appresource.MetricsBaseline{
					Fingerprint: "old", Requests: appresource.ResourceValues{CPU: cpuRequest},
				},
			}},
		},
	}
	fresh := &appresource.Application{
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
	}
	reader := func(string, string, string) appresource.MetricsBaseline {
		return appresource.MetricsBaseline{Fingerprint: "new", Requests: appresource.ResourceValues{CPU: "250m"}}
	}
	got := metrics.BaselineResourceChanges(stored, fresh, reader)
	if len(got) == constants.DefaultInitValue {
		t.Fatal("a moved baseline should produce a change")
	}

	if metrics.BaselineResourceChanges(nil, fresh, reader) != nil {
		t.Fatal("nil stored must yield nil")
	}
	if metrics.BaselineResourceChanges(stored, fresh, nil) != nil {
		t.Fatal("nil reader must yield nil")
	}
}

// A change log with an incident and a recovery is summarized into derived
// metrics; an empty log yields the zeroed summary.
func TestPopulateApplicationMetrics(t *testing.T) {
	app := &appresource.Application{
		Snapshots: []appresource.ApplicationSnapshot{{Generation: constants.DefaultAddValue}},
		History: appresource.ApplicationHistory{
			ChangeLog: []appresource.ChangeLogEntry{
				{
					DetectedAt: "2026-01-01T00:00:00Z", ChangeClass: "incident",
					Severity: "high", Fingerprint: "fp1", IsIncident: true,
					Changes: []appresource.ApplicationChange{{Field: "health"}},
				},
				{
					DetectedAt: "2026-01-03T00:00:00Z", ChangeClass: "recovery",
					Severity: "medium", Fingerprint: "fp2", IsRecovery: true,
					Changes: []appresource.ApplicationChange{{Field: "health"}},
				},
			},
		},
	}
	metrics.PopulateApplicationMetrics(app)
	d := app.Metrics.Derived
	if d.ChangesByClass == nil || d.ChangesBySeverity == nil {
		t.Fatal("derived class/severity maps must be initialized")
	}
	testutil.Equal(t, "recoveries", d.TotalRecoveries, constants.DefaultAddValue)
	testutil.Equal(t, "snapshot count", d.SnapshotCount, constants.DefaultAddValue)

	empty := &appresource.Application{}
	metrics.PopulateApplicationMetrics(empty)
	testutil.Equal(t, "empty total", empty.Metrics.Derived.TotalChanges, constants.DefaultInitValue)
	testutil.Equal(t, "empty velocity", empty.Metrics.Derived.ChangeVelocityPerDay, float64(constants.DefaultInitValue))
}

// A nil application is a no-op, not a panic.
func TestPopulateNilApp(*testing.T) {
	metrics.PopulateApplicationMetrics(nil)
}
