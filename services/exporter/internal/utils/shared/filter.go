package shared

import (
	"errors"
	"fmt"
	"maps"

	"github.com/telark/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FilterData(item any) (any, error) {
	switch v := item.(type) {
	case *unstructured.UnstructuredList:
		return filterList(v)
	case *unstructured.Unstructured:
		return filterSingleItem(v)
	default:
		return nil, fmt.Errorf(string(constants.ErrUnsupportedItemType), item)
	}
}

func filterList(list *unstructured.UnstructuredList) (*unstructured.UnstructuredList, error) {
	if list == nil || list.Items == nil {
		return nil, errors.New(string(constants.ErrInvalidListOrEmptyItems))
	}

	filteredItems := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{},
	}

	for i := range list.Items {
		spec, exists := list.Items[i].Object[constants.SpecField]
		if !exists {
			continue
		}

		specMap, ok := spec.(map[string]any)
		if !ok {
			continue
		}

		filteredItems.Items = append(filteredItems.Items, unstructured.Unstructured{
			Object: withDeletionTimestamp(&list.Items[i], specMap),
		})
	}

	return filteredItems, nil
}

func filterSingleItem(item *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if item == nil {
		return nil, errors.New(string(constants.ErrItemIsNil))
	}

	spec, exists := item.Object[constants.SpecField]
	if !exists {
		return nil, errors.New(string(constants.ErrSpecFieldNotFound))
	}

	specMap, ok := spec.(map[string]any)
	if !ok {
		return nil, errors.New(string(constants.ErrSpecIsNotValidMap))
	}

	out := make(map[string]any, len(specMap)+constants.DefaultIncrementValue)
	maps.Copy(out, specMap)
	out = withDeletionTimestamp(item, out)
	if meta := minimalMetadataSubset(item); meta != nil {
		out[constants.MetadataField] = meta
	}

	return &unstructured.Unstructured{Object: out}, nil
}

// Typed readers over HTTP (peers resolving grants) see a terminating record
// the way the in-process reader does. The input spec is never mutated: list
// items may come from a shared store.
func withDeletionTimestamp(item *unstructured.Unstructured, spec map[string]any) map[string]any {
	stamp, terminating := DeletionStamp(item)
	if !terminating {
		return spec
	}
	out := maps.Clone(spec)
	out[constants.FieldDeletionTimestamp] = stamp
	return out
}

func minimalMetadataSubset(item *unstructured.Unstructured) map[string]any {
	raw, ok := item.Object[constants.MetadataField].(map[string]any)
	if !ok || raw == nil {
		return nil
	}
	out := map[string]any{}
	for _, field := range []string{constants.ResourceVersionField, constants.FieldName, constants.FieldNamespace} {
		if v, ok := raw[field]; ok {
			out[field] = v
		}
	}
	if len(out) == constants.DefaultInitValue {
		return nil
	}
	return out
}
