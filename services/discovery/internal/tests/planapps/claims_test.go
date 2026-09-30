package planapps

import (
	"context"
	"slices"
	"testing"

	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/applications"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/telark/telark/services/discovery/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	appsGroup    = "apps"
	groupVersion = "v1"
)

const claimNamespace = "prod"

func workload(apiVersion, kind, name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": claimNamespace},
		"spec":       spec,
	}}
}

func podTemplate(claims ...string) map[string]any {
	volumes := make([]any, constants.DefaultInitValue, len(claims))
	for _, claim := range claims {
		volumes = append(volumes, map[string]any{
			"name":                  claim,
			"persistentVolumeClaim": map[string]any{"claimName": claim},
		})
	}
	return map[string]any{"spec": map[string]any{"volumes": volumes}}
}

func claimDyn(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	listKinds := map[schema.GroupVersionResource]string{
		{Group: appsGroup, Version: groupVersion, Resource: "deployments"}:  "DeploymentList",
		{Group: appsGroup, Version: groupVersion, Resource: "statefulsets"}: "StatefulSetList",
		{Group: appsGroup, Version: groupVersion, Resource: "daemonsets"}:   "DaemonSetList",
		{Group: "batch", Version: groupVersion, Resource: "jobs"}:           "JobList",
		{Group: "batch", Version: groupVersion, Resource: "cronjobs"}:       "CronJobList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
}

func resource(kind, name string) applicationmodel.Resource {
	return applicationmodel.Resource{Kind: kind, Name: name, Namespace: claimNamespace}
}

// A PersistentVolumeClaim is referenced by an application, never owned by it, so the claim names
// have to come off the workloads: a CronJob keeps its pod template one level deeper, and a
// StatefulSet's volumeClaimTemplates expand to one PVC per ordinal.
func TestClusterClaimReader(t *testing.T) {
	deploy := workload("apps/v1", "Deployment", "wa1", map[string]any{
		"template": podTemplate("pvc-a"),
	})
	cron := workload("batch/v1", "CronJob", "wa2", map[string]any{
		"jobTemplate": map[string]any{"spec": map[string]any{"template": podTemplate("pvc-b")}},
	})
	sts := workload("apps/v1", "StatefulSet", "wa3", map[string]any{
		"template": podTemplate("pvc-c"),
		"volumeClaimTemplates": []any{
			map[string]any{"metadata": map[string]any{"name": "data"}},
		},
	})

	read := applications.NewClusterClaimReader(claimDyn(deploy, cron, sts))
	got := read(context.Background(), []applicationmodel.Resource{
		resource("Deployment", "wa1"),
		resource("CronJob", "wa2"),
		resource("StatefulSet", "wa3"),
		resource("ConfigMap", "cm-a"), // not a workload, contributes nothing
		resource("Deployment", "gone"),
	})
	slices.Sort(got)

	want := []string{"data-wa3-*", "pvc-a", "pvc-b", "pvc-c"}
	if !slices.Equal(got, want) {
		t.Fatalf("claims = %v, want %v", got, want)
	}
}

// A workload the reader cannot fetch contributes no claims instead of failing the plan write.
func TestClusterClaimReaderToleratesMissingWorkloads(t *testing.T) {
	read := applications.NewClusterClaimReader(claimDyn())
	got := read(context.Background(), []applicationmodel.Resource{resource("Deployment", "gone")})
	if len(got) != constants.DefaultInitValue {
		t.Fatalf("claims = %v, want none", got)
	}
}
