package api

import (
	"encoding/json"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/messages"
	metadata "github.com/plsyro/data-pkg/metadata/base"
	"github.com/plsyro/kcore-pkg/constants"
	crdUtils "github.com/plsyro/kcore-pkg/crds/utils"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func PatchCustomResource(metadata metadata.Metadata, name string, patchData map[string]any) shared.KubernetesAPIData {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ERROR_RESOURCE_NAME_CANNOT_BE_EMPTY), nil, err)
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CRD_PATCH_TIMEOUT)
	defer cancel()

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ERROR_REST_MARSHALL_PAYLOAD), nil, err)
	}

	resource, err := resourceClient.Patch(ctx, name, types.MergePatchType, patchBytes, kubeApiMeta.PatchOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ERROR_UPDATE_RESOURCE), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SUCCESS_UPDATE_RESOURCE), resource, nil)
}
