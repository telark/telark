package k8sclient

import (
	"fmt"

	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/constants"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CreateCustomResourceClient(metadata base.Metadata) (dynamic.ResourceInterface, error) {
	dynamicClient, err := InitDynamicClient()
	if err != nil {
		return nil, err
	}

	if metadata.BaseGroup == constants.EmptyString || metadata.Version == constants.EmptyString ||
		metadata.Plural == constants.EmptyString || metadata.Namespace == constants.EmptyString {
		return nil, fmt.Errorf(string(constants.ErrInvalidMetadata), metadata)
	}

	resourceClient := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    metadata.BaseGroup,
		Version:  metadata.Version,
		Resource: metadata.Plural,
	}).Namespace(metadata.Namespace)

	return resourceClient, nil
}
