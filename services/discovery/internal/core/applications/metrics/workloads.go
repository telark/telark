package metrics

import (
	"context"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	kcoremetrics "github.com/telark/kcore/metrics"
	"github.com/telark/kcore/resources/workload"
)

func PopulateApplicationMetrics(ctx context.Context, app *application.Application) {
	_ = ctx
	if app == nil {
		return
	}
	app.Metrics.Derived = computeDerivedMetrics(app)
	app.Metrics.Workloads = collectWorkloadUsageEntries(app)
}

func collectWorkloadUsageEntries(app *application.Application) []application.WorkloadUsage {
	prevWorkloads := app.Metrics.Workloads
	seen := make(map[workloadAnchorKey]struct{})
	out := make([]application.WorkloadUsage, constants.DefaultInitValue)
	for i := range app.Resources {
		r := app.Resources[i]
		if !isWorkloadAnchorKind(r.Kind) {
			continue
		}
		k := workloadAnchorKey{r.Namespace, r.Kind, r.Name}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		existing := findExistingBaseline(prevWorkloads, r.Namespace, r.Kind, r.Name)
		out = append(out, workloadUsageForResource(r, existing))
	}
	return out
}

// findExistingBaseline returns the baseline from a prior WorkloadUsage slice if present.
// A non-zero Replicas field signals that the baseline was seeded from a V1 snapshot and
// must not be overwritten by a live ReadMetricsBaseline call.
func findExistingBaseline(
	workloads []application.WorkloadUsage,
	ns, kind, name string,
) *application.MetricsBaseline {
	for i := range workloads {
		w := &workloads[i]
		if w.Namespace == ns && w.ResourceKind == kind && w.ResourceName == name {
			return &w.Baseline
		}
	}
	return nil
}

func workloadUsageForResource(r application.Resource, existing *application.MetricsBaseline) application.WorkloadUsage {
	sel := workload.WorkloadPodMatchLabels(r.Namespace, r.Kind, r.Name)
	qos := kcoremetrics.GetWorkloadQualityOfService(r.Namespace, sel)
	u := kcoremetrics.BuildWorkloadUsage(r.Namespace, qos, sel)
	var baseline application.MetricsBaseline
	if existing != nil && existing.Replicas != constants.DefaultInitValue {
		baseline = *existing
	} else {
		baseline = workload.ReadMetricsBaseline(r.Namespace, r.Kind, r.Name)
	}
	wu := application.WorkloadUsage{
		ResourceName: r.Name,
		ResourceKind: r.Kind,
		Namespace:    r.Namespace,
		Baseline:     baseline,
	}
	if u != nil {
		wu.Usage = *u
	}
	return wu
}
