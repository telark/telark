package rollbackctl

import (
	"context"
	"testing"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/handlers/rollback"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	replaceNamespace = "prod"
	replaceName      = "web"
	missingName      = "web-missing"
	envName          = "MARK"
	addedEnvValue    = "c"
	kindDeployment   = "Deployment"
	containerName    = "web"
	imageOld         = "nginx:1.0"
	fieldSpec        = "spec"
	fieldTemplate    = "template"
	fieldContainers  = "containers"
	fieldEnv         = "env"
)

var deploymentsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

func deployment(name string, env []any) *unstructured.Unstructured {
	container := map[string]any{"name": containerName, "image": imageOld}
	if env != nil {
		container[fieldEnv] = env
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       kindDeployment,
		"metadata":   map[string]any{"name": name, "namespace": replaceNamespace},
		fieldSpec: map[string]any{fieldTemplate: map[string]any{fieldSpec: map[string]any{
			fieldContainers: []any{container},
		}}},
	}}
}

func containerEnv(t *testing.T, u *unstructured.Unstructured) []any {
	t.Helper()
	containers, _, err := unstructured.NestedSlice(u.Object, fieldSpec, fieldTemplate, fieldSpec, fieldContainers)
	if err != nil || len(containers) == constants.DefaultInitValue {
		t.Fatalf("containers: %v", err)
	}
	first, ok := containers[constants.DefaultInitValue].(map[string]any)
	testutil.Equal(t, "container shape", ok, true)
	env, ok := first[fieldEnv].([]any)
	if !ok {
		return nil
	}
	return env
}

// A server-side apply left env[MARK], added by kubectl after the snapshot, on the
// rolled-back Deployment: apply only prunes fields its own manager set. The replace
// restores the object as snapshotted and creates one the cluster no longer has.
func TestReplaceUnstructuredRemovesFieldsAddedAfterTheSnapshot(t *testing.T) {
	live := deployment(replaceName, []any{map[string]any{"name": envName, "value": addedEnvValue}})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{deploymentsGVR: "DeploymentList"}, live,
	)
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: kindDeployment}, meta.RESTScopeNamespace)
	snapshot := []unstructured.Unstructured{*deployment(replaceName, nil), *deployment(missingName, nil)}
	replaced := constants.DefaultInitValue

	err := rollback.ReplaceUnstructured(context.Background(), dyn, mapper, snapshot, false, func(string, string, string) {
		replaced++
	})
	testutil.Equal(t, "replace", err, nil)
	testutil.Equal(t, "objects replaced", replaced, len(snapshot))

	ns := dyn.Resource(deploymentsGVR).Namespace(replaceNamespace)
	got, err := ns.Get(context.Background(), replaceName, metav1.GetOptions{})
	testutil.Equal(t, "live read", err, nil)
	testutil.Equal(t, "added env removed", len(containerEnv(t, got)), constants.DefaultInitValue)
	_, err = ns.Get(context.Background(), missingName, metav1.GetOptions{})
	testutil.Equal(t, "missing object recreated", err, nil)
}
