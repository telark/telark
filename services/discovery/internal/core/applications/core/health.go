package core

import (
	"fmt"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	appshared "github.com/telark/discovery/core/applications/shared"
	"github.com/telark/kcore/resources/workload"
)

func ComputeHealth(app *application.Application) application.Health {
	var totalReady, totalDesired int
	type nk struct{ ns, kind string }
	seen := make(map[nk]bool)
	for _, r := range app.Resources {
		if r.Kind != appshared.KindDeployment &&
			r.Kind != appshared.KindStatefulSet &&
			r.Kind != appshared.KindDaemonSet {
			continue
		}
		seen[nk{r.Namespace, r.Kind}] = true
	}
	for nk := range seen {
		readyMap, desiredMap := getReplicaCountsForNamespaceKind(nk.ns, nk.kind)
		for _, r := range app.Resources {
			if r.Namespace != nk.ns || r.Kind != nk.kind {
				continue
			}
			totalReady += readyMap[r.Name]
			totalDesired += desiredMap[r.Name]
		}
	}
	return healthFromCounts(totalReady, totalDesired)
}

func getReplicaCountsForNamespaceKind(ns, kind string) (ready map[string]int, desired map[string]int) {
	switch kind {
	case appshared.KindDeployment:
		return replicaMapsFromDeployments(ns)
	case appshared.KindStatefulSet:
		return replicaMapsFromStatefulSets(ns)
	case appshared.KindDaemonSet:
		return replicaMapsFromDaemonSets(ns)
	default:
		return make(map[string]int), make(map[string]int)
	}
}

func replicaMapsFromDeployments(ns string) (ready map[string]int, desired map[string]int) {
	ready = make(map[string]int)
	desired = make(map[string]int)
	list, err := workload.GetDeploymentsByNamespace(ns)
	if err != nil {
		return ready, desired
	}
	for i := range list {
		d := &list[i]
		replicas := constants.DefaultAddValue
		if d.Spec.Replicas != nil {
			replicas = int(*d.Spec.Replicas)
		}
		desired[d.Name] = replicas
		ready[d.Name] = int(d.Status.ReadyReplicas)
	}
	return ready, desired
}

func replicaMapsFromStatefulSets(ns string) (ready map[string]int, desired map[string]int) {
	ready = make(map[string]int)
	desired = make(map[string]int)
	list, err := workload.GetStatefulSetsByNamespace(ns)
	if err != nil {
		return ready, desired
	}
	for i := range list {
		s := &list[i]
		replicas := constants.DefaultAddValue
		if s.Spec.Replicas != nil {
			replicas = int(*s.Spec.Replicas)
		}
		desired[s.Name] = replicas
		ready[s.Name] = int(s.Status.ReadyReplicas)
	}
	return ready, desired
}

func replicaMapsFromDaemonSets(ns string) (ready map[string]int, desired map[string]int) {
	ready = make(map[string]int)
	desired = make(map[string]int)
	list, err := workload.GetDaemonSetsByNamespace(ns)
	if err != nil {
		return ready, desired
	}
	for i := range list {
		d := &list[i]
		desired[d.Name] = int(d.Status.DesiredNumberScheduled)
		ready[d.Name] = int(d.Status.NumberReady)
	}
	return ready, desired
}

func healthFromCounts(totalReady, totalDesired int) application.Health {
	if totalDesired == constants.DefaultInitValue {
		return application.Health{
			Status:        appshared.HealthStatusUnknown,
			Reason:        nil,
			ReadyReplicas: constants.DefaultInitValue,
			TotalReplicas: constants.DefaultInitValue,
		}
	}
	reason := fmtReplicaReason(totalReady, totalDesired)
	var reasonPtr *string
	if reason != constants.EmptyString {
		reasonPtr = &reason
	}
	switch {
	case totalReady == constants.DefaultInitValue:
		return application.Health{
			Status: appshared.HealthStatusDown, Reason: reasonPtr, ReadyReplicas: totalReady,
			TotalReplicas: totalDesired,
		}
	case totalReady < totalDesired:
		return application.Health{
			Status: appshared.HealthStatusDegraded, Reason: reasonPtr,
			ReadyReplicas: totalReady, TotalReplicas: totalDesired,
		}
	default:
		return application.Health{
			Status: appshared.HealthStatusHealthy, Reason: nil,
			ReadyReplicas: totalReady, TotalReplicas: totalDesired,
		}
	}
}

func fmtReplicaReason(ready, total int) string {
	if total == constants.DefaultInitValue {
		return constants.EmptyString
	}
	if ready == total {
		return constants.EmptyString
	}
	return fmt.Sprintf(msgReplicasReady, ready, total)
}
