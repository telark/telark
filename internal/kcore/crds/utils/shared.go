package shared

import (
	"fmt"

	metadata "github.com/plsyro/data/data/base"
	"github.com/plsyro/data/errors"
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
