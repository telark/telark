package diff

import (
	"context"
	"strings"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
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
	preReplicas, preImages, sawWorkload := extractWorkloadStateFromSnapshots(ctx, getManifest, dopts)
	out := make([]application.ApplicationChange, constants.DefaultInitValue, len(appChanges))
	for i := range appChanges {
		c := appChanges[i]
		if c.Field == changes.ChangeFieldReplicas && sawWorkload &&
			preReplicas == int64(fresh.Health.TotalReplicas) {
			continue
		}
		if c.Field == changes.ChangeFieldImage && len(preImages) > constants.DefaultInitValue &&
			imageSetMatchesFresh(preImages, fresh.Images) {
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
) (replicas int64, images map[string]struct{}, sawWorkload bool) {
	scope := config.DefaultSnapshotScope()
	images = make(map[string]struct{})
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
				for _, img := range imagesFromPodTemplate(u) {
					if img != constants.EmptyString {
						images[img] = struct{}{}
					}
				}
			default:
			}
		}
	}
	return replicas, images, sawWorkload
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

func imageSetMatchesFresh(pre map[string]struct{}, fresh []string) bool {
	if len(pre) == constants.DefaultInitValue {
		return false
	}
	freshSet := make(map[string]struct{}, len(fresh))
	for i := range fresh {
		t := strings.TrimSpace(fresh[i])
		if t != constants.EmptyString {
			freshSet[t] = struct{}{}
		}
	}
	if len(pre) != len(freshSet) {
		return false
	}
	for img := range pre {
		if _, ok := freshSet[img]; !ok {
			return false
		}
	}
	return true
}
