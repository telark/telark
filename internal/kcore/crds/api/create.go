package api

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	metadata "github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	crdUtils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateCustomResource(template *unstructured.Unstructured, metadata metadata.Metadata) shared.KubernetesAPIData {
	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdCreateTimeout)
	defer cancel()

	resource, err := resourceClient.Create(ctx, template, kubeApiMeta.CreateOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrCreateResource), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessCreateResource), resource, nil)
}
