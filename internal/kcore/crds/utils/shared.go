package utils //nolint:revive // Directory name must remain "utils" for import compatibility.

import (
	"fmt"

	"github.com/telark/data/errors"
	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/k8sclient"
	"k8s.io/client-go/dynamic"
)

func GetResourceClient(metadata base.Metadata) (dynamic.ResourceInterface, error) {
	return k8sclient.CreateCustomResourceClient(metadata)
}

func ValidateResourceName(name string) error {
	if name == "" {
		return fmt.Errorf("%s", errors.ErrResourceNameCannotBeEmpty)
	}
	return nil
}
