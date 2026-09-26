package manifest

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func UnstructuredWithoutManagedFields(src *unstructured.Unstructured) *unstructured.Unstructured {
	if src == nil {
		return nil
	}
	out := src.DeepCopy()
	unstructured.RemoveNestedField(out.Object, manifestMetadataKey, "managedFields")
	return out
}
