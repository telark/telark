package k8sclient

import (
	"fmt"

	metadata "github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CreateCustomResourceClient(metadata metadata.Metadata) (dynamic.ResourceInterface, error) {
	dynamicClient, err := InitDynamicClient()
	if err != nil {
		return nil, err
	}

	if metadata.BaseGroup == "" || metadata.Version == "" || metadata.Plural == "" || metadata.Namespace == "" {
		return nil, fmt.Errorf(string(constants.ErrInvalidMetadata), metadata)
	}

	resourceClient := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    metadata.BaseGroup,
		Version:  metadata.Version,
		Resource: metadata.Plural,
	}).Namespace(metadata.Namespace)

	return resourceClient, nil
}
