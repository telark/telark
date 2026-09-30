package core

import (
	"fmt"
	"slices"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/internal/kcore/resources/workload"
	"github.com/telark/telark/services/discovery/internal/constants"
	appshared "github.com/telark/telark/services/discovery/internal/core/applications/shared"
	k8sbatchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if totalDesired == constants.DefaultInitValue {
		if h, ok := healthFromJobRuns(app); ok {
			return h
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

const (
	healthReasonJobRunning   = "job run in progress"
	healthReasonJobSucceeded = "last job run succeeded"
	healthReasonJobFailed    = "last job run failed"
)

type jobSelectors struct {
	cronByNS map[string]map[string]bool
	jobByNS  map[string]map[string]bool
}

// healthFromJobRuns scores applications made of CronJobs and Jobs by their most
// recent run: running or succeeded is healthy, failed is degraded.
func healthFromJobRuns(app *application.Application) (application.Health, bool) {
	sel := selectAppJobs(app)
	if len(sel.cronByNS) == constants.DefaultInitValue && len(sel.jobByNS) == constants.DefaultInitValue {
		return application.Health{}, false
	}
	latest := latestAppJob(sel)
	if latest == nil {
		return application.Health{}, false
	}
	return healthFromJob(latest), true
}

func selectAppJobs(app *application.Application) jobSelectors {
	sel := jobSelectors{cronByNS: make(map[string]map[string]bool), jobByNS: make(map[string]map[string]bool)}
	for _, r := range app.Resources {
		switch r.Kind {
		case appshared.KindCronJob:
			addName(sel.cronByNS, r.Namespace, r.Name)
		case appshared.KindJob:
			addName(sel.jobByNS, r.Namespace, r.Name)
		default:
		}
	}
	return sel
}

func addName(byNS map[string]map[string]bool, ns, name string) {
	if byNS[ns] == nil {
		byNS[ns] = make(map[string]bool)
	}
	byNS[ns][name] = true
}

func latestAppJob(sel jobSelectors) *k8sbatchv1.Job {
	namespaces := make(map[string]bool)
	for ns := range sel.cronByNS {
		namespaces[ns] = true
	}
	for ns := range sel.jobByNS {
		namespaces[ns] = true
	}
	var latest *k8sbatchv1.Job
	for ns := range namespaces {
		jobs, err := workload.GetJobsByNamespace(ns)
		if err != nil {
			continue
		}
		for i := range jobs {
			j := &jobs[i]
			if !sel.jobByNS[ns][j.Name] && !ownedByAppCronJob(j, sel.cronByNS[ns]) {
				continue
			}
			if latest == nil || j.CreationTimestamp.After(latest.CreationTimestamp.Time) {
				latest = j
			}
		}
	}
	return latest
}

func healthFromJob(j *k8sbatchv1.Job) application.Health {
	reason := healthReasonJobRunning
	status := appshared.HealthStatusHealthy
	switch {
	case j.Status.Active > constants.DefaultInitValue:
	case j.Status.Succeeded > constants.DefaultInitValue:
		reason = healthReasonJobSucceeded
	case j.Status.Failed > constants.DefaultInitValue:
		reason = healthReasonJobFailed
		status = appshared.HealthStatusDegraded
	default:
	}
	return application.Health{Status: status, Reason: &reason}
}

func ownedByAppCronJob(j *k8sbatchv1.Job, cronNames map[string]bool) bool {
	return slices.ContainsFunc(j.OwnerReferences, func(o metav1.OwnerReference) bool {
		return o.Kind == appshared.KindCronJob && cronNames[o.Name]
	})
}
