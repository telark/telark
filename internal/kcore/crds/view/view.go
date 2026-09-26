package view

import (
	"maps"
	"time"

	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Returns nil when the object carries no spec map, the case the exporter's
// filter skips in lists and rejects for single items.
func ToView(obj *unstructured.Unstructured, md base.Metadata) map[string]any {
	if obj == nil {
		return nil
	}
	spec, ok := obj.Object[constants.CrdSpecField].(map[string]any)
	if !ok {
		return nil
	}

	out := maps.Clone(spec)
	if out == nil {
		out = map[string]any{}
	}
	projectStatus(out, obj, md)
	out[constants.CrdViewIDField] = obj.GetName()
	if ts := obj.GetDeletionTimestamp(); ts != nil {
		out[constants.CrdViewDeletionTimestamp] = ts.UTC().Format(time.RFC3339)
	}
	if meta := metadataSubset(obj); meta != nil {
		out[constants.CrdMetadataField] = meta
	}
	return out
}

func SplitPatch(md base.Metadata, body map[string]any) (spec, status map[string]any) {
	spec = map[string]any{}
	status = map[string]any{}
	for key, value := range body {
		if isViewOnlyKey(key) {
			continue
		}
		statusKey, projected := md.StatusFields[key]
		switch {
		case !projected:
			spec[key] = value
		case statusKey == constants.CrdWholeStatusProjection:
			if whole, isMap := value.(map[string]any); isMap {
				maps.Copy(status, whole)
			}
		default:
			status[statusKey] = value
		}
	}
	return spec, status
}

func projectStatus(out map[string]any, obj *unstructured.Unstructured, md base.Metadata) {
	status, ok := obj.Object[constants.CrdStatusField].(map[string]any)
	if !ok {
		return
	}
	for viewKey, statusKey := range md.StatusFields {
		if statusKey == constants.CrdWholeStatusProjection {
			out[viewKey] = status
			continue
		}
		if value, found := status[statusKey]; found {
			out[viewKey] = value
		}
	}
}

func isViewOnlyKey(key string) bool {
	return key == constants.CrdViewIDField ||
		key == constants.CrdMetadataField ||
		key == constants.CrdViewDeletionTimestamp
}

func metadataSubset(obj *unstructured.Unstructured) map[string]any {
	raw, ok := obj.Object[constants.CrdMetadataField].(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]any{}
	for _, field := range []string{constants.CrdResourceVersionField, constants.CrdNameField, constants.CrdNamespaceField} {
		if v, found := raw[field]; found {
			out[field] = v
		}
	}
	if len(out) == constants.ZeroValue {
		return nil
	}
	return out
}
