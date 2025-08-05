package api

import (
	metadata "github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	crdUtils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckCustomResourceExistsByName(name string, metadata metadata.Metadata) (bool, error) {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return false, err
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdGetTimeout)
	defer cancel()

	_, err = resourceClient.Get(ctx, name, kubeApiMeta.GetOptions{})
	if err == nil {
		return true, nil
	}

	return false, nil
}
