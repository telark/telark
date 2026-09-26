package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
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

func buildCRDTemplate(md metadata.Metadata, meta, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			constants.FieldAPIVersion: md.GetAPIVersion(),
			constants.FieldKind:       md.Kind,
			constants.MetadataField:   meta,
			constants.SpecField:       spec,
		},
	}
}

// A resource with no spec is reported as no value rather than an error: callers
// treat it the same as a resource they did not ask to decode.
func SpecToStruct[T any](resource *unstructured.Unstructured) (*T, error) {
	spec, ok := resource.Object[constants.SpecField].(map[string]any)
	if !ok || spec == nil {
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

	specMap, ok := spec.(map[string]any)
	if !ok {
		return nil, errors.New(string(specInvalidErr))
	}

	specBytes, err := json.Marshal(specMap)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToMarshalSpec), err)
	}

	var result T
	if err := json.Unmarshal(specBytes, &result); err != nil {
		return nil, fmt.Errorf(string(unmarshalErr), err)
	}

	return &result, nil
}
