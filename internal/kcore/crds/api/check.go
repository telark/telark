package api

import (
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/kcore/constants"
	crdutils "github.com/telark/telark/internal/kcore/crds/utils"
	"github.com/telark/telark/internal/kcore/resilience/timeout"
	"github.com/telark/telark/internal/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckCustomResourceExistsByName(name string, metadata base.Metadata) (bool, error) {
	if err := crdutils.ValidateResourceName(name); err != nil {
		return false, err
	}

	resourceClient, err := crdutils.GetResourceClient(metadata)
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.CrdGetTimeout)
	defer cancel()

	_, err = resourceClient.Get(ctx, name, k8smetav1.GetOptions{})
	return shared.ExistsFromGetError(err)
}
