package metrics

import (
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/core/applications/history/changes"
	"github.com/telark/discovery/core/applications/history/utils"
	appshared "github.com/telark/discovery/core/applications/shared"
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
	var appChanges []application.ApplicationChange

	if prev.Requests.CPU != curr.Requests.CPU {
		oldVal := prev.Requests.CPU
		newVal := curr.Requests.CPU
		appChanges = append(appChanges, application.ApplicationChange{
			Field:       changes.ChangeFieldRequestsCPU,
			Description: changes.DescRequestsCPUChanged(oldVal, newVal),
			ChangeType:  changes.ChangeTypeUpdated,
			OldValue:    utils.StrPtr(oldVal),
			NewValue:    utils.StrPtr(newVal),
		})
	}
	if prev.Requests.Memory != curr.Requests.Memory {
		oldVal := prev.Requests.Memory
		newVal := curr.Requests.Memory
		appChanges = append(appChanges, application.ApplicationChange{
			Field:       changes.ChangeFieldRequestsMemory,
			Description: changes.DescRequestsMemoryChanged(oldVal, newVal),
			ChangeType:  changes.ChangeTypeUpdated,
			OldValue:    utils.StrPtr(oldVal),
			NewValue:    utils.StrPtr(newVal),
		})
	}
	if prev.Limits.CPU != curr.Limits.CPU {
		oldVal := prev.Limits.CPU
		newVal := curr.Limits.CPU
		appChanges = append(appChanges, application.ApplicationChange{
			Field:       changes.ChangeFieldLimitsCPU,
			Description: changes.DescLimitsCPUChanged(oldVal, newVal),
			ChangeType:  changes.ChangeTypeUpdated,
			OldValue:    utils.StrPtr(oldVal),
			NewValue:    utils.StrPtr(newVal),
		})
	}
	if prev.Limits.Memory != curr.Limits.Memory {
		oldVal := prev.Limits.Memory
		newVal := curr.Limits.Memory
		appChanges = append(appChanges, application.ApplicationChange{
			Field:       changes.ChangeFieldLimitsMemory,
			Description: changes.DescLimitsMemoryChanged(oldVal, newVal),
			ChangeType:  changes.ChangeTypeUpdated,
			OldValue:    utils.StrPtr(oldVal),
			NewValue:    utils.StrPtr(newVal),
		})
	}
	return appChanges
}
