package cleanup

import (
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func projectCleanupView(obj *unstructured.Unstructured, refKeys []string) resourcesshared.CleanupView {
	view := resourcesshared.CleanupView{
		Name:            obj.GetName(),
		ResourceVersion: obj.GetResourceVersion(),
		Finalizers:      obj.GetFinalizers(),
	}
	if ts := obj.GetDeletionTimestamp(); ts != nil {
		s := ts.UTC().Format(timeFormatRFC3339)
		view.DeletionTimestamp = &s
	}
	if len(refKeys) > constants.DefaultInitValue {
		view.Refs = collectRefs(obj.Object, refKeys)
	}
	return view
}

func collectRefs(obj map[string]any, refKeys []string) map[string][]string {
	spec, ok := obj[fieldSpec].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string][]string, len(refKeys))
	for _, key := range refKeys {
		out[key] = readStringList(spec[key])
	}
	return out
}

func readStringList(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, constants.DefaultInitValue, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
