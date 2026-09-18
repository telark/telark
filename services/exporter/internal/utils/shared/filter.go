package shared

import (
	"errors"
	"fmt"
	"maps"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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

	for _, item := range list.Items {
		spec, exists := item.Object[constants.SpecField]
		if !exists {
			continue
		}

		specMap, ok := spec.(map[string]any)
		if !ok {
			continue
		}

		filteredItems.Items = append(filteredItems.Items, unstructured.Unstructured{
			Object: specMap,
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
	if meta := minimalMetadataSubset(item); meta != nil {
		out[constants.MetadataField] = meta
	}

	return &unstructured.Unstructured{Object: out}, nil
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

func FilterResourceOrRespond(resource *unstructured.Unstructured) (any, bool) {
	filteredResource, err := FilterData(resource)
	if err != nil {
		responseutils.LogAndReturnResponse(
			http.StatusInternalServerError,
			response.OperationError,
			string(dataerrors.ErrFilterRes),
			nil,
			err,
		)
		return nil, false
	}
	return filteredResource, true
}
