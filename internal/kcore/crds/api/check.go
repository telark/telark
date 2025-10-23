package api

import (
	"github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	crdutils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
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
	if err == nil {
		return true, nil
	}

	return false, nil
}
