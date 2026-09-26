package applications

import (
	"context"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	kindDeployment  = "Deployment"
	kindStatefulSet = "StatefulSet"
	kindDaemonSet   = "DaemonSet"
	kindJob         = "Job"
	kindCronJob     = "CronJob"

	fieldSpec                 = "spec"
	fieldTemplate             = "template"
	fieldJobTemplate          = "jobTemplate"
	fieldVolumes              = "volumes"
	fieldPVC                  = "persistentVolumeClaim"
	fieldClaimName            = "claimName"
	fieldVolumeClaimTemplates = "volumeClaimTemplates"
	fieldMetadata             = "metadata"
	fieldName                 = "name"

	claimSeparator       = "-"
	claimOrdinalWildcard = "*"
)

// A PVC is referenced, not owned, so it is absent from the application's resource set and has
// to be read from the workloads that mount it.
type ClaimReader func(ctx context.Context, resources []applicationmodel.Resource) []string

var workloadGVRs = map[string]schema.GroupVersionResource{
	kindDeployment:  {Group: "apps", Version: "v1", Resource: "deployments"},
	kindStatefulSet: {Group: "apps", Version: "v1", Resource: "statefulsets"},
	kindDaemonSet:   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	kindJob:         {Group: "batch", Version: "v1", Resource: "jobs"},
	kindCronJob:     {Group: "batch", Version: "v1", Resource: "cronjobs"},
}

// A workload that cannot be read contributes no claims rather than failing the whole plan
// write: the storage rule then covers less, which the health check reports as drift.
func NewClusterClaimReader(dyn dynamic.Interface) ClaimReader {
	return func(ctx context.Context, resources []applicationmodel.Resource) []string {
		seen := map[string]struct{}{}
		for _, res := range resources {
			gvr, ok := workloadGVRs[res.Kind]
			if !ok {
				continue
			}
			obj, err := dyn.Resource(gvr).Namespace(res.Namespace).Get(ctx, res.Name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			collectClaims(obj, res.Kind, res.Name, seen)
		}
		out := make([]string, constants.DefaultInitValue, len(seen))
		for name := range seen {
			out = append(out, name)
		}
		return out
	}
}

func collectClaims(obj *unstructured.Unstructured, kind, name string, seen map[string]struct{}) {
	for _, claim := range claimsFromVolumes(obj, kind) {
		seen[claim] = struct{}{}
	}
	if kind == kindStatefulSet {
		for _, pattern := range claimTemplatePatterns(obj, name) {
			seen[pattern] = struct{}{}
		}
	}
}

func claimsFromVolumes(obj *unstructured.Unstructured, kind string) []string {
	volumes, found, err := unstructured.NestedSlice(obj.Object, podSpecFields(kind, fieldVolumes)...)
	if err != nil || !found {
		return nil
	}
	out := make([]string, constants.DefaultInitValue, len(volumes))
	for _, raw := range volumes {
		volume, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		claim, found, err := unstructured.NestedString(volume, fieldPVC, fieldClaimName)
		if err != nil || !found || claim == constants.EmptyString {
			continue
		}
		out = append(out, claim)
	}
	return out
}

// A volumeClaimTemplate produces one PVC per replica, named <template>-<statefulset>-<ordinal>.
// The ordinals are not knowable at render time and grow on scale-up, so the rule matches the
// wildcard Kyverno already supports in resource names.
func claimTemplatePatterns(obj *unstructured.Unstructured, name string) []string {
	templates, found, err := unstructured.NestedSlice(obj.Object, fieldSpec, fieldVolumeClaimTemplates)
	if err != nil || !found {
		return nil
	}
	out := make([]string, constants.DefaultInitValue, len(templates))
	for _, raw := range templates {
		template, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		templateName, found, err := unstructured.NestedString(template, fieldMetadata, fieldName)
		if err != nil || !found || templateName == constants.EmptyString {
			continue
		}
		out = append(out, templateName+claimSeparator+name+claimSeparator+claimOrdinalWildcard)
	}
	return out
}

func podSpecFields(kind, field string) []string {
	if kind == kindCronJob {
		return []string{fieldSpec, fieldJobTemplate, fieldSpec, fieldTemplate, fieldSpec, field}
	}
	return []string{fieldSpec, fieldTemplate, fieldSpec, field}
}
