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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UpdateCustomResource(name string, metadata metadata.Metadata, template *unstructured.Unstructured) shared.KubernetesAPIData {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ERROR_RESOURCE_NAME_CANNOT_BE_EMPTY), nil, err)
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CRD_PATCH_TIMEOUT)
	defer cancel()

	resource, err := resourceClient.Update(ctx, template, kubeApiMeta.UpdateOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ERROR_UPDATE_RESOURCE), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SUCCESS_UPDATE_RESOURCE), resource, nil)
}
