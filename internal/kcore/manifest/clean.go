package manifest

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func UnstructuredWithoutManagedFields(src *unstructured.Unstructured) *unstructured.Unstructured {
	if src == nil {
		return nil
	}
	out := src.DeepCopy()
	if out == nil || out.Object == nil {
		return out
	}
	meta, ok := out.Object["metadata"].(map[string]any)
	if !ok || meta == nil {
		return out
	}
	delete(meta, "managedFields")
	return out
}
