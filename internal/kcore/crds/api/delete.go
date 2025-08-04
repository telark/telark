package api

import (
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/messages"
	metadata "github.com/plsyro/data-pkg/metadata/base"
	"github.com/plsyro/kcore-pkg/constants"
	crdUtils "github.com/plsyro/kcore-pkg/crds/utils"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteCustomResourceByName(name string, metadata metadata.Metadata) shared.KubernetesAPIData {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ERROR_RESOURCE_NAME_CANNOT_BE_EMPTY), nil, err)
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdDeleteTimeout)
	defer cancel()

	err = resourceClient.Delete(ctx, name, kubeApiMeta.DeleteOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ERROR_DELETE_RESOURCE), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SUCCESS_DELETE_RESOURCE), nil, nil)
}
