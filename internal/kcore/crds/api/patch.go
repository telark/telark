package api

import (
	"encoding/json"

	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	metadata "github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	crdUtils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func PatchCustomResource(metadata metadata.Metadata, name string, payload map[string]any) shared.KubernetesAPIData {
	if err := crdUtils.ValidateResourceName(name); err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusBadRequest, string(errors.ErrResourceNameCannotBeEmpty), nil, err)
	}

	resourceClient, err := crdUtils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.CrdPatchTimeout)
	defer cancel()

	patchBytes, err := json.Marshal(payload)
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrRestMarshalPayload), nil, err)
	}

	resource, err := resourceClient.Patch(ctx, name, types.MergePatchType, patchBytes, kubeApiMeta.PatchOptions{})
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ErrUpdateResource), nil, err)
	}

	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessUpdateResource), resource, nil)
}
