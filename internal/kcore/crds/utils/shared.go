package shared

import (
	"fmt"

	"github.com/plsyro/data/errors"
	metadata "github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/k8sclient"
	"k8s.io/client-go/dynamic"
)

func GetResourceClient(metadata metadata.Metadata) (dynamic.ResourceInterface, error) {
	return k8sclient.CreateCustomResourceClient(metadata)
}

func ValidateResourceName(name string) error {
	if name == "" {
		return fmt.Errorf("%s", errors.ErrResourceNameCannotBeEmpty)
	}
	return nil
}
