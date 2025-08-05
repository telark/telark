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
)

func DeleteCustomResourceByName(name string, metadata metadata.Metadata) shared.KubernetesAPIData {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ErrResourceNameCannotBeEmpty), nil, err)
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdDeleteTimeout)
	defer cancel()

	err = resourceClient.Delete(ctx, name, kubeApiMeta.DeleteOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrDeleteResource), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessDeleteResource), nil, nil)
}
