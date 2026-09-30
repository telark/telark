package diff

import (
	"context"
	"fmt"
	"strings"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/changes"
	appshared "github.com/telark/telark/services/discovery/internal/core/applications/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func filterCancelledScalarChanges(
	ctx context.Context,
	getManifest func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error),
	fresh *application.Application,
	appChanges []application.ApplicationChange,
	dopts *DiffOptions,
) []application.ApplicationChange {
	if dopts == nil || len(dopts.PrewrittenSnapshots) == constants.DefaultInitValue {
		return appChanges
	}
	if getManifest == nil || fresh == nil {
		return appChanges
	}
	// Images have no counterpart: a workload whose pre-image the flush lost is snapshotted live, so a
	// match with fresh dropped a real change and the no-change publish made it the CR baseline.
	preReplicas, sawWorkload := extractWorkloadStateFromSnapshots(ctx, getManifest, dopts)
	out := make([]application.ApplicationChange, constants.DefaultInitValue, len(appChanges))
	for i := range appChanges {
		c := appChanges[i]
		if c.Field == changes.ChangeFieldReplicas && sawWorkload &&
			preReplicas == int64(fresh.Health.TotalReplicas) {
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Info(fmt.Sprintf(
				string(constants.InfoHistoryReplicaChangeCancelled), fresh.Name, preReplicas, fresh.Health.TotalReplicas))
			continue
		}
		out = append(out, c)
	}
	return out
}

func extractWorkloadStateFromSnapshots(
	ctx context.Context,
	getManifest func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error),
	dopts *DiffOptions,
) (replicas int64, sawWorkload bool) {
	scope := config.DefaultSnapshotScope()
	for i := range dopts.PrewrittenSnapshots {
		s := dopts.PrewrittenSnapshots[i]
		id := strings.TrimSpace(s.ID)
		ns := strings.TrimSpace(s.Namespace)
		if id == constants.EmptyString || ns == constants.EmptyString {
			continue
		}
		objs, err := getManifest(ctx, id, scope, ns, s.Generation)
		if err != nil || len(objs) == constants.DefaultInitValue {
			continue
		}
		for j := range objs {
			u := &objs[j]
			switch u.GetKind() {
			case appshared.KindDeployment, appshared.KindStatefulSet, appshared.KindDaemonSet:
				sawWorkload = true
				replicas += replicaCountFromWorkload(u)
			default:
			}
		}
	}
	return replicas, sawWorkload
}

func replicaCountFromWorkload(u *unstructured.Unstructured) int64 {
	if u == nil {
		return int64(constants.DefaultInitValue)
	}
	r, found, _ := unstructured.NestedInt64(
		u.Object,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldReplicas,
	)
	if found {
		return r
	}
	return int64(constants.DefaultAddValue)
}

func imagesFromPodTemplate(u *unstructured.Unstructured) []string {
	containers, found, _ := unstructured.NestedSlice(
		u.Object,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldTemplate,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldContainers,
	)
	if !found {
		return nil
	}
	var out []string
	for i := range containers {
		c, ok := containers[i].(map[string]any)
		if !ok {
			continue
		}
		imgRaw, has := c[constants.K8sObjectFieldImage]
		if !has {
			continue
		}
		imgStr, ok := imgRaw.(string)
		if !ok {
			continue
		}
		out = append(out, strings.TrimSpace(imgStr))
	}
	return out
}

// seedBaselineFromPreImage diffs replicas against the state captured before the
// change rather than the CR, which a concurrent consumer publish may already
// have refreshed to the post-change value.
// ponytail: replicas only; seed images/resources the same way if that race shows up.
func seedBaselineFromPreImage(
	ctx context.Context,
	getManifest func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error),
	stored *application.Application,
	dopts *DiffOptions,
) *application.Application {
	if dopts == nil || len(dopts.PrewrittenSnapshots) == constants.DefaultInitValue || getManifest == nil {
		return stored
	}
	replicas, sawWorkload := extractWorkloadStateFromSnapshots(ctx, getManifest, dopts)
	if !sawWorkload {
		return stored
	}
	seeded := *stored
	seeded.Health.TotalReplicas = int(replicas)
	return &seeded
}
