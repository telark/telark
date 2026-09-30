package utils

import (
	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/kcore/constants"
	"github.com/telark/telark/internal/kcore/k8sclient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
)

func GetResourceClient(metadata base.Metadata) (dynamic.ResourceInterface, error) {
	return k8sclient.CreateCustomResourceClient(metadata)
}

func ValidateResourceName(name string) error {
	if name == constants.EmptyString {
		return k8serrors.NewBadRequest(string(errors.ErrResourceNameCannotBeEmpty))
	}
	return nil
}
