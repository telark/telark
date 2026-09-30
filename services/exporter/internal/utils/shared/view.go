package shared

import (
	"maps"

	"github.com/telark/data/metadata/base"
	"github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/kcore/crds/view"
	kubeshared "github.com/telark/kcore/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var metadataByKind = map[string]base.Metadata{
	v1alpha1.KindApplication:    v1alpha1.ApplicationMetadata,
	v1alpha1.KindProtectionPlan: v1alpha1.ProtectionPlanMetadata,
	v1alpha1.KindTelarkConfig:   v1alpha1.TelarkConfigMetadata,
	v1alpha1.KindCategory:       v1alpha1.CategoryMetadata,
	v1alpha1.KindUser:           v1alpha1.UserMetadata,
	v1alpha1.KindGroup:          v1alpha1.GroupMetadata,
	v1alpha1.KindAccessRole:     v1alpha1.AccessRoleMetadata,
	v1alpha1.KindPasskey:        v1alpha1.PasskeyMetadata,
	v1alpha1.KindSession:        v1alpha1.SessionMetadata,
}

// An object of an unknown kind still gets its id; it just has no status to project.
func MetadataForKind(kind string) base.Metadata {
	return metadataByKind[kind]
}

// The REST shape of a CR: spec, the status fields its kind projects, and id = metadata.name.
func ToView(resource *unstructured.Unstructured) map[string]any {
	return view.ToView(resource, MetadataForKind(resource.GetKind()))
}

// Projected status keys, in spec or at the top level (discovery's rollbacks), must reach
// the /status subresource: a merge patch on the main resource silently drops them.
func PatchCustomResource(md base.Metadata, name string, payload map[string]any) kubeshared.KubernetesAPIData {
	body := map[string]any{}
	if specBody, isMap := payload[constants.SpecField].(map[string]any); isMap {
		body = maps.Clone(specBody)
	}
	main := make(map[string]any, len(payload))
	for key, value := range payload {
		if _, projected := md.StatusFields[key]; projected {
			body[key] = value
		} else if key != constants.SpecField {
			main[key] = value
		}
	}
	spec, status := view.SplitPatch(md, body)

	if _, hasSpec := payload[constants.SpecField]; hasSpec {
		main[constants.SpecField] = spec
	}

	wroteMain := len(status) == constants.DefaultInitValue || len(spec) > constants.DefaultInitValue
	var result kubeshared.KubernetesAPIData
	if wroteMain {
		result = api.PatchCustomResource(md, name, main)
		if result.Status != kubeshared.StatusOK || len(status) == constants.DefaultInitValue {
			return result
		}
	}

	statusPayload := map[string]any{constants.FieldStatus: status}
	// A resourceVersion guards only the first write; the second would always conflict with it.
	if meta, hasMeta := payload[constants.MetadataField]; hasMeta && !wroteMain {
		statusPayload[constants.MetadataField] = meta
	}
	return api.PatchCustomResourceStatus(md, name, statusPayload)
}
