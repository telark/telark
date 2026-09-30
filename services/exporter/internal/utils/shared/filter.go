package shared

import (
	"errors"
	"fmt"

	"github.com/telark/telark/services/exporter/internal/constants"
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

// List items may come from the informer store, so the view never writes into them.
func filterList(list *unstructured.UnstructuredList) (*unstructured.UnstructuredList, error) {
	if list == nil || list.Items == nil {
		return nil, errors.New(string(constants.ErrInvalidListOrEmptyItems))
	}

	filteredItems := &unstructured.UnstructuredList{
		Items: make([]unstructured.Unstructured, constants.DefaultInitValue, len(list.Items)),
	}
	for i := range list.Items {
		if out := ToView(&list.Items[i]); out != nil {
			filteredItems.Items = append(filteredItems.Items, unstructured.Unstructured{Object: out})
		}
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
	if _, ok := spec.(map[string]any); !ok {
		return nil, errors.New(string(constants.ErrSpecIsNotValidMap))
	}

	return &unstructured.Unstructured{Object: ToView(item)}, nil
}
