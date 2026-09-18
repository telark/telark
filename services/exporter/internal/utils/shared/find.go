package shared

import (
	"errors"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindResourceBySpecField(
	resourceMetadata metadata.Metadata,
	listFormatErr dataerrors.Error,
	fieldName string,
	fieldValue string,
	notFoundErr dataerrors.Error,
) (*unstructured.Unstructured, error) {
	list, err := ListResources(resourceMetadata, listFormatErr)
	if err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		if spec, exists := item.Object[constants.SpecField].(map[string]any); exists {
			if value, ok := spec[fieldName].(string); ok && value == fieldValue {
				return &item, nil
			}
		}
	}

	return nil, errors.New(string(notFoundErr))
}

func CheckFieldValueExists(
	resourceMetadata metadata.Metadata,
	listFormatErr dataerrors.Error,
	fieldName string,
	fieldValue string,
) (bool, error) {
	list, err := ListResources(resourceMetadata, listFormatErr)
	if err != nil {
		return false, err
	}

	for _, item := range list.Items {
		if spec, exists := item.Object[constants.SpecField].(map[string]any); exists {
			if value, ok := spec[fieldName].(string); ok && value == fieldValue {
				return true, nil
			}
		}
	}

	return false, nil
}
