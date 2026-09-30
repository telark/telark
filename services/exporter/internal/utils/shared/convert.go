package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dataerrors "github.com/telark/telark/internal/data/errors"
	metadata "github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/kcore/crds/view"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func DeletionStamp(resource *unstructured.Unstructured) (string, bool) {
	ts := resource.GetDeletionTimestamp()
	if ts == nil {
		return constants.EmptyString, false
	}
	return ts.UTC().Format(time.RFC3339), true
}

// The CRD schema prunes an unknown spec key, so the timestamp only ever exists
// in memory: a typed read of a freshly fetched record sees it as terminating.
func ProjectDeletionTimestamp(resource *unstructured.Unstructured) {
	stamp, terminating := DeletionStamp(resource)
	spec, ok := resource.Object[constants.SpecField].(map[string]any)
	if terminating && ok {
		spec[constants.FieldDeletionTimestamp] = stamp
	}
}

func ConvertToCRDTemplate(md metadata.Metadata, name string, spec map[string]any) *unstructured.Unstructured {
	return buildCRDTemplate(md, map[string]any{constants.FieldName: name}, spec)
}

func ConvertToCRDTemplateWithFinalizers(
	md metadata.Metadata,
	name string,
	spec map[string]any,
	finalizers []string,
) *unstructured.Unstructured {
	meta := map[string]any{constants.FieldName: name}
	if len(finalizers) > constants.DefaultInitValue {
		meta[constants.FieldFinalizers] = finalizers
	}
	return buildCRDTemplate(md, meta, spec)
}

// The id is metadata.name and never a spec key; projected status keys go under
// status, which CreateCustomResourceWithStatus writes through the subresource.
func buildCRDTemplate(md metadata.Metadata, meta, body map[string]any) *unstructured.Unstructured {
	spec, status := view.SplitPatch(md, body)
	object := map[string]any{
		constants.FieldAPIVersion: md.GetAPIVersion(),
		constants.FieldKind:       md.Kind,
		constants.MetadataField:   meta,
		constants.SpecField:       spec,
	}
	if len(status) > constants.DefaultInitValue {
		object[constants.FieldStatus] = status
	}
	return &unstructured.Unstructured{Object: object}
}

// Decodes the view (spec, projected status, id). No spec is no value, not an
// error: callers treat it like a resource they did not ask to decode.
func SpecToStruct[T any](resource *unstructured.Unstructured) (*T, error) {
	spec := ToView(resource)
	if spec == nil {
		return nil, nil
	}

	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal(specBytes, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func StructToSpecMap(v any) (map[string]any, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf(string(dataerrors.ErrRestMarshalPayload), err)
	}

	var spec map[string]any
	if err := json.Unmarshal(bytes, &spec); err != nil {
		return nil, fmt.Errorf(string(dataerrors.ErrRestUnmarshalResourceToJSON), err)
	}

	return spec, nil
}

func UnstructuredToStruct[T any](resource *unstructured.Unstructured, specNotFoundErr, specInvalidErr, unmarshalErr dataerrors.Error) (*T, error) {
	spec, exists := resource.Object[constants.SpecField]
	if !exists {
		return nil, errors.New(string(specNotFoundErr))
	}

	if _, ok := spec.(map[string]any); !ok {
		return nil, errors.New(string(specInvalidErr))
	}

	specBytes, err := json.Marshal(ToView(resource))
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToMarshalSpec), err)
	}

	var result T
	if err := json.Unmarshal(specBytes, &result); err != nil {
		return nil, fmt.Errorf(string(unmarshalErr), err)
	}

	return &result, nil
}
