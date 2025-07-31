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

func CreateCustomResource(metadata metadata.Metadata, template *unstructured.Unstructured) shared.KubernetesAPIData {
	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CRD_CREATE_TIMEOUT)
	defer cancel()

	resource, err := resourceClient.Create(ctx, template, kubeApiMeta.CreateOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ERROR_CREATE_RESOURCE), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SUCCESS_CREATE_RESOURCE), resource, nil)
}
