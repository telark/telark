package api

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	crdutils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UpdateCustomResource(name string, metadata base.Metadata, template *unstructured.Unstructured) shared.KubernetesAPIData {
	if err := crdutils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ErrResourceNameCannotBeEmpty), nil, err)
	}

	resourceClient, err := crdutils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdPatchTimeout)
	defer cancel()

	resource, err := resourceClient.Update(ctx, template, k8smetav1.UpdateOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrUpdateResource), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessUpdateResource), resource, nil)
}
