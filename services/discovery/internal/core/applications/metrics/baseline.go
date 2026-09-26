package metrics

import (
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/core/applications/history/utils"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
)

func BaselineResourceChanges(
	stored, fresh *application.Application,
	readBaseline WorkloadBaselineReader,
) []application.ApplicationChange {
	if stored == nil || fresh == nil || readBaseline == nil {
		return nil
	}
	seen := make(map[workloadAnchorKey]struct{})
	var out []application.ApplicationChange
	for i := range fresh.Resources {
		r := fresh.Resources[i]
		if !isWorkloadAnchorKind(r.Kind) {
			continue
		}
		k := workloadAnchorKey{r.Namespace, r.Kind, r.Name}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		curr := readBaseline(r.Namespace, r.Kind, r.Name)
		prevWU := findPrevWorkloadUsage(stored, r.Namespace, r.Kind, r.Name)
		if prevWU == nil {
			continue
		}
		if prevWU.Baseline.Fingerprint == curr.Fingerprint {
			continue
		}
		ch := DiffBaseline(prevWU.Baseline, curr)
		if len(ch) > constants.DefaultInitValue {
			out = append(out, ch...)
		}
	}
	return out
}

func isWorkloadAnchorKind(kind string) bool {
	switch kind {
	case appshared.KindDeployment, appshared.KindStatefulSet, appshared.KindDaemonSet:
		return true
	default:
		return false
	}
}

func findPrevWorkloadUsage(
	stored *application.Application,
	ns, kind, name string,
) *application.WorkloadUsage {
	for i := range stored.Metrics.Workloads {
		w := &stored.Metrics.Workloads[i]
		if w.Namespace == ns && w.ResourceKind == kind && w.ResourceName == name {
			return w
		}
	}
	return nil
}

func DiffBaseline(prev, curr application.MetricsBaseline) []application.ApplicationChange {
	fields := []struct {
		field    string
		describe func(oldVal, newVal string) string
		oldVal   string
		newVal   string
	}{
		{changes.ChangeFieldRequestsCPU, changes.DescRequestsCPUChanged, prev.Requests.CPU, curr.Requests.CPU},
		{changes.ChangeFieldRequestsMemory, changes.DescRequestsMemoryChanged, prev.Requests.Memory, curr.Requests.Memory},
		{changes.ChangeFieldLimitsCPU, changes.DescLimitsCPUChanged, prev.Limits.CPU, curr.Limits.CPU},
		{changes.ChangeFieldLimitsMemory, changes.DescLimitsMemoryChanged, prev.Limits.Memory, curr.Limits.Memory},
	}
	var appChanges []application.ApplicationChange
	for _, f := range fields {
		if f.oldVal == f.newVal {
			continue
		}
		appChanges = append(appChanges, application.ApplicationChange{
			Field:       f.field,
			Description: f.describe(f.oldVal, f.newVal),
			ChangeType:  changes.ChangeTypeUpdated,
			OldValue:    utils.StrPtr(f.oldVal),
			NewValue:    utils.StrPtr(f.newVal),
		})
	}
	return appChanges
}
