package api

import (
	"fmt"

	metadata "github.com/plsyro/data/data/base"
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/kcore/constants"
	crdUtils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"

	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCustomResourceByName(name string, metadata metadata.Metadata) shared.KubernetesAPIData {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ErrResourceNameCannotBeEmpty), nil, err)
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdGetTimeout)
	defer cancel()

	resource, err := resourceClient.Get(ctx, name, kubeApiMeta.GetOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrGetResource), nil, fmt.Errorf("%s %s :%v", string(errors.ErrGetResource), name, err))
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessGetResource), resource, nil)
}

func ListCustomResources(metadata metadata.Metadata) shared.KubernetesAPIData {
	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdListTimeout)
	defer cancel()

	resourceList, err := resourceClient.List(ctx, kubeApiMeta.ListOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrGetResource), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessListResources), resourceList, nil)
}
