package install

import (
	"fmt"
	"maps"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/check"
	"github.com/telark/telark/cmd/telark/internal/constants"
	helmkube "helm.sh/helm/v4/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
)

// Helm's dry run renders and validates the objects but never sends them, so each one goes to the server as a dry-run
// apply under helm's field manager: conflicts, immutable fields and admission refusals show before anything changes.
func serverApply(targets helmkube.ResourceList, namespace string, force bool) check.Result {
	var refused []string
	for _, target := range targets {
		if err := applyObject(target, namespace, force, true); err != nil {
			reason := strings.Join(strings.Fields(err.Error()), constants.Space)
			refused = append(refused, example(target.Mapping.Resource.Resource, target.Name, reason))
		}
	}
	if len(refused) > constants.DefaultInitValue {
		return check.Fail(constants.CheckServerDryRun, fmt.Sprintf(constants.MsgServerRefusesObjects, len(refused), len(targets), examples(refused)))
	}
	return check.Pass(constants.CheckServerDryRun, fmt.Sprintf(constants.MsgServerAccepts, len(targets)))
}

// Helm labels and annotates every object it applies, so the probe and the storage copy send them the same way.
func applyObject(target *resource.Info, namespace string, force, dryRun bool) error {
	if object, ok := target.Object.(metav1.Object); ok {
		object.SetLabels(merged(object.GetLabels(), map[string]string{constants.ManagedByLabel: constants.ManagedByHelm}))
		object.SetAnnotations(merged(object.GetAnnotations(), map[string]string{
			constants.ReleaseNameAnnotation: constants.ReleaseName, constants.ReleaseNamespaceAnnotation: namespace}))
	}
	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, target.Object)
	if err == nil {
		helper := resource.NewHelper(target.Client, target.Mapping).DryRun(dryRun).
			WithFieldManager(constants.HelmFieldManager).WithFieldValidation(metav1.FieldValidationStrict)
		_, err = helper.Patch(target.Namespace, target.Name, types.ApplyPatchType, data, &metav1.PatchOptions{Force: &force})
	}
	return err
}

func merged(current, added map[string]string) map[string]string {
	result := map[string]string{}
	maps.Copy(result, current)
	maps.Copy(result, added)
	return result
}
