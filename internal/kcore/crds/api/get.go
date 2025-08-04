package api

import (
	"fmt"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/messages"
	metadata "github.com/plsyro/data-pkg/metadata/base"
	"github.com/plsyro/kcore-pkg/constants"
	crdUtils "github.com/plsyro/kcore-pkg/crds/utils"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/shared"

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
