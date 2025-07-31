package api

import (
	metadata "github.com/plsyro/data-pkg/metadata/base"
	"github.com/plsyro/kcore-pkg/constants"
	crdUtils "github.com/plsyro/kcore-pkg/crds/utils"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckCustomResourceExistence(metadata metadata.Metadata, name string) (bool, error) {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return false, err
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CRD_GET_TIMEOUT)
	defer cancel()

	_, err = resourceClient.Get(ctx, name, kubeApiMeta.GetOptions{})
	if err == nil {
		return true, nil
	}

	return false, nil
}
