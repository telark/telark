package cleanup

import (
	"slices"
	"time"

	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func projectCleanupView(obj *unstructured.Unstructured, refKeys []string, hidden map[string]bool) resourcesshared.CleanupView {
	view := resourcesshared.CleanupView{
		Name:            obj.GetName(),
		ResourceVersion: obj.GetResourceVersion(),
		Finalizers:      obj.GetFinalizers(),
	}
	if ts := obj.GetDeletionTimestamp(); ts != nil {
		s := ts.UTC().Format(time.RFC3339)
		view.DeletionTimestamp = &s
	}
	if len(refKeys) > constants.DefaultInitValue {
		view.Refs = collectRefs(obj.Object, refKeys, hidden)
	}
	return view
}

func collectRefs(obj map[string]any, refKeys []string, hidden map[string]bool) map[string][]string {
	spec, ok := obj[constants.SpecField].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string][]string, len(refKeys))
	for _, key := range refKeys {
		refs := readStringList(spec[key])
		if key == constants.FieldUserRefs {
			refs = slices.DeleteFunc(refs, func(id string) bool { return hidden[id] })
		}
		out[key] = refs
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
