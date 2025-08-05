package api

import (
	"fmt"

	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	crdutils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCustomResourceByName(name string, metadata base.Metadata) shared.KubernetesAPIData {
	if err := crdutils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(
			shared.StatusBadRequest,
			string(errors.ErrResourceNameCannotBeEmpty),
			nil, err)
	}

	resourceClient, err := crdutils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdGetTimeout)
	defer cancel()

	resource, err := resourceClient.Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(
			shared.StatusInternalServerError,
			string(errors.ErrGetResource),
			nil, fmt.Errorf("%s %s :%v", string(errors.ErrGetResource), name, err))
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessGetResource), resource, nil)
}

func ListCustomResources(metadata base.Metadata) shared.KubernetesAPIData {
	resourceClient, err := crdutils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdListTimeout)
	defer cancel()

	resourceList, err := resourceClient.List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrGetResource), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessListResources), resourceList, nil)
}
