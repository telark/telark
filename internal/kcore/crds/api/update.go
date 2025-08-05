package api

import (
	metadata "github.com/plsyro/data/data/base"
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/kcore/constants"
	crdUtils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UpdateCustomResource(name string, metadata metadata.Metadata, template *unstructured.Unstructured) shared.KubernetesAPIData {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ErrResourceNameCannotBeEmpty), nil, err)
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdPatchTimeout)
	defer cancel()

	resource, err := resourceClient.Update(ctx, template, kubeApiMeta.UpdateOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrUpdateResource), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessUpdateResource), resource, nil)
}
