package shared

import (
	"fmt"

	"github.com/plsyro/data-pkg/errors"
	metadata "github.com/plsyro/data-pkg/metadata/base"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"k8s.io/client-go/dynamic"
)

func GetResourceClient(metadata metadata.Metadata) (dynamic.ResourceInterface, error) {
	return k8sclient.CreateCustomResourceClient(metadata)
}

func ValidateResourceName(name string) error {
	if name == "" {
		return fmt.Errorf(string(errors.ERROR_RESOURCE_NAME_CANNOT_BE_EMPTY))
	}
	return nil
}
